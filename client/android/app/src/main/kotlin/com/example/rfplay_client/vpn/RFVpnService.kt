package com.example.rfplay_client.vpn

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.system.OsConstants
import androidx.core.app.NotificationCompat
import com.example.rfplay_client.MainActivity
import com.example.rfplay_client.xray.XrayBridge
import org.json.JSONObject
import java.io.FileInputStream
import java.io.FileOutputStream
import java.net.InetSocketAddress
import java.net.Socket

/**
 * RFPlay VPN Service — Android VpnService implementation.
 *
 * Creates a TUN interface and hands the file descriptor to libXray (the Go
 * XTLS/Xray-core binding) via the `env["xray.tun.fd"]` config entry. When the
 * libXray .aar is not bundled, it falls back to the legacy TCP forwarder.
 */
class RFVpnService : VpnService() {

    companion object {
        const val ACTION_CONNECT = "com.example.rfplay_client.CONNECT"
        const val ACTION_DISCONNECT = "com.example.rfplay_client.DISCONNECT"
        const val EXTRA_PROXY_HOST = "proxy_host"
        const val EXTRA_PROXY_PORT = "proxy_port"
        const val EXTRA_SESSION_NAME = "session_name"
        const val EXTRA_XRAY_CONFIG = "xray_config"

        private const val VPN_MTU = 1500
        private const val NOTIFICATION_ID = 1001
        private const val CHANNEL_ID = "rfplay_vpn"
        private const val MAX_PACKET_SIZE = 65535

        @Volatile
        private var isRunning = false

        fun isVpnRunning(): Boolean = isRunning
    }

    private var tunInterface: ParcelFileDescriptor? = null
    private var proxyHost: String = ""
    private var proxyPort: Int = 0
    private var sessionName: String = "RFPlay VPN"
    private var xrayConfig: String? = null
    private var engineStarted = false
    private var forwarderThread: Thread? = null

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_CONNECT -> {
                proxyHost = intent.getStringExtra(EXTRA_PROXY_HOST) ?: ""
                proxyPort = intent.getIntExtra(EXTRA_PROXY_PORT, 0)
                sessionName = intent.getStringExtra(EXTRA_SESSION_NAME) ?: "RFPlay VPN"
                xrayConfig = intent.getStringExtra(EXTRA_XRAY_CONFIG)
                startVPN()
            }
            ACTION_DISCONNECT -> {
                stopVPN()
            }
        }
        return START_STICKY
    }

    override fun onDestroy() {
        stopVPN()
        super.onDestroy()
    }

    override fun onRevoke() {
        stopVPN()
        super.onRevoke()
    }

    private fun startVPN() {
        // Build VPN interface
        val builder = Builder()
        builder.setSession(sessionName)
        builder.setMtu(VPN_MTU)
        builder.setBlocking(true)

        // Add all addresses — capture all traffic
        builder.addAddress("0.0.0.0", 0)

        // Add DNS
        builder.addDnsServer("8.8.8.8")
        builder.addDnsServer("1.1.1.1")

        // Route all traffic
        builder.addRoute("0.0.0.0", 0)

        // Exclude the proxy server from VPN (to avoid loop)
        if (proxyHost.isNotEmpty()) {
            try {
                val resolved = InetSocketAddress(proxyHost, proxyPort).address
                if (resolved != null) {
                    builder.addRoute(resolved.hostAddress ?: proxyHost, 32)
                }
            } catch (_: Exception) {
                // Best effort
            }
        }

        tunInterface = builder.establish()

        // Start foreground notification
        val notification = createNotification()
        startForeground(NOTIFICATION_ID, notification)

        isRunning = true

        val xrayConfigJson = xrayConfig
        if (xrayConfigJson != null && XrayBridge.isAvailable()) {
            startXrayEngine(xrayConfigJson)
        } else {
            // Fallback: legacy TCP forwarder (no libXray bundled)
            if (xrayConfigJson == null) {
                startLegacyForwarder()
            } else {
                stopVPN()
            }
        }
    }

    /**
     * Start the real libXray engine with the TUN fd injected into the config.
     * libXray (since SetTunFd was removed) reads the fd from
     * `env["xray.tun.fd"]` at the config root.
     */
    private fun startXrayEngine(configJson: String) {
        val fd = getTunFd() ?: return
        Thread {
            try {
                val config = JSONObject(configJson)
                // libXray reads XRAY_TUN_FD or xray.tun.fd from the root env object.
                val env = config.optJSONObject("env") ?: JSONObject()
                env.put("xray.tun.fd", fd)
                env.put("XRAY_TUN_FD", fd)
                config.put("env", env)

                // Remove the placeholder fd from the tun inbound settings.
                val inbounds = config.optJSONArray("inbounds")
                if (inbounds != null) {
                    for (i in 0 until inbounds.length()) {
                        val inbound = inbounds.optJSONObject(i)
                        if (inbound?.optString("protocol") == "tun") {
                            inbound.optJSONObject("settings")?.remove("fd")
                        }
                    }
                }

                val result = XrayBridge.start(this, config.toString())
                engineStarted = result == "OK" || result == "ALREADY_RUNNING"
            } catch (e: Exception) {
                stopVPN()
            }
        }.apply { isDaemon = true }.start()
    }

    /**
     * The integer value of the TUN file descriptor. VpnService exposes it as a
     * java.io.FileDescriptor; the raw int is read reflectively because libXray
     * needs the native fd number for `xray.tun.fd`.
     */
    private fun getTunFd(): Int? {
        val fd = tunInterface?.fileDescriptor ?: return null
        return try {
            val field = java.io.FileDescriptor::class.java.getDeclaredField("fd")
            field.isAccessible = true
            field.getInt(fd)
        } catch (e: Exception) {
            null
        }
    }

    private fun stopVPN() {
        isRunning = false

        if (engineStarted) {
            try {
                XrayBridge.stop()
            } catch (_: Exception) {}
            engineStarted = false
        }

        forwarderThread?.interrupt()
        forwarderThread = null

        try {
            tunInterface?.close()
        } catch (_: Exception) {}
        tunInterface = null

        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    // ------------------------------------------------------------------
    // Legacy TCP forwarder (used only when libXray .aar is not bundled).
    // Kept as a dev fallback; production builds use the Xray engine above.
    // ------------------------------------------------------------------

    private fun startLegacyForwarder() {
        if (proxyHost.isEmpty() || proxyPort <= 0) {
            stopVPN()
            return
        }
        forwarderThread = Thread(null, ::forwardLoop, "RFVpnForwarder")
        forwarderThread?.start()
    }

    private fun forwardLoop() {
        val tunInput = tunInterface?.let { FileInputStream(it.fileDescriptor) } ?: return
        val tunOutput = tunInterface?.let { FileOutputStream(it.fileDescriptor) } ?: return

        val buffer = ByteArray(MAX_PACKET_SIZE)
        try {
            while (isRunning) {
                val length = tunInput.read(buffer)
                if (length <= 0) continue

                val ipVersion = (buffer[0].toInt() shr 4) and 0x0F
                if (ipVersion != 4) continue

                val protocol = buffer[9].toInt() and 0xFF
                if (protocol != OsConstants.IPPROTO_TCP) continue

                val ihl = (buffer[0].toInt() and 0x0F) * 4
                if (ihl < 20 || ihl > length) continue

                val flags = buffer[ihl + 13].toInt() and 0xFF
                val isSyn = (flags and 0x02) != 0
                val isFin = (flags and 0x01) != 0

                if (isSyn && !isFin) {
                    relayTCP(buffer, length, ihl, tunOutput)
                }
            }
        } catch (_: Exception) {
            // Socket closed or thread interrupted on stop
        } finally {
            try { tunInput.close() } catch (_: Exception) {}
            try { tunOutput.close() } catch (_: Exception) {}
        }
    }

    private fun relayTCP(packet: ByteArray, length: Int, ipHeaderLen: Int, tunOutput: FileOutputStream) {
        Thread {
            try {
                val proxySocket = Socket()
                proxySocket.connect(InetSocketAddress(proxyHost, proxyPort), 5000)
                proxySocket.soTimeout = 30000

                val outputStream = proxySocket.getOutputStream()
                val inputStream = proxySocket.getInputStream()

                val relayBuffer = ByteArray(4096)
                val readThread = Thread {
                    try {
                        while (isRunning && !proxySocket.isClosed) {
                            val n = inputStream.read(relayBuffer)
                            if (n <= 0) break
                            tunOutput.write(relayBuffer, 0, n)
                            tunOutput.flush()
                        }
                    } catch (_: Exception) {}
                }
                readThread.start()

                val tcpPayloadLen = length - ipHeaderLen
                if (tcpPayloadLen > 0) {
                    outputStream.write(packet, ipHeaderLen, tcpPayloadLen)
                    outputStream.flush()
                }

                readThread.join(5000)
                proxySocket.close()
            } catch (_: Exception) {
                // Connection failed or timeout
            }
        }.apply { isDaemon = true }.start()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "RFPlay VPN",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "VPN 连接通知"
            }
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(channel)
        }
    }

    private fun createNotification(): Notification {
        val disconnectIntent = Intent(this, RFVpnService::class.java).apply {
            action = ACTION_DISCONNECT
        }
        val disconnectPendingIntent = PendingIntent.getService(
            this, 0, disconnectIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        val openIntent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP
        }
        val openPendingIntent = PendingIntent.getActivity(
            this, 0, openIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("RFPlay VPN")
            .setContentText("$sessionName — 已连接")
            .setSmallIcon(android.R.drawable.ic_lock_idle_lock)
            .setContentIntent(openPendingIntent)
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "断开", disconnectPendingIntent)
            .setOngoing(true)
            .setSilent(true)
            .build()
    }
}
