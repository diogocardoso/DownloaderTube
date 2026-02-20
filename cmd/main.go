package main

//go:generate goversioninfo -manifest=downloadertube.exe.manifest -o resource_windows.syso

import (
	"fmt"
	"os"

	"github.com/webadvance/downloadertube/internal/cli"
	"github.com/webadvance/downloadertube/internal/config"
	"github.com/webadvance/downloadertube/internal/deps"
	"github.com/webadvance/downloadertube/internal/downloader"
)

var version = "dev"

func main() {
	if err := deps.EnsureDependencies(); err != nil {
		fmt.Fprintf(os.Stderr, "Erro: %v\n", err)
		os.Exit(1)
	}

	cfg := config.New()

	if err := cfg.EnsureDownloadDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao criar diretório de download: %v\n", err)
		os.Exit(1)
	}

	ytDownloader := downloader.NewYouTube()
	fbDownloader := downloader.NewFacebook()
	igDownloader := downloader.NewInstagram()
	app := cli.New(cfg, ytDownloader, fbDownloader, igDownloader)
	app.Run()
}
