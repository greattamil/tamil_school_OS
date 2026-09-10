import '../api/api_client.dart';
import '../db/db_key_store.dart';
import '../db/connection.dart' as db_connection;
import 'token_store.dart';

class SchoolRole {
  SchoolRole({
    required this.schoolId,
    required this.schoolName,
    required this.role,
  });

  factory SchoolRole.fromJson(Map<String, dynamic> json) => SchoolRole(
    schoolId: json['school_id'] as String,
    schoolName: json['school_name'] as String,
    role: json['role'] as String,
  );

  final String schoolId;
  final String schoolName;
  final String role;
}

class AuthSession {
  AuthSession({
    required this.accessToken,
    required this.refreshToken,
    required this.schools,
    this.activeSchoolId,
    this.activeSchoolName,
    this.activeRole,
  });

  final String accessToken;
  final String refreshToken;
  final List<SchoolRole> schools;
  final String? activeSchoolId;
  final String? activeSchoolName;
  final String? activeRole;

  bool get needsSchoolSelection => activeSchoolId == null && schools.length > 1;
}

/// Owns login, OTP verification, school selection, logout, and the
/// SESSION_REVOKED wipe path. Deliberately does not own the local encrypted
/// database beyond wiping it -- see AttendanceRepository for the day-to-day
/// per-school connection lifecycle.
class AuthRepository {
  AuthRepository(this._api, this._tokenStore, this._dbKeyStore, {String? deviceId})
    : _deviceId = deviceId ?? 'mobile-unknown';

  final ApiClient _api;
  final TokenStore _tokenStore;
  final DbKeyStore _dbKeyStore;
  final String _deviceId;

  Future<AuthSession> staffLogin(String identifier, String password) async {
    final body = await _api.postJson('/api/v1/auth/staff/login', {
      'identifier': identifier,
      'password': password,
      'device_id': _deviceId,
    });
    return _handleTokenResponse(body);
  }

  Future<String?> requestParentOtp(String mobile) async {
    final body = await _api.postJson('/api/v1/auth/parent/otp/request', {
      'mobile': mobile,
    });
    // Present only in local/dev environments; the real SMS gateway
    // integration (PRD 9 Phase 1 external process, PRD 9 Phase 5
    // implementation) never returns the code in the response body.
    return body['dev_only_code'] as String?;
  }

  Future<AuthSession> verifyParentOtp(String mobile, String code) async {
    final body = await _api.postJson('/api/v1/auth/parent/otp/verify', {
      'mobile': mobile,
      'code': code,
      'device_id': _deviceId,
    });
    return _handleTokenResponse(body);
  }

  Future<AuthSession> selectSchool(String refreshToken, String schoolId) async {
    final body = await _api.postJson('/api/v1/auth/select-school', {
      'refresh_token': refreshToken,
      'school_id': schoolId,
      'device_id': _deviceId,
    });
    return _handleTokenResponse(body);
  }

  Future<AuthSession> _handleTokenResponse(Map<String, dynamic> body) async {
    final schools = (body['schools'] as List<dynamic>)
        .map((s) => SchoolRole.fromJson(s as Map<String, dynamic>))
        .toList();
    SchoolRole? active;
    if (schools.length == 1) active = schools.first;

    final session = AuthSession(
      accessToken: body['access_token'] as String,
      refreshToken: body['refresh_token'] as String,
      schools: schools,
      activeSchoolId: active?.schoolId,
      activeSchoolName: active?.schoolName,
      activeRole: active?.role,
    );

    await _tokenStore.save(
      accessToken: session.accessToken,
      refreshToken: session.refreshToken,
      schoolId: session.activeSchoolId,
      schoolName: session.activeSchoolName,
      role: session.activeRole,
    );
    return session;
  }

  /// Plain logout: clear tokens and this school's local database and key.
  /// The difference from [handleSessionRevoked] is only in when it's called
  /// (user-initiated vs. server-forced) -- the cleanup is identical, which is
  /// deliberate: PRD 6.2 says revocation should be treated as a wipe signal,
  /// not a special case of logout, so logout is implemented as that same wipe.
  Future<void> logout() async {
    final schoolId = await _tokenStore.schoolId;
    await _tokenStore.clear();
    await _dbKeyStore.purge();
    if (schoolId != null) {
      await db_connection.deleteLocalDatabase(schoolId);
    }
  }

  /// Called when any API response comes back 401 with error "SESSION_REVOKED"
  /// (PRD 6.2: "The mobile client treats revocation as a wipe signal, not a
  /// logout. On receiving 401 with a SESSION_REVOKED reason, the app
  /// immediately purges the SQLCipher key from the Keystore, deletes the
  /// local encrypted database and any cached files, and returns to the login
  /// screen. It does not wait for the user to act.").
  Future<void> handleSessionRevoked() => logout();
}
