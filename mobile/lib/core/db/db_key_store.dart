import 'dart:convert';
import 'dart:math';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Manages the SQLCipher encryption key per PRD 6.4: "The key is the whole
/// control, and a key in the APK is no key at all." Requirements implemented
/// here:
///
/// - Generated with a cryptographically secure random source on first
///   launch, unique per installation -- never derived from a device
///   identifier, user ID, or anything else predictable.
/// - Stored in flutter_secure_storage, which on Android is backed by the
///   Keystore (hardware-backed where the device supports it) and never
///   touches application-readable disk in plaintext.
/// - Deliberately NOT bound to biometric authentication
///   (`setInvalidatedByBiometricEnrollment` equivalents): a teacher enrolling
///   a new fingerprint must never silently lose their local database. This
///   class does not opt into any biometric-gated storage option.
/// - Purged on logout and on remote-wipe (see [purge]); the caller is
///   responsible for also deleting the encrypted database file itself at the
///   same time (this class only owns the key).
class DbKeyStore {
  DbKeyStore(this._storage);

  final FlutterSecureStorage _storage;
  static const _keyName = 'sqlcipher_db_key_v1';

  /// Returns the existing key, or generates and persists a new one if this is
  /// the first launch. 256 bits of secure randomness, hex-encoded for use as
  /// a SQLCipher PRAGMA key.
  Future<String> getOrCreateKey() async {
    final existing = await _storage.read(key: _keyName);
    if (existing != null && existing.isNotEmpty) {
      return existing;
    }

    final random = Random.secure();
    final bytes = List<int>.generate(32, (_) => random.nextInt(256));
    final key = base64Url.encode(bytes);

    await _storage.write(key: _keyName, value: key);
    return key;
  }

  /// Called on logout, on SESSION_REVOKED, and whenever the key cannot be
  /// retrieved for any reason (PRD 6.4: "Handle key-unavailable errors
  /// gracefully regardless: if the key cannot be retrieved for any reason,
  /// discard the local database, re-authenticate, and resync from the server
  /// rather than crashing"). The database file itself must be deleted by the
  /// caller alongside this -- a key purge with no file purge is pointless,
  /// and a file purge with no key purge leaves an orphaned key in the
  /// Keystore.
  Future<void> purge() async {
    await _storage.delete(key: _keyName);
  }
}
