import 'package:drift/drift.dart';

part 'app_database.g.dart';

/// Local, offline-first mirror of the current attendance state. This is the
/// read source of truth for the marking screen (PRD 8.6: "Local database is
/// the read source of truth; reads never conditionally hit the network"): the
/// UI reads from here, never blocks on a network round trip.
///
/// serverRevision is the last revision this device has seen for the entry
/// (0 if never synced), and is exactly what gets echoed back as base_revision
/// on the next sync request -- the client half of the conflict-resolution
/// protocol implemented server-side in internal/attendance/conflict.go.
class LocalAttendanceEntries extends Table {
  TextColumn get enrollmentId => text()();
  DateTimeColumn get date => dateTime()();
  TextColumn get sectionId => text()();
  TextColumn get studentName => text()();
  TextColumn get rollNumber => text().nullable()();
  // 'present' / 'absent' / null (not yet marked).
  TextColumn get status => text().nullable()();
  TextColumn get reason => text().nullable()();
  IntColumn get serverRevision => integer().withDefault(const Constant(0))();
  // 'synced' | 'pending' | 'conflict' -- what the teacher sees per PRD 4.2.1
  // ("Visible sync status per register: pending, synced, conflict").
  TextColumn get syncStatus => text().withDefault(const Constant('synced'))();

  @override
  Set<Column> get primaryKey => {enrollmentId, date};
}

/// The offline write queue (PRD 8.6: "Writes go to local storage first, then
/// to an outbox queue with operation IDs and timestamps"). A row here means
/// "this edit has not yet been confirmed applied by the server." Rows are
/// deleted once the sync response for them has been processed, whatever the
/// outcome (applied, superseded, invalid) -- the outbox tracks what's still
/// in flight, not history.
class OutboxEntries extends Table {
  IntColumn get id => integer().autoIncrement()();
  TextColumn get enrollmentId => text()();
  DateTimeColumn get date => dateTime()();
  TextColumn get sectionId => text()();
  TextColumn get status => text()();
  TextColumn get reason => text().nullable()();
  // This device's monotonic counter value for this write (PRD 4.2.5 point 2:
  // "increments monotonically per device and never resets"). Captured from
  // DeviceState at write time, never recomputed later.
  IntColumn get localCounter => integer()();
  DateTimeColumn get clientTimestamp => dateTime()();
  // The server_revision this edit was based on, captured from
  // LocalAttendanceEntries at the moment the edit was made locally.
  IntColumn get baseRevision => integer()();
  DateTimeColumn get createdAt => dateTime().withDefault(currentDateAndTime)();
}

/// Small persistent key-value store for device identity: a random device_id
/// generated once per install, and the monotonic local_counter that must
/// never reset for as long as the install lives (PRD 4.2.5 point 2).
class DeviceState extends Table {
  TextColumn get key => text()();
  TextColumn get value => text()();

  @override
  Set<Column> get primaryKey => {key};
}

@DriftDatabase(tables: [LocalAttendanceEntries, OutboxEntries, DeviceState])
class AppDatabase extends _$AppDatabase {
  AppDatabase(super.e);

  @override
  int get schemaVersion => 1;
}
