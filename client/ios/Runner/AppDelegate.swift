import Flutter
import UIKit
import NetworkExtension

@main
@objc class AppDelegate: FlutterAppDelegate, FlutterImplicitEngineDelegate {
  private static let appGroup = "group.com.example.rfplay-client"
  private static let configKey = "rfplay_xray_config"
  private static let vpnChannel = "uk.rfplay.client/vpn"

  override func application(
    _ application: UIApplication,
    didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
  ) -> Bool {
    return super.application(application, didFinishLaunchingWithOptions: launchOptions)
  }

  func didInitializeImplicitFlutterEngine(_ engineBridge: FlutterImplicitEngineBridge) {
    GeneratedPluginRegistrant.register(with: engineBridge.pluginRegistry)
    let registrar = engineBridge.pluginRegistry.registrar(forPlugin: "RfplayVpnController")
    registerVpnChannel(with: registrar.messenger())
  }

  private func registerVpnChannel(with messenger: FlutterBinaryMessenger) {
    let channel = FlutterMethodChannel(name: Self.vpnChannel, binaryMessenger: messenger)
    channel.setMethodCallHandler { [weak self] call, result in
      switch call.method {
      case "startVpn":
        // Flutter passes the Xray config JSON which the tunnel provider reads
        // from the shared App Group.
        guard let args = call.arguments as? [String: Any],
              let config = args["config"] as? String else {
          result(FlutterError(code: "bad_args", message: "missing config", details: nil))
          return
        }
        UserDefaults(suiteName: Self.appGroup)?.set(config, forKey: Self.configKey)
        self?.startTunnel(result: result)

      case "stopVpn":
        self?.stopTunnel(result: result)

      case "vpnRunning":
        result(self?.vpnIsActive() ?? false)

      default:
        result(FlutterMethodNotImplemented)
      }
    }
  }

  private func startTunnel(result: @escaping FlutterResult) {
    guard #available(iOS 14.0, *) else {
      removeSharedConfig()
      result(FlutterError(code: "unsupported", message: "iOS 14+ required", details: nil))
      return
    }
    NETunnelProviderManager.loadAllFromPreferences { [weak self] managers, error in
      if let error {
        self?.removeSharedConfig()
        result(FlutterError(code: "load_failed", message: error.localizedDescription, details: nil))
        return
      }
      let manager = managers?.first ?? NETunnelProviderManager()
      manager.localizedDescription = "RFPlay VPN"
      manager.protocolConfiguration = {
        let proto = NETunnelProviderProtocol()
        proto.providerBundleIdentifier = "com.example.rfplay-client.PacketTunnel"
        proto.serverAddress = "rfplay-tunnel"
        proto.disconnectOnSleep = false
        return proto
      }()
      manager.isEnabled = true
      manager.saveToPreferences { saveError in
        if let saveError {
          self?.removeSharedConfig()
          result(FlutterError(code: "save_failed", message: saveError.localizedDescription, details: nil))
          return
        }
        do {
          // The tunnel provider reads the config from the shared defaults in
          // `startTunnel` and deletes it immediately after reading, so the
          // plaintext config (with node secrets) never persists here.
          try manager.connection.startVPNTunnel()
          result(true)
        } catch {
          self?.removeSharedConfig()
          result(FlutterError(code: "start_failed", message: error.localizedDescription, details: nil))
        }
      }
    }
  }

  private func stopTunnel(result: @escaping FlutterResult) {
    NETunnelProviderManager.loadAllFromPreferences { [weak self] managers, _ in
      guard let manager = managers?.first else {
        self?.removeSharedConfig()
        result(false)
        return
      }
      manager.connection.stopVPNTunnel()
      self?.removeSharedConfig()
      result(true)
    }
  }

  /// SECURITY: remove the plaintext xray config from the shared App Group
  /// defaults. The provider already deletes it after reading in `startTunnel`;
  /// this covers failure/stop paths where the provider never read it.
  private func removeSharedConfig() {
    UserDefaults(suiteName: Self.appGroup)?.removeObject(forKey: Self.configKey)
  }

  private func vpnIsActive() -> Bool {
    guard let manager = NETunnelProviderManager.loadAllFromPreferencesSync()?.first else {
      return false
    }
    return manager.connection.status == .connected
  }
}

private extension NETunnelProviderManager {
  static func loadAllFromPreferencesSync() -> [NETunnelProviderManager]? {
    var result: [NETunnelProviderManager]?
    let semaphore = DispatchSemaphore(value: 0)
    NETunnelProviderManager.loadAllFromPreferences { managers, _ in
      result = managers
      semaphore.signal()
    }
    semaphore.wait()
    return result
  }
}
