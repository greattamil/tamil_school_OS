import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Session tokens live in platform secure storage, never SharedPreferences
/// (PRD 6.4). On Android this is backed by the Keystore, same as the
/// SQLCipher key in [DbKeyStore] -- a separate key name, same underlying
/// protection.
class TokenStore {
  TokenStore(this._storage);

  final FlutterSecureStorage _storage;

  static const _accessTokenKey = 'access_token';
  static const _refreshTokenKey = 'refresh_token';
  static const _schoolIdKey = 'active_school_id';
  static const _schoolNameKey = 'active_school_name';
  static const _roleKey = 'active_role';

  Future<void> save({
    required String accessToken,
    required String refreshToken,
    String? schoolId,
    String? schoolName,
    String? role,
  }) async {
    await _storage.write(key: _accessTokenKey, value: accessToken);
    await _storage.write(key: _refreshTokenKey, value: refreshToken);
    if (schoolId != null) await _storage.write(key: _schoolIdKey, value: schoolId);
    if (schoolName != null) await _storage.write(key: _schoolNameKey, value: schoolName);
    if (role != null) await _storage.write(key: _roleKey, value: role);
  }

  Future<String?> get accessToken => _storage.read(key: _accessTokenKey);
  Future<String?> get refreshToken => _storage.read(key: _refreshTokenKey);
  Future<String?> get schoolId => _storage.read(key: _schoolIdKey);
  Future<String?> get schoolName => _storage.read(key: _schoolNameKey);
  Future<String?> get role => _storage.read(key: _roleKey);

  /// Clears every stored token. Called on logout and on SESSION_REVOKED (PRD
  /// 6.2: "the mobile client treats revocation as a wipe signal, not a
  /// logout"). The caller is responsible for also purging the SQLCipher key
  /// and deleting the local database file -- this only owns the tokens.
  Future<void> clear() async {
    await _storage.delete(key: _accessTokenKey);
    await _storage.delete(key: _refreshTokenKey);
    await _storage.delete(key: _schoolIdKey);
    await _storage.delete(key: _schoolNameKey);
    await _storage.delete(key: _roleKey);
  }
}
