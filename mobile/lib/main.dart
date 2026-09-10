import 'dart:io';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:sqlcipher_flutter_libs/sqlcipher_flutter_libs.dart';
import 'package:sqlite3/open.dart';

import 'core/providers.dart';
import 'features/home/home_screen.dart';
import 'features/login/login_screen.dart';
import 'features/parent/parent_dashboard_screen.dart';
import 'features/school_picker/school_picker_screen.dart';

void main() {
  // On Android both the plain sqlite3 assets and the SQLCipher build are
  // present; without this override package:sqlite3 opens the plain one by
  // default, which would silently defeat SQLCipher entirely (PRD 6.4 -- a
  // local roster stored unencrypted is exactly the failure this exists to
  // prevent). No equivalent override is needed on other platforms.
  if (!kIsWeb && Platform.isAndroid) {
    open.overrideFor(OperatingSystem.android, openCipherOnAndroid);
  }

  runApp(const ProviderScope(child: App()));
}

class App extends ConsumerStatefulWidget {
  const App({super.key});

  @override
  ConsumerState<App> createState() => _AppState();
}

class _AppState extends ConsumerState<App> {
  @override
  void initState() {
    super.initState();
    // Restoring from secure storage happens once, at app start, so a restart
    // doesn't force a fresh login every time (PRD 6.2 allows a 30-day
    // refresh lifetime precisely so a shared classroom device isn't forced to
    // re-authenticate constantly).
    Future.microtask(() => ref.read(sessionControllerProvider).restore());
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Tamil School OS',
      theme: ThemeData(colorScheme: ColorScheme.fromSeed(seedColor: Colors.indigo)),
      home: const _RootRouter(),
    );
  }
}

class _RootRouter extends ConsumerWidget {
  const _RootRouter();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final controller = ref.watch(sessionControllerProvider);

    if (controller.loading) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    if (!controller.isSignedIn) {
      return const LoginScreen();
    }
    if (!controller.hasActiveSchool) {
      return const SchoolPickerScreen();
    }
    if (controller.session!.activeRole == 'parent') {
      return const ParentDashboardScreen();
    }
    return const HomeScreen();
  }
}
