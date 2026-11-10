package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"musicdl-ex/internal/api"
	"musicdl-ex/internal/webassets"

	"github.com/guohuiyuan/go-music-dl/core"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	var (
		addr    = flag.String("addr", envOr("MUSIC_DL_ADDR", ":8080"), "HTTP listen address")
		dataDir = flag.String("data", envOr("MUSIC_DL_DATA_DIR", "data"), "persistent data directory")
		webDir  = flag.String("web", "", "optional directory with built web assets (overrides embedded assets)")
		showVer = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("go-music-dl-ex %s (%s), upstream core %s\n", version, buildTime, core.AppVersion)
		return
	}

	if err := prepareDataDir(*dataDir); err != nil {
		log.Fatalf("prepare data dir: %v", err)
	}

	// The upstream core resolves several paths relative to the process working
	// directory (settings.db, downloads, cookies). Chdir into the data dir so
	// an fnOS upgrade never loses the library.
	dataPath := absPath(*dataDir)
	if err := os.Chdir(dataPath); err != nil {
		log.Fatalf("chdir %s: %v", dataPath, err)
	}
	if os.Getenv("MUSIC_DL_CONFIG_DB") == "" {
		_ = os.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join("data", "settings.db"))
	}

	userStore, err := api.NewUserStore(filepath.Join("data", "users.db"))
	if err != nil {
		log.Fatalf("open user store: %v", err)
	}
	defer userStore.Close()

	core.CM.Load()

	var webHandler http.Handler
	if *webDir != "" {
		if _, err := os.Stat(filepath.Join(*webDir, "index.html")); err != nil {
			log.Fatalf("web dir %s is missing index.html: %v", *webDir, err)
		}
		webHandler = http.FileServer(http.Dir(*webDir))
	} else {
		webHandler = webassets.Handler()
	}

	srv := api.New(api.Options{
		Version:   version,
		BuildTime: buildTime,
		Core:      core.AppVersion,
		Web:       webHandler,
		Store:     userStore,
	})

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("go-music-dl-ex %s listening on %s (data: %s)", version, *addr, dataPath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	_ = core.CloseConfigDB()
	log.Println("shutdown complete")
}

func prepareDataDir(dir string) error {
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "downloads"), 0o755); err != nil {
		return err
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func absPath(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return a
}
