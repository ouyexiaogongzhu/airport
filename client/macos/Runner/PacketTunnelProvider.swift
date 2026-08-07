import Foundation
import NetworkExtension

/// macOS Packet Tunnel Provider for RFPlay.
///
/// macOS creates the utun interface via NetworkExtension (or, when running as
/// a helper with root, via /dev/utun directly). The fd is injected into the
/// libXray config `env` object as `xray.tun.fd` before `runXrayFromJson`.
///
/// Entitlements required (see macos/Runner/*.entitlements):
///   com.apple.security.network.server
///   com.apple.security.network.client
///   com.apple.security.application-groups (app group shared with the app)
class PacketTunnelProvider: NEPacketTunnelProvider {

    private let appGroup = "group.com.example.rfplay-client"

    override func startTunnel(options: [String: NSObject]?, completionHandler: @escaping (Error?) -> Void) {
        let configJson = readSharedConfig()
        guard !configJson.isEmpty else {
            completionHandler(NSError(
                domain: "RFPlayTunnel",
                code: 1,
                userInfo: [NSLocalizedDescriptionKey: "No xray config found in shared container"]
            ))
            return
        }

        let settings = NEPacketTunnelNetworkSettings(tunnelRemoteAddress: "10.9.0.1")
        settings.mtu = 1500
        settings.ipv4Settings = NEIPv4Settings(addresses: ["10.9.0.2"], subnetMasks: ["255.255.255.255"])
        settings.ipv4Settings?.includedRoutes = [NEIPv4Route.default()]
        settings.dnsSettings = NEDNSSettings(servers: ["8.8.8.8", "1.1.1.1"])

        setTunnelNetworkSettings(settings) { [weak self] error in
            guard let self else {
                completionHandler(error)
                return
            }
            if let error {
                completionHandler(error)
                return
            }

            guard let packetFlow = self.packetFlow as? NEPacketTunnelFlow else {
                completionHandler(NSError(domain: "RFPlayTunnel", code: 2,
                    userInfo: [NSLocalizedDescriptionKey: "packet flow unavailable"]))
                return
            }

            guard self.startXray(configJson: configJson) else {
                completionHandler(NSError(domain: "RFPlayTunnel", code: 3,
                    userInfo: [NSLocalizedDescriptionKey: "failed to start xray core"]))
                return
            }

            LibXrayBridge.shared.setPacketFlow(packetFlow)
            completionHandler(nil)
        }
    }

    override func stopTunnel(with reason: NEProviderStopReason, completionHandler: @escaping () -> Void) {
        UserDefaults(suiteName: appGroup)?.removeObject(forKey: "rfplay_xray_config")
        LibXrayBridge.shared.stop()
        completionHandler()
    }

    private func startXray(configJson: String) -> Bool {
        LibXrayBridge.shared.start(configJson: configJson)
    }

    /// Reads the shared xray config from the app group container.
    /// SECURITY: the config (with node VLESS UUID / SS / Trojan passwords) is
    /// removed from the shared defaults immediately after reading, so it never
    /// persists in plaintext past tunnel startup.
    private func readSharedConfig() -> String {
        guard let defaults = UserDefaults(suiteName: appGroup) else { return "" }
        let config = defaults.string(forKey: "rfplay_xray_config") ?? ""
        defaults.removeObject(forKey: "rfplay_xray_config")
        return config
    }
}
