package api

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/guohuiyuan/music-lib/model"

	"github.com/guohuiyuan/go-music-dl/core"
)

var audioExtensions = map[string]bool{
	".mp3": true, ".flac": true, ".wav": true, ".m4a": true,
	".aac": true, ".ogg": true, ".opus": true, ".ape": true,
	".wma": true, ".aiff": true, ".alac": true,
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

func (s *Server) userDir(user *AppUser) string {
	return filepath.Join(s.downloadDir(), user.Username)
}

func (s *Server) userLocalDir(user *AppUser) string {
	return filepath.Join(s.userDir(user), "local")
}

func isAudioFile(name string) bool {
	return audioExtensions[strings.ToLower(filepath.Ext(name))]
}

func cleanUploadName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, ".")
	if name == "" {
		name = "audio.mp3"
	}
	return name
}

func parseAudioName(filename string) (title, artist string) {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	base = strings.TrimSpace(base)
	// Strip a leading track number such as "01 - " or "01. ".
	if index := strings.Index(base, " - "); index > 0 {
		prefix := base[:index]
		if len(prefix) <= 3 && strings.Trim(prefix, "0123456789") == "" {
			base = strings.TrimSpace(base[index+3:])
		}
	}
	if parts := strings.SplitN(base, " - ", 2); len(parts) == 2 {
		return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0])
	}
	return base, ""
}

func (s *Server) localMusicItem(r *http.Request, name string, info os.FileInfo) MusicItem {
	title, artist := parseAudioName(name)
	streamURL := baseURL(r) + "/api/me/local/stream?name=" + url.QueryEscape(name)
	return MusicItem{
		Platform: Platform,
		ID:       "local:" + name,
		Source:   "local",
		SourceID: name,
		Title:    title,
		Artist:   artist,
		Album:    "本地音乐",
		Duration: 0,
		Size:     info.Size(),
		Ext:      strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."),
		URL:      streamURL,
		Extra:    map[string]string{"localName": name},
	}
}

func (s *Server) handleListLocalMusic(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	s.migrateRootAudioToLocal(user)
	s.backfillDownloadCovers(r, user)
	dir := s.userLocalDir(user)
	if err := ensureDir(dir); err != nil {
		writeError(w, http.StatusInternalServerError, "创建本地音乐目录失败")
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取本地音乐失败")
		return
	}
	type localEntry struct {
		name string
		info os.FileInfo
	}
	var files []localEntry
	for _, entry := range entries {
		if entry.IsDir() || !isAudioFile(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, localEntry{name: entry.Name(), info: info})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].info.ModTime().After(files[j].info.ModTime())
	})
	items := make([]MusicItem, 0, len(files))
	covers := map[string]string{}
	if s.opts.Store != nil {
		if records, recErr := s.opts.Store.ListDownloadRecords(user.ID, 500); recErr == nil {
			for _, record := range records {
				if record.Cover != "" {
					covers[record.Filename] = record.Cover
				}
			}
		}
	}
	for _, file := range files {
		item := s.localMusicItem(r, file.name, file.info)
		if cover := covers[file.name]; cover != "" {
			item.Artwork = cover
			item.Cover = cover
		}
		if duration, bitrate := audioMetadata(filepath.Join(dir, file.name), file.info.Size()); duration > 0 || bitrate > 0 {
			item.Duration = duration
			item.Bitrate = bitrate
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "dir": dir})
}

func (s *Server) handleUploadLocalMusic(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "上传文件过大或格式错误")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少上传文件")
		return
	}
	defer file.Close()
	name := cleanUploadName(header.Filename)
	if !isAudioFile(name) {
		writeError(w, http.StatusBadRequest, "仅支持音频文件")
		return
	}
	dir := s.userLocalDir(user)
	if err := ensureDir(dir); err != nil {
		writeError(w, http.StatusInternalServerError, "创建本地音乐目录失败")
		return
	}
	path := filepath.Join(dir, name)
	out, err := os.Create(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存文件失败")
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		_ = os.Remove(path)
		writeError(w, http.StatusInternalServerError, "写入文件失败")
		return
	}
	if err := out.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "保存文件失败")
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取文件失败")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"item": s.localMusicItem(r, name, info)})
}

func (s *Server) handleDeleteLocalMusic(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	name := cleanUploadName(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, "文件名不能为空")
		return
	}
	path := filepath.Join(s.userLocalDir(user), name)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "文件不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "删除文件失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleStreamLocalMusic(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	name := cleanUploadName(r.URL.Query().Get("name"))
	if name == "" || !isAudioFile(name) {
		writeError(w, http.StatusBadRequest, "文件名错误")
		return
	}
	path := filepath.Join(s.userLocalDir(user), name)
	file, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "文件不存在")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取文件失败")
		return
	}
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+url.PathEscape(name))
	http.ServeContent(w, r, name, info.ModTime(), file)
}

func musicItemToSong(item MusicItem) *model.Song {
	id := item.SourceID
	if id == "" {
		id = item.ID
		if index := strings.Index(id, ":"); index >= 0 {
			id = id[index+1:]
		}
	}
	return &model.Song{
		ID:        id,
		Name:      item.Title,
		Artist:    item.Artist,
		Album:     item.Album,
		AlbumID:   item.AlbumID,
		Duration:  item.Duration,
		Size:      item.Size,
		Bitrate:   item.Bitrate,
		Source:    item.Source,
		URL:       item.URL,
		Ext:       item.Ext,
		Cover:     item.Cover,
		Link:      item.Link,
		Extra:     item.Extra,
		IsInvalid: item.IsInvalid,
		IsVIP:     item.IsVIP,
	}
}

func (s *Server) handleDownloadSong(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	var input struct {
		Song             MusicItem `json:"song"`
		WithCover        bool      `json:"withCover"`
		WithLyrics       bool      `json:"withLyrics"`
		FilenameTemplate string    `json:"filenameTemplate"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if strings.TrimSpace(input.Song.Title) == "" || strings.TrimSpace(input.Song.Source) == "" {
		writeError(w, http.StatusBadRequest, "歌曲信息不完整")
		return
	}
	if s.opts.Store != nil && strings.TrimSpace(input.Song.SourceID) != "" {
		if existing, findErr := s.opts.Store.FindDownloadRecord(user.ID, input.Song.Source, input.Song.SourceID); findErr == nil && existing != nil {
			if info, statErr := os.Stat(existing.Path); statErr == nil {
				writeJSON(w, http.StatusOK, map[string]any{
					"skipped":  true,
					"path":     existing.Path,
					"filename": existing.Filename,
					"size":     info.Size(),
					"ext":      strings.TrimPrefix(filepath.Ext(existing.Filename), "."),
					"warning":  "",
				})
				return
			}
			_ = s.opts.Store.DeleteDownloadRecord(user.ID, existing.ID)
		}
	}

	dir := s.userLocalDir(user)
	if err := ensureDir(dir); err != nil {
		writeError(w, http.StatusInternalServerError, "创建下载目录失败")
		return
	}
	template := strings.TrimSpace(input.FilenameTemplate)
	if template == "" {
		template = core.DefaultDownloadFilenameTemplate
	}
	result, err := core.SaveSongToFileWithTemplate(musicItemToSong(input.Song), dir, input.WithCover, input.WithLyrics, template)
	if err != nil {
		writeError(w, http.StatusBadGateway, "下载失败: "+err.Error())
		return
	}
	size := int64(0)
	if info, statErr := os.Stat(result.SavedPath); statErr == nil {
		size = info.Size()
	} else if len(result.Data) > 0 {
		size = int64(len(result.Data))
	}
	if s.opts.Store != nil {
		_ = s.opts.Store.AddDownloadRecord(user.ID, input.Song, result.SavedPath, result.Filename, size)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":     result.SavedPath,
		"filename": result.Filename,
		"ext":      result.Ext,
		"size":     size,
		"warning":  result.Warning,
	})
}

func (s *Server) migrateRootAudioToLocal(user *AppUser) {
	root := s.userDir(user)
	local := s.userLocalDir(user)
	if err := ensureDir(local); err != nil {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !isAudioFile(entry.Name()) {
			continue
		}
		target := filepath.Join(local, entry.Name())
		if _, err := os.Stat(target); err == nil {
			continue
		}
		_ = os.Rename(filepath.Join(root, entry.Name()), target)
	}
}
