import 'dart:async';
import 'dart:math';

import 'package:flutter/foundation.dart';

class VpnService extends ChangeNotifier {
  bool _isConnected = false;
  bool _isConnecting = false;
  String? _selectedNode;
  double _downloadSpeed = 0;
  double _uploadSpeed = 0;
  Duration _connectedTime = Duration.zero;

  Timer? _speedTimer;
  Timer? _durationTimer;
  Timer? _connectTimer;
  final Random _random = Random();

  bool get isConnected => _isConnected;
  bool get isConnecting => _isConnecting;
  String? get selectedNode => _selectedNode;
  double get downloadSpeed => _downloadSpeed;
  double get uploadSpeed => _uploadSpeed;
  Duration get connectedTime => _connectedTime;

  void selectNode(String? node) {
    _selectedNode = node;
    notifyListeners();
  }

  Future<void> start() async {
    if (_isConnected || _isConnecting) return;

    _isConnecting = true;
    notifyListeners();

    _connectTimer?.cancel();
    _connectTimer = Timer(const Duration(seconds: 2), () {
      _isConnecting = false;
      _isConnected = true;
      _connectedTime = Duration.zero;
      _startSimulation();
      notifyListeners();
    });
  }

  void stop() {
    _connectTimer?.cancel();
    _connectTimer = null;
    _stopSimulation();

    _isConnected = false;
    _isConnecting = false;
    _downloadSpeed = 0;
    _uploadSpeed = 0;
    _connectedTime = Duration.zero;
    notifyListeners();
  }

  Future<void> toggle() async {
    if (_isConnected || _isConnecting) {
      stop();
    } else {
      await start();
    }
  }

  void _startSimulation() {
    _speedTimer?.cancel();
    _durationTimer?.cancel();

    _updateSpeeds();

    _speedTimer = Timer.periodic(const Duration(seconds: 2), (_) {
      if (_isConnected) {
        _updateSpeeds();
        notifyListeners();
      }
    });

    _durationTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (_isConnected) {
        _connectedTime += const Duration(seconds: 1);
        notifyListeners();
      }
    });
  }

  void _updateSpeeds() {
    _downloadSpeed = 15 + _random.nextDouble() * 35;
    _uploadSpeed = 5 + _random.nextDouble() * 10;
  }

  void _stopSimulation() {
    _speedTimer?.cancel();
    _speedTimer = null;
    _durationTimer?.cancel();
    _durationTimer = null;
  }

  @override
  void dispose() {
    _connectTimer?.cancel();
    _stopSimulation();
    super.dispose();
  }
}
