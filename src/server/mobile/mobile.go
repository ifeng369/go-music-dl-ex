// Package mobile boots the whole MusicDL server inside an Android process.
//
// The Android shell only has to call Start once, load the returned URL in a
// WebView, and call Stop when the app goes away. Everything (search, playback
// resolution, library storage, generated lists) runs on the device, so the
// client no longer depends on an external server.
package mobile

import (
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"musicdl-ex/internal/api"
	"musicdl-ex/internal/webassets"
)

var (
	mu       sync.Mutex
	listener net.Listener
	server   *http.Server
	baseURL  string
)

// Version reports the embedded server version.
func Version() string { return api.EmbeddedVersion() }

// Start opens (or creates) the local database inside dataDir and serves the UI
// on a loopback port. Calling it twice returns the existing URL.
func Start(dataDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if baseURL != "" {
		return baseURL, nil
	}

	store, err := api.NewUserStore(filepath.Join(dataDir, "musicdl.db"))
	if err != nil {
		return "", err
	}

	user, err := api.EnsureLocalUser(store)
	if err != nil {
		return "", err
	}

	srv := api.New(api.Options{
		Version:    Version(),
		BuildTime:  time.Now().Format(time.RFC3339),
		Core:       "go-music-dl",
		Web:        webassets.Handler(),
		Store:      store,
		SingleUser: user,
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	server = &http.Server{Handler: srv.Handler()}
	go func() {
		_ = server.Serve(ln)
	}()

	baseURL = "http://" + ln.Addr().String() + "/"
	return baseURL, nil
}

// Stop shuts the embedded server down.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if server != nil {
		_ = server.Close()
		server = nil
	}
	listener = nil
	baseURL = ""
}
