import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/providers.dart';

/// Shown when a staff member holds a role at more than one school (PRD 3.2.1).
/// Selecting a school exchanges the current token for one scoped to it --
/// never a parameter change on the existing token -- and opens that school's
/// own local database.
class SchoolPickerScreen extends ConsumerWidget {
  const SchoolPickerScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionControllerProvider).session!;

    return Scaffold(
      appBar: AppBar(title: const Text('Choose a school')),
      body: ListView.builder(
        itemCount: session.schools.length,
        itemBuilder: (context, index) {
          final school = session.schools[index];
          return ListTile(
            title: Text(school.schoolName),
            subtitle: Text(school.role),
            onTap: () => ref.read(sessionControllerProvider).selectSchool(school),
          );
        },
      ),
    );
  }
}
