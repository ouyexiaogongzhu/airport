#!/bin/bash
set -e
cd /home/vincent/code/airport/client

echo "=== COMMAND 1: dart format ==="
dart format lib/models/subscription.dart lib/services/subscription_service.dart lib/services/vpn_service.dart lib/screens/vpn_screen.dart lib/screens/dashboard_screen.dart lib/screens/main_shell.dart lib/main.dart 2>&1

echo "=== COMMAND 2: chmod 664 ==="
chmod 664 lib/models/subscription.dart lib/services/subscription_service.dart lib/services/vpn_service.dart lib/screens/vpn_screen.dart lib/screens/dashboard_screen.dart lib/screens/main_shell.dart lib/main.dart 2>&1

echo "=== COMMAND 3: flutter analyze ==="
flutter analyze 2>&1
