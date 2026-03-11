package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
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
	FormatID       string  `json:"format_id"`
	Ext            string  `json:"ext"`
	Height         int     `json:"height"`
	VCodec         string  `json:"vcodec"`
	ACodec         string  `json:"acodec"`
	Language       string  `json:"language"`
	FormatNote     string  `json:"format_note"`
	Format         string  `json:"format"`
	URL            string  `json:"url"`
	Filesize       float64 `json:"filesize"`
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

func youtubeCookiesFromBrowser() string {
	return strings.TrimSpace(os.Getenv("DT_COOKIES_FROM_BROWSER"))
}

func youtubeExtractorArgs() string {
	return strings.TrimSpace(os.Getenv("DT_YT_EXTRACTOR_ARGS"))
}

func appendYouTubeExtractorArgs(args []string) []string {
	extractorArgs := youtubeExtractorArgs()
	if extractorArgs == "" {
		return args
	}
	return append(args, "--extractor-args", extractorArgs)
}

func youtubeCookiesArgs() []string {
	browser := youtubeCookiesFromBrowser()
	if browser == "" {
		return nil
	}
	return []string{"--cookies-from-browser", browser}
}

func shouldRetryWithoutCookies(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cookie database") || strings.Contains(msg, "cookies-from-browser")
}

func collectAudioLanguages(formats []ytdlpFormat) []AudioLang {
	langSet := make(map[string]struct{})
	for _, f := range formats {
		if f.ACodec == "none" {
			continue
		}
		code := detectLanguageCode(f)
		if code == "" {
			continue
		}
		langSet[code] = struct{}{}
	}

	var languages []AudioLang
	for code := range langSet {
		languages = append(languages, AudioLang{
			Code: code,
			Name: resolveLangName(code),
		})
	}

	sort.Slice(languages, func(i, j int) bool {
		if languages[i].Name == languages[j].Name {
			return languages[i].Code < languages[j].Code
		}
		return languages[i].Name < languages[j].Name
	})

	return languages
}

var (
	langBracketRegex = regexp.MustCompile(`[\[\(]([a-z]{2,3}(?:-[A-Za-z0-9]{2,})?)[\]\)]`)
	langQueryRegex   = regexp.MustCompile(`(?:^|[?&:])(?:lang|language)=([a-z]{2,3}(?:-[A-Za-z0-9]{2,})?)`)
)

func detectLanguageCode(f ytdlpFormat) string {
	if code := normalizeLanguageCode(f.Language); code != "" {
		return code
	}
	if code := detectLanguageInText(f.Format); code != "" {
		return code
	}
	if code := detectLanguageInText(f.FormatNote); code != "" {
		return code
	}
	return detectLanguageInURL(f.URL)
}

func detectLanguageInText(text string) string {
	if text == "" {
		return ""
	}
	matches := langBracketRegex.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		if code := normalizeLanguageCode(m[1]); code != "" {
			return code
		}
	}
	return ""
}

func detectLanguageInURL(raw string) string {
	if raw == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}
	if m := langQueryRegex.FindStringSubmatch(strings.ToLower(decoded)); len(m) >= 2 {
		return normalizeLanguageCode(m[1])
	}
	return ""
}

func normalizeLanguageCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return ""
	}
	code = strings.ReplaceAll(code, "_", "-")
	parts := strings.Split(code, "-")
	if len(parts) == 0 {
		return ""
	}
	parts[0] = strings.ToLower(parts[0])
	if len(parts[0]) < 2 || len(parts[0]) > 3 {
		return ""
	}
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			return ""
		}
		parts[i] = strings.ToUpper(parts[i])
	}
	return strings.Join(parts, "-")
}

func (yd *YouTubeDownloader) GetVideoInfo(rawURL string) (*VideoInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cookieArgs := youtubeCookiesArgs()
	args := []string{"-j", "--no-warnings"}
	args = appendYouTubeExtractorArgs(args)
	args = append(args, cookieArgs...)
	args = append(args, rawURL)
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	output, err := cmd.Output()
	if err != nil && len(cookieArgs) > 0 && shouldRetryWithoutCookies(err) {
		debugLogf("[youtube] GetVideoInfo cookies failed, retry without cookies: %v", err)
		retryArgs := []string{"-j", "--no-warnings"}
		retryArgs = appendYouTubeExtractorArgs(retryArgs)
		retryArgs = append(retryArgs, rawURL)
		cmd = exec.CommandContext(ctx, "yt-dlp", retryArgs...)
		output, err = cmd.Output()
	}
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

	languages := collectAudioLanguages(info.Formats)

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
	startedAt := time.Now()
	debugLogf("[youtube] start url=%s height=%d lang=%s format=%s", videoURL, height, langCode, formatStr)
	cookieBrowser := youtubeCookiesFromBrowser()
	if cookieBrowser != "" {
		debugLogf("[youtube] cookies-from-browser enabled: %s", cookieBrowser)
	}
	if extractorArgs := youtubeExtractorArgs(); extractorArgs != "" {
		debugLogf("[youtube] extractor-args enabled: %s", extractorArgs)
	}

	args := []string{
		"-f", formatStr,
		"--progress",
		"--progress-template", "download:__DT_PROGRESS__:%(progress.downloaded_bytes)s:%(progress.total_bytes)s:%(progress.total_bytes_estimate)s:%(progress._percent_str)s",
		"--merge-output-format", "mp4",
		"--embed-thumbnail",
		"--embed-metadata",
		"--print", "after_move:__DT_PATH__:%(filepath)s",
		"--print", "after_move:__DT_ID__:%(id)s",
		"--newline",
		"--no-warnings",
		"-o", outputTemplate,
	}
	args = appendYouTubeExtractorArgs(args)
	args = append(args, youtubeCookiesArgs()...)
	args = append(args, videoURL)
	cmd := exec.Command("yt-dlp", args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return DownloadResult{}, fmt.Errorf("erro ao criar pipe stdout: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return DownloadResult{}, fmt.Errorf("erro ao criar pipe stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		debugLogf("[youtube] cmd start error: %v", err)
		return DownloadResult{}, fmt.Errorf("erro ao iniciar yt-dlp: %w", err)
	}

	var filePath string
	var mediaID string
	var mu sync.Mutex

	parseLine := func(line string) {
		if strings.Contains(line, "[download]") || strings.Contains(line, "ERROR") || strings.Contains(line, "WARNING") {
			debugLogf("[youtube] line: %s", line)
		}
		if progress != nil {
			if currentVal, totalVal, ok := parseProgressLine(line); ok {
				debugLogf("[youtube] progress parsed current=%d total=%d", currentVal, totalVal)
				progress(currentVal, totalVal)
			}
		}
		if p := extractFilePath(line); p != "" {
			debugLogf("[youtube] file path detected: %s", p)
			mu.Lock()
			filePath = p
			mu.Unlock()
		}
		if id := extractMediaID(line); id != "" {
			debugLogf("[youtube] media id detected: %s", id)
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
			debugLogf("[youtube] stdout scanner error: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		scanner := newProgressScanner(stderrPipe)
		for scanner.Scan() {
			parseLine(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			debugLogf("[youtube] stderr scanner error: %v", err)
		}
	}()

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		debugLogf("[youtube] cmd wait error: %v", err)
		return DownloadResult{}, fmt.Errorf("erro durante download: %w", err)
	}

	resolvedPath := resolveDownloadedFile(filePath, dest, mediaID, startedAt)
	debugLogf("[youtube] resolved path parsed=%s resolved=%s", filePath, resolvedPath)
	finalPath, warning := ensureWhatsAppCompatible(resolvedPath)
	namedPath, nameWarning := ensurePlatformFileName(finalPath, "youtube", mediaID)
	debugLogf("[youtube] done filePath=%s finalPath=%s namedPath=%s mediaID=%s warning=%s nameWarning=%s", resolvedPath, finalPath, namedPath, mediaID, warning, nameWarning)

	return DownloadResult{
		FilePath:             namedPath,
		CompatibilityWarning: joinWarnings(warning, nameWarning),
	}, nil
}

func buildFormatString(height int, langCode string) string {
	h := strconv.Itoa(height)

	if langCode != "" {
		// 1) bv+ba[language=X]         -> faixas dedicadas quando existirem
		// 2) b[language=X][height<=H]  -> streams HLS combinados para multi-audio
		// 3) fallback sem idioma por ultimo, para manter download funcional
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
	mergerRegex  = regexp.MustCompile(`\[Merger\] Merging formats into "(.+)"`)
	moveRegex    = regexp.MustCompile(`\[MoveFiles\] Moving file ".+" to "(.+)"`)
	destRegex    = regexp.MustCompile(`\[download\] Destination: (.+)`)
	alreadyRegex = regexp.MustCompile(`\[download\] (.+) has already been downloaded`)
	printRegex   = regexp.MustCompile(`^__DT_PATH__:(.+)$`)
	idPrintRegex = regexp.MustCompile(`^__DT_ID__:(.+)$`)
)

// extractFilePath extrai o caminho do arquivo final da saída do yt-dlp.
// Prioridade: MoveFiles > Merger > Destination (último capturado vence).
func extractFilePath(line string) string {
	if m := printRegex.FindStringSubmatch(strings.TrimSpace(line)); len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
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

func extractMediaID(line string) string {
	if m := idPrintRegex.FindStringSubmatch(strings.TrimSpace(line)); len(m) >= 2 {
		return sanitizeID(m[1])
	}
	return ""
}
