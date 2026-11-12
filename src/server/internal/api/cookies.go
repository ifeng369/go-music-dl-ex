package api

import (
	"net/http"
	"strings"
)

func (s *Server) handleGetCookies(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil || s.opts.Store == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	stored, err := s.opts.Store.UserCookies(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取 Cookie 失败")
		return
	}
	items := make(map[string]any, len(stored))
	for source, value := range stored {
		items[source] = map[string]any{
			"configured": strings.TrimSpace(value) != "",
			"preview":    maskSecret(value),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleSetCookies(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil || s.opts.Store == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	var payload struct {
		Source  string            `json:"source"`
		Cookie  string            `json:"cookie"`
		Cookies map[string]string `json:"cookies"`
	}
	if err := readJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}
	updates := make(map[string]string)
	for source, value := range payload.Cookies {
		source = strings.ToLower(strings.TrimSpace(source))
		if source != "" {
			updates[source] = strings.TrimSpace(value)
		}
	}
	if source := strings.ToLower(strings.TrimSpace(payload.Source)); source != "" {
		updates[source] = strings.TrimSpace(payload.Cookie)
	}
	if len(updates) == 0 {
		writeError(w, http.StatusBadRequest, "nothing to update")
		return
	}
	for source, value := range updates {
		if value == "" {
			_ = s.opts.Store.DeleteUserCookie(user.ID, source)
			continue
		}
		if err := s.opts.Store.SetUserCookie(user.ID, source, value); err != nil {
			writeError(w, http.StatusInternalServerError, "保存 Cookie 失败")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": len(updates)})
}

func (s *Server) handleDeleteCookie(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil || s.opts.Store == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	if source == "" {
		writeError(w, http.StatusBadRequest, "missing source")
		return
	}
	if err := s.opts.Store.DeleteUserCookie(user.ID, source); err != nil {
		writeError(w, http.StatusInternalServerError, "删除 Cookie 失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": source})
}
