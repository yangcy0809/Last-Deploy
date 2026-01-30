package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Addr        string
	DataDir     string
	HostDataDir string
}

func Load() Config {
	return Config{
		Addr:        getenv("LAST_DEPLOY_ADDR", "127.0.0.1:8080"),
		DataDir:     getenv("LAST_DEPLOY_DATA_DIR", "./data"),
		HostDataDir: getenv("LAST_DEPLOY_HOST_DATA_DIR", ""),
	}
}

func (c Config) DBPath() string {
	return filepath.Join(c.DataDir, "db.sqlite")
}

func (c Config) ReposDir() string {
	return filepath.Join(c.DataDir, "repos")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
