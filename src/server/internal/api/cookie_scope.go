package api

import (
	"net/http"
	"strings"
	"sync"

	"github.com/guohuiyuan/go-music-dl/core"
)

// The upstream core keeps a single global CookieManager. Requests are scoped to
// the signed-in account around each short source call so two users never share
// credentials. Streaming is excluded here and scoped only around URL
// resolution, so a long download never blocks other users.
var cookieScopeMu sync.Mutex

func replaceCookies(next map[string]string) {
	current := core.CM.GetAll()
	clear := make(map[string]string, len(current)+len(next))
	for source := range current {
		clear[source] = ""
	}
	core.CM.SetAll(clear)
	if len(next) > 0 {
		core.CM.SetAll(next)
	}
}

func (s *Server) scopeToUser(userID uint) func() {
	previous := core.CM.GetAll()
	cookies, err := s.opts.Store.UserCookies(userID)
	if err != nil {
		return func() {}
	}
	replaceCookies(cookies)
	return func() { replaceCookies(previous) }
}

func needsCookieScope(path string) bool {
	switch {
	case path == "/api/stream", path == "/api/cover":
		return false
	case strings.HasPrefix(path, "/api/auth/"), strings.HasPrefix(path, "/api/me/"):
		return false
	case path == "/api/home", path == "/api/cookies", path == "/api/settings",
		path == "/api/healthz", path == "/api/version", path == "/api/sources", path == "/api/detect":
		return false
	default:
		return true
	}
}

func (s *Server) withUserCookies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Metadata endpoints either do not touch sources or are already
		// user-scoped, so they skip the shared cookie swap and stay fast.
		if s.opts.Store == nil || !needsCookieScope(path) {
			next.ServeHTTP(w, r)
			return
		}
		user := s.currentUser(r)
		if user == nil {
			next.ServeHTTP(w, r)
			return
		}
		cookieScopeMu.Lock()
		restore := s.scopeToUser(user.ID)
		defer func() {
			restore()
			cookieScopeMu.Unlock()
		}()
		next.ServeHTTP(w, r)
	})
}

// migrateGlobalCookies copies cookies configured before per-user support to the
// first admin account so existing setups keep working.
func (s *Server) migrateGlobalCookies() {
	if s.opts.Store == nil {
		return
	}
	count, err := s.opts.Store.CountUserCookies()
	if err != nil || count > 0 {
		return
	}
	global := core.CM.GetAll()
	if len(global) == 0 {
		return
	}
	var admin AppUser
	if err := s.opts.Store.DB().Where("role = ?", "admin").Order("id ASC").First(&admin).Error; err != nil {
		return
	}
	for source, value := range global {
		if strings.TrimSpace(value) == "" {
			continue
		}
		_ = s.opts.Store.SetUserCookie(admin.ID, source, value)
	}
}
