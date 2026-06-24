import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:encrypt/encrypt.dart' as encrypt;

/// Lightweight AES-CBC encrypted key-value store.
///
/// Data is encrypted at rest in ~/.rfplay/vault.json (Linux/macOS) or
/// a temp-file fallback. The encryption key is derived from a fixed app
/// secret via SHA-256 — good enough for MVP; a production app would use
/// platform keystores (flutter_secure_storage) or hardware-backed keys.
///
/// Usage:
/// ```dart
/// final vault = Vault();
/// await vault.save('api_base_url', 'https://example.com');
/// final url = await vault.read('api_base_url');
/// ```
class Vault {
  static final Vault _instance = Vault._internal();
  factory Vault() => _instance;
  Vault._internal();

  // Hard-coded app secret for key derivation.
  // In production this should come from --dart-define or a remote config.
  static const String _appSecret = 'RFPlay2024!SecureVault#Key';

  String get _vaultPath {
    final home = Platform.environment['HOME'] ??
        Platform.environment['USERPROFILE'];
    if (home != null) {
      final dir = Directory('$home/.rfplay');
      if (!dir.existsSync()) {
        try {
          dir.createSync(recursive: true);
        } catch (_) {}
      }
      return '$home/.rfplay/vault.json';
    }
    return '${Directory.systemTemp.path}/rfplay_vault.json';
  }

  encrypt.Key _deriveKey() {
    final bytes = sha256.convert(utf8.encode(_appSecret)).bytes;
    return encrypt.Key(bytes);
  }

  /// Encrypt [plainText] and return a base64-url-safe string (IV + ciphertext).
  String encryptText(String plainText) {
    final key = _deriveKey();
    final iv = encrypt.IV.fromSecureRandom(16);
    final encrypter = encrypt.Encrypter(encrypt.AES(key));
    final encrypted = encrypter.encrypt(plainText, iv: iv);
    final combined = iv.bytes + encrypted.bytes;
    return base64UrlEncode(combined);
  }

  /// Decrypt a value previously produced by [encryptText].
  String decryptText(String encryptedBase64) {
    final key = _deriveKey();
    final combined = base64UrlDecode(encryptedBase64);
    final iv = encrypt.IV(combined.sublist(0, 16));
    final encryptedBytes = combined.sublist(16);
    final encrypter = encrypt.Encrypter(encrypt.AES(key));
    final encrypted = encrypt.Encrypted(encryptedBytes);
    return encrypter.decrypt(encrypted, iv: iv);
  }

  /// Persist [value] under [key] in the encrypted vault.
  Future<void> save(String key, String value) async {
    final file = File(_vaultPath);
    Map<String, dynamic> data = {};
    if (await file.exists()) {
      try {
        data = jsonDecode(await file.readAsString()) as Map<String, dynamic>;
      } catch (_) {}
    }
    data[key] = encryptText(value);
    await file.writeAsString(jsonEncode(data));
  }

  /// Read and decrypt the value stored under [key], or `null` if missing.
  Future<String?> read(String key) async {
    final file = File(_vaultPath);
    if (!await file.exists()) return null;
    try {
      final data =
          jsonDecode(await file.readAsString()) as Map<String, dynamic>;
      final encrypted = data[key] as String?;
      if (encrypted == null) return null;
      return decryptText(encrypted);
    } catch (_) {
      return null;
    }
  }

  /// Remove [key] from the encrypted vault.
  Future<void> delete(String key) async {
    final file = File(_vaultPath);
    if (!await file.exists()) return;
    try {
      final data =
          jsonDecode(await file.readAsString()) as Map<String, dynamic>;
      data.remove(key);
      await file.writeAsString(jsonEncode(data));
    } catch (_) {}
  }
}
