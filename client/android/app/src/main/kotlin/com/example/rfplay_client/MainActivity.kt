package com.example.rfplay_client

import android.content.Intent
import android.net.VpnService
import android.os.Build
import io.flutter.embedding.android.FlutterActivity
import io.flutter.embedding.engine.FlutterEngine
import io.flutter.plugin.common.MethodChannel
import com.example.rfplay_client.vpn.RFVpnService

class MainActivity : FlutterActivity() {
    private val CHANNEL = "com.example.rfplay_client/vpn"
    private val VPN_REQUEST_CODE = 9001

    override fun configureFlutterEngine(flutterEngine: FlutterEngine) {
        super.configureFlutterEngine(flutterEngine)

        MethodChannel(flutterEngine.dartExecutor.binaryMessenger, CHANNEL).setMethodCallHandler { call, result ->
            when (call.method) {
                "startVpn" -> {
                    val host = call.argument<String>("host") ?: ""
                    val port = call.argument<Int>("port") ?: 0
                    val name = call.argument<String>("name") ?: "RFPlay VPN"

                    if (host.isEmpty() || port <= 0) {
                        result.error("INVALID_ARGS", "host or port missing", null)
                        return@setMethodCallHandler
                    }

                    // Request VPN permission
                    val intent = VpnService.prepare(this@MainActivity)
                    if (intent != null) {
                        // Need to ask user for permission
                        startActivityForResult(intent, VPN_REQUEST_CODE)
                        // Store params for after permission grant
                        pendingVpnConfig = VpnConfig(host, port, name)
                        result.success("PERMISSION_REQUIRED")
                    } else {
                        // Permission already granted
                        startVpnService(host, port, name)
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
    }

    private data class VpnConfig(val host: String, val port: Int, val name: String)
    private var pendingVpnConfig: VpnConfig? = null

    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode == VPN_REQUEST_CODE) {
            if (resultCode == RESULT_OK) {
                val config = pendingVpnConfig
                if (config != null) {
                    startVpnService(config.host, config.port, config.name)
                    pendingVpnConfig = null
                }
            }
        }
    }

    private fun startVpnService(host: String, port: Int, name: String) {
        val intent = Intent(this@MainActivity, RFVpnService::class.java).apply {
            action = RFVpnService.ACTION_CONNECT
            putExtra(RFVpnService.EXTRA_PROXY_HOST, host)
            putExtra(RFVpnService.EXTRA_PROXY_PORT, port)
            putExtra(RFVpnService.EXTRA_SESSION_NAME, name)
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }
}
