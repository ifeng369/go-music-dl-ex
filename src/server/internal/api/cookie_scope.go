package api

import (
	"net/http"
	"strings"
	"sync"

	"github.com/guohuiyuan/go-music-dl/core"
)

// The upstream core keeps one global CookieManager, so requests are scoped to
// the signed-in account. A read/write gate lets every request of the *same*
// user run concurrently (the common case) and only serialises while the active
// cookie set is actually being switched to another account.
var (
	cookieGate       sync.RWMutex
	activeCookieUser uint
	activeCookieSet  bool

	cookieCacheMu sync.Mutex
	cookieCache   = map[uint]map[string]string{}
)

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

func (s *Server) userCookiesCached(userID uint) map[string]string {
	cookieCacheMu.Lock()
	if cached, ok := cookieCache[userID]; ok {
		cookieCacheMu.Unlock()
		return cached
	}
	cookieCacheMu.Unlock()

	stored, err := s.opts.Store.UserCookies(userID)
	if err != nil {
		return map[string]string{}
	}
	cookieCacheMu.Lock()
	cookieCache[userID] = stored
	cookieCacheMu.Unlock()
	return stored
}

// invalidateUserCookies must be called after a cookie change so the next
// request reloads from the database.
func (s *Server) invalidateUserCookies(userID uint) {
	cookieCacheMu.Lock()
	delete(cookieCache, userID)
	cookieCacheMu.Unlock()

	cookieGate.Lock()
	if activeCookieUser == userID {
		activeCookieSet = false
	}
	cookieGate.Unlock()
}

// enterUser makes the manager hold this account's cookies and returns the
// release function for the caller's request.
func (s *Server) enterUser(userID uint) func() {
	cookies := s.userCookiesCached(userID)
	for attempt := 0; attempt < 8; attempt++ {
		cookieGate.RLock()
		if activeCookieSet && activeCookieUser == userID {
			return func() { cookieGate.RUnlock() }
		}
		cookieGate.RUnlock()

		cookieGate.Lock()
		if !activeCookieSet || activeCookieUser != userID {
			replaceCookies(cookies)
			activeCookieUser = userID
			activeCookieSet = true
		}
		cookieGate.Unlock()
	}
	cookieGate.RLock()
	return func() { cookieGate.RUnlock() }
}

func needsCookieScope(path string) bool {
	switch {
	case path == "/api/stream", path == "/api/cover":
		return false
	case strings.HasPrefix(path, "/api/auth/"), strings.HasPrefix(path, "/api/me/"):
		return false
	case path == "/api/cookies", path == "/api/home", path == "/api/category/sheets",
		path == "/api/settings", path == "/api/healthz", path == "/api/version",
		path == "/api/sources", path == "/api/detect":
		return false
	default:
		return true
	}
}

func (s *Server) withUserCookies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.opts.Store == nil || !needsCookieScope(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		user := s.currentUser(r)
		if user == nil {
			next.ServeHTTP(w, r)
			return
		}
		release := s.enterUser(user.ID)
		defer release()
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
