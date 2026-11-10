package api

import (
	"net/http"
	"strings"

	"github.com/guohuiyuan/go-music-dl/core"
)

// backfillDownloadCovers fills cover metadata for download records created
// before covers were stored. It first searches the original source, then falls
// back to the main sources because some providers (notably Kuwo) often return
// search results without artwork even when the song exists elsewhere.
func (s *Server) backfillDownloadCovers(r *http.Request, user *AppUser) {
	if s == nil || s.opts.Store == nil || user == nil {
		return
	}
	records, err := s.opts.Store.ListDownloadRecords(user.ID, 200)
	if err != nil {
		return
	}
	for _, record := range records {
		if strings.TrimSpace(record.Cover) != "" {
			continue
		}
		title := strings.TrimSpace(record.Title)
		artist := strings.TrimSpace(record.Artist)
		if title == "" {
			continue
		}
		query := strings.TrimSpace(title + " " + artist)
		bestScore := 0.0
		bestCover := ""

		try := func(source string) {
			source = strings.ToLower(strings.TrimSpace(source))
			if source == "" || source == "local" {
				return
			}
			search := core.GetSearchFunc(source)
			if search == nil {
				return
			}
			songs, searchErr := search(query)
			if searchErr != nil {
				return
			}
			for i := range songs {
				song := songs[i]
				if strings.TrimSpace(song.Cover) == "" {
					continue
				}
				score := core.CalcSongSimilarity(title, artist, song.Name, song.Artist)
				if score <= bestScore {
					continue
				}
				if song.Source == "" {
					song.Source = source
				}
				cover := toMusicItem(r, &song).Artwork
				if cover == "" {
					continue
				}
				bestScore = score
				bestCover = cover
			}
		}

		try(record.Source)
		if bestCover == "" || bestScore < 0.6 {
			for _, fallback := range []string{"netease", "kugou", "qq"} {
				if strings.EqualFold(fallback, record.Source) {
					continue
				}
				try(fallback)
			}
		}

		if bestScore >= 0.45 && bestCover != "" {
			_ = s.opts.Store.UpdateDownloadRecordCover(user.ID, record.ID, bestCover)
		}
	}
}
