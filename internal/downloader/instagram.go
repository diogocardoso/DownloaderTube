package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type InstagramDownloader struct{}

type ytdlpPlaylistInfo struct {
	ytdlpInfo
	Entries []ytdlpInfo `json:"entries"`
}

func NewInstagram() *InstagramDownloader {
	return &InstagramDownloader{}
}

func (id *InstagramDownloader) GetVideoInfo(rawURL string) (*VideoInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp", "-J", "--no-warnings", rawURL)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("erro ao obter info do vídeo: %w", err)
	}

	var playlistInfo ytdlpPlaylistInfo
	if err := json.Unmarshal(output, &playlistInfo); err != nil {
		return nil, fmt.Errorf("erro ao parsear info do vídeo: %w", err)
	}

	info := pickInstagramInfo(playlistInfo)

	heightSeen := make(map[int]bool)
	var formats []Format

	for _, f := range info.Formats {
		if f.VCodec == "none" || f.Height == 0 {
			continue
		}
		if heightSeen[f.Height] {
			continue
		}
		heightSeen[f.Height] = true
		formats = append(formats, Format{
			Height: f.Height,
			Label:  fmt.Sprintf("%dp", f.Height),
		})
	}

	sort.Slice(formats, func(i, j int) bool {
		return formats[i].Height < formats[j].Height
	})

	return &VideoInfo{
		Title:    info.Title,
		Duration: info.DurationString,
		Formats:  formats,
	}, nil
}

func (id *InstagramDownloader) Download(videoURL string, height int, langCode string, dest string, progress func(current, total int64)) (DownloadResult, error) {
	outputTemplate := filepath.Join(dest, "%(title)s.%(ext)s")
	formatStr := buildInstagramFormatString(height)
	startedAt := time.Now()
	debugLogf("[instagram] start url=%s height=%d format=%s", videoURL, height, formatStr)

	cmd := exec.Command("yt-dlp",
		"-f", formatStr,
		"--progress",
		"--progress-template", "download:__DT_PROGRESS__:%(progress.downloaded_bytes)s:%(progress.total_bytes)s:%(progress.total_bytes_estimate)s:%(progress._percent_str)s",
		"--yes-playlist",
		"--ignore-errors",
		"--match-filter", "vcodec!=none",
		"--merge-output-format", "mp4",
		"--embed-metadata",
		"--print", "after_move:__DT_PATH__:%(filepath)s",
		"--print", "after_move:__DT_ID__:%(id)s",
		"--newline",
		"--no-warnings",
		"-o", outputTemplate,
		videoURL,
	)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return DownloadResult{}, fmt.Errorf("erro ao criar pipe stdout: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return DownloadResult{}, fmt.Errorf("erro ao criar pipe stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		debugLogf("[instagram] cmd start error: %v", err)
		return DownloadResult{}, fmt.Errorf("erro ao iniciar yt-dlp: %w", err)
	}

	var filePath string
	var mediaID string
	var mu sync.Mutex

	parseLine := func(line string) {
		if strings.Contains(line, "[download]") || strings.Contains(line, "ERROR") || strings.Contains(line, "WARNING") {
			debugLogf("[instagram] line: %s", line)
		}
		if progress != nil {
			if currentVal, totalVal, ok := parseProgressLine(line); ok {
				debugLogf("[instagram] progress parsed current=%d total=%d", currentVal, totalVal)
				progress(currentVal, totalVal)
			}
		}
		if p := extractFilePath(line); p != "" {
			debugLogf("[instagram] file path detected: %s", p)
			mu.Lock()
			filePath = p
			mu.Unlock()
		}
		if id := extractMediaID(line); id != "" {
			debugLogf("[instagram] media id detected: %s", id)
			mu.Lock()
			mediaID = id
			mu.Unlock()
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := newProgressScanner(stdoutPipe)
		for scanner.Scan() {
			parseLine(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			debugLogf("[instagram] stdout scanner error: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		scanner := newProgressScanner(stderrPipe)
		for scanner.Scan() {
			parseLine(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			debugLogf("[instagram] stderr scanner error: %v", err)
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		debugLogf("[instagram] cmd wait error: %v", err)
		return DownloadResult{}, fmt.Errorf("erro durante download: %w", err)
	}

	resolvedPath := resolveDownloadedFile(filePath, dest, mediaID, startedAt)
	debugLogf("[instagram] resolved path parsed=%s resolved=%s", filePath, resolvedPath)
	finalPath, warning := ensureWhatsAppCompatible(resolvedPath)
	namedPath, nameWarning := ensurePlatformFileName(finalPath, "instagram", mediaID)
	debugLogf("[instagram] done filePath=%s finalPath=%s namedPath=%s mediaID=%s warning=%s nameWarning=%s", resolvedPath, finalPath, namedPath, mediaID, warning, nameWarning)

	return DownloadResult{
		FilePath:             namedPath,
		CompatibilityWarning: joinWarnings(warning, nameWarning),
	}, nil
}

func buildInstagramFormatString(height int) string {
	h := strconv.Itoa(height)
	return fmt.Sprintf("bv[vcodec~='^(avc1|h264)'][height<=%s]+ba[acodec~='^(mp4a|aac)']/bv[height<=%s]+ba[ext=m4a]/bv[height<=%s]+ba/b[height<=%s]/b", h, h, h, h)
}

func pickInstagramInfo(playlistInfo ytdlpPlaylistInfo) ytdlpInfo {
	if len(playlistInfo.Entries) == 0 {
		return playlistInfo.ytdlpInfo
	}

	for _, entry := range playlistInfo.Entries {
		for _, f := range entry.Formats {
			if f.VCodec != "none" && f.Height > 0 {
				return entry
			}
		}
	}

	return playlistInfo.Entries[0]
}
