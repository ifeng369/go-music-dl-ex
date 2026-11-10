package api

import (
	"bytes"
	"net/http"
	"strings"

	"musicdl-ex/internal/webassets"
)

func (s *Server) pluginTokenURLs(r *http.Request, token string) map[string]any {
	base := strings.TrimRight(baseURL(r), "/")
	return map[string]any{
		"token":              token,
		"mintPluginUrl":      base + "/plugin/u/" + token + "/mint-plugin.js",
		"musicFreePluginUrl": base + "/plugin.js",
	}
}

func (s *Server) handleGetPluginToken(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	if s.opts.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "用户服务不可用")
		return
	}
	token, err := s.opts.Store.EnsurePluginToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "生成插件地址失败")
		return
	}
	writeJSON(w, http.StatusOK, s.pluginTokenURLs(r, token))
}

func (s *Server) handleResetPluginToken(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	if s.opts.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "用户服务不可用")
		return
	}
	token, err := s.opts.Store.ResetPluginToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "重置插件地址失败")
		return
	}
	writeJSON(w, http.StatusOK, s.pluginTokenURLs(r, token))
}

func (s *Server) handlePersonalPlugin(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.PathValue("token"))
	file := r.PathValue("file")
	if token == "" || s.opts.Store == nil {
		http.NotFound(w, r)
		return
	}
	user, err := s.opts.Store.UserByPluginToken(token)
	if err != nil || user == nil {
		http.NotFound(w, r)
		return
	}
	switch file {
	case "mint-plugin.js":
		data := webassets.MintPluginJS()
		data = bytes.Replace(data, []byte(pluginDefaultToken), []byte(`const DEFAULT_TOKEN = "`+token+`";`), 1)
		s.servePluginJS(w, r, data, "mint-plugin.js")
	case "plugin.js":
		s.servePluginJS(w, r, webassets.PluginJS(), "musicdl-plugin.js")
	default:
		http.NotFound(w, r)
	}
}
