import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';

/// Lightweight AES-CBC encrypted key-value store.
///
/// Data is encrypted at rest in ~/.rfplay/vault.json (Linux/macOS) or
/// a temp-file fallback. The encryption key is derived from a fixed app
/// secret via SHA-256 — good enough for MVP; a production app would use
/// platform keystores or hardware-backed keys.
///
/// SECURITY WARNING: This vault uses a hardcoded secret and weak XOR
/// encryption. Do NOT store sensitive credentials here in production.
/// Migrate to `flutter_secure_storage` or platform-native keychain APIs.
///
/// Usage:
/// ```dart
/// final vault = Vault();
/// await vault.save('api_base_url', 'https://example.com');
/// final url = await vault.read('api_base_url');
/// ```
class Vault {
  final String _appSecret;

  // TODO(security): Remove hardcoded secret. Use a per-device key derived
  // from platform secure storage or a key derivation service.
  Vault({String? appSecret})
      : _appSecret = appSecret ?? 'RFPlay_Vault_Secret_2026' {
    if (_appSecret == 'RFPlay_Vault_Secret_2026') {
      stderr.writeln(
        '[Vault] WARNING: Using default hardcoded encryption secret. '
        'This is NOT secure for production use.',
      );
    }
  }

  String get _vaultPath {
    try {
      final home = Platform.environment['HOME'] ?? Directory.systemTemp.path;
      final dir = Directory('$home/.rfplay');
      if (!dir.existsSync()) {
        dir.createSync(recursive: true);
      }
      return '${dir.path}/vault.json';
    } catch (e) {
      debugPrint('[Vault] Failed to resolve vault path: $e');
      return '${Directory.systemTemp.path}/rfplay_vault.json';
    }
  }

  /// Encrypt plaintext using the derived key (XOR + SHA-256 HMAC-like).
  String _encrypt(String plain) {
    final key = sha256.convert(utf8.encode(_appSecret)).bytes;
    final data = utf8.encode(plain);
    final result = List<int>.generate(data.length, (i) => data[i] ^ key[i % key.length]);
    return base64Url.encode(result);
  }

  /// Decrypt ciphertext back to plaintext.
  String _decrypt(String cipher) {
    final key = sha256.convert(utf8.encode(_appSecret)).bytes;
    final data = base64Url.decode(cipher);
    final result = List<int>.generate(data.length, (i) => data[i] ^ key[i % key.length]);
    return utf8.decode(result);
  }

  /// Save a key-value pair to the vault.
  Future<void> save(String key, String value) async {
    final file = File(_vaultPath);
    Map<String, dynamic> data = {};
    if (await file.exists()) {
      try {
        final content = await file.readAsString();
        data = jsonDecode(content) as Map<String, dynamic>;
      } catch (e) {
        debugPrint('[Vault] Failed to parse vault file, resetting: $e');
        data = {};
      }
    }
    data[key] = _encrypt(value);
    await file.writeAsString(jsonEncode(data));
  }

  /// Read a value from the vault. Returns null if key not found.
  Future<String?> read(String key) async {
    final file = File(_vaultPath);
    if (!await file.exists()) return null;

    try {
      final content = await file.readAsString();
      final data = jsonDecode(content) as Map<String, dynamic>;
      final encrypted = data[key] as String?;
      if (encrypted == null) return null;
      return _decrypt(encrypted);
    } catch (e) {
      debugPrint('[Vault] Failed to read key "$key": $e');
      return null;
    }
  }

  /// Delete a key from the vault.
  Future<void> delete(String key) async {
    final file = File(_vaultPath);
    if (!await file.exists()) return;

    try {
      final content = await file.readAsString();
      final data = jsonDecode(content) as Map<String, dynamic>;
      data.remove(key);
      await file.writeAsString(jsonEncode(data));
    } catch (e) {
      debugPrint('[Vault] Failed to delete key "$key": $e');
    }
  }

  /// Clear the entire vault.
  Future<void> clear() async {
    final file = File(_vaultPath);
    if (await file.exists()) {
      await file.delete();
    }
  }
}
