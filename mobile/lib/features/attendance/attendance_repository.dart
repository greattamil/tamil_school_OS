import 'package:drift/drift.dart';

import '../../core/api/api_client.dart';
import '../../core/db/app_database.dart';
import '../../core/db/device_identity.dart';

/// One entry the sync response reports as overridden (superseded or a
/// same-device replay), for the UI to surface to the teacher per PRD 4.2.5
/// point 6: "the teacher is told... which students were superseded, by whom,
/// and what the current value is." The API doesn't return "by whom" today
/// (see internal/attendance/models.go EntryResult) -- shown here is what the
/// current server value actually is, which is the part that matters for the
/// teacher to trust the screen again.
class OverriddenEntry {
  OverriddenEntry({
    required this.enrollmentId,
    required this.studentName,
    required this.currentStatus,
    required this.message,
  });

  final String enrollmentId;
  final String studentName;
  final String currentStatus;
  final String message;
}

class SyncFlushResult {
  SyncFlushResult({required this.appliedCount, required this.overridden});

  final int appliedCount;
  final List<OverriddenEntry> overridden;
}

/// Implements the mobile half of the offline-first attendance design
/// (PRD 4.2.1, 4.2.5, 8.6): every write lands in the local database first and
/// confirms to the UI immediately, an outbox queues what hasn't been
/// confirmed by the server yet, and flushing the outbox resolves each
/// entry's outcome independently against internal/attendance/conflict.go's
/// server-side algorithm.
class AttendanceRepository {
  AttendanceRepository({
    required AppDatabase database,
    required ApiClient api,
    required DeviceIdentity deviceIdentity,
    required String Function() accessToken,
  }) : _db = database,
       _api = api,
       _device = deviceIdentity,
       _accessToken = accessToken;

  final AppDatabase _db;
  final ApiClient _api;
  final DeviceIdentity _device;
  final String Function() _accessToken;

  /// Reactive roster for a section+date, read entirely from the local
  /// database (PRD 8.6: reads never conditionally hit the network). Call
  /// [refreshFromServer] separately to pull the latest server state into this
  /// same local table; the UI updates automatically via this stream once that
  /// completes.
  Stream<List<LocalAttendanceEntry>> watchRoster(
    String sectionId,
    DateTime date,
  ) {
    final query = _db.select(_db.localAttendanceEntries)
      ..where((t) => t.sectionId.equals(sectionId) & t.date.equals(date))
      ..orderBy([(t) => OrderingTerm(expression: t.studentName)]);
    return query.watch();
  }

  Stream<int> watchPendingCount(String sectionId, DateTime date) {
    final query = _db.selectOnly(_db.outboxEntries)
      ..addColumns([_db.outboxEntries.id.count()])
      ..where(
        _db.outboxEntries.sectionId.equals(sectionId) &
            _db.outboxEntries.date.equals(date),
      );
    return query
        .map((row) => row.read(_db.outboxEntries.id.count()) ?? 0)
        .watchSingle();
  }

  /// The oldest still-queued edit's timestamp for this register, or null if
  /// nothing is pending -- PRD 4.2.5: "Pending registers older than 24 hours
  /// raise an in-app warning to the teacher." Scoped to the register the
  /// teacher actually has open, since that's the one they can act on (retry
  /// sync, or flag it to the office); a cross-section version needs the
  /// "my sections" resolution PROGRESS.md notes isn't built yet.
  Stream<DateTime?> watchOldestPendingTimestamp(
    String sectionId,
    DateTime date,
  ) {
    final query = _db.selectOnly(_db.outboxEntries)
      ..addColumns([_db.outboxEntries.clientTimestamp.min()])
      ..where(
        _db.outboxEntries.sectionId.equals(sectionId) &
            _db.outboxEntries.date.equals(date),
      );
    return query
        .map((row) => row.read(_db.outboxEntries.clientTimestamp.min()))
        .watchSingle();
  }

  /// Pulls the section's roster and today's marks from the server and merges
  /// them into the local cache. Entries with an unsynced local edit are left
  /// alone -- a server refresh must never clobber a write the outbox hasn't
  /// flushed yet, or a teacher marking attendance offline and then briefly
  /// regaining signal (triggering a refresh before their own sync runs)
  /// would see their own tap silently reverted.
  Future<void> refreshFromServer(String sectionId, DateTime date) async {
    final path =
        '/api/v1/sections/$sectionId/attendance?date=${_formatDate(date)}';
    final body = await _api.getJson(path, accessToken: _accessToken());
    final entries = (body['entries'] as List<dynamic>)
        .cast<Map<String, dynamic>>();

    final pendingIds = await _pendingEnrollmentIds(sectionId, date);

    await _db.batch((batch) {
      for (final e in entries) {
        final enrollmentId = e['enrollment_id'] as String;
        if (pendingIds.contains(enrollmentId)) continue;

        batch.insert(
          _db.localAttendanceEntries,
          LocalAttendanceEntriesCompanion.insert(
            enrollmentId: enrollmentId,
            date: date,
            sectionId: sectionId,
            studentName: e['student_name'] as String,
            rollNumber: Value(e['roll_number'] as String?),
            status: Value(e['status'] as String?),
            reason: Value(e['reason'] as String?),
            serverRevision: Value((e['server_revision'] as num?)?.toInt() ?? 0),
            syncStatus: const Value('synced'),
          ),
          mode: InsertMode.insertOrReplace,
        );
      }
    });
  }

  /// The local-first write (PRD 4.2.1: "Writes to local SQLite first and
  /// confirms immediately to the user"). Returns as soon as the local
  /// transaction commits -- the caller does not wait for any network call.
  Future<void> markAttendance({
    required String enrollmentId,
    required String sectionId,
    required DateTime date,
    required String status,
    String? reason,
  }) async {
    final localCounter = await _device.nextLocalCounter();
    final now = DateTime.now().toUtc();

    await _db.transaction(() async {
      final existing =
          await (_db.select(_db.localAttendanceEntries)..where(
                (t) =>
                    t.enrollmentId.equals(enrollmentId) & t.date.equals(date),
              ))
              .getSingleOrNull();
      final baseRevision = existing?.serverRevision ?? 0;

      await _db
          .into(_db.localAttendanceEntries)
          .insertOnConflictUpdate(
            LocalAttendanceEntriesCompanion(
              enrollmentId: Value(enrollmentId),
              date: Value(date),
              sectionId: Value(sectionId),
              studentName: Value(existing?.studentName ?? ''),
              rollNumber: Value(existing?.rollNumber),
              status: Value(status),
              reason: Value(reason),
              serverRevision: Value(baseRevision),
              syncStatus: const Value('pending'),
            ),
          );

      await _db
          .into(_db.outboxEntries)
          .insert(
            OutboxEntriesCompanion.insert(
              enrollmentId: enrollmentId,
              date: date,
              sectionId: sectionId,
              status: status,
              reason: Value(reason),
              localCounter: localCounter,
              clientTimestamp: now,
              baseRevision: baseRevision,
            ),
          );
    });
  }

  /// The batch counterpart to [markAttendance] matching PRD 4.2.1's actual
  /// flow: the grid defaults every student to present, the teacher taps only
  /// the exceptions, and "a single confirm action saves the whole register"
  /// -- one write covering every student, not one network call per tap.
  /// [marks] must include every student the teacher is confirming, present
  /// and absent alike.
  Future<void> confirmRegister({
    required String sectionId,
    required DateTime date,
    required Map<String, ({String status, String? reason})> marks,
  }) async {
    final now = DateTime.now().toUtc();

    await _db.transaction(() async {
      for (final entry in marks.entries) {
        final enrollmentId = entry.key;
        final mark = entry.value;
        final localCounter = await _device.nextLocalCounter();

        final existing =
            await (_db.select(_db.localAttendanceEntries)..where(
                  (t) =>
                      t.enrollmentId.equals(enrollmentId) & t.date.equals(date),
                ))
                .getSingleOrNull();
        final baseRevision = existing?.serverRevision ?? 0;

        await _db
            .into(_db.localAttendanceEntries)
            .insertOnConflictUpdate(
              LocalAttendanceEntriesCompanion(
                enrollmentId: Value(enrollmentId),
                date: Value(date),
                sectionId: Value(sectionId),
                studentName: Value(existing?.studentName ?? ''),
                rollNumber: Value(existing?.rollNumber),
                status: Value(mark.status),
                reason: Value(mark.reason),
                serverRevision: Value(baseRevision),
                syncStatus: const Value('pending'),
              ),
            );

        await _db
            .into(_db.outboxEntries)
            .insert(
              OutboxEntriesCompanion.insert(
                enrollmentId: enrollmentId,
                date: date,
                sectionId: sectionId,
                status: mark.status,
                reason: Value(mark.reason),
                localCounter: localCounter,
                clientTimestamp: now,
                baseRevision: baseRevision,
              ),
            );
      }
    });
  }

  Future<Set<String>> _pendingEnrollmentIds(
    String sectionId,
    DateTime date,
  ) async {
    final rows = await (_db.select(
      _db.outboxEntries,
    )..where((t) => t.sectionId.equals(sectionId) & t.date.equals(date))).get();
    return rows.map((r) => r.enrollmentId).toSet();
  }

  /// Flushes every queued edit for one section+date to the server. Safe to
  /// call from a foreground trigger, a connectivity-regained trigger, or a
  /// manual "sync now" tap (PRD 4.2.5: "Foreground and connectivity events
  /// are the primary sync triggers, executed immediately and synchronously").
  /// If a student has more than one queued edit (the teacher tapped twice
  /// before a flush ran), only the latest is sent -- the earlier ones are
  /// obsolete the moment a newer local edit exists, and all of them are
  /// cleared together once the sent edit's outcome is known.
  Future<SyncFlushResult> flushOutbox(String sectionId, DateTime date) async {
    final deviceId = await _device.deviceId();
    final allPending = await (_db.select(
      _db.outboxEntries,
    )..where((t) => t.sectionId.equals(sectionId) & t.date.equals(date))).get();
    if (allPending.isEmpty) {
      return SyncFlushResult(appliedCount: 0, overridden: []);
    }

    final latestByEnrollment = <String, OutboxEntry>{};
    final allIdsByEnrollment = <String, List<int>>{};
    for (final row in allPending) {
      allIdsByEnrollment.putIfAbsent(row.enrollmentId, () => []).add(row.id);
      final current = latestByEnrollment[row.enrollmentId];
      if (current == null || row.localCounter > current.localCounter) {
        latestByEnrollment[row.enrollmentId] = row;
      }
    }

    final edits = latestByEnrollment.values
        .map(
          (row) => {
            'enrollment_id': row.enrollmentId,
            'status': row.status,
            if (row.reason != null) 'reason': row.reason,
            'device_id': deviceId,
            'local_counter': row.localCounter,
            // Drift's SQLite round-trip loses the UTC flag on DateTime columns
            // (the value read back is always isUtc: false, even though it was
            // written from a .toUtc() DateTime) -- so toIso8601String() here
            // would omit the 'Z' suffix and Go's RFC3339 time.Parse on the
            // backend rejects it outright. Force UTC again before formatting.
            'client_timestamp': row.clientTimestamp.toUtc().toIso8601String(),
            'base_revision': row.baseRevision,
          },
        )
        .toList();

    final body = await _api.postJson(
      '/api/v1/sections/$sectionId/attendance/sync',
      {'date': _formatDate(date), 'edits': edits},
      accessToken: _accessToken(),
    );
    final results = (body['results'] as List<dynamic>)
        .cast<Map<String, dynamic>>();

    var applied = 0;
    final overridden = <OverriddenEntry>[];

    await _db.transaction(() async {
      for (final result in results) {
        final enrollmentId = result['enrollment_id'] as String;
        final outcome = result['outcome'] as String;
        final serverRevision = (result['server_revision'] as num).toInt();
        final currentStatus = result['current_status'] as String;
        final currentReason = result['current_reason'] as String?;

        await (_db.update(_db.localAttendanceEntries)..where(
              (t) => t.enrollmentId.equals(enrollmentId) & t.date.equals(date),
            ))
            .write(
              LocalAttendanceEntriesCompanion(
                status: Value(currentStatus),
                reason: Value(currentReason),
                serverRevision: Value(serverRevision),
                syncStatus: const Value('synced'),
              ),
            );

        final idsToDelete = allIdsByEnrollment[enrollmentId] ?? [];
        if (idsToDelete.isNotEmpty) {
          await (_db.delete(
            _db.outboxEntries,
          )..where((t) => t.id.isIn(idsToDelete))).go();
        }

        if (outcome == 'applied') {
          applied++;
        } else {
          final studentRow =
              await (_db.select(_db.localAttendanceEntries)..where(
                    (t) =>
                        t.enrollmentId.equals(enrollmentId) &
                        t.date.equals(date),
                  ))
                  .getSingleOrNull();
          overridden.add(
            OverriddenEntry(
              enrollmentId: enrollmentId,
              studentName: studentRow?.studentName ?? enrollmentId,
              currentStatus: currentStatus,
              message:
                  (result['message'] as String?) ?? 'Edit was not applied.',
            ),
          );
        }
      }
    });

    return SyncFlushResult(appliedCount: applied, overridden: overridden);
  }

  String _formatDate(DateTime d) =>
      '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';
}
