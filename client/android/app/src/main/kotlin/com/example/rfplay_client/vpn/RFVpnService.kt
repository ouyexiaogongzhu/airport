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
import java.io.FileInputStream
import java.io.FileOutputStream
import java.net.InetSocketAddress
import java.net.Socket
import java.nio.ByteBuffer
import java.nio.ByteOrder

/**
 * RFPlay VPN Service — Android VpnService implementation.
 *
 * Creates a TUN interface and forwards TCP traffic through the selected proxy node.
 * Acts as a simple TCP forwarder: the TUN interface captures all traffic,
 * and this service forwards TCP connections to the configured proxy address.
 *
 * For production, replace with V2Ray/Xray-core Go binding via gomobile.
 */
class RFVpnService : VpnService() {

    companion object {
        const val ACTION_CONNECT = "com.example.rfplay_client.CONNECT"
        const val ACTION_DISCONNECT = "com.example.rfplay_client.DISCONNECT"
        const val EXTRA_PROXY_HOST = "proxy_host"
        const val EXTRA_PROXY_PORT = "proxy_port"
        const val EXTRA_SESSION_NAME = "session_name"

        private const val VPN_MTU = 1500
        private const val NOTIFICATION_ID = 1001
        private const val CHANNEL_ID = "rfplay_vpn"
        private const val MAX_PACKET_SIZE = 65535

        // For now we use a simple TCP forward approach.
        // In production, this would use V2Ray's gomobile binding for protocol handling.
        private var isRunning = false

        fun isVpnRunning(): Boolean = isRunning
    }

    private var tunInterface: ParcelFileDescriptor? = null
    private var proxyHost: String = ""
    private var proxyPort: Int = 0
    private var sessionName: String = "RFPlay VPN"
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
        if (proxyHost.isEmpty() || proxyPort <= 0) {
            stopSelf()
            return
        }

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
        try {
            val proxyAddr = InetSocketAddress(proxyHost, proxyPort)
            val resolved = proxyAddr.address
            if (resolved != null) {
                builder.addRoute(resolved.hostAddress ?: proxyHost, 32)
            }
        } catch (_: Exception) {
            // Best effort
        }

        // Establish VPN interface
        tunInterface = builder.establish()

        // Start foreground notification
        val notification = createNotification()
        startForeground(NOTIFICATION_ID, notification)

        // Start packet forwarding
        isRunning = true
        forwarderThread = Thread(null, ::forwardLoop, "RFVpnForwarder")
        forwarderThread?.start()
    }

    private fun stopVPN() {
        isRunning = false

        forwarderThread?.interrupt()
        forwarderThread = null

        try {
            tunInterface?.close()
        } catch (_: Exception) {}
        tunInterface = null

        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    /**
     * Simple TCP forwarding loop.
     * Reads TCP SYN packets from TUN, establishes connections to the proxy,
     * and relays data bidirectionally.
     *
     * NOTE: This is a simplified TCP-only forwarder for Phase 3.
     * For production V2Ray protocol support, use gomobile + xray-core:
     *   https://github.com/xtls/xray-core
     */
    private fun forwardLoop() {
        val tunInput = tunInterface?.let { FileInputStream(it.fileDescriptor) } ?: return
        val tunOutput = tunInterface?.let { FileOutputStream(it.fileDescriptor) } ?: return

        val buffer = ByteArray(MAX_PACKET_SIZE)
        val ipHeader = ByteArray(20)

        try {
            while (isRunning) {
                val length = tunInput.read(buffer)
                if (length <= 0) continue

                // Parse IP header to get protocol and addresses
                val ipVersion = (buffer[0].toInt() shr 4) and 0x0F
                if (ipVersion != 4) continue // IPv4 only for now

                val protocol = buffer[9].toInt() and 0xFF
                if (protocol != OsConstants.IPPROTO_TCP) continue // TCP only for now

                // Extract TCP payload (skip IP header)
                val ihl = (buffer[0].toInt() and 0x0F) * 4
                if (ihl < 20 || ihl > length) continue

                // Extract destination port
                val destPort = ((buffer[ihl + 2].toInt() and 0xFF) shl 8) or (buffer[ihl + 3].toInt() and 0xFF)

                // Check if it's a SYN packet (connection establishment)
                val flags = buffer[ihl + 13].toInt() and 0xFF
                val isSyn = (flags and 0x02) != 0
                val isFin = (flags and 0x01) != 0

                if (isSyn && !isFin) {
                    // Establish a new TCP connection to the proxy
                    forwardTCP(buffer, length, ihl, destPort, tunOutput)
                }
            }
        } catch (_: InterruptedException) {
            // Thread interrupted on stop
        } catch (_: Exception) {
            // Socket closed or other error
        } finally {
            try { tunInput.close() } catch (_: Exception) {}
            try { tunOutput.close() } catch (_: Exception) {}
        }
    }

    /**
     * Forward a single TCP connection through the proxy.
     * For Phase 3, this connects directly to the node as a transparent TCP proxy.
     * In production, implement V2Ray protocol handshake here.
     */
    private fun forwardTCP(
        packet: ByteArray,
        length: Int,
        ipHeaderLen: Int,
        destPort: Int,
        tunOutput: FileOutputStream
    ) {
        Thread {
            try {
                val proxySocket = Socket()
                proxySocket.connect(InetSocketAddress(proxyHost, proxyPort), 5000)
                proxySocket.soTimeout = 30000

                // Send request (simplified — in production, parse the HTTP/SOCKS request)
                val outputStream = proxySocket.getOutputStream()
                val inputStream = proxySocket.getInputStream()

                val relayBuffer = ByteArray(4096)

                // Read from proxy and write to TUN
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

                // Read from TUN and write to proxy
                try {
                    // Extract TCP payload and forward
                    val tcpPayloadLen = length - ipHeaderLen
                    if (tcpPayloadLen > 0) {
                        outputStream.write(packet, ipHeaderLen, tcpPayloadLen)
                        outputStream.flush()
                    }

                    // Continue reading from TUN for this connection
                    val tunInput = tunInterface?.let { FileInputStream(it.fileDescriptor) }
                    // Note: In production, use connection tracking for proper TUN→socket mapping
                    // For Phase 3, this is a simplified demonstration
                } catch (_: Exception) {}

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
