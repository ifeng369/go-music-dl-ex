package webassets

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// The Vite build writes directly into internal/webassets/dist. A placeholder
// index.html is committed so `go build` works before the frontend is built.
//
//go:embed all:dist
var distFS embed.FS

// Handler serves the embedded SPA and falls back to index.html for client-side
// routes.
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			serveIndex(w, r, sub)
			return
		}

		if f, err := sub.Open(name); err == nil {
			_ = f.Close()
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, r)
			return
		}

		serveIndex(w, r, sub)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, sub fs.FS) {
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		http.Error(w, "web assets are not built", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

//go:embed plugin.js
var pluginJS []byte

// PluginJS returns the embedded MusicFree plugin source.
func PluginJS() []byte {
	return pluginJS
}

//go:embed mint-plugin.js
var mintPluginJS []byte

// MintPluginJS returns the embedded Mint-Music plugin source.
func MintPluginJS() []byte {
	return mintPluginJS
}
