package com.musicdl.shell;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.drawable.Icon;
import android.media.MediaMetadata;
import android.media.session.MediaSession;
import android.media.session.PlaybackState;
import android.os.Build;
import android.os.IBinder;
import android.util.Log;

import org.json.JSONObject;

import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;

/**
 * Foreground service that publishes the current track to the system media
 * session, so the lock screen, the notification shade, Bluetooth headsets and
 * car units can show metadata and drive playback. The WebView keeps owning the
 * actual audio; this service only mirrors the state and forwards commands.
 */
public class PlaybackService extends Service {

    public interface CommandSink {
        void onCommand(String command);
    }

    private static final String TAG = "MusicDLPlayback";
    private static final String CHANNEL_ID = "musicdl_playback";
    private static final int NOTIFICATION_ID = 0x4d55;

    static final String ACTION_UPDATE = "com.musicdl.shell.UPDATE";
    static final String ACTION_CLEAR = "com.musicdl.shell.CLEAR";
    static final String ACTION_TOGGLE = "com.musicdl.shell.TOGGLE";
    static final String ACTION_PREV = "com.musicdl.shell.PREV";
    static final String ACTION_NEXT = "com.musicdl.shell.NEXT";
    static final String ACTION_STOP = "com.musicdl.shell.STOP";
    static final String ACTION_LIKE = "com.musicdl.shell.LIKE";
    static final String EXTRA_STATE = "state";

    private static volatile CommandSink commandSink;

    private MediaSession session;
    private String title = "";
    private String artist = "";
    private String album = "";
    private String artworkUrl = "";
    private boolean playing;
    private boolean liked;
    private boolean hasQueue;
    private long durationMs;
    private long positionMs;
    private Bitmap artwork;
    private String artworkLoading = "";
    private boolean foregrounded;

    public static void setCommandSink(CommandSink sink) {
        commandSink = sink;
    }

    /** Mirrors a JSON state blob coming from the web player. */
    public static void pushState(Context context, String json) {
        if (context == null || json == null) {
            return;
        }
        try {
            Intent intent = new Intent(context, PlaybackService.class);
            intent.setAction(ACTION_UPDATE);
            intent.putExtra(EXTRA_STATE, json);
            start(context, intent);
        } catch (Exception err) {
            Log.w(TAG, "pushState failed", err);
        }
    }

    public static void clear(Context context) {
        if (context == null) {
            return;
        }
        try {
            Intent intent = new Intent(context, PlaybackService.class);
            intent.setAction(ACTION_CLEAR);
            start(context, intent);
        } catch (Exception err) {
            Log.w(TAG, "clear failed", err);
        }
    }

    private static void start(Context context, Intent intent) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent);
        } else {
            context.startService(intent);
        }
    }

    @Override
    public void onCreate() {
        super.onCreate();
        createChannel();
        session = new MediaSession(this, "MusicDL");
        session.setFlags(MediaSession.FLAG_HANDLES_MEDIA_BUTTONS
                | MediaSession.FLAG_HANDLES_TRANSPORT_CONTROLS);
        session.setCallback(new MediaSession.Callback() {
            @Override
            public void onPlay() {
                dispatch("play");
            }

            @Override
            public void onPause() {
                dispatch("pause");
            }

            @Override
            public void onStop() {
                dispatch("stop");
            }

            @Override
            public void onSkipToNext() {
                dispatch("next");
            }

            @Override
            public void onSkipToPrevious() {
                dispatch("prev");
            }

            @Override
            public void onSeekTo(long pos) {
                dispatch("seek:" + Math.max(0, pos / 1000));
            }
        });
        session.setActive(true);
    }

    private void dispatch(String command) {
        CommandSink sink = commandSink;
        if (sink != null) {
            sink.onCommand(command);
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent == null) {
            return START_STICKY;
        }
        String action = intent.getAction();
        if (ACTION_CLEAR.equals(action)) {
            clearSession();
            stopSelf();
            return START_NOT_STICKY;
        }
        if (ACTION_TOGGLE.equals(action)) {
            ensureForeground();
            dispatch("toggle");
            return START_STICKY;
        }
        if (ACTION_PREV.equals(action)) {
            ensureForeground();
            dispatch("prev");
            return START_STICKY;
        }
        if (ACTION_NEXT.equals(action)) {
            ensureForeground();
            dispatch("next");
            return START_STICKY;
        }
        if (ACTION_STOP.equals(action)) {
            ensureForeground();
            dispatch("stop");
            return START_STICKY;
        }
        if (ACTION_LIKE.equals(action)) {
            ensureForeground();
            dispatch("like");
            return START_STICKY;
        }
        String json = intent.getStringExtra(EXTRA_STATE);
        if (json != null) {
            applyState(json);
        }
        return START_STICKY;
    }

    private void applyState(String json) {
        try {
            JSONObject obj = new JSONObject(json);
            title = obj.optString("title", "");
            artist = obj.optString("artist", "");
            album = obj.optString("album", "");
            playing = obj.optBoolean("playing", false);
            liked = obj.optBoolean("liked", false);
            hasQueue = obj.optBoolean("hasQueue", false);
            durationMs = (long) (obj.optDouble("duration", 0) * 1000);
            positionMs = (long) (obj.optDouble("position", 0) * 1000);
            String url = obj.optString("artwork", "");
            if (!url.equals(artworkUrl)) {
                artworkUrl = url;
                artwork = null;
                loadArtwork(url);
            }
            if (title.isEmpty() && artist.isEmpty()) {
                clearSession();
                stopSelf();
                return;
            }
            updateSession();
            startForegroundCompat();
        } catch (Exception err) {
            Log.w(TAG, "bad state payload", err);
        }
    }

    private void updateSession() {
        if (session == null) {
            return;
        }
        MediaMetadata.Builder meta = new MediaMetadata.Builder()
                .putString(MediaMetadata.METADATA_KEY_TITLE, title)
                .putString(MediaMetadata.METADATA_KEY_ARTIST, artist)
                .putString(MediaMetadata.METADATA_KEY_ALBUM, album)
                .putLong(MediaMetadata.METADATA_KEY_DURATION, durationMs);
        if (artwork != null) {
            meta.putBitmap(MediaMetadata.METADATA_KEY_ALBUM_ART, artwork);
        }
        session.setMetadata(meta.build());

        long actions = PlaybackState.ACTION_PLAY
                | PlaybackState.ACTION_PAUSE
                | PlaybackState.ACTION_PLAY_PAUSE
                | PlaybackState.ACTION_STOP
                | PlaybackState.ACTION_SEEK_TO;
        if (hasQueue) {
            actions |= PlaybackState.ACTION_SKIP_TO_NEXT | PlaybackState.ACTION_SKIP_TO_PREVIOUS;
        }
        PlaybackState.Builder state = new PlaybackState.Builder()
                .setActions(actions)
                .setState(playing ? PlaybackState.STATE_PLAYING : PlaybackState.STATE_PAUSED,
                        positionMs, playing ? 1f : 0f);
        session.setPlaybackState(state.build());
    }

    private void loadArtwork(String url) {
        if (url == null || url.isEmpty() || url.equals(artworkLoading)) {
            return;
        }
        artworkLoading = url;
        final String target = url;
        new Thread(new Runnable() {
            @Override
            public void run() {
                Bitmap bitmap = null;
                try {
                    HttpURLConnection conn = (HttpURLConnection) new URL(target).openConnection();
                    conn.setConnectTimeout(8000);
                    conn.setReadTimeout(8000);
                    conn.setDoInput(true);
                    try (InputStream in = conn.getInputStream()) {
                        bitmap = BitmapFactory.decodeStream(in);
                    }
                    conn.disconnect();
                } catch (Exception err) {
                    Log.w(TAG, "artwork failed", err);
                }
                if (bitmap != null && target.equals(artworkUrl)) {
                    artwork = bitmap;
                    updateSession();
                    startForegroundCompat();
                }
            }
        }).start();
    }

    private void clearSession() {
        playing = false;
        title = "";
        artist = "";
        album = "";
        artworkUrl = "";
        artwork = null;
        if (session != null) {
            session.setActive(false);
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(STOP_FOREGROUND_REMOVE);
        } else {
            stopForeground(true);
        }
        foregrounded = false;
    }

    private void startForegroundCompat() {
        Notification notification = buildNotification();
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(NOTIFICATION_ID, notification,
                    android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK);
        } else {
            startForeground(NOTIFICATION_ID, notification);
        }
        foregrounded = true;
    }

    /** Control actions can start the service from a notification action. */
    private void ensureForeground() {
        if (!foregrounded) {
            startForegroundCompat();
        }
    }

    @SuppressWarnings("deprecation")
    private Notification buildNotification() {
        Notification.Builder builder;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            builder = new Notification.Builder(this, CHANNEL_ID);
        } else {
            builder = new Notification.Builder(this);
        }

        Intent openIntent = new Intent(this, MainActivity.class);
        openIntent.setFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_CLEAR_TOP);
        int flags = PendingIntent.FLAG_UPDATE_CURRENT;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            flags |= PendingIntent.FLAG_IMMUTABLE;
        }
        PendingIntent contentIntent = PendingIntent.getActivity(this, 0, openIntent, flags);
        if (session != null) {
            // Lets the lock screen and Bluetooth panels open the app on tap.
            session.setSessionActivity(contentIntent);
        }

        builder.setSmallIcon(R.drawable.ic_stat_music)
                .setOnlyAlertOnce(true)
                .setCategory(Notification.CATEGORY_TRANSPORT)
                .setColorized(true)
                .setColor(0xFFEC4141)
                .setContentTitle(title.isEmpty() ? "MusicDL Ex" : title)
                .setContentText(artist.isEmpty() ? (album.isEmpty() ? "正在播放" : album) : artist)
                .setSubText(album.isEmpty() ? null : album)
                .setContentIntent(contentIntent)
                .setShowWhen(false)
                .setOngoing(title != null && !title.isEmpty())
                .setVisibility(Notification.VISIBILITY_PUBLIC);

        if (artwork != null) {
            builder.setLargeIcon(artwork);
        }

        // Custom vectors render crisp and larger than the thin platform media
        // icons, which looked blurred on high-density screens.
        addAction(builder, R.drawable.ic_prev, "上一曲", actionIntent(ACTION_PREV, 1));
        addAction(builder, playing ? R.drawable.ic_pause : R.drawable.ic_play,
                playing ? "暂停" : "播放", actionIntent(ACTION_TOGGLE, 2));
        addAction(builder, R.drawable.ic_next, "下一曲", actionIntent(ACTION_NEXT, 3));
        addAction(builder, liked ? R.drawable.ic_liked : R.drawable.ic_like,
                liked ? "取消收藏" : "收藏", actionIntent(ACTION_LIKE, 4));

        Notification.MediaStyle style = new Notification.MediaStyle().setShowActionsInCompactView(0, 1, 2);
        if (session != null) {
            style.setMediaSession(session.getSessionToken());
        }
        builder.setStyle(style);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            builder.setChannelId(CHANNEL_ID);
        }
        return builder.build();
    }

    private void addAction(Notification.Builder builder, int iconRes, CharSequence title, PendingIntent intent) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            builder.addAction(new Notification.Action.Builder(
                    Icon.createWithResource(this, iconRes), title, intent).build());
        } else {
            builder.addAction(iconRes, title, intent);
        }
    }

    private PendingIntent actionIntent(String action, int code) {
        Intent intent = new Intent(this, PlaybackService.class);
        intent.setAction(action);
        int flags = PendingIntent.FLAG_UPDATE_CURRENT;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            flags |= PendingIntent.FLAG_IMMUTABLE;
        }
        return PendingIntent.getService(this, code, intent, flags);
    }

    private void createChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            return;
        }
        NotificationManager manager = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
        if (manager == null) {
            return;
        }
        NotificationChannel channel = new NotificationChannel(CHANNEL_ID, "播放控制",
                NotificationManager.IMPORTANCE_DEFAULT);
        channel.setDescription("锁屏与通知栏的音乐播放控制");
        channel.setShowBadge(false);
        channel.setLockscreenVisibility(Notification.VISIBILITY_PUBLIC);
        manager.createNotificationChannel(channel);
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    @Override
    public void onDestroy() {
        if (session != null) {
            session.setActive(false);
            session.release();
            session = null;
        }
        super.onDestroy();
    }
}
