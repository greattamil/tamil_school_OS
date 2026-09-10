import 'dart:io';

import 'package:drift/drift.dart';
import 'package:drift/native.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';
import 'package:sqlcipher_flutter_libs/sqlcipher_flutter_libs.dart';
import 'package:sqlite3/open.dart';

/// Opens the encrypted local database for one school.
///
/// One database file per school (PRD 3.2.1): "Local database isolation is per
/// tenant... Switching schools closes the Drift connection for one and opens
/// the other." This is what makes revocation correct and simple -- when a
/// user's role at one school is revoked, the wipe deletes exactly
/// `app_db_<school_id>.sqlite` and nothing else; their access at any other
/// school is untouched.
///
/// [key] must come from [DbKeyStore] -- a random, per-installation secret,
/// never anything derived from the school ID, user ID, or device identifier
/// (PRD 6.4).
Future<QueryExecutor> openEncryptedConnection({
  required String schoolId,
  required String key,
}) async {
  final file = await _dbFileForSchool(schoolId);

  return NativeDatabase.createInBackground(
    file,
    // NativeDatabase.createInBackground spawns a SEPARATE isolate to run all
    // database operations, and Dart isolates do not share the process-wide
    // sqlite3 "open" override installed in main() -- each isolate starts
    // fresh. Without re-applying the override here, this isolate silently
    // opens the plain (non-SQLCipher) sqlite3 library that also ships on
    // Android, and every query fails the PRAGMA cipher_version check below,
    // which looks identical to "the database is unreachable" from the
    // caller's point of view. This is exactly the gap the sqlcipher_flutter_
    // libs README warns about: "When using package:sqlite3 in a background
    // isolate (even if just indirectly through say package:drift),
    // overrideFor should also be called on that isolate."
    isolateSetup: () async {
      if (!kIsWeb && Platform.isAndroid) {
        open.overrideFor(OperatingSystem.android, openCipherOnAndroid);
      }
    },
    setup: (rawDb) {
      // PRAGMA key against a plain (non-SQLCipher) sqlite3 build fails
      // silently rather than raising an error, which would otherwise mean
      // "the app just quietly uses plaintext storage" -- exactly the failure
      // mode PRD 6.4 exists to prevent. Verify SQLCipher is actually the
      // library in use before trusting the key pragma at all.
      final cipherVersion = rawDb.select('PRAGMA cipher_version;');
      if (cipherVersion.isEmpty) {
        throw StateError(
          'SQLCipher is not available in this build -- refusing to open the '
          'local database unencrypted rather than silently storing student '
          'and attendance data in plaintext.',
        );
      }
      rawDb.execute("PRAGMA key = '$key';");
    },
  );
}

Future<File> _dbFileForSchool(String schoolId) async {
  final dir = await getApplicationDocumentsDirectory();
  return File(p.join(dir.path, 'app_db_$schoolId.sqlite'));
}

/// Deletes the local database file for a school, e.g. on logout, on
/// SESSION_REVOKED, or on a key-unavailable failure that forces a fresh
/// resync (PRD 6.4: "if the key cannot be retrieved for any reason, discard
/// the local database, re-authenticate, and resync from the server rather
/// than crashing"). The caller must also purge the key itself via
/// [DbKeyStore.purge] -- deleting the file without the key (or vice versa)
/// leaves the two out of sync.
Future<void> deleteLocalDatabase(String schoolId) async {
  final file = await _dbFileForSchool(schoolId);
  if (await file.exists()) {
    await file.delete();
  }
}
