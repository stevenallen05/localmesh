package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	envPrefix         = "AGENT_"
	heartbeatInterval = 5 * time.Second
)

type Config struct {
	ID                     string `koanf:"id"`
	MaxBufferSize          int    `koanf:"max_buffer_size"`
	MinReportingInterval   int    `koanf:"min_reporting_interval"`
	ShutdownTimeoutSeconds int    `koanf:"shutdown_timeout_seconds"`
	Server                 struct {
		Addr string `koanf:"addr"`
	} `koanf:"server"`
	Log struct {
		Level  string `koanf:"level"`
		Format string `koanf:"format"`
	} `koanf:"log"`
	Docker struct {
		Socket string `koanf:"socket"`
	} `koanf:"docker"`
	Collectors map[string]map[string]any `koanf:"collectors"`
}

// nestedPrefixes are the env-key first segments that trigger underscore→dot
// expansion. Anything else stays flat. See spec §2.2.
var nestedPrefixes = []string{"server_", "log_", "docker_", "collectors_"}

func defaults() map[string]any {
	return map[string]any{
		"id":                       "",
		"max_buffer_size":          0,
		"min_reporting_interval":   300,
		"shutdown_timeout_seconds": 10,
		"server.addr":              "server:50051",
		"log.level":                "info",
		"log.format":               "json",
		"docker.socket":            "unix:///var/run/docker.sock",
	}
}

// loadConfig returns the typed Config for runtime use and the *koanf.Koanf
// snapshot for the boot-time config dump (k.All() renders with the same
// dotted snake_case keys as TOML/env, which is what spec §7.5 expects to see).
func loadConfig() (Config, *koanf.Koanf, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaults(), "."), nil); err != nil {
		return Config{}, nil, fmt.Errorf("defaults: %w", err)
	}
	path := os.Getenv("AGENT_CONFIG_FILE")
	if path == "" {
		path = "./agent.toml"
	}
	if _, err := os.Stat(path); err == nil {
		if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
			return Config{}, nil, fmt.Errorf("toml %s: %w", path, err)
		}
	}
	envCb := func(s string) string {
		s = strings.ToLower(strings.TrimPrefix(s, envPrefix))
		for _, p := range nestedPrefixes {
			if strings.HasPrefix(s, p) {
				return strings.ReplaceAll(s, "_", ".")
			}
		}
		return s
	}
	if err := k.Load(env.Provider(envPrefix, ".", envCb), nil); err != nil {
		return Config{}, nil, fmt.Errorf("env: %w", err)
	}
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, nil, fmt.Errorf("unmarshal: %w", err)
	}
	if cfg.ID == "" {
		h, err := os.Hostname()
		if err != nil {
			return cfg, nil, fmt.Errorf("hostname: %w", err)
		}
		cfg.ID = h
		_ = k.Set("id", h) // mirror into the snapshot so the dump shows the resolved value
	}
	return cfg, k, nil
}

func newLogger(cfg Config) (*slog.Logger, error) {
	var level slog.Level
	switch strings.ToLower(cfg.Log.Level) {
	case "debug":
		level = slog.LevelDebug
	case "", "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid log.level %q", cfg.Log.Level)
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	switch strings.ToLower(cfg.Log.Format) {
	case "text":
		h = slog.NewTextHandler(os.Stdout, opts)
	case "", "json":
		h = slog.NewJSONHandler(os.Stdout, opts)
	default:
		return nil, fmt.Errorf("invalid log.format %q", cfg.Log.Format)
	}
	return slog.New(h).With("agent_id", cfg.ID), nil
}

func runHeartbeat(ctx context.Context, log *slog.Logger) {
	boot := time.Now()
	t := time.NewTicker(heartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			log.Info("heartbeat", "uptime", time.Since(boot).Round(time.Second).String())
		}
	}
}

func main() {
	cfg, k, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	log, err := newLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	log.Info("config resolved", "config", k.All())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		runHeartbeat(ctx, log)
	}()

	<-ctx.Done()
	timer := time.AfterFunc(time.Duration(cfg.ShutdownTimeoutSeconds)*time.Second, func() {
		log.Error("shutdown deadline exceeded")
		os.Exit(2)
	})
	defer timer.Stop()
	<-done
	log.Info("shutdown complete")
}
