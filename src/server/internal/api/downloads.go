package api

import (
	"net/http"
	"strconv"
)

func (s *Server) handleListDownloadRecords(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	s.backfillDownloadCovers(r, user)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	limit := queryInt(r, "limit", 200)
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	items, err := s.opts.Store.ListDownloadRecords(user.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取下载记录失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleDeleteDownloadRecord(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	s.backfillDownloadCovers(r, user)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		writeError(w, http.StatusBadRequest, "无效的下载记录")
		return
	}
	if err := s.opts.Store.DeleteDownloadRecord(user.ID, uint(id)); err != nil {
		writeError(w, http.StatusInternalServerError, "删除下载记录失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleClearDownloadRecords(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	s.backfillDownloadCovers(r, user)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "请先登录")
		return
	}
	if err := s.opts.Store.ClearDownloadRecords(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "清空下载记录失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
