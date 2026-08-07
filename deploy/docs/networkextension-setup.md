# RFPlay Client — iOS/macOS NetworkExtension 整合指南

本文件說明如何把 `client/ios/` 與 `client/macos/` 下已寫好的原生源碼接入 Xcode 工程。

## 已就緒的源碼

| 檔案 | 用途 |
| --- | --- |
| `ios/Runner/PacketTunnelProvider.swift` | iOS 的 `NEPacketTunnelProvider`：配置 TUN → 注入 fd → 啟動 libXray |
| `ios/Runner/LibXrayBridge.swift` | iOS Swift 橋接 `LibXray.xcframework`（`#if canImport(LibXray)` 條件編譯） |
| `ios/Runner/AppDelegate.swift` | 註冊 `uk.rfplay.client/vpn` MethodChannel：start/stop/status |
| `ios/Runner/Runner.entitlements` | App Group + packet-tunnel-provider entitlement |
| `ios/Runner/Info.plist` | 已加 `UIBackgroundModes = [networkextension]` |
| `macos/Runner/PacketTunnelProvider.swift` | macOS 版 provider（utun） |
| `macos/Runner/LibXrayBridge.swift` | macOS Swift 橋接 |
| `macos/Runner/*.entitlements` | App Sandbox + App Group + network 權限 |

## Xcode 整合步驟（iOS）

### 1. 建立 Network Extension Target

Xcode → `File > New > Target` → 選 **Network Extension** → Product Name 設為 `PacketTunnel` → Language **Swift**。確認 Bundle ID 為：

```
com.example.rfplay-client.PacketTunnel
```

（`AppDelegate.swift` 中的 `providerBundleIdentifier` 即此值。）

### 2. 把源碼加入 Target

- 把 `PacketTunnelProvider.swift`、`LibXrayBridge.swift` 加入 **PacketTunnel** target 的 Sources。
- 確認 **Runner** target **不要**編譯這兩個檔案（避免 symbol 衝突）。
- `AppDelegate.swift` 保持只在 Runner target。

### 3. Entitlements

- **Runner target**：`Signing & Capabilities` → 加 **App Groups**（`group.com.example.rfplay-client`）與 **Network Extensions**（勾選 Packet Tunnel Provider）。
- **PacketTunnel target**：同樣加 App Groups（同一個 group id）。
- 已提供 `Runner.entitlements`，可在 target 的 `CODE_SIGN_ENTITLEMENTS` 指向它。

### 4. 連結 LibXray

- 把 `ios/LibXray.xcframework`（由 `scripts/build_libxray.sh apple` 生成）拖入 **PacketTunnel** target 的 `Frameworks, Libraries, and Embedded Content`。
- `LibXrayBridge.swift` 有 `#if canImport(LibXray)`，framework 不存在時也能編譯，只是運行時回傳失敗。

### 5. iOS 上架注意事項（藍圖 Phase 5 已標記）

- 必須使用**組織/公司 Apple Developer Program 帳號**（Guideline 5.4）。
- NetworkExtension entitlement 需在 Developer Portal 開啟（self-serve）。
- 提交審核時需附隱私政策與可連通的生產節點。

## macOS 整合步驟

1. `File > New > Target` → **Packet Tunnel Provider**（macOS 也有此模板）→ Bundle ID `com.example.rfplay-client.PacketTunnel`。
2. 把 `macos/Runner/PacketTunnelProvider.swift`、`macos/Runner/LibXrayBridge.swift` 加入該 target。
3. **Runner app target** 的 Signing & Capabilities 加 **App Sandbox**（network server/client）+ **App Groups**（`group.com.example.rfplay-client`）。
4. 把 `macos/LibXray.xcframework` 加入 PacketTunnel target 的 Frameworks。

> macOS 的另一種做法：因為 app 是 sandboxed，需要 root 或 helper 才能創建 utun。`NEPacketTunnelProvider` 由系統以 root 啟動，所以不需要額外 helper。

## Dart 側對接

`client/lib/services/xray_engine.dart` 的 `NativeXrayEngine` 在 iOS 上自動改用 `uk.rfplay.client/vpn` channel：

- `start` → `startVpn {config}`（AppDelegate 存 App Group → 啟動 tunnel）
- `stop` → `stopVpn`
- `isRunning` → `vpnRunning`

桌面（macOS/Windows/Linux）走 `FfiXrayEngine`（`xray_ffi.dart`），FFI 載入共享庫：

- macOS：`Contents/Frameworks/libxray.dylib`（由 `scripts/build_libxray.sh macos-dylib` 生成）
- Windows：`libxray.dll`（`build_libxray.sh windows`）
- Linux：`libxray.so`（`build_libxray.sh linux`）

## 驗證

```bash
flutter analyze
flutter test
```

原生部分需在 Xcode 中 build；沒有 libXray framework/aar 時專案仍可編譯（條件編譯與 reflection 降級），運行時會回報「LIBXRAY_NOT_FOUND / 啟動失敗」。
