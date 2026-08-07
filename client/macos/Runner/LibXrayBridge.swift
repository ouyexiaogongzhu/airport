import Foundation
import NetworkExtension
#if canImport(LibXray)
import LibXray
#endif

/// macOS bridge to libXray.
///
/// Uses the same LibXray.xcframework module as iOS. When absent, calls are
/// no-ops so the app still compiles.
final class LibXrayBridge {
    static let shared = LibXrayBridge()

    private var packetFlow: NEPacketTunnelFlow?
    private var running = false

    private var available: Bool {
#if canImport(LibXray)
        return true
#else
        return false
#endif
    }

    func start(configJson: String) -> Bool {
        guard available else { return false }
        running = true
        return invoke("runXrayFromJson", configJson: configJson)
    }

    func stop() {
        guard available else { return }
        _ = invoke("stopXray", configJson: "")
        running = false
    }

    func isRunning() -> Bool {
        guard available else { return false }
        return running
    }

    func stats() -> (upload: Int64, download: Int64) {
        guard available else { return (0, 0) }
        let state = invoke("getXrayState", configJson: "")
        guard let data = parseState(state) else { return (0, 0) }
        let up = data["upload"] as? NSNumber ?? data["uplink"] as? NSNumber ?? 0
        let down = data["download"] as? NSNumber ?? data["downlink"] as? NSNumber ?? 0
        return (up.int64Value, down.int64Value)
    }

    func setPacketFlow(_ flow: NEPacketTunnelFlow) {
        packetFlow = flow
    }

    private func invoke(_ method: String, configJson: String) -> Bool {
#if canImport(LibXray)
        let request: [String: Any] = [
            "apiVersion": 1,
            "method": method,
            "payload": [
                "dat_dir": FileManager.default.temporaryDirectory.path,
                "mph_cache_path": "",
                "config_json": configJson,
            ],
        ]
        guard let data = try? JSONSerialization.data(withJSONObject: request),
              let requestJSON = String(data: data, encoding: .utf8) else {
            return false
        }
        let response = LibXray.Invoke(requestJSON: requestJSON)
        return response.contains("\"success\":true")
#else
        return false
#endif
    }

    private func parseState(_ state: String) -> [String: Any]? {
        guard let data = state.data(using: .utf8),
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            return nil
        }
        return json["data"] as? [String: Any] ?? json
    }
}
