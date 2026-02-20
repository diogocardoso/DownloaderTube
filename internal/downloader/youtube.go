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
	"strings"
	"sync"
	"time"
)

type YouTubeDownloader struct{}

func NewYouTube() *YouTubeDownloader {
	return &YouTubeDownloader{}
}


type ytdlpInfo struct {
	Title          string        `json:"title"`
	DurationString string        `json:"duration_string"`
	Formats        []ytdlpFormat `json:"formats"`
}

type ytdlpFormat struct {
	FormatID   string  `json:"format_id"`
	Ext        string  `json:"ext"`
	Height     int     `json:"height"`
	VCodec     string  `json:"vcodec"`
	ACodec     string  `json:"acodec"`
	Language   string  `json:"language"`
	FormatNote string  `json:"format_note"`
	Filesize   float64 `json:"filesize"`
	FilesizeApprox float64 `json:"filesize_approx"`
}

var langNames = map[string]string{
	"pt":    "Português",
	"pt-BR": "Português (Brasil)",
	"en":    "English",
	"en-US": "English (US)",
	"es":    "Español",
	"es-US": "Español (US)",
	"fr":    "Français",
	"fr-FR": "Français",
	"de":    "Deutsch",
	"de-DE": "Deutsch",
	"it":    "Italiano",
	"ja":    "日本語",
	"ko":    "한국어",
	"zh":    "中文",
	"ru":    "Русский",
	"ar":    "العربية",
	"hi":    "हिन्दी",
	"nl":    "Nederlands",
	"pl":    "Polski",
	"tr":    "Türkçe",
	"sv":    "Svenska",
	"id":    "Bahasa Indonesia",
	"th":    "ไทย",
	"vi":    "Tiếng Việt",
}

func resolveLangName(code string) string {
	if name, ok := langNames[code]; ok {
		return name
	}
	base := strings.SplitN(code, "-", 2)[0]
	if name, ok := langNames[base]; ok {
		return name
	}
	return code
}

func baseLang(code string) string {
	return strings.SplitN(code, "-", 2)[0]
}

func (yd *YouTubeDownloader) GetVideoInfo(rawURL string) (*VideoInfo, error) {
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

	// Detecta idiomas de TODOS os formatos com áudio (incluindo combinados HLS)
	langMap := make(map[string]string) // baseLang -> código mais específico
	for _, f := range info.Formats {
		if f.ACodec == "none" || f.Language == "" {
			continue
		}
		base := baseLang(f.Language)
		existing, exists := langMap[base]
		if !exists || len(f.Language) > len(existing) {
			langMap[base] = f.Language
		}
	}

	var languages []AudioLang
	for _, code := range langMap {
		languages = append(languages, AudioLang{
			Code: code,
			Name: resolveLangName(code),
		})
	}

	sort.Slice(languages, func(i, j int) bool {
		return languages[i].Name < languages[j].Name
	})

	return &VideoInfo{
		Title:     info.Title,
		Duration:  info.DurationString,
		Formats:   formats,
		Languages: languages,
	}, nil
}

func (yd *YouTubeDownloader) Download(videoURL string, height int, langCode string, dest string, progress func(current, total int64)) (DownloadResult, error) {
	outputTemplate := filepath.Join(dest, "%(title)s.%(ext)s")
	formatStr := buildFormatString(height, langCode)

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

func buildFormatString(height int, langCode string) string {
	h := strconv.Itoa(height)

	if langCode != "" {
		// 1) bv+ba[language=X]       → faixas dedicadas (idioma original, melhor qualidade)
		// 2) b[language=X][height<=H] → streams HLS combinados (idiomas alternativos)
		// 3) bv+ba[ext=m4a]           → fallback sem filtro de idioma
		// 4) bv+ba / b               → último recurso
		base := strings.SplitN(langCode, "-", 2)[0]
		if base != langCode {
			return fmt.Sprintf(
				"bv[height<=%s]+ba[language=%s]/bv[height<=%s]+ba[language=%s]/b[language=%s][height<=%s]/b[language=%s][height<=%s]/bv[height<=%s]+ba[ext=m4a]/bv[height<=%s]+ba/b[height<=%s]",
				h, langCode, h, base, langCode, h, base, h, h, h, h,
			)
		}
		return fmt.Sprintf(
			"bv[height<=%s]+ba[language=%s]/b[language=%s][height<=%s]/bv[height<=%s]+ba[ext=m4a]/bv[height<=%s]+ba/b[height<=%s]",
			h, langCode, langCode, h, h, h, h,
		)
	}

	return fmt.Sprintf("bv[height<=%s]+ba[ext=m4a]/bv[height<=%s]+ba/b[height<=%s]", h, h, h)
}

var (
	mergerRegex   = regexp.MustCompile(`\[Merger\] Merging formats into "(.+)"`)
	moveRegex     = regexp.MustCompile(`\[MoveFiles\] Moving file ".+" to "(.+)"`)
	destRegex     = regexp.MustCompile(`\[download\] Destination: (.+)`)
	alreadyRegex  = regexp.MustCompile(`\[download\] (.+) has already been downloaded`)
)

// extractFilePath extrai o caminho do arquivo final da saída do yt-dlp.
// Prioridade: MoveFiles > Merger > Destination (último capturado vence).
func extractFilePath(line string) string {
	if m := moveRegex.FindStringSubmatch(line); len(m) >= 2 {
		return m[1]
	}
	if m := mergerRegex.FindStringSubmatch(line); len(m) >= 2 {
		return m[1]
	}
	if m := destRegex.FindStringSubmatch(line); len(m) >= 2 {
		return m[1]
	}
	if m := alreadyRegex.FindStringSubmatch(line); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func toBytes(val float64, unit string) int64 {
	switch strings.ToLower(unit) {
	case "kib":
		return int64(val * 1024)
	case "mib":
		return int64(val * 1024 * 1024)
	case "gib":
		return int64(val * 1024 * 1024 * 1024)
	default:
		return int64(val)
	}
}
