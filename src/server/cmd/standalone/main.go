// Command standalone boots the embedded server exactly like the android build
// does, so the single-user mode can be verified on a desktop.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"musicdl-ex/mobile"
)

func main() {
	dir, err := os.MkdirTemp("", "musicdl-standalone")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	url, err := mobile.Start(filepath.Join(dir, "data"))
	if err != nil {
		panic(err)
	}
	fmt.Println("base:", url)

	time.Sleep(300 * time.Millisecond)
	for _, path := range []string{"/api/healthz", "/api/auth/me", "/api/me/library"} {
		resp, err := http.Get(url + path[1:])
		if err != nil {
			fmt.Println(path, "error:", err)
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		resp.Body.Close()
		fmt.Printf("%s -> %d %s\n", path, resp.StatusCode, string(body))
	}
	mobile.Stop()
}
