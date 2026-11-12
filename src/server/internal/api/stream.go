package api

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/soda"
)

// Resolved playback URLs are reused for a few minutes. Providers reject a
// share of resolve calls and their signed URLs stay valid much longer than
// this window, so caching keeps a single song streaming without re-resolving
// on every range request the browser makes.
const streamURLCacheTTL = 5 * time.Minute

type streamURLCacheEntry struct {
	url     string
	expires time.Time
}

var (
	streamURLCacheMu sync.Mutex
	streamURLCache   = map[string]streamURLCacheEntry{}
)

func (s *Server) handleSong(w http.ResponseWriter, r *http.Request) {
	source, id := sourceAndID(r)
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	if source == "" {
		writeError(w, http.StatusBadRequest, "missing source")
		return
	}
	meta := metaFromQuery(r)
	item := MusicItem{
		Platform: Platform,
		ID:       itemID(source, id),
		Source:   source,
		SourceID: id,
		Title:    meta.Title,
		Artist:   meta.Artist,
		Album:    meta.Album,
		Duration: meta.Duration,
		Artwork:  coverURL(r, meta.Artwork, source),
		Cover:    meta.Artwork,
		Ext:      normalizeExt(meta.Ext),
		Link:     meta.Link,
		Extra:    parseExtra(r.URL.Query().Get("extra")),
	}
	if queryBool(r, "resolve") {
		if u, err := resolveStreamURL(source, id, item.Extra, meta); err == nil {
			item.URL = u
		}
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleSongURL(w http.ResponseWriter, r *http.Request) {
	source, id := sourceAndID(r)
	if source == "" || id == "" {
		writeError(w, http.StatusBadRequest, "missing source or id")
		return
	}
	meta := metaFromQuery(r)
	extra := parseExtra(r.URL.Query().Get("extra"))
	var direct string
	var err error
	if user := s.currentUser(r); user != nil && s.opts.Store != nil {
		release := s.enterUser(user.ID)
		direct, err = resolveStreamURL(source, id, extra, meta)
		release()
	} else {
		direct, err = resolveStreamURL(source, id, extra, meta)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	proxy := baseURL(r) + "/api/stream?source=" + url.QueryEscape(source) + "&id=" + url.QueryEscape(id)
	if raw := r.URL.Query().Get("extra"); raw != "" {
		proxy += "&extra=" + url.QueryEscape(raw)
	}
	payload := map[string]any{
		"url":      direct,
		"proxyUrl": proxy,
		"source":   source,
		"id":       id,
	}
	// Only the player asks for a probe: it costs one ranged request.
	if r.URL.Query().Get("probe") == "1" {
		if info, infoErr := fetchAudioInfo(direct, source); infoErr == nil {
			payload["size"] = info.Size
			payload["duration"] = info.Duration
			payload["bitrate"] = info.Bitrate
			payload["truncated"] = info.Truncated
		}
	}
	writeJSON(w, http.StatusOK, payload)
}

func resolveStreamURL(source, id string, extra map[string]string, meta itemMeta) (string, error) {
	key := streamCacheKey(source, id, extra)
	now := time.Now()
	streamURLCacheMu.Lock()
	if entry, ok := streamURLCache[key]; ok && entry.expires.After(now) {
		streamURLCacheMu.Unlock()
		return entry.url, nil
	}
	streamURLCacheMu.Unlock()

	// Resolving a playable URL hits the upstream provider and fails
	// intermittently (netease answers "download url not found" roughly one
	// call in three). A single failed resolve during playback aborts a media
	// range request, which the browser reports as a stall, so retry a few
	// times before giving up.
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		direct, err := resolveStreamURLOnce(source, id, extra, meta)
		if err == nil && strings.TrimSpace(direct) != "" {
			streamURLCacheMu.Lock()
			if len(streamURLCache) > 256 {
				for k, v := range streamURLCache {
					if v.expires.Before(now) {
						delete(streamURLCache, k)
					}
				}
			}
			streamURLCache[key] = streamURLCacheEntry{url: direct, expires: now.Add(streamURLCacheTTL)}
			streamURLCacheMu.Unlock()
			return direct, nil
		}
		if err != nil {
			lastErr = err
		}
		if attempt < 3 {
			time.Sleep(time.Duration(120*(attempt+1)) * time.Millisecond)
		}
	}
	if lastErr == nil {
		lastErr = errors.New("failed to resolve stream url")
	}
	return "", lastErr
}

func resolveStreamURLOnce(source, id string, extra map[string]string, meta itemMeta) (string, error) {
	fn := core.GetDownloadFunc(source)
	if fn == nil {
		return "", errors.New("source does not support playback: " + source)
	}
	song := &model.Song{
		ID:       id,
		Source:   source,
		Extra:    extra,
		Name:     meta.Title,
		Artist:   meta.Artist,
		Album:    meta.Album,
		Duration: meta.Duration,
	}
	return fn(song)
}

func streamCacheKey(source, id string, extra map[string]string) string {
	if len(extra) == 0 {
		return source + "|" + id
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(source)
	b.WriteString("|")
	b.WriteString(id)
	for _, k := range keys {
		b.WriteString("|")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(extra[k])
	}
	return b.String()
}

func (s *Server) handleLyric(w http.ResponseWriter, r *http.Request) {
	source, id := sourceAndID(r)
	if source == "" || id == "" {
		writeError(w, http.StatusBadRequest, "missing source or id")
		return
	}
	meta := metaFromQuery(r)
	song := &model.Song{
		ID:       id,
		Source:   source,
		Extra:    parseExtra(r.URL.Query().Get("extra")),
		Name:     meta.Title,
		Artist:   meta.Artist,
		Album:    meta.Album,
		Duration: meta.Duration,
	}
	if source == "local" {
		lyric := ""
		if user := s.currentUser(r); user != nil {
			lyric = s.localLyric(user, id)
		}
		if strings.TrimSpace(lyric) == "" {
			writeJSON(w, http.StatusOK, map[string]any{"rawLrc": "", "lrc": "", "source": source, "id": id})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rawLrc": lyric, "lrc": lyric, "source": source, "id": id})
		return
	}
	fn := core.GetLyricFunc(source)
	if fn == nil {
		writeJSON(w, http.StatusOK, map[string]any{"rawLrc": "", "source": source, "id": id})
		return
	}
	lrc, err := fn(song)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"rawLrc": "", "source": source, "id": id, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rawLrc": lrc, "lrc": lrc, "source": source, "id": id})
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	source, id := sourceAndID(r)
	if source == "" || id == "" {
		writeError(w, http.StatusBadRequest, "missing source or id")
		return
	}
	meta := metaFromQuery(r)
	extra := parseExtra(r.URL.Query().Get("extra"))
	if source == "soda" {
		s.proxySoda(w, r, id, extra, meta)
		return
	}
	var direct string
	var err error
	if user := s.currentUser(r); user != nil && s.opts.Store != nil {
		release := s.enterUser(user.ID)
		direct, err = resolveStreamURL(source, id, extra, meta)
		release()
	} else {
		direct, err = resolveStreamURL(source, id, extra, meta)
	}
	if err != nil || strings.TrimSpace(direct) == "" {
		msg := "failed to resolve stream url"
		if err != nil {
			msg = err.Error()
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}
	s.proxyHTTP(w, r, direct, source, meta)
}

func (s *Server) proxyHTTP(w http.ResponseWriter, r *http.Request, direct, source string, meta itemMeta) {
	rangeHeader := r.Header.Get("Range")
	if fetch, handled, err := core.NewSourceRangeFetch(direct, source, rangeHeader); err != nil {
		writeError(w, http.StatusBadGateway, "upstream range error: "+err.Error())
		return
	} else if handled {
		ext := normalizeExt(fetch.Ext)
		if ext == "" {
			ext = normalizeExt(meta.Ext)
		}
		if ext == "" {
			ext = extFromURL(direct)
		}
		if ext == "" {
			ext = "mp3"
		}
		setStreamHeaders(w, ext)
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", int64String(fetch.ContentLength))
		if fetch.ContentRange != "" {
			w.Header().Set("Content-Range", fetch.ContentRange)
		}
		w.WriteHeader(fetch.StatusCode)
		if r.Method == http.MethodHead {
			return
		}
		// Stream the range over one upstream connection instead of scheduling
		// the whole file as parallel chunks: playback only needs a steady
		// trickle, and ordered chunk writes stall whenever one early chunk is
		// slow, which is audible as stuttering.
		if streamRangeSequential(w, direct, source, fetch.Start, fetch.End) {
			return
		}
		_ = fetch.WriteRangeTo(w)
		return
	}

	method := http.MethodGet
	if r.Method == http.MethodHead {
		method = http.MethodHead
	}
	req, err := core.BuildSourceRequest(method, direct, source, rangeHeader)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream request error: "+err.Error())
		return
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: 20 * time.Second,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream stream error: "+err.Error())
		return
	}
	defer resp.Body.Close()

	copyStreamHeaders(w.Header(), resp.Header)
	contentType := strings.TrimSpace(strings.ToLower(resp.Header.Get("Content-Type")))
	if contentType == "" || strings.HasPrefix(contentType, "application/octet-stream") {
		ext := normalizeExt(meta.Ext)
		if ext == "" {
			ext = extFromURL(direct)
		}
		if ext == "" {
			ext = "mp3"
		}
		w.Header().Set("Content-Type", core.AudioMimeByExt(ext))
	}
	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, resp.Body)
}

// streamRangeSequential copies a byte range from the upstream audio server to
// the client as it arrives. It reports false when the source does not answer a
// plain range request with 206 so the caller can fall back to the core helper.
func streamRangeSequential(w http.ResponseWriter, direct, source string, start, end int64) bool {
	if end < start {
		return false
	}
	req, err := core.BuildSourceRequest(http.MethodGet, direct, source, fmt.Sprintf("bytes=%d-%d", start, end))
	if err != nil {
		return false
	}
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: 20 * time.Second,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return false
	}
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return true
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if readErr != nil {
			return true
		}
	}
}

func (s *Server) proxySoda(w http.ResponseWriter, r *http.Request, id string, extra map[string]string, meta itemMeta) {
	client := core.NewSodaClient(core.CM.Get("soda"))
	info, err := client.GetDownloadInfo(&model.Song{
		ID:       id,
		Source:   "soda",
		Extra:    extra,
		Name:     meta.Title,
		Artist:   meta.Artist,
		Album:    meta.Album,
		Duration: meta.Duration,
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "soda info error: "+err.Error())
		return
	}
	req, err := core.BuildSourceRequest(http.MethodGet, info.URL, "soda", "")
	if err != nil {
		writeError(w, http.StatusBadGateway, "soda request error: "+err.Error())
		return
	}
	resp, err := (&http.Client{Timeout: 3 * time.Minute}).Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "soda stream error: "+err.Error())
		return
	}
	defer resp.Body.Close()
	encrypted, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "soda read error: "+err.Error())
		return
	}
	data := encrypted
	if strings.TrimSpace(info.PlayAuth) != "" {
		data, err = soda.DecryptAudio(encrypted, info.PlayAuth)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "soda decrypt error: "+err.Error())
			return
		}
	}
	ext := normalizeExt(core.DetectAudioExt(data))
	if ext == "" {
		ext = "m4a"
	}
	w.Header().Set("Content-Type", core.AudioMimeByExt(ext))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, safeFilename(meta.Title, id)+"."+ext, time.Now(), bytes.NewReader(data))
}

func (s *Server) handleCover(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	if raw == "" {
		writeError(w, http.StatusBadRequest, "missing url")
		return
	}
	source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	data, contentType, err := core.FetchBytesWithMime(raw, source)
	if err != nil || len(data) == 0 {
		msg := "failed to fetch cover"
		if err != nil {
			msg = err.Error()
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" || contentType == "image/jpg" {
		contentType = "image/jpeg"
	}
	etag := `"` + fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))[:20] + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if match := strings.TrimSpace(r.Header.Get("If-None-Match")); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", int64String(int64(len(data))))
	_, _ = w.Write(data)
}

func setStreamHeaders(w http.ResponseWriter, ext string) {
	w.Header().Set("Content-Type", core.AudioMimeByExt(ext))
	// Browser-only cache: never let a shared cache (CDN) reuse one user's audio.
	w.Header().Set("Cache-Control", "private, max-age=86400")
}

func copyStreamHeaders(dst, src http.Header) {
	for k, values := range src {
		switch http.CanonicalHeaderKey(k) {
		case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
			"Te", "Trailer", "Transfer-Encoding", "Upgrade", "Set-Cookie",
			"Access-Control-Allow-Origin":
			continue
		}
		for _, v := range values {
			dst.Add(k, v)
		}
	}
}

func normalizeExt(v string) string {
	v = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(v), "."))
	switch v {
	case "mp3", "flac", "ogg", "m4a", "aac", "wav", "opus", "ape", "wma":
		return v
	}
	return ""
}

func extFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return normalizeExt(strings.TrimPrefix(path.Ext(u.Path), "."))
}

func safeFilename(title, fallback string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = fallback
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	title = replacer.Replace(title)
	if len([]rune(title)) > 80 {
		title = string([]rune(title)[:80])
	}
	if title == "" {
		title = "audio"
	}
	return title
}

func int64String(v int64) string {
	if v <= 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}
