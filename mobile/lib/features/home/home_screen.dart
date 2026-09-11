import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/power/battery_optimization_service.dart';
import '../../core/providers.dart';
import '../attendance/attendance_screen.dart';

/// A minimal landing screen: pick academic year -> class -> section, then
/// open the attendance register for today. A production build would resolve
/// "my section" automatically from section_teachers for a class teacher;
/// that assignment lookup doesn't exist yet server-side (see
/// docs/PROGRESS.md), so this picker is the honest stand-in for it.
class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  List<Map<String, dynamic>> _years = [];
  List<Map<String, dynamic>> _classes = [];
  List<Map<String, dynamic>> _sections = [];

  String? _yearId;
  String? _classId;
  String? _sectionId;
  bool _loading = true;
  String? _error;

  final _battery = BatteryOptimizationService();
  bool _showBatteryPrompt = false;

  @override
  void initState() {
    super.initState();
    _loadYears();
    _checkBatteryOptimization();
  }

  // PRD 4.2.5: "Onboarding tells teachers to exempt the app from battery
  // optimisation, with a one-tap prompt to the OEM setting." Checked (not
  // just shown once at install) because OEMs let the user re-enable
  // optimization later, and a teacher who did that should see the prompt
  // again next time they open the app, not just on first install.
  Future<void> _checkBatteryOptimization() async {
    final exempted = await _battery.isIgnoringBatteryOptimizations();
    if (mounted && !exempted) setState(() => _showBatteryPrompt = true);
  }

  Future<void> _loadYears() async {
    setState(() => _loading = true);
    try {
      final session = ref.read(sessionControllerProvider).session!;
      final api = ref.read(apiClientProvider);
      final body = await api.getJson(
        '/api/v1/academic-years',
        accessToken: session.accessToken,
      );
      setState(() {
        _years = (body['items'] as List<dynamic>).cast<Map<String, dynamic>>();
        if (_years.isNotEmpty) _yearId = _years.first['id'] as String;
      });
      if (_yearId != null) await _loadClasses();
    } catch (e) {
      setState(() => _error = 'Could not load academic years.');
    } finally {
      setState(() => _loading = false);
    }
  }

  Future<void> _loadClasses() async {
    final session = ref.read(sessionControllerProvider).session!;
    final api = ref.read(apiClientProvider);
    final body = await api.getJson(
      '/api/v1/classes?academic_year_id=$_yearId',
      accessToken: session.accessToken,
    );
    setState(() {
      _classes = (body['items'] as List<dynamic>).cast<Map<String, dynamic>>();
      _classId = _classes.isNotEmpty ? _classes.first['id'] as String : null;
      _sections = [];
      _sectionId = null;
    });
    if (_classId != null) await _loadSections();
  }

  Future<void> _loadSections() async {
    final session = ref.read(sessionControllerProvider).session!;
    final api = ref.read(apiClientProvider);
    final body = await api.getJson(
      '/api/v1/sections?class_id=$_classId',
      accessToken: session.accessToken,
    );
    setState(() {
      _sections = (body['items'] as List<dynamic>).cast<Map<String, dynamic>>();
      _sectionId = _sections.isNotEmpty
          ? _sections.first['id'] as String
          : null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final controller = ref.watch(sessionControllerProvider);
    final session = controller.session!;

    return Scaffold(
      appBar: AppBar(
        title: Text(session.activeSchoolName ?? 'Tamil School OS'),
        actions: [
          IconButton(
            icon: const Icon(Icons.logout),
            onPressed: () => controller.logout(),
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (_showBatteryPrompt) ...[
                    Card(
                      color: Theme.of(context).colorScheme.secondaryContainer,
                      child: Padding(
                        padding: const EdgeInsets.all(12),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            const Text(
                              'Allow this app to run in the background so attendance '
                              'syncs even if your phone tries to save battery by '
                              'pausing the app.',
                            ),
                            const SizedBox(height: 8),
                            Row(
                              mainAxisAlignment: MainAxisAlignment.end,
                              children: [
                                TextButton(
                                  onPressed: () => setState(
                                    () => _showBatteryPrompt = false,
                                  ),
                                  child: const Text('Skip'),
                                ),
                                const SizedBox(width: 8),
                                FilledButton(
                                  onPressed: () async {
                                    await _battery.requestExemption();
                                    if (mounted) {
                                      setState(
                                        () => _showBatteryPrompt = false,
                                      );
                                    }
                                  },
                                  child: const Text('Allow'),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                    ),
                    const SizedBox(height: 16),
                  ],
                  if (_error != null)
                    Text(_error!, style: const TextStyle(color: Colors.red)),
                  _dropdown('Academic year', _years, _yearId, (v) {
                    setState(() => _yearId = v);
                    _loadClasses();
                  }),
                  const SizedBox(height: 12),
                  _dropdown('Class', _classes, _classId, (v) {
                    setState(() => _classId = v);
                    _loadSections();
                  }),
                  const SizedBox(height: 12),
                  _dropdown('Section', _sections, _sectionId, (v) {
                    setState(() => _sectionId = v);
                  }),
                  const SizedBox(height: 24),
                  FilledButton(
                    onPressed: _sectionId == null
                        ? null
                        : () {
                            Navigator.of(context).push(
                              MaterialPageRoute(
                                builder: (_) => AttendanceScreen(
                                  sectionId: _sectionId!,
                                  date: DateTime.now(),
                                ),
                              ),
                            );
                          },
                    child: const Text('Open attendance'),
                  ),
                ],
              ),
            ),
    );
  }

  Widget _dropdown(
    String label,
    List<Map<String, dynamic>> items,
    String? value,
    ValueChanged<String?> onChanged,
  ) {
    return DropdownButtonFormField<String>(
      initialValue: value,
      decoration: InputDecoration(labelText: label),
      items: items
          .map(
            (item) => DropdownMenuItem(
              value: item['id'] as String,
              child: Text((item['label'] ?? item['name']) as String),
            ),
          )
          .toList(),
      onChanged: items.isEmpty ? null : onChanged,
    );
  }
}
