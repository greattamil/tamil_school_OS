import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/providers.dart';

/// The parent-facing landing screen (PRD 4.8): a child selector for parents
/// with more than one child in the school, and that child's attendance and
/// dues. Notices are the recipient's own inbox (PRD 4.5.1: delivery/read
/// tracking is per recipient), not filtered by the selected child, since a
/// notice can concern any of a parent's children or the whole school.
class ParentDashboardScreen extends ConsumerStatefulWidget {
  const ParentDashboardScreen({super.key});

  @override
  ConsumerState<ParentDashboardScreen> createState() => _ParentDashboardScreenState();
}

class _ParentDashboardScreenState extends ConsumerState<ParentDashboardScreen> {
  List<Map<String, dynamic>> _children = [];
  String? _selectedStudentId;
  Map<String, dynamic>? _attendance;
  Map<String, dynamic>? _dues;
  List<Map<String, dynamic>> _notices = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    try {
      final session = ref.read(sessionControllerProvider).session!;
      final api = ref.read(apiClientProvider);

      final childrenBody = await api.getJson(
        '/api/v1/parent/children',
        accessToken: session.accessToken,
      );
      _children = (childrenBody['items'] as List<dynamic>).cast<Map<String, dynamic>>();
      _selectedStudentId ??= _children.isNotEmpty ? _children.first['student_id'] as String : null;

      final noticesBody = await api.getJson(
        '/api/v1/notices',
        accessToken: session.accessToken,
      );
      _notices = (noticesBody['items'] as List<dynamic>).cast<Map<String, dynamic>>();

      if (_selectedStudentId != null) {
        await _loadChildDetail(session.accessToken);
      }
    } catch (e) {
      setState(() => _error = 'Could not load your dashboard.');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _loadChildDetail(String accessToken) async {
    final api = ref.read(apiClientProvider);
    final now = DateTime.now();
    final yearStart = DateTime(now.year, 1, 1);

    final attendanceBody = await api.getJson(
      '/api/v1/students/$_selectedStudentId/attendance-percentage'
      '?from=${_formatDate(yearStart)}&to=${_formatDate(now)}',
      accessToken: accessToken,
    );
    final duesBody = await api.getJson(
      '/api/v1/students/$_selectedStudentId/dues',
      accessToken: accessToken,
    );
    setState(() {
      _attendance = attendanceBody;
      _dues = duesBody;
    });
  }

  String _formatDate(DateTime d) =>
      '${d.year.toString().padLeft(4, '0')}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

  @override
  Widget build(BuildContext context) {
    final controller = ref.watch(sessionControllerProvider);
    final session = controller.session!;

    return Scaffold(
      appBar: AppBar(
        title: Text(session.activeSchoolName ?? 'Tamil School OS'),
        actions: [
          IconButton(icon: const Icon(Icons.logout), onPressed: () => controller.logout()),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : RefreshIndicator(
              onRefresh: _load,
              child: ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  if (_error != null) Text(_error!, style: const TextStyle(color: Colors.red)),
                  if (_children.isEmpty)
                    const Text('No children are linked to your account yet.')
                  else ...[
                    if (_children.length > 1)
                      DropdownButtonFormField<String>(
                        initialValue: _selectedStudentId,
                        decoration: const InputDecoration(labelText: 'Child'),
                        items: _children
                            .map(
                              (c) => DropdownMenuItem(
                                value: c['student_id'] as String,
                                child: Text(c['name_english'] as String),
                              ),
                            )
                            .toList(),
                        onChanged: (v) async {
                          setState(() => _selectedStudentId = v);
                          await _loadChildDetail(session.accessToken);
                        },
                      )
                    else
                      Text(
                        _children.first['name_english'] as String,
                        style: Theme.of(context).textTheme.headlineSmall,
                      ),
                    const SizedBox(height: 16),
                    _card(
                      title: 'Attendance this year',
                      child: Text(
                        _attendance == null
                            ? '--'
                            : '${(_attendance!['percentage'] as num).toStringAsFixed(1)}% '
                                  '(${_attendance!['present_days']}/${_attendance!['total_days']} days)',
                        style: Theme.of(context).textTheme.headlineSmall,
                      ),
                    ),
                    const SizedBox(height: 12),
                    _card(
                      title: 'Fee dues',
                      child: Text(
                        _dues == null
                            ? '--'
                            : '₹${(((_dues!['outstanding_amount_paise'] as num?) ?? 0) / 100).toStringAsFixed(2)}'
                                  '${_dues!['due_date'] != null ? '  (due ${(_dues!['due_date'] as String).substring(0, 10)})' : ''}',
                        style: Theme.of(context).textTheme.headlineSmall,
                      ),
                    ),
                  ],
                  const SizedBox(height: 24),
                  Text('Notices', style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 8),
                  if (_notices.isEmpty) const Text('No notices yet.'),
                  for (final n in _notices)
                    Card(
                      child: ListTile(
                        title: Text(n['title'] as String),
                        subtitle: Text(n['body_en'] as String),
                      ),
                    ),
                ],
              ),
            ),
    );
  }

  Widget _card({required String title, required Widget child}) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: Theme.of(context).textTheme.titleSmall),
            const SizedBox(height: 8),
            child,
          ],
        ),
      ),
    );
  }
}
