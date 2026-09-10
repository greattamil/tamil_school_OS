import 'package:flutter/foundation.dart';

import '../db/app_database.dart';
import '../db/connection.dart';
import '../db/db_key_store.dart';
import '../db/device_identity.dart';
import 'auth_repository.dart';
import 'token_store.dart';

/// The single source of truth the UI reads from: who is signed in, which
/// school (if any) is active, and -- once a school is active -- the open
/// connection to that school's own encrypted local database.
///
/// Opening/closing the per-school database lives here rather than in
/// AuthRepository because switching schools (PRD 3.2.1: "school selector in
/// the app that exchanges the current token for one scoped to the other
/// school") always means closing one Drift connection and opening another,
/// which is exactly session-lifecycle state, not auth-request logic.
class SessionController extends ChangeNotifier {
  SessionController({
    required this.authRepository,
    required this.tokenStore,
    required this.dbKeyStore,
  });

  final AuthRepository authRepository;
  final TokenStore tokenStore;
  final DbKeyStore dbKeyStore;

  AuthSession? session;
  AppDatabase? database;
  DeviceIdentity? deviceIdentity;
  bool loading = true;

  bool get isSignedIn => session != null;
  bool get hasActiveSchool => session?.activeSchoolId != null;

  /// Called once at app start to rebuild session state from secure storage,
  /// so a restart doesn't force a fresh login every time.
  Future<void> restore() async {
    final accessToken = await tokenStore.accessToken;
    final refreshToken = await tokenStore.refreshToken;
    if (accessToken == null || refreshToken == null) {
      loading = false;
      notifyListeners();
      return;
    }

    final schoolId = await tokenStore.schoolId;
    final schoolName = await tokenStore.schoolName;
    final role = await tokenStore.role;

    session = AuthSession(
      accessToken: accessToken,
      refreshToken: refreshToken,
      schools: const [],
      activeSchoolId: schoolId,
      activeSchoolName: schoolName,
      activeRole: role,
    );

    if (schoolId != null) {
      await _openDatabaseForSchool(schoolId);
    }

    loading = false;
    notifyListeners();
  }

  Future<void> loginStaff(String identifier, String password) async {
    session = await authRepository.staffLogin(identifier, password);
    if (session!.activeSchoolId != null) {
      await _openDatabaseForSchool(session!.activeSchoolId!);
    }
    notifyListeners();
  }

  Future<String?> requestParentOtp(String mobile) =>
      authRepository.requestParentOtp(mobile);

  Future<void> verifyParentOtp(String mobile, String code) async {
    session = await authRepository.verifyParentOtp(mobile, code);
    if (session!.activeSchoolId != null) {
      await _openDatabaseForSchool(session!.activeSchoolId!);
    }
    notifyListeners();
  }

  Future<void> selectSchool(SchoolRole school) async {
    final refreshToken = session!.refreshToken;
    session = await authRepository.selectSchool(refreshToken, school.schoolId);
    await _openDatabaseForSchool(school.schoolId);
    notifyListeners();
  }

  Future<void> _openDatabaseForSchool(String schoolId) async {
    await database?.close();
    final key = await dbKeyStore.getOrCreateKey();
    final connection = await openEncryptedConnection(
      schoolId: schoolId,
      key: key,
    );
    database = AppDatabase(connection);
    deviceIdentity = DeviceIdentity(database!);
  }

  Future<void> logout() async {
    await database?.close();
    await authRepository.logout();
    session = null;
    database = null;
    deviceIdentity = null;
    notifyListeners();
  }

  /// See AuthRepository.handleSessionRevoked -- same wipe, triggered by the
  /// server rather than the user.
  Future<void> handleSessionRevoked() => logout();
}
