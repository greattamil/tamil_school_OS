package com.tamilschoolos.mobile

import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.PowerManager
import android.provider.Settings
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel

// A plain MethodChannel rather than a pub.dev battery-optimization package
// (PRD 4.2.5: "Onboarding tells teachers to exempt the app from battery
// optimisation, with a one-tap prompt to the OEM setting") -- this is two
// small platform calls, and this codebase has already hit real dependency
// breakage once from an unrelated transitive package pulling in the
// Dart-hooks native-assets build system build_runner doesn't yet support
// (see pubspec.yaml's path_provider_foundation override). Not worth the risk
// for something this narrow.
class MainActivity : FlutterActivity() {
    private val channel = "com.tamilschoolos.mobile/battery_optimization"

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)
        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, channel).setMethodCallHandler { call, result ->
            when (call.method) {
                "isIgnoringBatteryOptimizations" -> {
                    val pm = getSystemService(POWER_SERVICE) as PowerManager
                    result.success(pm.isIgnoringBatteryOptimizations(packageName))
                }
                "requestIgnoreBatteryOptimizations" -> {
                    // ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS shows the OEM's own
                    // system dialog directly (no settings-list navigation needed) --
                    // exactly the "one-tap prompt" PRD 4.2.5 asks for. Falls back to
                    // opening the general battery-optimization settings screen if the
                    // direct-request intent isn't handled (some OEM ROMs restrict it).
                    try {
                        val intent = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS)
                        intent.data = Uri.parse("package:$packageName")
                        startActivity(intent)
                    } catch (e: Exception) {
                        startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS))
                    }
                    result.success(null)
                }
                else -> result.notImplemented()
            }
        }
    }
}
