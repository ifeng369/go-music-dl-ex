package com.musicdl.shell;

import android.util.Log;

import org.json.JSONObject;

import java.io.BufferedInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.InetAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.net.URL;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.Locale;
import java.util.Map;

/**
 * Loopback HTTP proxy used by the WebView.
 *
 * <p>Everything is forwarded to the configured server, except covers and audio:
 * those are resolved through {@code /api/resolve} and then fetched straight from
 * the CDN with the referer/user-agent the server tells us to use. That keeps the
 * media bytes off the tunnel, which is what made remote playback slow.</p>
 */
final class LocalProxy {

    private static final String TAG = "MusicDLProxy";
    private static final int BUFFER = 32 * 1024;

    private final String base;
    private ServerSocket server;
    private Thread acceptThread;
    private volatile boolean running;

    LocalProxy(String baseUrl) {
        String value = baseUrl == null ? "" : baseUrl.trim();
        while (value.endsWith("/")) {
            value = value.substring(0, value.length() - 1);
        }
        this.base = value;
    }

    String localUrl() {
        return "http://127.0.0.1:" + server.getLocalPort() + "/";
    }

    void start() throws IOException {
        server = new ServerSocket(0, 64, InetAddress.getByName("127.0.0.1"));
        running = true;
        acceptThread = new Thread(new Runnable() {
            @Override
            public void run() {
                acceptLoop();
            }
        }, "musicdl-proxy-accept");
        acceptThread.setDaemon(true);
        acceptThread.start();
    }

    void stop() {
        running = false;
        try {
            if (server != null) {
                server.close();
            }
        } catch (Exception ignored) {
            // Closing the socket is best effort.
        }
        server = null;
    }

    private void acceptLoop() {
        while (running) {
            final Socket socket;
            try {
                socket = server.accept();
            } catch (Exception err) {
                if (running) {
                    Log.w(TAG, "accept failed", err);
                }
                return;
            }
            Thread worker = new Thread(new Runnable() {
                @Override
                public void run() {
                    handle(socket);
                }
            }, "musicdl-proxy-conn");
            worker.setDaemon(true);
            worker.start();
        }
    }

    private void handle(Socket socket) {
        try {
            socket.setSoTimeout(30000);
            InputStream in = new BufferedInputStream(socket.getInputStream());
            OutputStream out = socket.getOutputStream();

            String requestLine = readLine(in);
            if (requestLine == null || requestLine.isEmpty()) {
                socket.close();
                return;
            }
            String[] parts = requestLine.split(" ");
            if (parts.length < 2) {
                socket.close();
                return;
            }
            String method = parts[0].toUpperCase(Locale.US);
            String path = parts[1];

            Map<String, String> headers = new LinkedHashMap<>();
            String line;
            while ((line = readLine(in)) != null && !line.isEmpty()) {
                int colon = line.indexOf(':');
                if (colon > 0) {
                    headers.put(line.substring(0, colon).trim(), line.substring(colon + 1).trim());
                }
            }
            int length = 0;
            try {
                length = Integer.parseInt(headers.containsKey("Content-Length") ? headers.get("Content-Length") : "0");
            } catch (Exception ignored) {
                length = 0;
            }
            byte[] body = new byte[length];
            int read = 0;
            while (read < length) {
                int got = in.read(body, read, length - read);
                if (got < 0) {
                    break;
                }
                read += got;
            }

            boolean isCover = path.startsWith("/api/cover");
            boolean isStream = path.startsWith("/api/stream");
            if ((isCover || isStream) && direct(method, path, headers, out)) {
                socket.close();
                return;
            }
            forward(method, base + path, headers, body, out);
            socket.close();
        } catch (Exception err) {
            Log.w(TAG, "connection failed", err);
            try {
                socket.close();
            } catch (Exception ignored) {
                // Nothing else to do.
            }
        }
    }

    /** Resolves the media URL and streams it from the CDN. */
    private boolean direct(String method, String path, Map<String, String> headers, OutputStream out) {
        HttpURLConnection resolve = null;
        HttpURLConnection media = null;
        try {
            String kind = path.startsWith("/api/cover") ? "cover" : "audio";
            String query = path.contains("?") ? path.substring(path.indexOf('?') + 1) : "";
            String resolveUrl = base + "/api/resolve?kind=" + kind + (query.isEmpty() ? "" : "&" + query);

            resolve = (HttpURLConnection) new URL(resolveUrl).openConnection();
            resolve.setRequestMethod("GET");
            resolve.setConnectTimeout(10000);
            resolve.setReadTimeout(15000);
            resolve.setRequestProperty("Accept", "application/json");
            copyCookie(headers, resolve);
            if (resolve.getResponseCode() != 200) {
                return false;
            }
            String payload = readAll(resolve.getInputStream());
            JSONObject json = new JSONObject(payload);
            String mediaUrl = json.optString("url", "");
            if (mediaUrl.isEmpty()) {
                return false;
            }

            media = (HttpURLConnection) new URL(mediaUrl).openConnection();
            media.setRequestMethod(method.equals("HEAD") ? "HEAD" : "GET");
            media.setConnectTimeout(10000);
            media.setReadTimeout(30000);
            media.setInstanceFollowRedirects(true);
            media.setRequestProperty("Accept-Encoding", "identity");
            JSONObject extra = json.optJSONObject("headers");
            if (extra != null) {
                java.util.Iterator<String> keys = extra.keys();
                while (keys.hasNext()) {
                    String key = keys.next();
                    try {
                        media.setRequestProperty(key, extra.getString(key));
                    } catch (Exception ignored) {
                        // Skip a malformed header instead of failing the request.
                    }
                }
            }
            String range = headers.get("Range");
            if (range != null && !range.isEmpty()) {
                media.setRequestProperty("Range", range);
            }

            int status = media.getResponseCode();
            if (status >= 400) {
                return false;
            }
            StringBuilder head = new StringBuilder();
            head.append("HTTP/1.1 ").append(status).append(status == 206 ? " Partial Content" : " OK").append("\r\n");
            appendHeader(head, "Content-Type", media.getHeaderField("Content-Type"));
            appendHeader(head, "Content-Length", media.getHeaderField("Content-Length"));
            appendHeader(head, "Content-Range", media.getHeaderField("Content-Range"));
            appendHeader(head, "Accept-Ranges", media.getHeaderField("Accept-Ranges"));
            appendHeader(head, "Cache-Control", media.getHeaderField("Cache-Control"));
            appendHeader(head, "ETag", media.getHeaderField("ETag"));
            head.append("Connection: close\r\n\r\n");
            out.write(head.toString().getBytes("UTF-8"));
            out.flush();

            if (!"HEAD".equals(method)) {
                InputStream stream = media.getInputStream();
                byte[] buffer = new byte[BUFFER];
                int got;
                while ((got = stream.read(buffer)) > 0) {
                    out.write(buffer, 0, got);
                    out.flush();
                }
                stream.close();
            }
            return true;
        } catch (Exception err) {
            Log.w(TAG, "direct fetch failed, falling back", err);
            return false;
        } finally {
            if (media != null) {
                media.disconnect();
            }
            if (resolve != null) {
                resolve.disconnect();
            }
        }
    }

    /** Plain reverse proxy for everything else (HTML, JS, JSON APIs). */
    private void forward(String method, String target, Map<String, String> headers, byte[] body, OutputStream out) {
        HttpURLConnection conn = null;
        try {
            conn = (HttpURLConnection) new URL(target).openConnection();
            conn.setRequestMethod(method);
            conn.setConnectTimeout(15000);
            conn.setReadTimeout(60000);
            conn.setInstanceFollowRedirects(false);
            conn.setDoInput(true);
            for (Map.Entry<String, String> entry : headers.entrySet()) {
                String key = entry.getKey();
                String lower = key.toLowerCase(Locale.US);
                if (lower.equals("host") || lower.equals("connection") || lower.equals("keep-alive")
                        || lower.equals("accept-encoding") || lower.equals("content-length")
                        || lower.startsWith("proxy-")) {
                    continue;
                }
                conn.setRequestProperty(key, entry.getValue());
            }
            if (body.length > 0) {
                conn.setDoOutput(true);
                conn.setFixedLengthStreamingMode(body.length);
                OutputStream bodyOut = conn.getOutputStream();
                bodyOut.write(body);
                bodyOut.flush();
                bodyOut.close();
            }
            int status = conn.getResponseCode();
            StringBuilder head = new StringBuilder();
            head.append("HTTP/1.1 ").append(status).append("\r\n");
            Map<String, java.util.List<String>> fields = conn.getHeaderFields();
            for (Map.Entry<String, java.util.List<String>> entry : fields.entrySet()) {
                String key = entry.getKey();
                if (key == null) {
                    continue;
                }
                String lower = key.toLowerCase(Locale.US);
                if (lower.equals("transfer-encoding") || lower.equals("connection")
                        || lower.equals("keep-alive") || lower.equals("content-encoding")) {
                    continue;
                }
                for (String value : entry.getValue()) {
                    String clean = lower.equals("set-cookie") ? stripDomain(value) : value;
                    head.append(key).append(": ").append(clean).append("\r\n");
                }
            }
            head.append("Connection: close\r\n\r\n");
            out.write(head.toString().getBytes("UTF-8"));

            InputStream stream = status >= 400 ? conn.getErrorStream() : conn.getInputStream();
            if (stream != null && !"HEAD".equals(method)) {
                byte[] buffer = new byte[BUFFER];
                int got;
                while ((got = stream.read(buffer)) > 0) {
                    out.write(buffer, 0, got);
                }
                stream.close();
            }
            out.flush();
        } catch (Exception err) {
            Log.w(TAG, "forward failed", err);
            try {
                out.write("HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\n".getBytes("UTF-8"));
            } catch (Exception ignored) {
                // Client already gone.
            }
        } finally {
            if (conn != null) {
                conn.disconnect();
            }
        }
    }

    private void copyCookie(Map<String, String> headers, HttpURLConnection conn) {
        String cookie = headers.get("Cookie");
        if (cookie != null && !cookie.isEmpty()) {
            conn.setRequestProperty("Cookie", cookie);
        }
        String auth = headers.get("Authorization");
        if (auth != null && !auth.isEmpty()) {
            conn.setRequestProperty("Authorization", auth);
        }
    }

    private static String stripDomain(String cookie) {
        String[] parts = cookie.split(";");
        StringBuilder out = new StringBuilder();
        for (String part : parts) {
            String piece = part.trim();
            if (piece.toLowerCase(Locale.US).startsWith("domain=")) {
                continue;
            }
            if (out.length() > 0) {
                out.append("; ");
            }
            out.append(piece);
        }
        return out.toString();
    }

    private static void appendHeader(StringBuilder head, String name, String value) {
        if (value != null && !value.isEmpty()) {
            head.append(name).append(": ").append(value).append("\r\n");
        }
    }

    private static String readLine(InputStream in) throws IOException {
        ByteArrayOutputStream buffer = new ByteArrayOutputStream();
        int ch;
        while ((ch = in.read()) != -1) {
            if (ch == '\n') {
                break;
            }
            if (ch != '\r') {
                buffer.write(ch);
            }
            if (buffer.size() > 8192) {
                break;
            }
        }
        if (ch == -1 && buffer.size() == 0) {
            return null;
        }
        return new String(buffer.toByteArray(), "UTF-8");
    }

    private static String readAll(InputStream in) throws IOException {
        ByteArrayOutputStream buffer = new ByteArrayOutputStream();
        byte[] chunk = new byte[4096];
        int got;
        while ((got = in.read(chunk)) > 0) {
            buffer.write(chunk, 0, got);
            if (buffer.size() > 262144) {
                break;
            }
        }
        return new String(buffer.toByteArray(), "UTF-8");
    }
}
