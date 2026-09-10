import 'package:flutter_riverpod/flutter_riverpod.dart';
// ChangeNotifierProvider moved out of the main barrel file in Riverpod 3.x.
import 'package:flutter_riverpod/legacy.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

import 'api/api_client.dart';
import 'auth/auth_repository.dart';
import 'auth/session_controller.dart';
import 'auth/token_store.dart';
import 'db/db_key_store.dart';

final secureStorageProvider = Provider((ref) => const FlutterSecureStorage());

final apiClientProvider = Provider((ref) => ApiClient());

final tokenStoreProvider = Provider(
  (ref) => TokenStore(ref.watch(secureStorageProvider)),
);

final dbKeyStoreProvider = Provider(
  (ref) => DbKeyStore(ref.watch(secureStorageProvider)),
);

final authRepositoryProvider = Provider(
  (ref) => AuthRepository(
    ref.watch(apiClientProvider),
    ref.watch(tokenStoreProvider),
    ref.watch(dbKeyStoreProvider),
  ),
);

final sessionControllerProvider = ChangeNotifierProvider(
  (ref) => SessionController(
    authRepository: ref.watch(authRepositoryProvider),
    tokenStore: ref.watch(tokenStoreProvider),
    dbKeyStore: ref.watch(dbKeyStoreProvider),
  ),
);
