import 'package:connectivity_plus/connectivity_plus.dart';

/// A thin wrapper so the rest of the app depends on a boolean stream, not on
/// connectivity_plus's richer (and easy-to-misuse) result type directly.
///
/// PRD 4.2.5 is explicit that a connectivity *change* is a primary,
/// synchronous sync trigger -- not something deferred to a periodic
/// background worker, which battery-optimising OEM Android builds (MIUI,
/// FunTouch, ColorOS) routinely defer for hours. Callers should listen to
/// [onConnectivityChanged] and flush the outbox immediately on a transition
/// to online, not poll it.
class ConnectivityService {
  ConnectivityService([Connectivity? connectivity])
    : _connectivity = connectivity ?? Connectivity();

  final Connectivity _connectivity;

  Future<bool> isOnline() async {
    final results = await _connectivity.checkConnectivity();
    return _hasConnection(results);
  }

  Stream<bool> get onConnectivityChanged =>
      _connectivity.onConnectivityChanged.map(_hasConnection);

  bool _hasConnection(List<ConnectivityResult> results) =>
      results.any((r) => r != ConnectivityResult.none);
}
