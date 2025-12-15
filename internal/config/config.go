package config

import (
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	MemoryBackend string `toml:"memory_backend"`

	CureThreshold    int  `toml:"cure_threshold_percent"`
	NaRemovalEnabled bool `toml:"na_removal_enabled"`
}

var active atomic.Value

func init() {
	Load()
	go watch()
}

func Get() *Config {
	return active.Load().(*Config)
}

func Load() {
	data, err := os.ReadFile("config.toml")
	if err != nil {
		slog.Error("failed to read config.toml, using defaults", "err", err)
		data = []byte(defaultConfig)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		slog.Error("invalid TOML, using defaults", "err", err)
		data = []byte(defaultConfig)
	}
	if cfg.MemoryBackend == "" {
		cfg.MemoryBackend = "elite"
	}
	active.Store(&cfg)
	slog.Info("config loaded", "backend", cfg.MemoryBackend)
}

var defaultConfig = `
memory_backend = "elite"

cure_threshold_percent = 70
na_remove_enabled = true
`

func watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("fsnotify failed", "err", err)
		return
	}
	defer watcher.Close()

	if err := watcher.Add("config.toml"); err != nil {
		if err := watcher.Add("./config.toml"); err != nil {
			slog.Error("cannot watch config.toml", "err", err)
			return
		}
	}

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				slog.Info("config file changed, reloading...")
				Load()
			}
		case err := <-watcher.Errors:
			slog.Error("fsnotify error", "err", err)
		}
	}
}
