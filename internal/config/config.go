package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	DownloadDir string
	AppName     string
	Copyright   string
}

func New() *Config {
	home, _ := os.UserHomeDir()
	downloadDir := filepath.Join(home, "Downloads", "DownloaderTube")

	return &Config{
		DownloadDir: downloadDir,
		AppName:     "Downloader Tube",
		Copyright:   "@Copyright - https://webadvance.com.br | Diogo-dev",
	}
}

func (c *Config) EnsureDownloadDir() error {
	return os.MkdirAll(c.DownloadDir, os.ModePerm)
}
