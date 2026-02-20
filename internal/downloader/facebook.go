package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"
)

type FacebookDownloader struct{}

func NewFacebook() *FacebookDownloader {
	return &FacebookDownloader{}
}

func (fd *FacebookDownloader) GetVideoInfo(rawURL string) (*VideoInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "yt-dlp", "-j", "--no-warnings", rawURL)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("erro ao obter info do vídeo: %w", err)
	}

	var info ytdlpInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("erro ao parsear info do vídeo: %w", err)
	}

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

func (fd *FacebookDownloader) Download(videoURL string, height int, langCode string, dest string, progress func(current, total int64)) (DownloadResult, error) {
	outputTemplate := filepath.Join(dest, "%(title)s.%(ext)s")
	formatStr := buildFacebookFormatString(height)

	cmd := exec.Command("yt-dlp",
		"-f", formatStr,
		"--merge-output-format", "mp4",
		"--embed-thumbnail",
		"--embed-metadata",
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
		return DownloadResult{}, fmt.Errorf("erro ao iniciar yt-dlp: %w", err)
	}

	progressRegex := regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?\s*([\d.]+)([\w]+)`)

	var filePath string
	var mu sync.Mutex

	parseLine := func(line string) {
		if matches := progressRegex.FindStringSubmatch(line); len(matches) >= 4 && progress != nil {
			pct, _ := strconv.ParseFloat(matches[1], 64)
			totalVal, _ := strconv.ParseFloat(matches[2], 64)
			unit := matches[3]

			totalBytes := toBytes(totalVal, unit)
			currentBytes := int64(float64(totalBytes) * pct / 100)

			progress(currentBytes, totalBytes)
		}
		if p := extractFilePath(line); p != "" {
			mu.Lock()
			filePath = p
			mu.Unlock()
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			parseLine(scanner.Text())
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			parseLine(scanner.Text())
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return DownloadResult{}, fmt.Errorf("erro durante download: %w", err)
	}

	return DownloadResult{FilePath: filePath}, nil
}

func buildFacebookFormatString(height int) string {
	h := strconv.Itoa(height)
	return fmt.Sprintf("bv[height<=%s]+ba/b[height<=%s]/b", h, h)
}
