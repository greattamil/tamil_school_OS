// Tests the local-only half of the offline-first attendance design (PRD
// 4.2.1, 8.6) against an in-memory database -- no SQLCipher, no network, no
// platform channels needed, since markAttendance/confirmRegister/watchRoster
// only ever touch the local Drift database. The server-side conflict
// resolution these writes feed into is tested separately and exhaustively in
// backend/internal/attendance/conflict_test.go; what matters here is that the
// client faithfully produces the inputs that algorithm depends on --
// especially base_revision, which must be captured from the entry's state at
// write time, not recomputed later.

import 'package:drift/drift.dart';
import 'package:drift/native.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mobile/core/api/api_client.dart';
import 'package:mobile/core/db/app_database.dart';
import 'package:mobile/core/db/device_identity.dart';
import 'package:mobile/features/attendance/attendance_repository.dart';

void main() {
  late AppDatabase db;
  late AttendanceRepository repo;

  setUp(() {
    db = AppDatabase(NativeDatabase.memory());
    repo = AttendanceRepository(
      database: db,
      api: ApiClient(),
      deviceIdentity: DeviceIdentity(db),
      accessToken: () => 'test-token',
    );
  });

  tearDown(() async {
    await db.close();
  });

  const sectionId = 'section-1';
  const enrollmentId = 'enrollment-1';
  final date = DateTime(2026, 6, 15);

  test('markAttendance writes locally as pending with base_revision 0 for a new entry', () async {
    await repo.markAttendance(
      enrollmentId: enrollmentId,
      sectionId: sectionId,
      date: date,
      status: 'absent',
      reason: 'sick',
    );

    final local = await (db.select(
      db.localAttendanceEntries,
    )..where((t) => t.enrollmentId.equals(enrollmentId) & t.date.equals(date))).getSingle();
    expect(local.status, 'absent');
    expect(local.reason, 'sick');
    expect(local.syncStatus, 'pending');

    final outbox = await db.select(db.outboxEntries).get();
    expect(outbox, hasLength(1));
    expect(outbox.single.baseRevision, 0);
    expect(outbox.single.localCounter, 1);
  });

  test('a second edit captures the current local server_revision as its base, not 0', () async {
    // Simulate a prior successful sync: the entry is already at revision 3,
    // authored by someone else, with no outbox row left (as flushOutbox
    // leaves things once applied).
    await db
        .into(db.localAttendanceEntries)
        .insert(
          LocalAttendanceEntriesCompanion.insert(
            enrollmentId: enrollmentId,
            date: date,
            sectionId: sectionId,
            studentName: 'Test Student',
            status: const Value('present'),
            serverRevision: const Value(3),
            syncStatus: const Value('synced'),
          ),
        );

    await repo.markAttendance(
      enrollmentId: enrollmentId,
      sectionId: sectionId,
      date: date,
      status: 'absent',
    );

    final outbox = await db.select(db.outboxEntries).get();
    expect(outbox, hasLength(1));
    expect(
      outbox.single.baseRevision,
      3,
      reason: 'the edit must declare the revision it was actually based on, '
          'or the server has no way to detect a stale write',
    );
  });

  test('local_counter is strictly increasing across writes and never resets', () async {
    await repo.markAttendance(
      enrollmentId: enrollmentId,
      sectionId: sectionId,
      date: date,
      status: 'absent',
    );
    await repo.markAttendance(
      enrollmentId: enrollmentId,
      sectionId: sectionId,
      date: date,
      status: 'present',
    );

    final outbox = await (db.select(db.outboxEntries)
          ..orderBy([(t) => OrderingTerm(expression: t.id)]))
        .get();
    expect(outbox, hasLength(2));
    expect(outbox[1].localCounter, greaterThan(outbox[0].localCounter));
  });

  test('confirmRegister writes every student in the batch, defaulting absentees only where marked', () async {
    await repo.confirmRegister(
      sectionId: sectionId,
      date: date,
      marks: {
        'enrollment-1': (status: 'present', reason: null),
        'enrollment-2': (status: 'absent', reason: 'sick'),
        'enrollment-3': (status: 'present', reason: null),
      },
    );

    final local = await db.select(db.localAttendanceEntries).get();
    expect(local, hasLength(3));
    expect(
      local.where((e) => e.status == 'absent').map((e) => e.enrollmentId),
      ['enrollment-2'],
    );

    final outbox = await db.select(db.outboxEntries).get();
    expect(outbox, hasLength(3), reason: 'confirming the register queues one edit per student');
  });

  test('watchRoster reflects writes reactively', () async {
    final stream = repo.watchRoster(sectionId, date);
    final firstEmpty = await stream.first;
    expect(firstEmpty, isEmpty);

    await repo.markAttendance(
      enrollmentId: enrollmentId,
      sectionId: sectionId,
      date: date,
      status: 'present',
    );

    await expectLater(
      repo.watchRoster(sectionId, date).map((rows) => rows.length),
      emits(1),
    );
  });
}
