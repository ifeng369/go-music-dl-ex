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

// minAlternativeScore is intentionally a little forgiving: live versions and
// slightly different titles are common, but unrelated songs must not appear in
// the source switcher. core.CalcSongSimilarity weights the title at 70%.
const minAlternativeScore = 0.62

type alternativeCandidate struct {
	Item        MusicItem
	Score       float64
	ExactArtist bool
}

// unknownArtist reports whether an artist field carries no usable identity.
func unknownArtist(raw string) bool {
	normalized := core.NormalizeText(raw)
	return normalized == "" || normalized == "未知歌手" || normalized == "unknown" || normalized == "unknownartist"
}

// sameArtistName treats collaborations and platform-specific name variants as
// the same act ("周杰伦" vs "周杰伦、费玉清", "G.E.M.邓紫棋" vs "邓紫棋"), but a
// cover by a different singer never matches.
func sameArtistName(wanted, candidate string) bool {
	a := core.NormalizeText(wanted)
	b := core.NormalizeText(candidate)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if len([]rune(b)) >= 2 && strings.Contains(b, a) {
		return true
	}
	return len([]rune(a)) >= 2 && strings.Contains(a, b)
}

// handleAlternatives searches every other music source for the same song so the
// player can offer a manual source switcher and automatically fall back when a
// stream is unavailable. One best candidate per source is returned.
func (s *Server) handleAlternatives(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.URL.Query().Get("title"))
	if title == "" {
		title = strings.TrimSpace(r.URL.Query().Get("name"))
	}
	artist := strings.TrimSpace(r.URL.Query().Get("artist"))
	if title == "" {
		writeError(w, http.StatusBadRequest, "missing title")
		return
	}
	currentSource := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("source")))
	if currentSource == "" {
		currentSource = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("from")))
	}
	duration := queryInt(r, "duration", 0)

	// The `source` parameter means the platform currently playing, not a
	// candidate filter. Search across every known source so a broken stream can
	// fall back to any other platform. `sources` can still narrow the list.
	sources := core.GetAllSourceNames()
	if raw := strings.TrimSpace(r.URL.Query().Get("sources")); raw != "" {
		sources = nil
		for _, v := range strings.Split(raw, ",") {
			if v = strings.ToLower(strings.TrimSpace(v)); v != "" {
				sources = append(sources, v)
			}
		}
	}
	filtered := make([]string, 0, len(sources))
	for _, source := range sources {
		if source == "" || source == "local" || source == currentSource {
			continue
		}
		filtered = append(filtered, source)
	}
	if len(filtered) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"items": []MusicItem{}, "errors": map[string]string{}})
		return
	}

	keyword := strings.TrimSpace(title + " " + artist)
	results, errs := parallelFetch(filtered, 12*time.Second, func(source string) ([]model.Song, error) {
		fn := core.GetSearchFunc(source)
		if fn == nil {
			return nil, fmt.Errorf("source does not support search")
		}
		items, err := fn(keyword)
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

	artistKnown := !unknownArtist(artist)
	candidates := make([]alternativeCandidate, 0, len(filtered))
	for _, source := range filtered {
		bestScore := -1.0
		var best *model.Song
		for i := range results[source] {
			song := &results[source][i]
			if strings.TrimSpace(song.Name) == "" {
				continue
			}
			// Prefer the same singer: never fall back to a cover or another
			// artist that happens to use the same title.
			if artistKnown && !sameArtistName(artist, song.Artist) {
				continue
			}
			score := core.CalcSongSimilarity(title, artist, song.Name, song.Artist)
			// Prefer the version with a similar running time, but do not
			// exclude live/remastered versions that are still valid matches.
			if duration > 0 && song.Duration > 0 {
				diff := core.IntAbs(song.Duration - duration)
				if diff > 10 && diff > duration/10 {
					score -= 0.05
				}
			}
			if score > bestScore {
				bestScore = score
				best = song
			}
		}
		if best == nil || bestScore < minAlternativeScore {
			continue
		}
		item := toMusicItem(r, best)
		if item.Source == "" {
			item.Source = source
			item.ID = itemID(source, best.ID)
		}
		exactArtist := core.NormalizeText(artist) == core.NormalizeText(best.Artist)
		candidates = append(candidates, alternativeCandidate{
			Item:        item,
			Score:       bestScore,
			ExactArtist: exactArtist,
		})
	}

	// Same singer first (prefer the exact name over collaborations), then the
	// closest title match. sort.SliceStable keeps the configured source order
	// for equally good matches.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].ExactArtist != candidates[j].ExactArtist {
			return candidates[i].ExactArtist
		}
		return candidates[i].Score > candidates[j].Score
	})
	items := make([]MusicItem, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, candidate.Item)
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items, "errors": errs})
}
