package com.example.rfplay_client.xray

import android.content.Context
import android.util.Log

/**
 * Thin bridge between the Flutter MethodChannel and the libXray Go binding.
 *
 * libXray (https://github.com/XTLS/libXray) is compiled with gomobile into an
 * .aar whose Java package is `libxray`. Build it with:
 *
 *   git clone https://github.com/XTLS/libXray
 *   cd libXray
 *   python3 build/main.py android
 *   # copy android/libXray.aar into android/app/libs/
 *
 * The generated class exposes `Xray.runXrayFromJson(base64Text)` where the
 * base64 payload is a JSON RunXrayFromJSONRequest { dat_dir, mph_cache_path,
 * config_json }. It returns a base64-encoded CallResponse.
 *
 * All calls here are made on a background thread so the UI thread is never
 * blocked by Go bridge calls.
 */
object XrayBridge {
    private const val TAG = "RFXrayBridge"

    private var loaded = false
    private var running = false

    /** @return true when the libxray class was found on the classpath. */
    fun isAvailable(): Boolean {
        return try {
            Class.forName("libxray.Xray")
            true
        } catch (e: Throwable) {
            false
        }
    }

    /**
     * Start the Xray engine with a standard config JSON. The config must
     * already contain the TUN fd in `env["xray.tun.fd"]` (libXray no longer
     * exposes SetTunFd; the fd is read from the root env object).
     */
    @Synchronized
    fun start(context: Context, configJson: String): String {
        if (!isAvailable()) {
            return "LIBXRAY_NOT_FOUND"
        }
        if (running) {
            return "ALREADY_RUNNING"
        }
        return try {
            val req = mapOf(
                "dat_dir" to context.filesDir.absolutePath,
                "mph_cache_path" to "",
                "config_json" to configJson,
            )
            val payload = encodeBase64(org.json.JSONObject(req).toString())
            val response = invokeStatic("runXrayFromJson", payload)
            running = true
            decodeResponse(response)
        } catch (e: Throwable) {
            Log.e(TAG, "start failed", e)
            "ERROR: ${e.message}"
        }
    }

    @Synchronized
    fun stop(): String {
        if (!isAvailable()) return "LIBXRAY_NOT_FOUND"
        return try {
            val response = invokeStatic("stopXray", "")
            running = false
            decodeResponse(response)
        } catch (e: Throwable) {
            Log.e(TAG, "stop failed", e)
            "ERROR: ${e.message}"
        }
    }

    @Synchronized
    fun isRunning(): Boolean = running && isAvailable()

    /** Poll cumulative traffic counters from the engine state. */
    fun stats(): Pair<Long, Long> {
        if (!isAvailable() || !running) return 0L to 0L
        return try {
            val state = invokeStatic("getXrayState", "")
            parseTraffic(state)
        } catch (e: Throwable) {
            Log.e(TAG, "stats failed", e)
            0L to 0L
        }
    }

    /**
     * Invoke a static method on the libxray.Xray class reflectively so the
     * app compiles even before the .aar is dropped into app/libs.
     */
    private fun invokeStatic(method: String, arg: String): String {
        val clazz = Class.forName("libxray.Xray")
        val m = clazz.getMethod(method, String::class.java)
        return m.invoke(null, arg) as String
    }

    private fun parseTraffic(base64State: String): Pair<Long, Long> {
        return try {
            val json = org.json.JSONObject(decodeBase64(base64State))
            val uplink = json.optJSONArray("uplink") ?: return 0L to 0L
            var up = 0L
            var down = 0L
            for (i in 0 until uplink.length()) {
                up += uplink.optJSONObject(i)?.optLong("value") ?: 0L
            }
            val downlink = json.optJSONArray("downlink") ?: json.optJSONArray("downlink_speed") ?: org.json.JSONArray()
            for (i in 0 until downlink.length()) {
                down += downlink.optJSONObject(i)?.optLong("value") ?: 0L
            }
            up to down
        } catch (e: Throwable) {
            0L to 0L
        }
    }

    private fun encodeBase64(s: String): String =
        android.util.Base64.encodeToString(s.toByteArray(Charsets.UTF_8), android.util.Base64.NO_WRAP)

    private fun decodeBase64(s: String): String =
        String(android.util.Base64.decode(s, android.util.Base64.NO_WRAP), Charsets.UTF_8)

    private fun decodeResponse(base64Response: String): String {
        return try {
            val json = org.json.JSONObject(decodeBase64(base64Response))
            if (json.optBoolean("success", false)) "OK" else "ERROR: ${json.optString("message")}"
        } catch (e: Throwable) {
            "OK"
        }
    }
}
