import 'dart:convert';

import 'package:http/http.dart' as http;

/// Base URL for the Go backend. Overridable at build time with
/// `--dart-define=API_BASE_URL=https://api.example.com` for a real device
/// pointed at something other than a local dev server; 10.0.2.2 is the
/// standard Android-emulator alias for the host machine's localhost.
const _defaultBaseUrl = 'http://10.0.2.2:8080';

class ApiException implements Exception {
  ApiException(this.statusCode, this.errorCode, this.message);

  final int statusCode;
  final String errorCode;
  final String message;

  @override
  String toString() => 'ApiException($statusCode, $errorCode, $message)';
}

/// Thin REST client for /api/v1 (PRD 8.5). Deliberately minimal -- no
/// generated SDK, matching the same choice made for the admin panel's
/// lib/api.ts.
class ApiClient {
  ApiClient({http.Client? httpClient, String? baseUrl})
    : _client = httpClient ?? http.Client(),
      baseUrl = baseUrl ?? const String.fromEnvironment(
        'API_BASE_URL',
        defaultValue: _defaultBaseUrl,
      );

  final http.Client _client;
  final String baseUrl;

  Future<Map<String, dynamic>> postJson(
    String path,
    Map<String, dynamic> body, {
    String? accessToken,
  }) async {
    final res = await _client.post(
      Uri.parse('$baseUrl$path'),
      headers: _headers(accessToken),
      body: jsonEncode(body),
    );
    return _decode(res);
  }

  Future<Map<String, dynamic>> getJson(
    String path, {
    String? accessToken,
  }) async {
    final res = await _client.get(
      Uri.parse('$baseUrl$path'),
      headers: _headers(accessToken),
    );
    return _decode(res);
  }

  Map<String, String> _headers(String? accessToken) => {
    'Content-Type': 'application/json',
    if (accessToken != null) 'Authorization': 'Bearer $accessToken',
  };

  Map<String, dynamic> _decode(http.Response res) {
    final body = res.body.isEmpty
        ? <String, dynamic>{}
        : jsonDecode(res.body) as Map<String, dynamic>;
    if (res.statusCode >= 200 && res.statusCode < 300) {
      return body;
    }
    throw ApiException(
      res.statusCode,
      (body['error'] as String?) ?? 'unknown_error',
      (body['message'] as String?) ?? res.reasonPhrase ?? 'Request failed',
    );
  }
}
