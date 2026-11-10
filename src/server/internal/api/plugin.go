package api

import (
	"bytes"
	"net/http"
	"strings"

	"musicdl-ex/internal/webassets"
)

const pluginDefaultBase = `const DEFAULT_BASE = "http://127.0.0.1:8080";`
const pluginDefaultToken = `const DEFAULT_TOKEN = "";`

func (s *Server) servePluginJS(w http.ResponseWriter, r *http.Request, data []byte, filename string) {
	base := strings.TrimRight(baseURL(r), "/")
	if base != "" {
		replacement := `const DEFAULT_BASE = "` + base + `";`
		data = bytes.Replace(data, []byte(pluginDefaultBase), []byte(replacement), 1)
	}
	if filename == "mint-plugin.js" && s.opts.Store != nil {
		if user := s.currentUser(r); user != nil {
			if token, _, err := s.opts.Store.CreateSession(user.ID); err == nil && token != "" {
				data = bytes.Replace(data, []byte(pluginDefaultToken), []byte(`const DEFAULT_TOKEN = "`+token+`";`), 1)
			}
		}
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) handlePluginJS(w http.ResponseWriter, r *http.Request) {
	s.servePluginJS(w, r, webassets.PluginJS(), "musicdl-plugin.js")
}

func (s *Server) handleMintPluginJS(w http.ResponseWriter, r *http.Request) {
	s.servePluginJS(w, r, webassets.MintPluginJS(), "mint-plugin.js")
}
