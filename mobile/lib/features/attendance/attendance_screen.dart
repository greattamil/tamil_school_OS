import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/connectivity/connectivity_service.dart';
import '../../core/db/app_database.dart';
import '../../core/providers.dart';
import 'attendance_repository.dart';

/// The attendance marking screen -- the single most important screen in the
/// product (PRD 4.2.1: "Will abandon the app permanently if it loses a
/// period's attendance"). Every write lands in the local database
/// immediately; the network is something that happens to a queue in the
/// background, never something the teacher is left waiting on.
class AttendanceScreen extends ConsumerStatefulWidget {
  const AttendanceScreen({
    super.key,
    required this.sectionId,
    required this.date,
  });

  final String sectionId;
  final DateTime date;

  @override
  ConsumerState<AttendanceScreen> createState() => _AttendanceScreenState();
}

class _AttendanceScreenState extends ConsumerState<AttendanceScreen>
    with WidgetsBindingObserver {
  late final AttendanceRepository _repo;
  late final ConnectivityService _connectivity;
  StreamSubscription<bool>? _connectivitySub;

  final Map<String, ({String status, String? reason})> _marks = {};
  bool _syncing = false;
  String? _refreshError;

  DateTime get _day =>
      DateTime(widget.date.year, widget.date.month, widget.date.day);

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);

    final session = ref.read(sessionControllerProvider);
    _repo = AttendanceRepository(
      database: session.database!,
      api: ref.read(apiClientProvider),
      deviceIdentity: session.deviceIdentity!,
      accessToken: () =>
          ref.read(sessionControllerProvider).session!.accessToken,
    );
    _connectivity = ConnectivityService();

    // Foreground and connectivity-regained are the primary sync triggers
    // (PRD 4.2.5) -- not a periodic background task, which OEM battery
    // management on the devices teachers actually own (MIUI, FunTouch,
    // ColorOS) routinely defers for hours.
    _repo.refreshFromServer(widget.sectionId, _day).catchError((Object e) {
      // Offline on open is expected and fine -- the screen still renders
      // from whatever is already in the local cache. But a failure that
      // ISN'T just "no network" (a local database error, for instance)
      // deserves more than silence: this is what caught a real on-device
      // bug during development (the background isolate opening the wrong
      // native SQLite library -- see connection.dart) that would otherwise
      // have looked identical to an empty section from the UI alone.
      if (mounted) setState(() => _refreshError = e.toString());
    });
    _flushIfPending();
    _connectivitySub = _connectivity.onConnectivityChanged.listen((online) {
      if (online) _flushIfPending();
    });
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _flushIfPending();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _connectivitySub?.cancel();
    super.dispose();
  }

  Future<void> _flushIfPending() async {
    if (_syncing) return;
    setState(() => _syncing = true);
    try {
      final result = await _repo.flushOutbox(widget.sectionId, _day);
      if (result.overridden.isNotEmpty && mounted) {
        _showOverriddenDialog(result.overridden);
      }
    } catch (e) {
      // Sync failure just leaves the outbox pending -- the visible pending
      // count and manual sync button are the teacher's remedy (PRD 4.2.5).
      debugPrint('Attendance sync failed, left pending: $e');
    } finally {
      if (mounted) setState(() => _syncing = false);
    }
  }

  void _showOverriddenDialog(List<OverriddenEntry> overridden) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Some edits were overridden'),
        content: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: overridden
                .map(
                  (o) => Padding(
                    padding: const EdgeInsets.symmetric(vertical: 4),
                    child: Text(
                      '${o.studentName}: now "${o.currentStatus}" -- ${o.message}',
                    ),
                  ),
                )
                .toList(),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('OK'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Attendance'),
        actions: [
          StreamBuilder<int>(
            stream: _repo.watchPendingCount(widget.sectionId, _day),
            builder: (context, snapshot) {
              final pending = snapshot.data ?? 0;
              return Padding(
                padding: const EdgeInsets.symmetric(horizontal: 12),
                child: Center(
                  child: Row(
                    children: [
                      if (pending > 0) ...[
                        Text('$pending pending'),
                        const SizedBox(width: 8),
                      ],
                      IconButton(
                        icon: _syncing
                            ? const SizedBox(
                                width: 18,
                                height: 18,
                                child: CircularProgressIndicator(
                                  strokeWidth: 2,
                                ),
                              )
                            : const Icon(Icons.sync),
                        tooltip: 'Sync now',
                        onPressed: _syncing ? null : _flushIfPending,
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
        ],
      ),
      body: Column(
        children: [
          // PRD 4.2.5: "Pending registers older than 24 hours raise an
          // in-app warning to the teacher."
          StreamBuilder<DateTime?>(
            stream: _repo.watchOldestPendingTimestamp(widget.sectionId, _day),
            builder: (context, snapshot) {
              final oldest = snapshot.data;
              if (oldest == null ||
                  DateTime.now().difference(oldest) <
                      const Duration(hours: 24)) {
                return const SizedBox.shrink();
              }
              return MaterialBanner(
                backgroundColor: Colors.amber.shade100,
                content: const Text(
                  'This register has been waiting to sync for over 24 hours. '
                  'Check your connection or tell the office if this continues.',
                ),
                actions: [
                  TextButton(
                    onPressed: _syncing ? null : _flushIfPending,
                    child: const Text('Retry sync'),
                  ),
                ],
              );
            },
          ),
          Expanded(child: _rosterBody()),
        ],
      ),
    );
  }

  Widget _rosterBody() {
    return StreamBuilder<List<LocalAttendanceEntry>>(
      stream: _repo.watchRoster(widget.sectionId, _day),
      builder: (context, snapshot) {
        final roster = snapshot.data ?? [];
        if (roster.isEmpty) {
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text('No students found for this section.'),
                  if (_refreshError != null) ...[
                    const SizedBox(height: 8),
                    Text(
                      'Could not refresh from the server: $_refreshError',
                      style: Theme.of(context).textTheme.bodySmall,
                      textAlign: TextAlign.center,
                    ),
                  ],
                ],
              ),
            ),
          );
        }

        // Seed the working set: whatever's already recorded, or a present
        // default for anyone not yet marked (PRD 4.2.1: "all defaulted to
        // present").
        for (final entry in roster) {
          _marks.putIfAbsent(
            entry.enrollmentId,
            () => (status: entry.status ?? 'present', reason: entry.reason),
          );
        }

        return Column(
          children: [
            Expanded(
              child: ListView.builder(
                itemCount: roster.length,
                itemBuilder: (context, index) {
                  final entry = roster[index];
                  final mark = _marks[entry.enrollmentId]!;
                  final isAbsent = mark.status == 'absent';
                  return ListTile(
                    leading: CircleAvatar(
                      backgroundColor: isAbsent
                          ? Colors.red.shade100
                          : Colors.green.shade100,
                      child: Icon(
                        isAbsent ? Icons.close : Icons.check,
                        color: isAbsent ? Colors.red : Colors.green,
                      ),
                    ),
                    title: Text(entry.studentName),
                    subtitle: Text(
                      [
                        if (entry.rollNumber != null)
                          'Roll ${entry.rollNumber}',
                        isAbsent && mark.reason != null ? mark.reason! : null,
                      ].whereType<String>().join(' -- '),
                    ),
                    trailing: entry.syncStatus == 'pending'
                        ? const Icon(Icons.cloud_upload_outlined, size: 18)
                        : const Icon(
                            Icons.cloud_done_outlined,
                            size: 18,
                            color: Colors.grey,
                          ),
                    onTap: () => _toggle(entry.enrollmentId),
                    onLongPress: isAbsent
                        ? () => _pickReason(entry.enrollmentId)
                        : null,
                  );
                },
              ),
            ),
            SafeArea(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: FilledButton(
                  onPressed: () => _confirm(roster),
                  child: const Text('Confirm & sync register'),
                ),
              ),
            ),
          ],
        );
      },
    );
  }

  void _toggle(String enrollmentId) {
    setState(() {
      final current = _marks[enrollmentId]!;
      _marks[enrollmentId] = current.status == 'present'
          ? (status: 'absent', reason: 'unexcused')
          : (status: 'present', reason: null);
    });
  }

  Future<void> _pickReason(String enrollmentId) async {
    final reason = await showDialog<String>(
      context: context,
      builder: (context) => SimpleDialog(
        title: const Text('Reason'),
        children: [
          for (final r in ['sick', 'permitted', 'unexcused'])
            SimpleDialogOption(
              onPressed: () => Navigator.of(context).pop(r),
              child: Text(r),
            ),
        ],
      ),
    );
    if (reason != null) {
      setState(() {
        final current = _marks[enrollmentId]!;
        _marks[enrollmentId] = (status: current.status, reason: reason);
      });
    }
  }

  Future<void> _confirm(List<LocalAttendanceEntry> roster) async {
    final marksForConfirmedRoster = {
      for (final entry in roster)
        entry.enrollmentId: _marks[entry.enrollmentId]!,
    };
    await _repo.confirmRegister(
      sectionId: widget.sectionId,
      date: _day,
      marks: marksForConfirmedRoster,
    );
    await _flushIfPending();
    if (mounted) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('Register saved.')));
    }
  }
}
