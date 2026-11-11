package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/soda"
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
	direct, err := resolveStreamURL(source, id, extra, meta)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	proxy := baseURL(r) + "/api/stream?source=" + url.QueryEscape(source) + "&id=" + url.QueryEscape(id)
	if raw := r.URL.Query().Get("extra"); raw != "" {
		proxy += "&extra=" + url.QueryEscape(raw)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":      direct,
		"proxyUrl": proxy,
		"source":   source,
		"id":       id,
	})
}

func resolveStreamURL(source, id string, extra map[string]string, meta itemMeta) (string, error) {
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
	direct, err := resolveStreamURL(source, id, extra, meta)
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
	w.Header().Set("Cache-Control", "no-store")
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
	if contentType == "" {
		contentType = "image/jpeg"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", int64String(int64(len(data))))
	w.Header().Set("Cache-Control", "public, max-age=21600")
	_, _ = w.Write(data)
}

func setStreamHeaders(w http.ResponseWriter, ext string) {
	w.Header().Set("Content-Type", core.AudioMimeByExt(ext))
	w.Header().Set("Cache-Control", "no-store")
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
