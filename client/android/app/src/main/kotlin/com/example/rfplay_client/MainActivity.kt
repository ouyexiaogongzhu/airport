package com.example.rfplay_client

import android.content.Intent
import android.net.VpnService
import android.os.Build
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import com.example.rfplay_client.vpn.RFVpnService
import com.example.rfplay_client.xray.XrayBridge

class MainActivity : FlutterActivity() {
    private val VPN_CHANNEL = "uk.rfplay.client/vpn"
    private val XRAY_CHANNEL = "uk.rfplay.client/xray"
    private val VPN_REQUEST_CODE = 9001

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, VPN_CHANNEL).setMethodCallHandler { call, result ->
            when (call.method) {
                "startVpn" -> {
                    val host = call.argument<String>("host") ?: ""
                    val port = call.argument<Int>("port") ?: 0
                    val name = call.argument<String>("name") ?: "RFPlay VPN"
                    val config = call.argument<String>("config")

                    if (host.isEmpty() && config == null) {
                        result.error("INVALID_ARGS", "host/port or config required", null)
                        return@setMethodCallHandler
                    }

                    // Request VPN permission
                    val intent = VpnService.prepare(this@MainActivity)
                    if (intent != null) {
                        startActivityForResult(intent, VPN_REQUEST_CODE)
                        pendingVpnConfig = VpnConfig(host, port, name, config)
                        result.success("PERMISSION_REQUIRED")
                    } else {
                        startVpnService(host, port, name, config)
                        result.success("VPN_STARTED")
                    }
                }

                "stopVpn" -> {
                    val stopIntent = Intent(this@MainActivity, RFVpnService::class.java).apply {
                        action = RFVpnService.ACTION_DISCONNECT
                    }
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                        startForegroundService(stopIntent)
                    } else {
                        startService(stopIntent)
                    }
                    result.success("VPN_STOPPED")
                }

                "isVpnRunning" -> {
                    result.success(RFVpnService.isVpnRunning())
                }

                "checkVpnPermission" -> {
                    val intent = VpnService.prepare(this@MainActivity)
                    result.success(intent == null)
                }

                else -> result.notImplemented()
            }
        }

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, XRAY_CHANNEL).setMethodCallHandler { call, result ->
            when (call.method) {
                "xrayStart" -> {
                    val config = call.argument<String>("config")
                    if (config == null) {
                        result.error("INVALID_ARGS", "config required", null)
                        return@setMethodCallHandler
                    }
                    val vpnIntent = VpnService.prepare(this@MainActivity)
                    if (vpnIntent != null) {
                        startActivityForResult(vpnIntent, VPN_REQUEST_CODE)
                        pendingVpnConfig = VpnConfig("", 0, "RFPlay VPN", config)
                        result.success("PERMISSION_REQUIRED")
                    } else {
                        startVpnService("", 0, "RFPlay VPN", config)
                        result.success("VPN_STARTED")
                    }
                }

                "xrayStop" -> {
                    val stopIntent = Intent(this@MainActivity, RFVpnService::class.java).apply {
                        action = RFVpnService.ACTION_DISCONNECT
                    }
                    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                        startForegroundService(stopIntent)
                    } else {
                        startService(stopIntent)
                    }
                    result.success("VPN_STOPPED")
                }

                "xrayRunning" -> {
                    result.success(RFVpnService.isVpnRunning())
                }

                "xrayStats" -> {
                    val (up, down) = XrayBridge.stats()
                    result.success(mapOf("upload" to up, "download" to down))
                }

                else -> result.notImplemented()
            }
        }
    }

    private data class VpnConfig(val host: String, val port: Int, val name: String, val config: String?)
    private var pendingVpnConfig: VpnConfig? = null

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VPN_REQUEST_CODE) {
            if (resultCode == RESULT_OK) {
                val config = pendingVpnConfig
                if (config != null) {
                    startVpnService(config.host, config.port, config.name, config.config)
                    pendingVpnConfig = null
                }
            }
        }
    }

    private fun startVpnService(host: String, port: Int, name: String, config: String?) {
        val intent = Intent(this@MainActivity, RFVpnService::class.java).apply {
            action = RFVpnService.ACTION_CONNECT
            putExtra(RFVpnService.EXTRA_PROXY_HOST, host)
            putExtra(RFVpnService.EXTRA_PROXY_PORT, port)
            putExtra(RFVpnService.EXTRA_SESSION_NAME, name)
            if (config != null) {
                putExtra(RFVpnService.EXTRA_XRAY_CONFIG, config)
            }
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }
}
