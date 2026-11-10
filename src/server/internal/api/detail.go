package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

func (s *Server) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	s.handleSheetDetail(w, r, "playlist")
}

func (s *Server) handleAlbum(w http.ResponseWriter, r *http.Request) {
	s.handleSheetDetail(w, r, "album")
}

func (s *Server) handleSheetDetail(w http.ResponseWriter, r *http.Request, typ string) {
	source, id := sourceAndID(r)
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing id")
		return
	}
	if source == "" {
		writeError(w, http.StatusBadRequest, "missing source")
		return
	}

	var (
		fn   func(string) ([]model.Song, error)
		kind string
	)
	switch typ {
	case "album":
		fn = core.GetAlbumDetailFunc(source)
		kind = "album"
	default:
		fn = core.GetPlaylistDetailFunc(source)
		kind = "playlist"
	}
	if fn == nil {
		writeError(w, http.StatusBadRequest, "source does not support "+kind+" detail")
		return
	}

	songs, err := fn(id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	for i := range songs {
		if songs[i].Source == "" {
			songs[i].Source = source
		}
	}

	meta := metaFromQuery(r)
	sheet := SheetItem{
		Platform:    Platform,
		ID:          itemID(source, id),
		Source:      source,
		SourceID:    id,
		Title:       firstNonEmpty(meta.Title, id),
		Artwork:     coverURL(r, meta.Artwork, source),
		Cover:       meta.Artwork,
		Description: meta.Description,
		Artist:      meta.Artist,
		Link:        meta.Link,
		Type:        typ,
		WorksNum:    len(songs),
		PlayCount:   meta.PlayCount,
	}
	page, limit := pageParams(r)
	pageItems, isEnd := slicePage(songs, page, limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"isEnd":     isEnd,
		"page":      page,
		"total":     len(songs),
		"musicList": toMusicItems(r, pageItems),
		"sheetItem": sheet,
	})
}

func (s *Server) handleTopListDetail(w http.ResponseWriter, r *http.Request) {
	source, id := sourceAndID(r)
	if id == "" || source == "" {
		writeError(w, http.StatusBadRequest, "missing id or source")
		return
	}
	fn := core.GetTopListSongsFunc(source)
	if fn == nil {
		writeError(w, http.StatusBadRequest, "source does not support toplist detail")
		return
	}
	songs, err := fn(id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	for i := range songs {
		if songs[i].Source == "" {
			songs[i].Source = source
		}
	}
	meta := metaFromQuery(r)
	sheet := SheetItem{
		Platform:    Platform,
		ID:          itemID(source, id),
		Source:      source,
		SourceID:    id,
		Title:       firstNonEmpty(meta.Title, id),
		Artwork:     coverURL(r, meta.Artwork, source),
		Cover:       meta.Artwork,
		Description: meta.Description,
		Link:        meta.Link,
		Type:        "toplist",
		WorksNum:    len(songs),
		PlayCount:   meta.PlayCount,
	}
	page, limit := pageParams(r)
	pageItems, isEnd := slicePage(songs, page, limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"isEnd":     isEnd,
		"page":      page,
		"total":     len(songs),
		"musicList": toMusicItems(r, pageItems),
		"sheetItem": sheet,
	})
}

func (s *Server) handleTopLists(w http.ResponseWriter, r *http.Request) {
	sources := sourcesFor(r, "toplist")
	groups, errs := s.fetchTopListGroups(r, sources)
	flat := make([]SheetItem, 0)
	for _, g := range groups {
		flat = append(flat, g.Data...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": groups, "items": flat, "errors": errs})
}

func (s *Server) fetchTopListGroups(r *http.Request, sources []string) ([]TopListGroup, map[string]string) {
	results, errs := parallelFetch(sources, 18*time.Second, func(source string) ([]model.TopList, error) {
		fn := core.GetTopListsFunc(source)
		if fn == nil {
			return nil, fmt.Errorf("source does not support toplists")
		}
		items, err := fn()
		if err != nil {
			return nil, err
		}
		for i := range items {
			if items[i].Source == "" {
				items[i].Source = source
			}
		}
		return items, nil
	})

	groups := make([]TopListGroup, 0, len(sources))
	for _, source := range sources {
		lists := results[source]
		if len(lists) == 0 {
			continue
		}
		data := make([]SheetItem, 0, len(lists))
		for i := range lists {
			data = append(data, toTopListItem(r, &lists[i]))
		}
		groups = append(groups, TopListGroup{Source: source, Title: sourceDisplayName(source), Data: data})
	}
	return groups, errs
}

func (s *Server) handleRecommend(w http.ResponseWriter, r *http.Request) {
	sources := sourcesFor(r, "recommend")
	items, groups, errs := s.fetchRecommend(r, sources)
	_ = groups
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "errors": errs})
}

func (s *Server) fetchRecommend(r *http.Request, sources []string) ([]SheetItem, map[string][]SheetItem, map[string]string) {
	results, errs := parallelFetch(sources, 18*time.Second, func(source string) ([]model.Playlist, error) {
		fn := core.GetRecommendFunc(source)
		if fn == nil {
			return nil, fmt.Errorf("source does not support recommendation")
		}
		items, err := fn()
		if err != nil {
			return nil, err
		}
		for i := range items {
			if items[i].Source == "" {
				items[i].Source = source
			}
		}
		return items, nil
	})

	groups := make(map[string][]SheetItem, len(results))
	flat := make([]SheetItem, 0)
	for _, source := range sources {
		list := toSheetItems(r, results[source], "playlist")
		if len(list) == 0 {
			continue
		}
		groups[source] = list
		flat = append(flat, list...)
	}
	return flat, groups, errs
}

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	sources := sourcesFor(r, "category")
	items, errs := s.fetchCategories(sources)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "errors": errs})
}

func (s *Server) fetchCategories(sources []string) ([]CategoryItem, map[string]string) {
	results, errs := parallelFetch(sources, 15*time.Second, func(source string) ([]model.PlaylistCategory, error) {
		fn := core.GetPlaylistCategoriesFunc(source)
		if fn == nil {
			return nil, fmt.Errorf("source does not support categories")
		}
		items, err := fn()
		if err != nil {
			return nil, err
		}
		for i := range items {
			if items[i].Source == "" {
				items[i].Source = source
			}
		}
		return items, nil
	})

	out := make([]CategoryItem, 0)
	seen := make(map[string]bool)
	for _, source := range sources {
		for i := range results[source] {
			item := toCategoryItem(&results[source][i])
			key := item.Source + "\x00" + item.ID + "\x00" + item.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, item)
		}
	}
	return out, errs
}

func (s *Server) handleCategorySheets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	source := strings.ToLower(strings.TrimSpace(q.Get("source")))
	category := strings.TrimSpace(q.Get("category"))
	if category == "" {
		category = strings.TrimSpace(q.Get("id"))
	}
	// An empty category id means "all categories" upstream. MusicFree clients may
	// send the display name instead of an empty id, so normalize it here.
	if category == "全部" || strings.EqualFold(category, "all") {
		category = ""
	}
	if source == "" || source == "all" {
		source = core.GetPlaylistCategorySourceNames()[0]
	}
	fn := core.GetCategoryPlaylistsFunc(source)
	if fn == nil {
		writeError(w, http.StatusBadRequest, "source does not support category playlists")
		return
	}
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 30)
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	type categoryResult struct {
		playlists []model.Playlist
		err       error
	}
	resultCh := make(chan categoryResult, 1)
	go func() {
		playlists, err := fn(category, page, limit)
		resultCh <- categoryResult{playlists: playlists, err: err}
	}()

	var result categoryResult
	select {
	case result = <-resultCh:
	case <-time.After(15 * time.Second):
		writeError(w, http.StatusGatewayTimeout, "category playlists request timed out")
		return
	}
	if result.err != nil {
		writeError(w, http.StatusBadGateway, result.err.Error())
		return
	}
	playlists := result.playlists
	for i := range playlists {
		if playlists[i].Source == "" {
			playlists[i].Source = source
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  toSheetItems(r, playlists, "playlist"),
		"page":   page,
		"isEnd":  len(playlists) < limit,
		"source": source,
	})
}

func (s *Server) handleUserPlaylists(w http.ResponseWriter, r *http.Request) {
	source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	if source == "" {
		source = "netease"
	}
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 50)
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var (
		playlists []model.Playlist
		err       error
	)
	if source == "kuwo" {
		playlists, err = fetchKuwoUserPlaylists(core.CM.Get("kuwo"), page, limit)
	} else {
		fn := core.GetUserPlaylistsFunc(source)
		if fn == nil {
			writeError(w, http.StatusBadRequest, "source does not support user playlists")
			return
		}
		playlists, err = fn(page, limit)
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	for i := range playlists {
		if playlists[i].Source == "" {
			playlists[i].Source = source
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  toSheetItems(r, playlists, "playlist"),
		"page":   page,
		"isEnd":  len(playlists) < limit,
		"source": source,
	})
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	recSources := core.GetRecommendSourceNames()
	categorySources := []string{"netease", "qq", "kugou"}
	if source != "" && source != "all" {
		recSources = []string{source}
		categorySources = []string{source}
	}

	recommend, groups, recErrs := s.fetchRecommend(r, recSources)
	categories, catErrs := s.fetchCategories(categorySources)

	recommendGroups := make([]SheetGroup, 0, len(recSources))
	for _, recSource := range recSources {
		list := groups[recSource]
		if len(list) == 0 {
			continue
		}
		recommendGroups = append(recommendGroups, SheetGroup{
			Source: recSource,
			Title:  sourceDisplayName(recSource),
			Data:   list,
		})
	}

	errs := make(map[string]string)
	for _, m := range []map[string]string{recErrs, catErrs} {
		for k, v := range m {
			errs[k] = v
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"recommend":       recommend,
		"recommendGroups": recommendGroups,
		"toplists":        []TopListGroup{},
		"categories":      categories,
		"errors":          errs,
	})
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
