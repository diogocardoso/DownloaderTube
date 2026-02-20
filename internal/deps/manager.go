package deps

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func getBinDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("não foi possível determinar diretório de cache: %w", err)
	}
	return filepath.Join(cacheDir, "DownloaderTube", "bin"), nil
}

func isAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func binaryName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// EnsureDependencies verifica se yt-dlp e ffmpeg estão disponíveis.
// Se não encontrados, baixa automaticamente do GitHub.
func EnsureDependencies() error {
	binDir, err := getBinDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de dependências: %w", err)
	}

	currentPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+currentPath)

	needYtDlp := !isAvailable("yt-dlp")
	needFfmpeg := !isAvailable("ffmpeg")

	if !needYtDlp && !needFfmpeg {
		return nil
	}

	fmt.Println()
	fmt.Println(" Dependências necessárias não encontradas.")
	fmt.Println(" Iniciando download automático...")
	fmt.Printf(" Local: %s\n", binDir)
	fmt.Println(" -------------------------------")

	if needYtDlp {
		fmt.Println()
		fmt.Println(" yt-dlp: baixando...")
		if err := installYtDlp(binDir); err != nil {
			return fmt.Errorf("falha ao instalar yt-dlp: %w", err)
		}
		fmt.Println(" yt-dlp: instalado com sucesso!")
	}

	if needFfmpeg {
		fmt.Println()
		fmt.Println(" ffmpeg: baixando (pode demorar alguns minutos)...")
		if err := installFfmpeg(binDir); err != nil {
			return fmt.Errorf("falha ao instalar ffmpeg: %w", err)
		}
		fmt.Println(" ffmpeg: instalado com sucesso!")
	}

	fmt.Println()
	fmt.Println(" -------------------------------")

	if err := verifyDependencies(); err != nil {
		return err
	}

	fmt.Println(" Todas as dependências estão prontas!")
	fmt.Println()

	return nil
}

func verifyDependencies() error {
	if !isAvailable("yt-dlp") {
		return fmt.Errorf("yt-dlp não está acessível após instalação. Verifique permissões")
	}
	if !isAvailable("ffmpeg") {
		return fmt.Errorf("ffmpeg não está acessível após instalação. Verifique permissões")
	}
	return nil
}

func installYtDlp(binDir string) error {
	var url string
	switch runtime.GOOS {
	case "windows":
		url = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
	case "linux":
		url = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp"
	case "darwin":
		url = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_macos"
	default:
		return fmt.Errorf("plataforma %s não suportada para auto-download do yt-dlp", runtime.GOOS)
	}

	destPath := filepath.Join(binDir, binaryName("yt-dlp"))
	return downloadFile(url, destPath, "yt-dlp")
}

func installFfmpeg(binDir string) error {
	switch runtime.GOOS {
	case "windows":
		return installFfmpegWindows(binDir)
	case "linux":
		return installFfmpegLinux(binDir)
	default:
		return fmt.Errorf("instale ffmpeg manualmente: https://ffmpeg.org/download.html")
	}
}

func installFfmpegWindows(binDir string) error {
	zipURL := "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-win64-gpl.zip"
	zipPath := filepath.Join(binDir, "ffmpeg-temp.zip")

	if err := downloadFile(zipURL, zipPath, "ffmpeg"); err != nil {
		return err
	}
	defer os.Remove(zipPath)

	fmt.Println(" Extraindo ffmpeg...")
	return extractFromZip(zipPath, binDir, []string{"ffmpeg.exe", "ffprobe.exe"})
}

func installFfmpegLinux(binDir string) error {
	tarURL := "https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/ffmpeg-master-latest-linux64-gpl.tar.xz"
	tarPath := filepath.Join(binDir, "ffmpeg-temp.tar.xz")

	if err := downloadFile(tarURL, tarPath, "ffmpeg"); err != nil {
		return err
	}
	defer os.Remove(tarPath)

	fmt.Println(" Extraindo ffmpeg...")
	return extractFromTarXz(tarPath, binDir)
}

var httpClient = &http.Client{
	Timeout: 10 * time.Minute,
}

// downloadFile baixa um arquivo de url para destPath, exibindo barra de progresso.
func downloadFile(rawURL, destPath, label string) error {
	fmt.Printf(" URL: %s\n", rawURL)

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("erro ao criar request para %s: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "DownloaderTube/1.0 (https://webadvance.com.br)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("erro na conexão com %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download de %s falhou com status HTTP %d", label, resp.StatusCode)
	}

	tmpPath := destPath + ".download"

	os.Remove(tmpPath)

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("erro ao criar arquivo temporário %s: %w", tmpPath, err)
	}

	pw := &progressWriter{
		total: resp.ContentLength,
		label: label,
	}

	_, copyErr := io.Copy(file, io.TeeReader(resp.Body, pw))
	file.Close()

	if copyErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("erro durante download de %s: %w", label, copyErr)
	}

	fmt.Println()

	info, statErr := os.Stat(tmpPath)
	if statErr != nil || info.Size() == 0 {
		os.Remove(tmpPath)
		return fmt.Errorf("arquivo baixado de %s está vazio ou corrompido", label)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("erro ao salvar arquivo %s: %w", destPath, err)
	}

	if runtime.GOOS != "windows" {
		os.Chmod(destPath, 0755)
	}

	fmt.Printf(" Salvo: %s (%.1fMB)\n", destPath, float64(info.Size())/1024/1024)
	return nil
}

func extractFromZip(zipPath, destDir string, targets []string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("erro ao abrir zip: %w", err)
	}
	defer r.Close()

	targetSet := make(map[string]bool)
	for _, t := range targets {
		targetSet[strings.ToLower(t)] = true
	}

	extracted := 0
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := strings.ToLower(filepath.Base(f.Name))
		if !targetSet[base] {
			continue
		}

		destPath := filepath.Join(destDir, filepath.Base(f.Name))
		if err := extractZipEntry(f, destPath); err != nil {
			return fmt.Errorf("erro ao extrair %s: %w", base, err)
		}
		extracted++
	}

	if extracted == 0 {
		return fmt.Errorf("arquivos alvo não encontrados dentro do zip")
	}

	return nil
}

func extractZipEntry(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func extractFromTarXz(tarPath, binDir string) error {
	tmpDir := filepath.Join(binDir, "ffmpeg-extract-tmp")
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("tar", "xf", tarPath, "-C", tmpDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("erro ao extrair tar.xz (tar está instalado?): %w", err)
	}

	targets := []string{"ffmpeg", "ffprobe"}
	found := 0

	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		for _, target := range targets {
			if info.Name() == target {
				destPath := filepath.Join(binDir, target)
				if moveErr := moveFile(path, destPath); moveErr == nil {
					os.Chmod(destPath, 0755)
					found++
				}
			}
		}
		return nil
	})

	if found == 0 {
		return fmt.Errorf("ffmpeg não encontrado no arquivo extraído")
	}

	return nil
}

func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(dst)
		if err != nil {
			return err
		}
		defer out.Close()

		_, err = io.Copy(out, in)
		return err
	}
	return nil
}

type progressWriter struct {
	total      int64
	downloaded int64
	label      string
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.downloaded += int64(n)
	pw.printProgress()
	return n, nil
}

func (pw *progressWriter) printProgress() {
	if pw.total <= 0 {
		fmt.Printf("\r Baixando %s... %.1fMB", pw.label, float64(pw.downloaded)/1024/1024)
		return
	}

	pct := float64(pw.downloaded) / float64(pw.total) * 100
	barLen := 30
	filled := int(pct / 100 * float64(barLen))
	if filled > barLen {
		filled = barLen
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
	currentMB := float64(pw.downloaded) / 1024 / 1024
	totalMB := float64(pw.total) / 1024 / 1024

	fmt.Printf("\r [%s] %.0f%% - %.1fMB/%.1fMB", bar, pct, currentMB, totalMB)
}
