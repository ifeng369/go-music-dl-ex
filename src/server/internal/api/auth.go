package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type userContextKey struct{}

func withUser(r *http.Request, user *AppUser) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey{}, user))
}

func userFromContext(r *http.Request) *AppUser {
	user, _ := r.Context().Value(userContextKey{}).(*AppUser)
	return user
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

func (s *Server) sessionToken(r *http.Request) string {
	if token := bearerToken(r); token != "" {
		return token
	}
	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	// Media elements cannot attach an Authorization header. Allow a token query
	// parameter for plugin/stream consumers while keeping the normal paths.
	return strings.TrimSpace(r.URL.Query().Get("access_token"))
}

func (s *Server) currentUser(r *http.Request) *AppUser {
	if user := userFromContext(r); user != nil {
		return user
	}
	if s.opts.Store == nil {
		return nil
	}
	user, err := s.opts.Store.GetSession(s.sessionToken(r))
	if err != nil {
		return nil
	}
	return user
}

func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.opts.Store == nil {
			writeError(w, http.StatusServiceUnavailable, "用户服务未启用")
			return
		}
		user := s.currentUser(r)
		if user == nil {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		next(w, withUser(r, user))
	}
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r)
		if user == nil || user.Role != "admin" {
			writeError(w, http.StatusForbidden, "仅管理员可操作")
			return
		}
		next(w, r)
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.opts.Store == nil {
		writeError(w, http.StatusServiceUnavailable, "用户服务未启用")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	user, err := s.opts.Store.Authenticate(input.Username, input.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	token, expires, err := s.opts.Store.CreateSession(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "登录失败")
		return
	}
	s.setSessionCookie(w, r, token, expires)
	writeJSON(w, http.StatusOK, map[string]any{
		"user":      user,
		"token":     token,
		"expiresAt": expires,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := s.sessionToken(r)
	if token != "" && s.opts.Store != nil {
		_ = s.opts.Store.DeleteSession(token)
	}
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := s.currentUser(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.opts.Store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取用户失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": users})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	user, err := s.opts.Store.CreateUser(input.Username, input.Password, input.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(w, http.StatusBadRequest, "用户 ID 错误")
		return
	}
	current := userFromContext(r)
	if current != nil && current.ID == uint(id) {
		writeError(w, http.StatusBadRequest, "不能删除当前登录账号")
		return
	}
	target, err := s.opts.Store.GetUser(uint(id))
	if err != nil {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}
	if target.Role == "admin" {
		count, err := s.opts.Store.CountAdmins()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "读取用户失败")
			return
		}
		if count <= 1 {
			writeError(w, http.StatusBadRequest, "至少需要保留一个管理员")
			return
		}
	}
	if err := s.opts.Store.DeleteUser(uint(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "删除用户失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleUpdatePassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(w, http.StatusBadRequest, "用户 ID 错误")
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := s.opts.Store.UpdatePassword(uint(id), input.Password); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, http.StatusNotFound, "用户不存在")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGetLibrary(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	data, err := s.opts.Store.GetLibrary(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取音乐库失败")
		return
	}
	raw := json.RawMessage(`{}`)
	if data != "" && json.Valid([]byte(data)) {
		raw = json.RawMessage(data)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": raw})
}

func (s *Server) handleSaveLibrary(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "读取请求失败")
		return
	}
	if len(body) == 0 || !json.Valid(body) {
		writeError(w, http.StatusBadRequest, "音乐库数据格式错误")
		return
	}
	if err := s.opts.Store.SaveLibrary(user.ID, string(body)); err != nil {
		writeError(w, http.StatusInternalServerError, "保存音乐库失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) downloadDir() string {
	if s.opts.Store != nil {
		return s.opts.Store.GetSetting("downloadDir", "data/downloads")
	}
	return "data/downloads"
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	dir := s.downloadDir()
	writeJSON(w, http.StatusOK, map[string]any{
		"downloadDir":        dir,
		"defaultDownloadDir": "data/downloads",
	})
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DownloadDir string `json:"downloadDir"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	dir := strings.TrimSpace(input.DownloadDir)
	if dir == "" {
		writeError(w, http.StatusBadRequest, "下载目录不能为空")
		return
	}
	dir = filepath.Clean(dir)
	if err := s.opts.Store.SetSetting("downloadDir", dir); err != nil {
		writeError(w, http.StatusInternalServerError, "保存设置失败")
		return
	}
	_ = ensureDir(dir)
	writeJSON(w, http.StatusOK, map[string]any{"downloadDir": dir})
}
