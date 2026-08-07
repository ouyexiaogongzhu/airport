import 'dart:convert';
import 'dart:ffi';
import 'dart:io';

import 'package:ffi/ffi.dart';
import 'package:flutter/foundation.dart';

import 'xray_engine.dart';

/// Desktop/mobile FFI binding for libXray.
///
/// libXray built with cgo (`go build -buildmode=c-shared`) exports:
///   char* CGoInvoke(char* requestJSON);
///   void  CGoFree(char* value);
///
/// The request is a JSON object:
///   {
///     "apiVersion": 1,
///     "method": "runXrayFromJson",
///     "payload": { "dat_dir": "...", "mph_cache_path": "", "config_json": "..." }
///   }
/// The response is JSON: { "success": bool, "data": ..., "error": "" }.
///
/// Build the shared library with:
///   cd third_party/libXray
///   python3 build/main.py --target  (see scripts/build_libxray.sh)
/// and place libxray.so / xray.dll / libxray.dylib next to the app bundle.
class FfiXrayEngine implements XrayEngine {
  DynamicLibrary? _lib;
  bool _initialized = false;
  String? _initError;

  @override
  Future<void> start(String configJson) async {
    final result = _invoke('runXrayFromJson', configJson);
    if (result != null) {
      final decoded = _decodeInvoke(result);
      if (!(decoded['success'] as bool? ?? false)) {
        throw StateError('libXray start failed: ${decoded['error']}');
      }
    }
  }

  @override
  Future<void> stop() async {
    _invoke('stopXray', '');
  }

  @override
  Future<bool> isRunning() async {
    final result = _invoke('getXrayState', '');
    if (result == null) return false;
    final decoded = _decodeInvoke(result);
    final data = decoded['data'];
    if (data is Map) {
      return data['running'] as bool? ?? false;
    }
    return false;
  }

  @override
  Future<({int upload, int download})> stats() async {
    final result = _invoke('getXrayState', '');
    if (result == null) return (upload: 0, download: 0);
    final decoded = _decodeInvoke(result);
    final data = decoded['data'];
    if (data is Map) {
      final up = data['upload'] ?? data['uplink'] ?? 0;
      final down = data['download'] ?? data['downlink'] ?? 0;
      return (
        upload: _numToInt(up),
        download: _numToInt(down),
      );
    }
    return (upload: 0, download: 0);
  }

  int _numToInt(dynamic v) {
    if (v is num) return v.toInt();
    if (v is String) return int.tryParse(v) ?? 0;
    return 0;
  }

  /// Locate and open the libXray shared library.
  DynamicLibrary? _loadLibrary() {
    if (_lib != null) return _lib;
    if (_initialized) return null;
    _initialized = true;

    final candidates = <String>[];
    if (Platform.isWindows) {
      // Look next to the executable first, then the project libs dir.
      final exeDir = File(Platform.resolvedExecutable).parent.path;
      candidates.addAll([
        '$exeDir\\libxray.dll',
        '$exeDir\\xray.dll',
        'xray.dll',
        'libxray.dll',
        'C:\\Windows\\System32\\xray.dll',
      ]);
    } else if (Platform.isLinux) {
      final exeDir = File(Platform.resolvedExecutable).parent.path;
      candidates.addAll([
        '$exeDir/lib/libxray.so',
        '$exeDir/libxray.so',
        'libxray.so',
        '/usr/lib/libxray.so',
        '/usr/local/lib/libxray.so',
      ]);
    } else if (Platform.isMacOS) {
      final bundle = Directory(
        '${Platform.resolvedExecutable}/../../..',
      );
      candidates.addAll([
        // Inside the .app bundle: <app>/Contents/Frameworks/libxray.dylib
        '${bundle.path}/Contents/Frameworks/libxray.dylib',
        // Relative to the repo when running from `flutter run`.
        'libxray.dylib',
        'macos/libs/libxray.dylib',
        'LibXray.xcframework/macos-arm64/libxray.dylib',
      ]);
    } else if (Platform.isIOS) {
      // On iOS the binding is statically linked into the app; the
      // NetworkExtension provider calls libXray directly in Swift.
      return null;
    }

    for (final path in candidates) {
      try {
        final lib = DynamicLibrary.open(path);
        _lib = lib;
        return lib;
      } catch (e) {
        _initError = '$path: $e';
      }
    }
    debugPrint('[FfiXrayEngine] failed to load libXray: $_initError');
    return null;
  }

  /// Returns the base64 response from CGoInvoke, or null if unavailable.
  String? _invoke(String method, String configJsonOrEmpty) {
    final lib = _loadLibrary();
    if (lib == null) return null;

    final invokeC = lib
        .lookupFunction<Pointer<Utf8> Function(Pointer<Utf8>),
            Pointer<Utf8> Function(Pointer<Utf8>)>('CGoInvoke');
    final freeC = lib.lookupFunction<Void Function(Pointer<Utf8>),
        void Function(Pointer<Utf8>)>('CGoFree');

    final request = jsonEncode({
      'apiVersion': 1,
      'method': method,
      'payload': {
        'dat_dir': '.',
        'mph_cache_path': '',
        'config_json': configJsonOrEmpty,
      },
    });

    final reqPtr = request.toNativeUtf8();
    final respPtr = invokeC(reqPtr);
    calloc.free(reqPtr);

    if (respPtr == nullptr) {
      return null;
    }
    final response = respPtr.toDartString();
    freeC(respPtr);
    return response;
  }

  Map<String, dynamic> _decodeInvoke(String response) {
    try {
      final decoded = jsonDecode(response);
      if (decoded is Map<String, dynamic>) return decoded;
      return <String, dynamic>{'success': false, 'error': 'bad response'};
    } catch (e) {
      return <String, dynamic>{'success': false, 'error': e.toString()};
    }
  }
}
