package api

import (
	"net/http"
	"strings"

	"github.com/guohuiyuan/go-music-dl/core"
)

// resolveHeaders turns the headers the core uses for upstream requests into a
// plain map so a native client can replay them when fetching bytes directly.
func resolveHeaders(req *http.Request) map[string]string {
	out := make(map[string]string, len(req.Header))
	for key, values := range req.Header {
		if len(values) == 0 {
			continue
		}
		if strings.EqualFold(key, "Range") || strings.EqualFold(key, "Host") {
			continue
		}
		out[key] = values[0]
	}
	return out
}

// handleResolve returns a direct URL plus the exact headers needed to fetch it.
// The android shell uses it so covers and audio go straight from the CDN to the
// phone instead of travelling through the NAS.
func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	kind := strings.ToLower(strings.TrimSpace(q.Get("kind")))
	source := strings.ToLower(strings.TrimSpace(q.Get("source")))

	if kind == "cover" {
		raw := strings.TrimSpace(q.Get("url"))
		if raw == "" {
			writeError(w, http.StatusBadRequest, "missing url")
			return
		}
		req, err := core.BuildSourceRequest(http.MethodGet, raw, source, "")
		if err != nil {
			writeError(w, http.StatusBadGateway, "invalid cover url")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"url":       raw,
			"headers":   resolveHeaders(req),
			"expiresIn": 86400,
		})
		return
	}

	id := strings.TrimSpace(q.Get("id"))
	if source == "" || id == "" {
		writeError(w, http.StatusBadRequest, "missing source or id")
		return
	}
	meta := metaFromQuery(r)
	extra := parseExtra(q.Get("extra"))
	direct, err := resolveStreamURL(source, id, extra, meta)
	if err != nil || strings.TrimSpace(direct) == "" {
		msg := "failed to resolve stream url"
		if err != nil {
			msg = err.Error()
		}
		writeError(w, http.StatusBadGateway, msg)
		return
	}
	req, err := core.BuildSourceRequest(http.MethodGet, direct, source, "")
	if err != nil {
		writeError(w, http.StatusBadGateway, "invalid stream url")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":       direct,
		"headers":   resolveHeaders(req),
		"expiresIn": 1200,
	})
}
