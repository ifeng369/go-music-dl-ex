package com.musicdl.shell;

import android.annotation.SuppressLint;
import android.app.Activity;
import android.app.AlertDialog;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.graphics.Color;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.text.InputType;
import android.view.Gravity;
import android.view.KeyEvent;
import android.view.Menu;
import android.view.MenuItem;
import android.view.View;
import android.view.ViewGroup;
import android.webkit.DownloadListener;
import android.webkit.JavascriptInterface;
import android.webkit.PermissionRequest;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.widget.Toast;

/**
 * Thin WebView shell for the MusicDL Ex web UI. The server address is asked once
 * and stored, so later launches go straight to the player.
 */
public class MainActivity extends Activity {

    private static final String PREFS = "musicdl";
    private static final String KEY_URL = "base_url";
    private static final int REQ_FILE = 1001;

    private SharedPreferences prefs;
    private WebView webView;
    private LocalProxy proxy;
    private ValueCallback<Uri[]> fileCallback;

    private final PlaybackService.CommandSink commandSink = new PlaybackService.CommandSink() {
        @Override
        public void onCommand(final String command) {
            runOnUiThread(new Runnable() {
                @Override
                public void run() {
                    if (webView == null) {
                        return;
                    }
                    webView.evaluateJavascript(
                            "window.__musicdlCommand && window.__musicdlCommand('" + command + "')", null);
                }
            });
        }
    };

    /** Exposed to the web player as window.MusicDLNative. */
    private class NativeBridge {
        @JavascriptInterface
        public void updateMediaState(String json) {
            PlaybackService.pushState(MainActivity.this, json);
        }

        @JavascriptInterface
        public void clearMediaState() {
            PlaybackService.clear(MainActivity.this);
        }
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        PlaybackService.setCommandSink(commandSink);
        requestNotificationPermission();
        prefs = getSharedPreferences(PREFS, Context.MODE_PRIVATE);
        String saved = prefs.getString(KEY_URL, "");
        if (saved == null || saved.trim().isEmpty()) {
            showSetup("");
        } else {
            showWeb(saved);
        }
    }

    private void requestNotificationPermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU
                && checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS)
                != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{android.Manifest.permission.POST_NOTIFICATIONS}, 2001);
        }
    }

    private int dp(int value) {
        return (int) (value * getResources().getDisplayMetrics().density);
    }

    private void showSetup(String current) {
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setGravity(Gravity.CENTER);
        root.setPadding(dp(24), dp(24), dp(24), dp(24));
        root.setBackgroundColor(Color.WHITE);

        TextView title = new TextView(this);
        title.setText("MusicDL Ex");
        title.setTextSize(26);
        title.setTextColor(Color.parseColor("#EC4141"));
        title.setGravity(Gravity.CENTER);
        root.addView(title);

        TextView desc = new TextView(this);
        desc.setText("第一次使用请输入服务器地址，例如 http://192.168.1.10:18080");
        desc.setTextSize(13);
        desc.setTextColor(Color.parseColor("#77777F"));
        desc.setGravity(Gravity.CENTER);
        LinearLayout.LayoutParams descLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        descLp.topMargin = dp(10);
        root.addView(desc, descLp);

        final EditText input = new EditText(this);
        input.setInputType(InputType.TYPE_TEXT_VARIATION_URI);
        input.setSingleLine(true);
        input.setHint("http://192.168.1.10:18080");
        input.setText(current);
        LinearLayout.LayoutParams inputLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        inputLp.topMargin = dp(20);
        root.addView(input, inputLp);

        Button open = new Button(this);
        open.setText("打开");
        LinearLayout.LayoutParams btnLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        btnLp.topMargin = dp(12);
        root.addView(open, btnLp);

        TextView tip = new TextView(this);
        tip.setText("提示：进入后长按页面或使用菜单可以修改地址");
        tip.setTextSize(11);
        tip.setTextColor(Color.parseColor("#A1A1AB"));
        tip.setGravity(Gravity.CENTER);
        LinearLayout.LayoutParams tipLp = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        tipLp.topMargin = dp(14);
        root.addView(tip, tipLp);

        open.setOnClickListener(new View.OnClickListener() {
            @Override
            public void onClick(View v) {
                String url = normalize(input.getText().toString());
                if (url.isEmpty()) {
                    Toast.makeText(MainActivity.this, "请输入服务器地址", Toast.LENGTH_SHORT).show();
                    return;
                }
                prefs.edit().putString(KEY_URL, url).apply();
                showWeb(url);
            }
        });

        setContentView(root);
    }

    private static String normalize(String raw) {
        String url = raw == null ? "" : raw.trim();
        if (url.isEmpty()) {
            return "";
        }
        if (!url.startsWith("http://") && !url.startsWith("https://")) {
            url = "http://" + url;
        }
        while (url.endsWith("/")) {
            url = url.substring(0, url.length() - 1);
        }
        return url;
    }

    @SuppressLint("SetJavaScriptEnabled")
    private void showWeb(String url) {
        if (webView == null) {
            webView = new WebView(this);
            WebSettings settings = webView.getSettings();
            settings.setJavaScriptEnabled(true);
            settings.setDomStorageEnabled(true);
            settings.setDatabaseEnabled(true);
            settings.setMediaPlaybackRequiresUserGesture(false);
            settings.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
            settings.setUseWideViewPort(true);
            settings.setLoadWithOverviewMode(true);
            settings.setAllowFileAccess(true);
            settings.setAllowContentAccess(true);
            settings.setCacheMode(WebSettings.LOAD_DEFAULT);

            webView.setWebViewClient(new WebViewClient() {
                @Override
                public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                    Uri uri = request.getUrl();
                    String scheme = uri.getScheme() == null ? "" : uri.getScheme();
                    if (scheme.startsWith("http")) {
                        String path = uri.getPath() == null ? "" : uri.getPath().toLowerCase();
                        if (path.endsWith(".apk")) {
                            try {
                                startActivity(new Intent(Intent.ACTION_VIEW, uri));
                            } catch (Exception ignored) {
                                // Fall through to loading in the WebView.
                            }
                            return true;
                        }
                        return false;
                    }
                    try {
                        startActivity(new Intent(Intent.ACTION_VIEW, uri));
                    } catch (Exception ignored) {
                        // Ignore unknown schemes.
                    }
                    return true;
                }
            });

            webView.setWebChromeClient(new WebChromeClient() {
                @Override
                public void onPermissionRequest(final PermissionRequest request) {
                    runOnUiThread(new Runnable() {
                        @Override
                        public void run() {
                            request.grant(request.getResources());
                        }
                    });
                }

                @Override
                public boolean onShowFileChooser(WebView view, ValueCallback<Uri[]> callback,
                                                 FileChooserParams params) {
                    if (fileCallback != null) {
                        fileCallback.onReceiveValue(null);
                    }
                    fileCallback = callback;
                    try {
                        startActivityForResult(params.createIntent(), REQ_FILE);
                        return true;
                    } catch (Exception err) {
                        fileCallback = null;
                        return false;
                    }
                }
            });

            webView.setOnLongClickListener(new View.OnLongClickListener() {
                @Override
                public boolean onLongClick(View v) {
                    showMenu();
                    return true;
                }
            });

            // Release assets (the APK) are served as attachments; WebView cannot
            // save them, so hand every download to the system browser.
            webView.setDownloadListener(new DownloadListener() {
                @Override
                public void onDownloadStart(String url, String userAgent, String contentDisposition,
                                            String mimeType, long contentLength) {
                    try {
                        startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(url)));
                    } catch (Exception err) {
                        Toast.makeText(MainActivity.this, "无法打开下载链接", Toast.LENGTH_SHORT).show();
                    }
                }
            });

            webView.addJavascriptInterface(new NativeBridge(), "MusicDLNative");
            setContentView(webView);
        }
        // Media is fetched straight from the CDN through a loopback proxy so
        // covers and audio never travel through the tunnel.
        if (proxy != null) {
            proxy.stop();
            proxy = null;
        }
        try {
            proxy = new LocalProxy(url);
            proxy.start();
            webView.loadUrl(proxy.localUrl());
        } catch (Exception err) {
            proxy = null;
            webView.loadUrl(url + "/");
        }
    }

    private void showMenu() {
        final String[] items = new String[]{"刷新", "更改服务器地址"};
        new AlertDialog.Builder(this)
                .setItems(items, (dialog, which) -> {
                    if (which == 0 && webView != null) {
                        webView.reload();
                    } else if (which == 1) {
                        String current = prefs.getString(KEY_URL, "");
                        if (webView != null) {
                            webView.stopLoading();
                            webView.destroy();
                            webView = null;
                        }
                        showSetup(current == null ? "" : current);
                    }
                })
                .show();
    }

    @Override
    public boolean onCreateOptionsMenu(Menu menu) {
        menu.add(0, 1, 0, "刷新");
        menu.add(0, 2, 0, "更改服务器地址");
        return true;
    }

    @Override
    public boolean onOptionsItemSelected(MenuItem item) {
        if (item.getItemId() == 1 && webView != null) {
            webView.reload();
            return true;
        }
        if (item.getItemId() == 2) {
            showMenu();
            return true;
        }
        return super.onOptionsItemSelected(item);
    }

    @Override
    public boolean onKeyDown(int keyCode, KeyEvent event) {
        if (keyCode == KeyEvent.KEYCODE_BACK) {
            onBackPressed();
            return true;
        }
        return super.onKeyDown(keyCode, event);
    }

    @Override
    public void onBackPressed() {
        if (webView == null) {
            super.onBackPressed();
            return;
        }
        // Let the page close its playing view / queue first: edge swipes on
        // gesture navigation arrive here as a normal back press.
        webView.evaluateJavascript(
                "(function(){try{return window.__musicdlBack ? window.__musicdlBack() : false}catch(e){return false}})()",
                new ValueCallback<String>() {
                    @Override
                    public void onReceiveValue(String value) {
                        if ("true".equals(value)) {
                            return;
                        }
                        if (webView != null && webView.canGoBack()) {
                            webView.goBack();
                        } else {
                            MainActivity.super.onBackPressed();
                        }
                    }
                });
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        if (requestCode == REQ_FILE) {
            if (fileCallback != null) {
                fileCallback.onReceiveValue(
                        WebChromeClient.FileChooserParams.parseResult(resultCode, data));
                fileCallback = null;
            }
            return;
        }
        super.onActivityResult(requestCode, resultCode, data);
    }

    @Override
    protected void onDestroy() {
        if (proxy != null) {
            proxy.stop();
            proxy = null;
        }
        PlaybackService.setCommandSink(null);
        if (webView != null) {
            webView.destroy();
            webView = null;
        }
        super.onDestroy();
    }
}
