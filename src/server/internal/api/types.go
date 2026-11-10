package api

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/guohuiyuan/go-music-dl/core"
	"github.com/guohuiyuan/music-lib/model"
)

const (
	// Platform is the MusicFree platform id. Keep it stable: it is embedded in
	// every item id returned to clients.
	Platform = "go-music-dl"
)

type MusicItem struct {
	Platform  string            `json:"platform"`
	ID        string            `json:"id"`
	Source    string            `json:"source"`
	SourceID  string            `json:"sourceId"`
	Title     string            `json:"title"`
	Artist    string            `json:"artist"`
	Album     string            `json:"album,omitempty"`
	AlbumID   string            `json:"albumId,omitempty"`
	Duration  int               `json:"duration,omitempty"`
	Artwork   string            `json:"artwork,omitempty"`
	Cover     string            `json:"cover,omitempty"`
	Ext       string            `json:"ext,omitempty"`
	Bitrate   int               `json:"bitrate,omitempty"`
	Size      int64             `json:"size,omitempty"`
	URL       string            `json:"url,omitempty"`
	Link      string            `json:"link,omitempty"`
	IsVIP     bool              `json:"isVip,omitempty"`
	IsInvalid bool              `json:"isInvalid,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

type SheetItem struct {
	Platform    string            `json:"platform"`
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	SourceID    string            `json:"sourceId"`
	Title       string            `json:"title"`
	Artwork     string            `json:"artwork,omitempty"`
	Cover       string            `json:"cover,omitempty"`
	Description string            `json:"description,omitempty"`
	Artist      string            `json:"artist,omitempty"`
	Link        string            `json:"link,omitempty"`
	Type        string            `json:"type"` // playlist | album | toplist
	WorksNum    int               `json:"worksNum,omitempty"`
	PlayCount   int64             `json:"playCount,omitempty"`
	Extra       map[string]string `json:"extra,omitempty"`
}

type ArtistItem struct {
	Platform    string `json:"platform"`
	ID          string `json:"id"`
	Source      string `json:"source"`
	SourceID    string `json:"sourceId"`
	Name        string `json:"name"`
	Avatar      string `json:"avatar,omitempty"`
	Description string `json:"description,omitempty"`
	WorksNum    int    `json:"worksNum,omitempty"`
}

type CategoryItem struct {
	Platform string `json:"platform"`
	Source   string `json:"source"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Group    string `json:"group,omitempty"`
	Count    int    `json:"count,omitempty"`
	Hot      bool   `json:"hot,omitempty"`
}

type TopListGroup struct {
	Source string      `json:"source"`
	Title  string      `json:"title"`
	Data   []SheetItem `json:"data"`
}

// SheetGroup is used for per-source recommendation sections on the discover
// page. Keeping the source on the group lets the web client render one row per
// platform instead of a mixed, deduplicated list.
type SheetGroup struct {
	Source string      `json:"source"`
	Title  string      `json:"title"`
	Data   []SheetItem `json:"data"`
}

type SourceInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Search        bool   `json:"search"`
	Album         bool   `json:"album"`
	Sheet         bool   `json:"sheet"`
	TopList       bool   `json:"toplist"`
	Categories    bool   `json:"categories"`
	Recommend     bool   `json:"recommend"`
	UserPlaylists bool   `json:"userPlaylists"`
	RequiresLogin bool   `json:"requiresLogin"`
}

func itemID(source, id string) string {
	if source == "" {
		return id
	}
	if strings.HasPrefix(id, source+":") {
		return id
	}
	return source + ":" + id
}

func splitItemID(composite string) (source, id string) {
	if idx := strings.Index(composite, ":"); idx > 0 {
		return composite[:idx], composite[idx+1:]
	}
	return "", composite
}

func coverURL(r *http.Request, cover, source string) string {
	cover = strings.TrimSpace(cover)
	if cover == "" {
		return ""
	}
	if !strings.HasPrefix(cover, "http://") && !strings.HasPrefix(cover, "https://") {
		return cover
	}
	// Clients pass the already-proxied artwork URL back as metadata; do not
	// wrap our own cover endpoint in another proxy layer.
	if strings.Contains(cover, "/api/cover?") {
		return cover
	}
	return baseURL(r) + "/api/cover?source=" + url.QueryEscape(source) + "&url=" + url.QueryEscape(cover)
}

func toMusicItem(r *http.Request, song *model.Song) MusicItem {
	if song == nil {
		return MusicItem{Platform: Platform}
	}
	source := song.Source
	if source == "" {
		source, _ = splitItemID(song.ID)
	}
	art := coverURL(r, song.Cover, source)
	return MusicItem{
		Platform:  Platform,
		ID:        itemID(source, song.ID),
		Source:    source,
		SourceID:  song.ID,
		Title:     song.Name,
		Artist:    song.Artist,
		Album:     song.Album,
		AlbumID:   song.AlbumID,
		Duration:  song.Duration,
		Artwork:   art,
		Cover:     song.Cover,
		Ext:       strings.TrimPrefix(strings.ToLower(song.Ext), "."),
		Bitrate:   song.Bitrate,
		Size:      song.Size,
		URL:       song.URL,
		Link:      song.Link,
		IsVIP:     song.IsVIP,
		IsInvalid: song.IsInvalid,
		Extra:     song.Extra,
	}
}

func toMusicItems(r *http.Request, songs []model.Song) []MusicItem {
	out := make([]MusicItem, 0, len(songs))
	for i := range songs {
		out = append(out, toMusicItem(r, &songs[i]))
	}
	return out
}

func toSheetItem(r *http.Request, playlist *model.Playlist, typ string) SheetItem {
	if playlist == nil {
		return SheetItem{Platform: Platform, Type: typ}
	}
	source := playlist.Source
	if source == "" {
		source, _ = splitItemID(playlist.ID)
	}
	return SheetItem{
		Platform:    Platform,
		ID:          itemID(source, playlist.ID),
		Source:      source,
		SourceID:    playlist.ID,
		Title:       playlist.Name,
		Artwork:     coverURL(r, playlist.Cover, source),
		Cover:       playlist.Cover,
		Description: playlist.Description,
		Artist:      playlist.Creator,
		Link:        playlist.Link,
		Type:        typ,
		WorksNum:    playlist.TrackCount,
		PlayCount:   int64(playlist.PlayCount),
		Extra:       playlist.Extra,
	}
}

func toSheetItems(r *http.Request, playlists []model.Playlist, typ string) []SheetItem {
	out := make([]SheetItem, 0, len(playlists))
	for i := range playlists {
		out = append(out, toSheetItem(r, &playlists[i], typ))
	}
	return out
}

func toTopListItem(r *http.Request, top *model.TopList) SheetItem {
	if top == nil {
		return SheetItem{Platform: Platform, Type: "toplist"}
	}
	source := top.Source
	if source == "" {
		source, _ = splitItemID(top.ID)
	}
	return SheetItem{
		Platform:    Platform,
		ID:          itemID(source, top.ID),
		Source:      source,
		SourceID:    top.ID,
		Title:       top.Name,
		Artwork:     coverURL(r, top.Cover, source),
		Cover:       top.Cover,
		Description: top.Description,
		Link:        top.Link,
		Type:        "toplist",
		WorksNum:    top.TrackCount,
		PlayCount:   top.PlayCount,
		Extra:       top.Extra,
	}
}

func toCategoryItem(c *model.PlaylistCategory) CategoryItem {
	if c == nil {
		return CategoryItem{Platform: Platform}
	}
	return CategoryItem{
		Platform: Platform,
		Source:   c.Source,
		ID:       c.ID,
		Name:     c.Name,
		Group:    c.Group,
		Count:    c.Count,
		Hot:      c.Hot,
	}
}

func toArtistItem(source, name string, works int, avatar string) ArtistItem {
	return ArtistItem{
		Platform: Platform,
		ID:       itemID(source, "artist:"+name),
		Source:   source,
		SourceID: name,
		Name:     name,
		Avatar:   avatar,
		WorksNum: works,
	}
}

func baseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := firstHeaderValue(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	}
	host := r.Host
	if forwarded := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = forwarded
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return scheme + "://" + host
}

func firstHeaderValue(v string) string {
	if idx := strings.IndexByte(v, ','); idx >= 0 {
		v = v[:idx]
	}
	return strings.TrimSpace(v)
}

func sourceDisplayName(source string) string {
	name := core.GetSourceDescription(source)
	if name == "" {
		return source
	}
	return name
}
