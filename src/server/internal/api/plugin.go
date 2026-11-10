package api

import (
	"bytes"
	"net/http"
	"strings"

	"musicdl-ex/internal/webassets"
)

const pluginDefaultBase = `const DEFAULT_BASE = "http://127.0.0.1:8080";`

func (s *Server) handlePluginJS(w http.ResponseWriter, r *http.Request) {
	data := webassets.PluginJS()
	base := strings.TrimRight(baseURL(r), "/")
	if base != "" {
		replacement := `const DEFAULT_BASE = "` + base + `";`
		data = bytes.Replace(data, []byte(pluginDefaultBase), []byte(replacement), 1)
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Disposition", `inline; filename="musicdl-plugin.js"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}