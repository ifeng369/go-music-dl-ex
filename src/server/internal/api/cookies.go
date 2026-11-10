package api

import (
	"net/http"
	"strings"

	"github.com/guohuiyuan/go-music-dl/core"
)

func (s *Server) handleGetCookies(w http.ResponseWriter, r *http.Request) {
	all := core.CM.GetAll()
	items := make(map[string]any, len(all))
	for source, value := range all {
		items[source] = map[string]any{
			"configured": strings.TrimSpace(value) != "",
			"preview":    maskSecret(value),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleSetCookies(w http.ResponseWriter, r *http.Request) {
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
	core.CM.SetAll(updates)
	core.CM.Save()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "updated": len(updates)})
}

func (s *Server) handleDeleteCookie(w http.ResponseWriter, r *http.Request) {
	source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	if source == "" {
		writeError(w, http.StatusBadRequest, "missing source")
		return
	}
	core.CM.SetAll(map[string]string{source: ""})
	core.CM.Save()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "source": source})
}
