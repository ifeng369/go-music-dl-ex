package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

// Sync coverage: every piece of user data that can meaningfully be shared
// between the standalone client and a server. Audio bytes, resolved URLs and
// download records stay local.
const (
	syncKindFavorites       = "favorites"
	syncKindHistory         = "history"
	syncKindPlaylists       = "playlists"
	syncKindSheetFavorites  = "sheetFavorites"
	syncKindOverrides       = "sourceOverrides"
	syncKindPlayerQueue     = "playerQueue"
	syncKindRecs            = "recs"
	syncKindCookies         = "cookies"
	syncSingletonKey        = "all"
	syncTombstoneRetention  = 90 * 24 * time.Hour
	maxSyncRequestBodyBytes = 16 << 20
)

// syncItem is one versioned record. A non-zero DeletedAt is a tombstone: the
// record must be removed on the other side instead of being re-created.
type syncItem struct {
	Kind      string          `json:"kind"`
	Key       string          `json:"key"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	UpdatedAt int64           `json:"updatedAt"`
	DeletedAt int64           `json:"deletedAt,omitempty"`
}

// appSyncVersion stores the last known version of each record so two clients
// can merge with last-write-wins. The library blob stays the single source of
// truth; this table only carries timestamps and tombstones.
type appSyncVersion struct {
	UserID    uint   `gorm:"primaryKey;autoIncrement:false"`
	Kind      string `gorm:"primaryKey;size:32"`
	ItemKey   string `gorm:"column:item_key;primaryKey;size:255"`
	UpdatedAt int64
	DeletedAt int64
}

func (appSyncVersion) TableName() string { return "app_sync_versions" }

func nowMillis() int64 { return time.Now().UnixMilli() }

func syncRecordTime(updatedAt, deletedAt int64) int64 {
	if deletedAt > updatedAt {
		return deletedAt
	}
	return updatedAt
}

func syncCollectionKind(kind string) bool {
	switch kind {
	case syncKindFavorites, syncKindHistory, syncKindPlaylists, syncKindSheetFavorites:
		return true
	}
	return false
}

func syncSingletonKind(kind string) bool {
	return kind == syncKindPlayerQueue || kind == syncKindRecs
}

func syncKnownKind(kind string) bool {
	return syncCollectionKind(kind) || syncSingletonKind(kind) ||
		kind == syncKindOverrides || kind == syncKindCookies
}

func (s *UserStore) ensureSyncTable() error {
	return s.db.AutoMigrate(&appSyncVersion{})
}

func syncVersionKey(kind, key string) string { return kind + "\x00" + key }

func (s *UserStore) syncVersions(userID uint) (map[string]appSyncVersion, error) {
	if err := s.ensureSyncTable(); err != nil {
		return nil, err
	}
	var rows []appSyncVersion
	if err := s.db.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]appSyncVersion, len(rows))
	for _, row := range rows {
		out[syncVersionKey(row.Kind, row.ItemKey)] = row
	}
	return out, nil
}

func (s *UserStore) upsertSyncVersions(rows []appSyncVersion) error {
	if len(rows) == 0 {
		return nil
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "kind"}, {Name: "item_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "deleted_at"}),
	}).Create(&rows).Error
}

func (s *UserStore) touchSyncVersion(userID uint, kind, key string, updatedAt, deletedAt int64) error {
	return s.upsertSyncVersions([]appSyncVersion{{
		UserID:    userID,
		Kind:      kind,
		ItemKey:   key,
		UpdatedAt: updatedAt,
		DeletedAt: deletedAt,
	}})
}

// ---------------------------------------------------------------- library

func decodeLibraryDoc(raw string) map[string]json.RawMessage {
	doc := make(map[string]json.RawMessage)
	if strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &doc)
	}
	return doc
}

func syncItemKey(raw json.RawMessage) string {
	var probe struct {
		ID     any    `json:"id"`
		Source string `json:"source"`
		Type   string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	id := ""
	switch value := probe.ID.(type) {
	case string:
		id = strings.TrimSpace(value)
	case float64:
		id = strconv.FormatInt(int64(value), 10)
	}
	if id == "" {
		return ""
	}
	// Sheet favourites are keyed the same way as the web UI: source|type|id.
	if probe.Source != "" && probe.Type != "" {
		return probe.Source + "|" + probe.Type + "|" + id
	}
	return id
}

func syncCollectionList(doc map[string]json.RawMessage, kind string) []json.RawMessage {
	body, ok := doc[kind]
	if !ok || len(body) == 0 {
		return nil
	}
	var list []json.RawMessage
	if err := json.Unmarshal(body, &list); err != nil {
		return nil
	}
	return list
}

func writeSyncCollection(doc map[string]json.RawMessage, kind string, list []json.RawMessage) {
	if list == nil {
		list = []json.RawMessage{}
	}
	body, err := json.Marshal(list)
	if err != nil {
		return
	}
	doc[kind] = body
}

// libraryRecords decomposes the library blob into kind/key records.
func libraryRecords(doc map[string]json.RawMessage) map[string]map[string]json.RawMessage {
	out := make(map[string]map[string]json.RawMessage)
	for _, kind := range []string{syncKindFavorites, syncKindHistory, syncKindPlaylists, syncKindSheetFavorites} {
		records := make(map[string]json.RawMessage)
		for _, item := range syncCollectionList(doc, kind) {
			if key := syncItemKey(item); key != "" {
				records[key] = append(json.RawMessage(nil), item...)
			}
		}
		out[kind] = records
	}
	overrides := make(map[string]json.RawMessage)
	if body, ok := doc[syncKindOverrides]; ok && len(body) > 0 {
		_ = json.Unmarshal(body, &overrides)
	}
	out[syncKindOverrides] = overrides
	for _, kind := range []string{syncKindPlayerQueue, syncKindRecs} {
		records := make(map[string]json.RawMessage)
		if body, ok := doc[kind]; ok && len(body) > 0 {
			records[syncSingletonKey] = append(json.RawMessage(nil), body...)
		}
		out[kind] = records
	}
	return out
}

// SaveLibrary persists the blob and stamps every record that actually changed.
// The web client still saves the whole document, so the diff is what turns it
// into record-level versions the sync endpoint can merge.
func (s *UserStore) SaveLibrary(userID uint, data string) error {
	previous, err := s.GetLibrary(userID)
	if err != nil {
		return err
	}
	now := nowMillis()
	before := libraryRecords(decodeLibraryDoc(previous))
	after := libraryRecords(decodeLibraryDoc(data))

	var versions []appSyncVersion
	for kind, records := range after {
		for key, payload := range records {
			old, ok := before[kind][key]
			if ok && bytes.Equal(old, payload) {
				continue
			}
			versions = append(versions, appSyncVersion{
				UserID: userID, Kind: kind, ItemKey: key, UpdatedAt: now,
			})
		}
	}
	for kind, records := range before {
		for key := range records {
			if _, ok := after[kind][key]; ok {
				continue
			}
			versions = append(versions, appSyncVersion{
				UserID: userID, Kind: kind, ItemKey: key, UpdatedAt: now, DeletedAt: now,
			})
		}
	}

	if err := s.saveLibraryRaw(userID, data); err != nil {
		return err
	}
	return s.upsertSyncVersions(versions)
}

func (s *UserStore) saveLibraryRaw(userID uint, data string) error {
	item := appUserLibrary{UserID: userID, Data: data, UpdatedAt: time.Now()}
	return s.db.Save(&item).Error
}

// ---------------------------------------------------------------- cookies

type syncCookiePayload struct {
	Value string `json:"value"`
}

func (s *UserStore) cookieRecords(userID uint) (map[string]json.RawMessage, error) {
	rows, err := s.userCookieRows(userID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]json.RawMessage, len(rows))
	for _, row := range rows {
		payload, marshalErr := json.Marshal(syncCookiePayload{Value: row.Value})
		if marshalErr != nil {
			continue
		}
		out[row.Source] = payload
	}
	return out, nil
}

// ---------------------------------------------------------------- collect

func (s *UserStore) SyncItems(userID uint, since int64) ([]syncItem, error) {
	versions, err := s.syncVersions(userID)
	if err != nil {
		return nil, err
	}
	raw, err := s.GetLibrary(userID)
	if err != nil {
		return nil, err
	}
	records := libraryRecords(decodeLibraryDoc(raw))
	cookies, err := s.cookieRecords(userID)
	if err != nil {
		return nil, err
	}
	records[syncKindCookies] = cookies

	items := make([]syncItem, 0)
	for kind, byKey := range records {
		keys := make([]string, 0, len(byKey))
		for key := range byKey {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			version := versions[syncVersionKey(kind, key)]
			updated := version.UpdatedAt
			if updated <= 0 {
				// Records saved before sync existed get the oldest possible
				// version, so any real change wins over them.
				updated = 1
			}
			authoritative := kind == syncKindRecs
			if since > 0 && updated <= since && !authoritative {
				continue
			}
			items = append(items, syncItem{
				Kind:      kind,
				Key:       key,
				Payload:   append(json.RawMessage(nil), byKey[key]...),
				UpdatedAt: updated,
			})
		}
	}

	// Tombstones let a deletion travel between devices.
	tombstones := make([]syncItem, 0)
	for _, version := range versions {
		if version.DeletedAt <= 0 {
			continue
		}
		if _, alive := records[version.Kind][version.ItemKey]; alive {
			continue
		}
		if since > 0 && version.DeletedAt <= since {
			continue
		}
		tombstones = append(tombstones, syncItem{
			Kind:      version.Kind,
			Key:       version.ItemKey,
			UpdatedAt: version.UpdatedAt,
			DeletedAt: version.DeletedAt,
		})
	}
	sort.Slice(tombstones, func(i, j int) bool {
		if tombstones[i].Kind != tombstones[j].Kind {
			return tombstones[i].Kind < tombstones[j].Kind
		}
		return tombstones[i].Key < tombstones[j].Key
	})
	return append(items, tombstones...), nil
}

// ---------------------------------------------------------------- merge

func applyCollectionRecord(doc map[string]json.RawMessage, kind, key string, payload json.RawMessage, remove bool) bool {
	list := syncCollectionList(doc, kind)
	out := make([]json.RawMessage, 0, len(list)+1)
	found := false
	changed := false
	for _, item := range list {
		if syncItemKey(item) != key {
			out = append(out, item)
			continue
		}
		found = true
		if remove {
			changed = true
			continue
		}
		if !bytes.Equal(item, payload) {
			changed = true
		}
		out = append(out, payload)
	}
	if !remove && !found {
		out = append([]json.RawMessage{payload}, out...)
		changed = true
	}
	if !changed {
		return false
	}
	writeSyncCollection(doc, kind, out)
	return true
}

// sortCollectionRecords puts merged records in the same order the web UI
// expects: newest first. Favourites have no timestamp of their own, so their
// sync version is used.
func sortCollectionRecords(doc map[string]json.RawMessage, kind string, versions map[string]appSyncVersion) {
	list := syncCollectionList(doc, kind)
	if len(list) < 2 {
		return
	}
	timeOf := func(raw json.RawMessage) int64 {
		var probe struct {
			PlayedAt  int64 `json:"playedAt"`
			UpdatedAt int64 `json:"updatedAt"`
			SavedAt   int64 `json:"savedAt"`
		}
		_ = json.Unmarshal(raw, &probe)
		switch kind {
		case syncKindHistory:
			return probe.PlayedAt
		case syncKindPlaylists:
			return probe.UpdatedAt
		case syncKindSheetFavorites:
			return probe.SavedAt
		default:
			version := versions[syncVersionKey(kind, syncItemKey(raw))]
			return syncRecordTime(version.UpdatedAt, version.DeletedAt)
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return timeOf(list[i]) > timeOf(list[j]) })
	writeSyncCollection(doc, kind, list)
}

func applyOverrideRecord(doc map[string]json.RawMessage, key string, payload json.RawMessage, remove bool) bool {
	overrides := make(map[string]json.RawMessage)
	if body, ok := doc[syncKindOverrides]; ok && len(body) > 0 {
		if err := json.Unmarshal(body, &overrides); err != nil {
			overrides = make(map[string]json.RawMessage)
		}
	}
	if remove {
		if _, ok := overrides[key]; !ok {
			return false
		}
		delete(overrides, key)
	} else {
		if old, ok := overrides[key]; ok && bytes.Equal(old, payload) {
			return false
		}
		overrides[key] = payload
	}
	body, err := json.Marshal(overrides)
	if err != nil {
		return false
	}
	doc[syncKindOverrides] = body
	return true
}

// MergeSync applies incoming records with last-write-wins, then returns the
// full merged state so the caller can write it back to the other side.
func (s *UserStore) MergeSync(userID uint, incoming []syncItem) ([]syncItem, error) {
	versions, err := s.syncVersions(userID)
	if err != nil {
		return nil, err
	}
	raw, err := s.GetLibrary(userID)
	if err != nil {
		return nil, err
	}
	doc := decodeLibraryDoc(raw)
	now := nowMillis()
	changed := false
	docChanged := false
	var versionRows []appSyncVersion
	recordVersion := func(row appSyncVersion) {
		versions[syncVersionKey(row.Kind, row.ItemKey)] = row
		versionRows = append(versionRows, row)
	}

	applyDoc := func() {
		if !docChanged {
			return
		}
		for _, kind := range []string{syncKindFavorites, syncKindHistory, syncKindPlaylists, syncKindSheetFavorites} {
			sortCollectionRecords(doc, kind, versions)
		}
		body, marshalErr := json.Marshal(doc)
		if marshalErr == nil {
			if err := s.saveLibraryRaw(userID, string(body)); err == nil {
				changed = true
			}
		}
		docChanged = false
	}

	for _, item := range incoming {
		kind := strings.TrimSpace(item.Kind)
		key := strings.TrimSpace(item.Key)
		if kind == "" || key == "" || !syncKnownKind(kind) {
			continue
		}
		itemTime := syncRecordTime(item.UpdatedAt, item.DeletedAt)
		if itemTime <= 0 {
			continue
		}
		current := versions[syncVersionKey(kind, key)]

		if kind == syncKindRecs {
			// Generated lists are server-authoritative: accept them even when
			// the local copy is newer, but never upload local changes.
			if len(item.Payload) == 0 {
				continue
			}
			doc[kind] = append(json.RawMessage(nil), item.Payload...)
			docChanged = true
			recordVersion(appSyncVersion{
				UserID: userID, Kind: kind, ItemKey: key, UpdatedAt: now,
			})
			continue
		}
		if itemTime <= syncRecordTime(current.UpdatedAt, current.DeletedAt) {
			continue
		}

		switch {
		case kind == syncKindCookies:
			if item.DeletedAt > 0 {
				if err := s.deleteCookieRow(userID, key); err != nil {
					return nil, err
				}
				recordVersion(appSyncVersion{
					UserID: userID, Kind: kind, ItemKey: key,
					UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt,
				})
				continue
			}
			var payload syncCookiePayload
			if err := json.Unmarshal(item.Payload, &payload); err != nil {
				continue
			}
			if err := s.setCookieRow(userID, key, payload.Value); err != nil {
				return nil, err
			}
			recordVersion(appSyncVersion{
				UserID: userID, Kind: kind, ItemKey: key, UpdatedAt: item.UpdatedAt,
			})
		case kind == syncKindOverrides:
			if applyOverrideRecord(doc, key, item.Payload, item.DeletedAt > 0) {
				docChanged = true
			}
			recordVersion(appSyncVersion{
				UserID: userID, Kind: kind, ItemKey: key,
				UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt,
			})
		case syncCollectionKind(kind):
			if item.DeletedAt > 0 {
				if applyCollectionRecord(doc, kind, key, nil, true) {
					docChanged = true
				}
			} else {
				if len(item.Payload) == 0 {
					continue
				}
				if applyCollectionRecord(doc, kind, key, item.Payload, false) {
					docChanged = true
				}
			}
			recordVersion(appSyncVersion{
				UserID: userID, Kind: kind, ItemKey: key,
				UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt,
			})
		case syncSingletonKind(kind):
			if item.DeletedAt > 0 {
				if _, ok := doc[kind]; ok {
					delete(doc, kind)
					docChanged = true
				}
			} else {
				if len(item.Payload) == 0 {
					continue
				}
				doc[kind] = append(json.RawMessage(nil), item.Payload...)
				docChanged = true
			}
			recordVersion(appSyncVersion{
				UserID: userID, Kind: kind, ItemKey: key,
				UpdatedAt: item.UpdatedAt, DeletedAt: item.DeletedAt,
			})
		}
	}

	applyDoc()
	if !changed && len(versionRows) == 0 {
		// Nothing to persist, but the caller still needs the current state.
	} else if err := s.upsertSyncVersions(versionRows); err != nil {
		return nil, err
	}
	if err := s.pruneSyncTombstones(userID, now); err != nil {
		return nil, err
	}
	return s.SyncItems(userID, 0)
}

func (s *UserStore) pruneSyncTombstones(userID uint, now int64) error {
	cutoff := now - syncTombstoneRetention.Milliseconds()
	return s.db.Where("user_id = ? AND deleted_at > 0 AND deleted_at < ?", userID, cutoff).
		Delete(&appSyncVersion{}).Error
}

// ---------------------------------------------------------------- handlers

func (s *Server) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	since, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("since")), 10, 64)
	items, err := s.opts.Store.SyncItems(user.ID, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取同步数据失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"now": nowMillis(), "items": items})
}

func (s *Server) handleSyncPush(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	defer r.Body.Close()
	var payload struct {
		Since int64      `json:"since"`
		Items []syncItem `json:"items"`
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, maxSyncRequestBodyBytes))
	if err := dec.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "同步数据格式错误")
		return
	}
	items, err := s.opts.Store.MergeSync(user.ID, payload.Items)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "合并同步数据失败")
		return
	}
	s.invalidateUserCookies(user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"now": nowMillis(), "items": items})
}
