package api

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func newSyncTestStore(t *testing.T) (*UserStore, *AppUser) {
	t.Helper()
	store, err := NewUserStore(filepath.Join(t.TempDir(), "sync.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if sqlDB, err := store.DB().DB(); err == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	user, err := EnsureLocalUser(store)
	if err != nil {
		t.Fatalf("local user: %v", err)
	}
	return store, user
}

func syncItemFor(t *testing.T, items []syncItem, kind, key string) (syncItem, bool) {
	t.Helper()
	for _, item := range items {
		if item.Kind == kind && item.Key == key {
			return item, true
		}
	}
	return syncItem{}, false
}

func TestSyncPublishesSavedLibraryRecords(t *testing.T) {
	store, user := newSyncTestStore(t)
	if err := store.SaveLibrary(user.ID, `{"favorites":[{"id":"a","title":"A"}],"history":[],"playlists":[],"sheetFavorites":[]}`); err != nil {
		t.Fatalf("save library: %v", err)
	}
	items, err := store.SyncItems(user.ID, 0)
	if err != nil {
		t.Fatalf("sync items: %v", err)
	}
	item, ok := syncItemFor(t, items, syncKindFavorites, "a")
	if !ok {
		t.Fatalf("favorite not published: %+v", items)
	}
	if item.UpdatedAt <= 0 || item.DeletedAt != 0 {
		t.Fatalf("unexpected version: %+v", item)
	}
	if string(item.Payload) != `{"id":"a","title":"A"}` {
		t.Fatalf("unexpected payload: %s", item.Payload)
	}
}

func TestSyncMergeKeepsNewestAndAddsRecords(t *testing.T) {
	store, user := newSyncTestStore(t)
	if err := store.SaveLibrary(user.ID, `{"favorites":[{"id":"a","title":"old"}],"history":[],"playlists":[],"sheetFavorites":[]}`); err != nil {
		t.Fatalf("save library: %v", err)
	}
	base := nowMillis() + 5_000
	merged, err := store.MergeSync(user.ID, []syncItem{
		{Kind: syncKindFavorites, Key: "a", Payload: json.RawMessage(`{"id":"a","title":"new"}`), UpdatedAt: base},
		{Kind: syncKindFavorites, Key: "b", Payload: json.RawMessage(`{"id":"b","title":"B"}`), UpdatedAt: base},
	})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	gotA, _ := syncItemFor(t, merged, syncKindFavorites, "a")
	if string(gotA.Payload) != `{"id":"a","title":"new"}` {
		t.Fatalf("newest did not win: %s", gotA.Payload)
	}
	if _, ok := syncItemFor(t, merged, syncKindFavorites, "b"); !ok {
		t.Fatalf("new record missing: %+v", merged)
	}

	// An older update must not roll the record back.
	merged, err = store.MergeSync(user.ID, []syncItem{
		{Kind: syncKindFavorites, Key: "a", Payload: json.RawMessage(`{"id":"a","title":"stale"}`), UpdatedAt: base - 1_000},
	})
	if err != nil {
		t.Fatalf("merge stale: %v", err)
	}
	gotA, _ = syncItemFor(t, merged, syncKindFavorites, "a")
	if string(gotA.Payload) != `{"id":"a","title":"new"}` {
		t.Fatalf("stale update won: %s", gotA.Payload)
	}
}

func TestSyncTombstoneDeletesRecord(t *testing.T) {
	store, user := newSyncTestStore(t)
	if err := store.SaveLibrary(user.ID, `{"favorites":[{"id":"a","title":"A"}],"history":[],"playlists":[],"sheetFavorites":[]}`); err != nil {
		t.Fatalf("save library: %v", err)
	}
	moment := nowMillis() + 5_000
	merged, err := store.MergeSync(user.ID, []syncItem{
		{Kind: syncKindFavorites, Key: "a", DeletedAt: moment, UpdatedAt: moment},
	})
	if err != nil {
		t.Fatalf("merge delete: %v", err)
	}
	item, ok := syncItemFor(t, merged, syncKindFavorites, "a")
	if !ok || item.DeletedAt == 0 {
		t.Fatalf("tombstone missing: %+v", merged)
	}
	raw, err := store.GetLibrary(user.ID)
	if err != nil {
		t.Fatalf("get library: %v", err)
	}
	doc := decodeLibraryDoc(raw)
	if list := syncCollectionList(doc, syncKindFavorites); len(list) != 0 {
		t.Fatalf("favorite not deleted: %s", raw)
	}
}

func TestSyncCookiesRoundTrip(t *testing.T) {
	store, user := newSyncTestStore(t)
	if err := store.SetUserCookie(user.ID, "netease", "MUSIC_U=remote"); err != nil {
		t.Fatalf("set cookie: %v", err)
	}
	items, err := store.SyncItems(user.ID, 0)
	if err != nil {
		t.Fatalf("sync items: %v", err)
	}
	cookie, ok := syncItemFor(t, items, syncKindCookies, "netease")
	if !ok {
		t.Fatalf("cookie not published: %+v", items)
	}
	var payload syncCookiePayload
	if err := json.Unmarshal(cookie.Payload, &payload); err != nil || payload.Value != "MUSIC_U=remote" {
		t.Fatalf("unexpected cookie payload: %s", cookie.Payload)
	}

	newer := cookie.UpdatedAt + 1_000
	if _, err := store.MergeSync(user.ID, []syncItem{{
		Kind: syncKindCookies, Key: "netease",
		Payload: json.RawMessage(`{"value":"MUSIC_U=local"}`), UpdatedAt: newer,
	}}); err != nil {
		t.Fatalf("merge cookie: %v", err)
	}
	stored, err := store.UserCookies(user.ID)
	if err != nil {
		t.Fatalf("user cookies: %v", err)
	}
	if stored["netease"] != "MUSIC_U=local" {
		t.Fatalf("cookie not merged: %v", stored)
	}
}
