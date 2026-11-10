package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

type searchResult struct {
	Items  any               `json:"items"`
	Page   int               `json:"page"`
	IsEnd  bool              `json:"isEnd"`
	Errors map[string]string `json:"errors,omitempty"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		q = strings.TrimSpace(r.URL.Query().Get("keyword"))
	}
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing q")
		return
	}

	typ := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	switch typ {
	case "", "music", "song":
		typ = "music"
	case "sheet", "playlist":
		typ = "sheet"
	case "album":
		typ = "album"
	case "artist":
		typ = "artist"
	case "lyric":
		writeJSON(w, http.StatusOK, searchResult{Items: []MusicItem{}, Page: 1, IsEnd: true, Errors: map[string]string{"lyric": "lyric search is not supported"}})
		return
	default:
		writeError(w, http.StatusBadRequest, "unsupported type: "+typ)
		return
	}

	page := queryInt(r, "page", 1)
	if page < 1 {
		page = 1
	}
	// Upstream search endpoints do not expose pagination. Returning an empty
	// page after the first one stops MusicFree's infinite scroll cleanly.
	if page > 1 {
		var empty any = []MusicItem{}
		if typ == "sheet" || typ == "album" {
			empty = []SheetItem{}
		}
		if typ == "artist" {
			empty = []ArtistItem{}
		}
		writeJSON(w, http.StatusOK, searchResult{Items: empty, Page: page, IsEnd: true})
		return
	}

	perSource := queryInt(r, "limit", 30)
	if perSource <= 0 || perSource > 100 {
		perSource = 30
	}
	sources := sourcesFor(r, typ)

	switch typ {
	case "music":
		items, errs := s.searchSongs(r, q, sources, perSource)
		writeJSON(w, http.StatusOK, searchResult{Items: items, Page: page, IsEnd: true, Errors: errs})
	case "sheet":
		items, errs := s.searchSheets(r, q, sources, perSource)
		writeJSON(w, http.StatusOK, searchResult{Items: items, Page: page, IsEnd: true, Errors: errs})
	case "album":
		items, errs := s.searchAlbums(r, q, sources, perSource)
		writeJSON(w, http.StatusOK, searchResult{Items: items, Page: page, IsEnd: true, Errors: errs})
	case "artist":
		items, errs := s.searchArtists(r, q, sources, perSource)
		writeJSON(w, http.StatusOK, searchResult{Items: items, Page: page, IsEnd: true, Errors: errs})
	}
}

func (s *Server) searchSongs(r *http.Request, keyword string, sources []string, perSource int) ([]MusicItem, map[string]string) {
	results, errs := parallelFetch(sources, 18*time.Second, func(source string) ([]model.Song, error) {
		fn := core.GetSearchFunc(source)
		if fn == nil {
			return nil, fmt.Errorf("source does not support search")
		}
		items, err := fn(keyword)
		if err != nil {
			return nil, err
		}
		if len(items) > perSource {
			items = items[:perSource]
		}
		for i := range items {
			if items[i].Source == "" {
				items[i].Source = source
			}
		}
		return items, nil
	})

	out := make([]MusicItem, 0, len(sources)*perSource)
	seen := make(map[string]bool)
	for _, source := range sources {
		for i := range results[source] {
			item := toMusicItem(r, &results[source][i])
			if item.ID == "" || seen[item.ID] {
				continue
			}
			seen[item.ID] = true
			out = append(out, item)
		}
	}
	return out, errs
}

func (s *Server) searchSheets(r *http.Request, keyword string, sources []string, perSource int) ([]SheetItem, map[string]string) {
	results, errs := parallelFetch(sources, 18*time.Second, func(source string) ([]model.Playlist, error) {
		fn := core.GetPlaylistSearchFunc(source)
		if fn == nil {
			return nil, fmt.Errorf("source does not support playlist search")
		}
		items, err := fn(keyword)
		if err != nil {
			return nil, err
		}
		if len(items) > perSource {
			items = items[:perSource]
		}
		for i := range items {
			if items[i].Source == "" {
				items[i].Source = source
			}
		}
		return items, nil
	})

	out := make([]SheetItem, 0, len(sources)*perSource)
	seen := make(map[string]bool)
	for _, source := range sources {
		for i := range results[source] {
			item := toSheetItem(r, &results[source][i], "playlist")
			if item.ID == "" || seen[item.ID] {
				continue
			}
			seen[item.ID] = true
			out = append(out, item)
		}
	}
	return out, errs
}

func (s *Server) searchAlbums(r *http.Request, keyword string, sources []string, perSource int) ([]SheetItem, map[string]string) {
	results, errs := parallelFetch(sources, 18*time.Second, func(source string) ([]model.Playlist, error) {
		fn := core.GetAlbumSearchFunc(source)
		if fn == nil {
			return nil, fmt.Errorf("source does not support album search")
		}
		items, err := fn(keyword)
		if err != nil {
			return nil, err
		}
		if len(items) > perSource {
			items = items[:perSource]
		}
		for i := range items {
			if items[i].Source == "" {
				items[i].Source = source
			}
		}
		return items, nil
	})

	out := make([]SheetItem, 0, len(sources)*perSource)
	seen := make(map[string]bool)
	for _, source := range sources {
		for i := range results[source] {
			item := toSheetItem(r, &results[source][i], "album")
			if item.ID == "" || seen[item.ID] {
				continue
			}
			seen[item.ID] = true
			out = append(out, item)
		}
	}
	return out, errs
}

type artistBucket struct {
	Name    string
	Source  string
	Avatar  string
	Works   int
	Sources map[string]bool
}

func (s *Server) searchArtists(r *http.Request, keyword string, sources []string, perSource int) ([]ArtistItem, map[string]string) {
	songs, errs := s.searchSongs(r, keyword, sources, perSource)
	buckets := make(map[string]*artistBucket)
	for _, song := range songs {
		names := splitArtists(song.Artist)
		if len(names) == 0 {
			continue
		}
		for _, name := range names {
			key := song.Source + "\x00" + strings.ToLower(name)
			b, ok := buckets[key]
			if !ok {
				b = &artistBucket{Name: name, Source: song.Source, Avatar: song.Artwork, Sources: map[string]bool{}}
				buckets[key] = b
			}
			b.Works++
			if b.Avatar == "" {
				b.Avatar = song.Artwork
			}
		}
	}

	artists := make([]ArtistItem, 0, len(buckets))
	for _, b := range buckets {
		artists = append(artists, toArtistItem(b.Source, b.Name, b.Works, b.Avatar))
	}
	sort.SliceStable(artists, func(i, j int) bool {
		if artists[i].WorksNum == artists[j].WorksNum {
			return artists[i].Name < artists[j].Name
		}
		return artists[i].WorksNum > artists[j].WorksNum
	})
	if len(artists) > 100 {
		artists = artists[:100]
	}
	return artists, errs
}

func splitArtists(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case '/', '\\', ',', '，', '、', ';', '；', '&', '＆', '|':
			return true
		}
		return false
	})
	out := make([]string, 0, len(fields))
	seen := make(map[string]bool)
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" || seen[strings.ToLower(f)] {
			continue
		}
		seen[strings.ToLower(f)] = true
		out = append(out, f)
	}
	return out
}

func (s *Server) handleArtistWorks(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		name = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing name")
		return
	}
	source := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	sources := []string{source}
	if source == "" {
		sources = sourcesFor(r, "music")
	}
	page, limit := pageParams(r)
	if limit > 100 {
		limit = 100
	}
	items, errs := s.searchSongs(r, name, sources, limit)
	// MusicFree asks for artist works page by page; search only has one page.
	if page > 1 {
		items = []MusicItem{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"isEnd":  page > 1,
		"errors": errs,
		"page":   page,
	})
}
