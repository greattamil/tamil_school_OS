// Smoke test: the app builds and renders without crashing. The real coverage
// for this app's safety-critical piece (offline-first attendance) lives in
// attendance_repository_test.dart.
//
// This deliberately doesn't wait for SessionController.restore() to finish
// (no pumpAndSettle): that call goes through flutter_secure_storage's
// platform channel, which has no mock implementation registered in a plain
// widget test and never resolves here. Asserting the loading state that's
// visible immediately after the first frame is enough to prove the widget
// tree wires together; exercising what happens after a session is restored
// belongs in an integration test against a real platform, not a widget test.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:mobile/main.dart';

void main() {
  testWidgets('renders a loading state on first frame', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(const ProviderScope(child: App()));
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });
}
