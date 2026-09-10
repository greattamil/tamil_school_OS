import 'package:uuid/uuid.dart';

import 'app_database.dart';

/// A stable per-installation device identity and monotonic write counter,
/// backing the two client-side halves of the sync protocol in
/// internal/attendance/conflict.go:
///
/// - `deviceId` is sent as `device_id` on every sync edit.
/// - `nextLocalCounter()` hands out `local_counter` values that only ever
///   increase for the lifetime of this install (PRD 4.2.5 point 2: "increments
///   monotonically per device and never resets"). Persisted in the encrypted
///   local database itself (the `DeviceState` table) rather than in memory,
///   so an app restart mid-session doesn't reissue a counter value the server
///   has already seen -- that would make a legitimate new edit look like a
///   replay and get silently dropped.
class DeviceIdentity {
  DeviceIdentity(this._db);

  final AppDatabase _db;
  static const _deviceIdKey = 'device_id';
  static const _counterKey = 'local_counter';

  String? _cachedDeviceId;

  Future<String> deviceId() async {
    if (_cachedDeviceId != null) return _cachedDeviceId!;

    final row = await (_db.select(
      _db.deviceState,
    )..where((t) => t.key.equals(_deviceIdKey))).getSingleOrNull();
    if (row != null) {
      _cachedDeviceId = row.value;
      return row.value;
    }

    final newId = 'mobile-${const Uuid().v4()}';
    await _db
        .into(_db.deviceState)
        .insertOnConflictUpdate(
          DeviceStateCompanion.insert(key: _deviceIdKey, value: newId),
        );
    _cachedDeviceId = newId;
    return newId;
  }

  /// Atomically reads, increments and persists the counter, returning the new
  /// value. Must be called once per outbox write -- never reuse a value, and
  /// never compute it from anything but the last persisted value.
  Future<int> nextLocalCounter() async {
    return _db.transaction(() async {
      final row = await (_db.select(
        _db.deviceState,
      )..where((t) => t.key.equals(_counterKey))).getSingleOrNull();
      final current = row == null ? 0 : int.parse(row.value);
      final next = current + 1;
      await _db
          .into(_db.deviceState)
          .insertOnConflictUpdate(
            DeviceStateCompanion.insert(
              key: _counterKey,
              value: next.toString(),
            ),
          );
      return next;
    });
  }
}
