package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

type importPayload struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var payload importPayload
	if r.Method == http.MethodPost {
		if err := readJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
	} else {
		payload.URL = r.URL.Query().Get("url")
		payload.Type = r.URL.Query().Get("type")
	}
	link := extractLink(payload.URL)
	if link == "" {
		writeError(w, http.StatusBadRequest, "missing url")
		return
	}
	source := core.DetectSource(link)
	if source == "" {
		writeError(w, http.StatusBadRequest, "unable to detect source from url")
		return
	}

	typ := strings.ToLower(strings.TrimSpace(payload.Type))
	if typ == "" {
		typ = guessImportType(link)
	}

	result, err := s.doImport(r, source, link, typ)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) doImport(r *http.Request, source, link, typ string) (map[string]any, error) {
	var errs []string

	trySong := func() (map[string]any, bool) {
		fn := core.GetParseFunc(source)
		if fn == nil {
			return nil, false
		}
		song, err := fn(link)
		if err != nil || song == nil || song.ID == "" {
			if err != nil {
				errs = append(errs, "song: "+err.Error())
			}
			return nil, false
		}
		if song.Source == "" {
			song.Source = source
		}
		if song.Link == "" {
			song.Link = link
		}
		return map[string]any{"type": "music", "source": source, "music": toMusicItem(r, song)}, true
	}

	trySheet := func(kind string) (map[string]any, bool) {
		var fn func(string) (*model.Playlist, []model.Song, error)
		if kind == "album" {
			fn = core.GetParseAlbumFunc(source)
		} else {
			fn = core.GetParsePlaylistFunc(source)
		}
		if fn == nil {
			return nil, false
		}
		playlist, songs, err := fn(link)
		if err != nil || playlist == nil {
			if err != nil {
				errs = append(errs, kind+": "+err.Error())
			}
			return nil, false
		}
		if playlist.Source == "" {
			playlist.Source = source
		}
		if playlist.Link == "" {
			playlist.Link = link
		}
		for i := range songs {
			if songs[i].Source == "" {
				songs[i].Source = source
			}
		}
		return map[string]any{
			"type":      kind,
			"source":    source,
			"sheet":     toSheetItem(r, playlist, kind),
			"musicList": toMusicItems(r, songs),
		}, true
	}

	// Try the guessed type first, then the remaining types. Share links are
	// frequently mislabelled, so this is deliberately forgiving.
	order := []string{typ, "music", "sheet", "album"}
	seen := make(map[string]bool)
	for _, kind := range order {
		if kind == "" || seen[kind] {
			continue
		}
		seen[kind] = true
		switch kind {
		case "music", "song":
			if out, ok := trySong(); ok {
				return out, nil
			}
		case "sheet", "playlist":
			if out, ok := trySheet("playlist"); ok {
				return out, nil
			}
		case "album":
			if out, ok := trySheet("album"); ok {
				return out, nil
			}
		}
	}
	if len(errs) == 0 {
		return nil, errors.New("no parser available for this link")
	}
	return nil, errors.New(strings.Join(errs, "; "))
}

func guessImportType(link string) string {
	lower := strings.ToLower(link)
	switch {
	case strings.Contains(lower, "album"):
		return "album"
	case strings.Contains(lower, "playlist"), strings.Contains(lower, "playsquare"),
		strings.Contains(lower, "sheet"), strings.Contains(lower, "toplist"),
		strings.Contains(lower, "rank"):
		return "sheet"
	default:
		return "music"
	}
}

func extractLink(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if idx := strings.Index(raw, "http://"); idx >= 0 {
		raw = raw[idx:]
	} else if idx := strings.Index(raw, "https://"); idx >= 0 {
		raw = raw[idx:]
	} else {
		return ""
	}
	if idx := strings.IndexAny(raw, " \t\r\n\"'<>"); idx >= 0 {
		raw = raw[:idx]
	}
	return strings.TrimRight(raw, ".,;，。；)")
}
