package downloader

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type fileCandidate struct {
	path    string
	modTime time.Time
}

// resolveDownloadedFile tenta encontrar o arquivo real quando o caminho parseado
// pelo output do yt-dlp vier corrompido/ilegível no Windows.
func resolveDownloadedFile(parsedPath, destDir, mediaID string, startedAt time.Time) string {
	parsedPath = strings.TrimSpace(parsedPath)
	if parsedPath != "" {
		if _, err := os.Stat(parsedPath); err == nil {
			return parsedPath
		}
	}

	entries, err := os.ReadDir(destDir)
	if err != nil {
		return parsedPath
	}

	threshold := startedAt.Add(-3 * time.Second)
	var candidates []fileCandidate
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(threshold) {
			continue
		}

		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".mp4", ".mkv", ".webm", ".mov", ".m4v":
			candidates = append(candidates, fileCandidate{
				path:    filepath.Join(destDir, e.Name()),
				modTime: info.ModTime(),
			})
		}
	}

	if len(candidates) == 0 {
		return parsedPath
	}

	if mediaID != "" {
		idLower := strings.ToLower(mediaID)
		for _, c := range candidates {
			if strings.Contains(strings.ToLower(filepath.Base(c.path)), idLower) {
				return c.path
			}
		}
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.modTime.After(best.modTime) {
			best = c
		}
	}
	return best.path
}
