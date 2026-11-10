package api

import (
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/guohuiyuan/go-music-dl/core"
)

type Options struct {
	Version   string
	BuildTime string
	Core      string
	Web       http.Handler
	Store     *UserStore
}

type Server struct {
	opts    Options
	mux     *http.ServeMux
	started time.Time
}

func New(opts Options) *Server {
	if opts.Web == nil {
		opts.Web = http.NotFoundHandler()
	}
	s := &Server{opts: opts, mux: http.NewServeMux(), started: time.Now()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.recoverer(s.cors(s.logRequests(s.mux)))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/healthz", s.handleHealth)
	s.mux.HandleFunc("GET /plugin.js", s.handlePluginJS)
	s.mux.HandleFunc("GET /mint-plugin.js", s.handleMintPluginJS)
	s.mux.HandleFunc("GET /api/version", s.handleHealth)
	s.mux.HandleFunc("GET /api/sources", s.handleSources)
	s.mux.HandleFunc("GET /api/detect", s.handleDetect)

	s.mux.HandleFunc("GET /api/search", s.handleSearch)
	s.mux.HandleFunc("GET /api/artist/works", s.handleArtistWorks)
	s.mux.HandleFunc("GET /api/song", s.handleSong)
	s.mux.HandleFunc("GET /api/song/url", s.handleSongURL)
	s.mux.HandleFunc("GET /api/song/alternatives", s.handleAlternatives)
	s.mux.HandleFunc("GET /api/lyric", s.handleLyric)
	s.mux.HandleFunc("GET /api/stream", s.handleStream)
	s.mux.HandleFunc("HEAD /api/stream", s.handleStream)
	s.mux.HandleFunc("GET /api/cover", s.handleCover)

	s.mux.HandleFunc("GET /api/playlist", s.handlePlaylist)
	s.mux.HandleFunc("GET /api/album", s.handleAlbum)
	s.mux.HandleFunc("GET /api/toplists", s.handleTopLists)
	s.mux.HandleFunc("GET /api/toplist/detail", s.handleTopListDetail)
	s.mux.HandleFunc("GET /api/recommend", s.handleRecommend)
	s.mux.HandleFunc("GET /api/categories", s.handleCategories)
	s.mux.HandleFunc("GET /api/category/sheets", s.handleCategorySheets)
	s.mux.HandleFunc("GET /api/user/playlists", s.handleUserPlaylists)
	s.mux.HandleFunc("GET /api/home", s.handleHome)

	s.mux.HandleFunc("GET /api/import", s.handleImport)
	s.mux.HandleFunc("POST /api/import", s.handleImport)

	s.mux.HandleFunc("GET /api/cookies", s.handleGetCookies)
	s.mux.HandleFunc("POST /api/cookies", s.handleSetCookies)
	s.mux.HandleFunc("DELETE /api/cookies", s.handleDeleteCookie)

	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/auth/me", s.handleMe)
	s.mux.HandleFunc("GET /api/users", s.requireAdmin(s.handleListUsers))
	s.mux.HandleFunc("POST /api/users", s.requireAdmin(s.handleCreateUser))
	s.mux.HandleFunc("PUT /api/users/{id}", s.requireAdmin(s.handleUpdatePassword))
	s.mux.HandleFunc("PUT /api/users/{id}/password", s.requireAdmin(s.handleUpdatePassword))
	s.mux.HandleFunc("DELETE /api/users/{id}", s.requireAdmin(s.handleDeleteUser))
	s.mux.HandleFunc("GET /api/me/library", s.requireUser(s.handleGetLibrary))
	s.mux.HandleFunc("PUT /api/me/library", s.requireUser(s.handleSaveLibrary))
	s.mux.HandleFunc("GET /api/settings", s.requireAdmin(s.handleGetSettings))
	s.mux.HandleFunc("PUT /api/settings", s.requireAdmin(s.handleSaveSettings))

	s.mux.HandleFunc("GET /api/me/local", s.requireUser(s.handleListLocalMusic))
	s.mux.HandleFunc("POST /api/me/local/upload", s.requireUser(s.handleUploadLocalMusic))
	s.mux.HandleFunc("DELETE /api/me/local", s.requireUser(s.handleDeleteLocalMusic))
	s.mux.HandleFunc("GET /api/me/local/stream", s.requireUser(s.handleStreamLocalMusic))
	s.mux.HandleFunc("HEAD /api/me/local/stream", s.requireUser(s.handleStreamLocalMusic))
	s.mux.HandleFunc("POST /api/me/download", s.requireUser(s.handleDownloadSong))
	s.mux.HandleFunc("GET /api/me/downloads", s.requireUser(s.handleListDownloadRecords))
	s.mux.HandleFunc("DELETE /api/me/downloads", s.requireUser(s.handleClearDownloadRecords))
	s.mux.HandleFunc("DELETE /api/me/downloads/{id}", s.requireUser(s.handleDeleteDownloadRecord))

	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "unknown api endpoint: "+r.URL.Path)
	})
	s.mux.Handle("/", s.opts.Web)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"version":     s.opts.Version,
		"buildTime":   s.opts.BuildTime,
		"core":        s.opts.Core,
		"platform":    Platform,
		"uptimeSec":   int(time.Since(s.started).Seconds()),
		"sourceCount": len(core.GetAllSourceNames()),
	})
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	loginNeeded := map[string]bool{
		"netease": true, "qq": true, "kugou": true, "kuwo": true, "migu": true,
		"soda": true, "spotify": true, "apple": true, "qianqian": true,
	}
	searchSources := setName(core.GetAllSourceNames())
	albumSources := setName(core.GetAlbumSourceNames())
	sheetSources := setName(core.GetPlaylistSourceNames())
	topSources := setName(core.GetTopListSourceNames())
	catSources := setName(core.GetPlaylistCategorySourceNames())
	recSources := setName(core.GetRecommendSourceNames())
	userSources := setName(core.GetUserPlaylistSourceNames())
	// Kuwo's upstream music-lib handler is a stub; our API implements it directly.
	userSources["kuwo"] = true

	items := make([]SourceInfo, 0, len(core.GetAllSourceNames()))
	for _, source := range core.GetAllSourceNames() {
		if source == "local" {
			continue
		}
		items = append(items, SourceInfo{
			ID:            source,
			Name:          sourceDisplayName(source),
			Description:   core.GetSourceDescription(source),
			Search:        searchSources[source],
			Album:         albumSources[source],
			Sheet:         sheetSources[source],
			TopList:       topSources[source],
			Categories:    catSources[source],
			Recommend:     recSources[source],
			UserPlaylists: userSources[source],
			RequiresLogin: loginNeeded[source],
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"platform": Platform, "items": items})
}

func (s *Server) handleDetect(w http.ResponseWriter, r *http.Request) {
	link := strings.TrimSpace(r.URL.Query().Get("url"))
	if link == "" {
		writeError(w, http.StatusBadRequest, "missing url")
		return
	}
	source := core.DetectSource(link)
	writeJSON(w, http.StatusOK, map[string]any{"source": source, "name": sourceDisplayName(source), "url": link})
}

func setName(list []string) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, v := range list {
		m[v] = true
	}
	return m
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %s %s", r.Method, r.URL.RequestURI(), r.RemoteAddr, time.Since(start).Round(time.Millisecond))
		}
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				log.Printf("panic: %v\n%s", p, debug.Stack())
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
