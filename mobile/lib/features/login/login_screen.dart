import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/api_client.dart';
import '../../core/providers.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Tamil School OS'),
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Staff'),
            Tab(text: 'Parent'),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: const [_StaffLoginForm(), _ParentOtpForm()],
      ),
    );
  }
}

class _StaffLoginForm extends ConsumerStatefulWidget {
  const _StaffLoginForm();

  @override
  ConsumerState<_StaffLoginForm> createState() => _StaffLoginFormState();
}

class _StaffLoginFormState extends ConsumerState<_StaffLoginForm> {
  final _identifierController = TextEditingController();
  final _passwordController = TextEditingController();
  String? _error;
  bool _pending = false;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          TextField(
            controller: _identifierController,
            decoration: const InputDecoration(labelText: 'Email or mobile'),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _passwordController,
            decoration: const InputDecoration(labelText: 'Password'),
            obscureText: true,
          ),
          const SizedBox(height: 16),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(_error!, style: const TextStyle(color: Colors.red)),
            ),
          FilledButton(
            onPressed: _pending ? null : _submit,
            child: Text(_pending ? 'Signing in...' : 'Sign in'),
          ),
        ],
      ),
    );
  }

  Future<void> _submit() async {
    setState(() {
      _pending = true;
      _error = null;
    });
    try {
      await ref
          .read(sessionControllerProvider)
          .loginStaff(_identifierController.text.trim(), _passwordController.text);
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (e) {
      setState(() => _error = 'Could not reach the server.');
    } finally {
      if (mounted) setState(() => _pending = false);
    }
  }
}

class _ParentOtpForm extends ConsumerStatefulWidget {
  const _ParentOtpForm();

  @override
  ConsumerState<_ParentOtpForm> createState() => _ParentOtpFormState();
}

class _ParentOtpFormState extends ConsumerState<_ParentOtpForm> {
  final _mobileController = TextEditingController();
  final _codeController = TextEditingController();
  String? _error;
  String? _devCode;
  bool _otpRequested = false;
  bool _pending = false;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          TextField(
            controller: _mobileController,
            decoration: const InputDecoration(labelText: 'Mobile number'),
            keyboardType: TextInputType.phone,
            enabled: !_otpRequested,
          ),
          if (_otpRequested) ...[
            const SizedBox(height: 12),
            TextField(
              controller: _codeController,
              decoration: const InputDecoration(labelText: 'OTP code'),
              keyboardType: TextInputType.number,
            ),
            if (_devCode != null)
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(
                  'Dev only -- code is $_devCode',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
          ],
          const SizedBox(height: 16),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.only(bottom: 12),
              child: Text(_error!, style: const TextStyle(color: Colors.red)),
            ),
          FilledButton(
            onPressed: _pending ? null : (_otpRequested ? _verify : _requestOtp),
            child: Text(
              _pending
                  ? 'Please wait...'
                  : (_otpRequested ? 'Verify' : 'Send OTP'),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _requestOtp() async {
    setState(() {
      _pending = true;
      _error = null;
    });
    try {
      final devCode = await ref
          .read(sessionControllerProvider)
          .requestParentOtp(_mobileController.text.trim());
      setState(() {
        _otpRequested = true;
        _devCode = devCode;
      });
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (e) {
      setState(() => _error = 'Could not reach the server.');
    } finally {
      if (mounted) setState(() => _pending = false);
    }
  }

  Future<void> _verify() async {
    setState(() {
      _pending = true;
      _error = null;
    });
    try {
      await ref
          .read(sessionControllerProvider)
          .verifyParentOtp(_mobileController.text.trim(), _codeController.text.trim());
    } on ApiException catch (e) {
      setState(() => _error = e.message);
    } catch (e) {
      setState(() => _error = 'Could not reach the server.');
    } finally {
      if (mounted) setState(() => _pending = false);
    }
  }
}
