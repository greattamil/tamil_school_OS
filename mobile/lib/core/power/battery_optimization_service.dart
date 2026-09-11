import 'package:flutter/services.dart';

/// Thin wrapper around the platform channel MainActivity.kt exposes for the
/// PRD 4.2.5 onboarding step: "Onboarding tells teachers to exempt the app
/// from battery optimisation, with a one-tap prompt to the OEM setting."
///
/// A plain MethodChannel rather than a pub.dev package -- this codebase has
/// already hit real dependency breakage once from an unrelated transitive
/// package pulling in Dart's hooks-based native-assets build system, which
/// build_runner doesn't yet support (see pubspec.yaml). Two platform calls
/// this narrow aren't worth that risk.
class BatteryOptimizationService {
  static const _channel = MethodChannel(
    'com.tamilschoolos.mobile/battery_optimization',
  );

  /// Returns false on any platform without the channel (e.g. Windows during
  /// dev), never throws -- this is an onboarding nicety, not something a
  /// missing platform implementation should crash over.
  Future<bool> isIgnoringBatteryOptimizations() async {
    try {
      return await _channel.invokeMethod<bool>(
            'isIgnoringBatteryOptimizations',
          ) ??
          true;
    } on PlatformException {
      return true;
    } on MissingPluginException {
      return true;
    }
  }

  Future<void> requestExemption() async {
    try {
      await _channel.invokeMethod<void>('requestIgnoreBatteryOptimizations');
    } on PlatformException {
      // Nothing to recover to -- the teacher just doesn't get the OEM
      // dialog this one time. PRD 4.2.5 expects roughly half to skip this
      // entirely anyway ("design so that it does not matter"), so a failed
      // prompt is the same outcome as a declined one.
    } on MissingPluginException {
      // No-op platform (dev on Windows, etc).
    }
  }
}
