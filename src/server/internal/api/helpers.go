package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/guohuiyuan/go-music-dl/core"
)

const maxRequestBody = 1 << 20

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, maxRequestBody)
	dec := json.NewDecoder(limited)
	return dec.Decode(dst)
}

func queryInt(r *http.Request, key string, def int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

func queryBool(r *http.Request, key string) bool {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// requestedSources reads source/sources/platform and normalises them.
func requestedSources(r *http.Request) []string {
	q := r.URL.Query()
	var raw []string
	raw = append(raw, q["source"]...)
	raw = append(raw, q["platform"]...)
	for _, v := range q["sources"] {
		raw = append(raw, strings.Split(v, ",")...)
	}
	seen := make(map[string]bool)
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func defaultSources(kind string) []string {
	switch kind {
	case "album":
		return core.GetAlbumSourceNames()
	case "sheet":
		return core.GetPlaylistSourceNames()
	case "toplist":
		return core.GetTopListSourceNames()
	case "category":
		return core.GetPlaylistCategorySourceNames()
	case "recommend":
		return core.GetRecommendSourceNames()
	case "user":
		return core.GetUserPlaylistSourceNames()
	default:
		return core.GetDefaultSourceNames()
	}
}

func allowedSources(kind string) map[string]bool {
	all := core.GetAllSourceNames()
	switch kind {
	case "album":
		all = core.GetAlbumSourceNames()
	case "sheet":
		all = core.GetPlaylistSourceNames()
	case "toplist":
		all = core.GetTopListSourceNames()
	case "category":
		all = core.GetPlaylistCategorySourceNames()
	case "recommend":
		all = core.GetRecommendSourceNames()
	case "user":
		all = core.GetUserPlaylistSourceNames()
	}
	allowed := make(map[string]bool, len(all))
	for _, s := range all {
		allowed[s] = true
	}
	return allowed
}

// sourcesFor resolves the requested source list for a capability. Unknown
// sources are preserved so the caller can report a useful per-source error.
func sourcesFor(r *http.Request, kind string) []string {
	requested := requestedSources(r)
	if len(requested) == 0 {
		return defaultSources(kind)
	}
	allowed := allowedSources(kind)
	// "all" (or a wildcard) expands to every supported source.
	for _, v := range requested {
		if v == "all" || v == "*" {
			return defaultSources(kind)
		}
	}
	out := make([]string, 0, len(requested))
	for _, v := range requested {
		if v == "local" {
			continue
		}
		if !allowed[v] {
			out = append(out, v)
			continue
		}
		out = append(out, v)
	}
	return out
}

func sourceAndID(r *http.Request) (source, id string) {
	source = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	if source == "" {
		source = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("platform")))
	}
	id = strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		id = strings.TrimSpace(r.URL.Query().Get("sourceId"))
	}
	if source == "" {
		source, id = splitItemID(id)
	} else if strings.HasPrefix(id, source+":") {
		id = strings.TrimPrefix(id, source+":")
	}
	return source, id
}

type itemMeta struct {
	Title       string
	Artist      string
	Artwork     string
	Description string
	Link        string
	Album       string
	WorksNum    int
	PlayCount   int64
	Duration    int
	Ext         string
}

func metaFromQuery(r *http.Request) itemMeta {
	q := r.URL.Query()
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(q.Get(k)); v != "" {
				return v
			}
		}
		return ""
	}
	works, _ := strconv.Atoi(pick("worksNum", "trackCount", "track_count"))
	playCount, _ := strconv.ParseInt(pick("playCount", "play_count"), 10, 64)
	duration, _ := strconv.Atoi(pick("duration"))
	return itemMeta{
		Title:       pick("title", "name"),
		Artist:      pick("artist", "creator"),
		Artwork:     pick("artwork", "cover"),
		Description: pick("description", "desc"),
		Link:        pick("link"),
		Album:       pick("album"),
		WorksNum:    works,
		PlayCount:   playCount,
		Duration:    duration,
		Ext:         pick("ext"),
	}
}

func parseExtra(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err == nil && len(m) > 0 {
		return m
	}
	var anyMap map[string]any
	if err := json.Unmarshal([]byte(raw), &anyMap); err == nil && len(anyMap) > 0 {
		out := make(map[string]string, len(anyMap))
		for k, v := range anyMap {
			out[k] = fmt.Sprint(v)
		}
		return out
	}
	if vals, err := url.ParseQuery(raw); err == nil && len(vals) > 0 {
		out := make(map[string]string, len(vals))
		for k, v := range vals {
			if len(v) > 0 {
				out[k] = v[0]
			}
		}
		return out
	}
	return nil
}

func pageParams(r *http.Request) (page, limit int) {
	page = queryInt(r, "page", 1)
	limit = queryInt(r, "limit", 500)
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 500
	}
	if limit > 2000 {
		limit = 2000
	}
	return page, limit
}

func slicePage[T any](items []T, page, limit int) ([]T, bool) {
	total := len(items)
	start := (page - 1) * limit
	if start >= total {
		return []T{}, true
	}
	end := start + limit
	if end > total {
		end = total
	}
	return items[start:end], end >= total
}

type fetchOutcome[T any] struct {
	source string
	items  []T
	err    error
}

// parallelFetch fans out to music sources with a small concurrency cap and a
// hard deadline. Some upstream providers are slow or hang; a partial result is
// always better than a stuck request.
func parallelFetch[T any](sources []string, timeout time.Duration, fn func(string) ([]T, error)) (map[string][]T, map[string]string) {
	results := make(map[string][]T, len(sources))
	errs := make(map[string]string)
	if len(sources) == 0 {
		return results, errs
	}

	ch := make(chan fetchOutcome[T], len(sources))
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	for _, src := range sources {
		wg.Add(1)
		go func(src string) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					ch <- fetchOutcome[T]{source: src, err: fmt.Errorf("panic: %v", p)}
				}
			}()
			sem <- struct{}{}
			defer func() { <-sem }()
			items, err := fn(src)
			ch <- fetchOutcome[T]{source: src, items: items, err: err}
		}(src)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	collect := func(o fetchOutcome[T]) {
		if o.err != nil {
			errs[o.source] = o.err.Error()
			return
		}
		results[o.source] = o.items
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case o := <-ch:
			collect(o)
		case <-done:
			for {
				select {
				case o := <-ch:
					collect(o)
				default:
					return results, errs
				}
			}
		case <-timer.C:
			for {
				select {
				case o := <-ch:
					collect(o)
				default:
					return results, errs
				}
			}
		}
	}
}

func maskSecret(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	runes := []rune(v)
	if len(runes) <= 8 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:4]) + "..." + string(runes[len(runes)-4:])
}
