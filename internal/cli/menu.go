package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/webadvance/downloadertube/internal/config"
	"github.com/webadvance/downloadertube/internal/downloader"
	"github.com/webadvance/downloadertube/pkg/validator"
)

type App struct {
	cfg          *config.Config
	reader       *bufio.Reader
	ytDownloader downloader.Downloader
	fbDownloader downloader.Downloader
	igDownloader downloader.Downloader
}

func New(cfg *config.Config, ytDL downloader.Downloader, fbDL downloader.Downloader, igDL downloader.Downloader) *App {
	return &App{
		cfg:          cfg,
		reader:       bufio.NewReader(os.Stdin),
		ytDownloader: ytDL,
		fbDownloader: fbDL,
		igDownloader: igDL,
	}
}

func (a *App) Run() {
	for {
		a.clearScreen()
		a.printHeader()
		fmt.Println(" 1 - Youtube")
		fmt.Println(" 2 - Facebook")
		fmt.Println(" 3 - Instagram")
		fmt.Println()
		fmt.Println(" x - Sair")
		a.printFooter()

		choice := a.readInput()

		switch strings.ToLower(choice) {
		case "1":
			a.youtubeMenu()
		case "2":
			a.facebookMenu()
		case "3":
			a.instagramMenu()
		case "x":
			fmt.Println("\n Até logo!")
			return
		default:
			a.showError("Opção inválida!")
		}
	}
}

func (a *App) youtubeMenu() {
	for {
		a.clearScreen()
		fmt.Println(" Youtube url:")
		fmt.Println()
		fmt.Println(" 0 - Voltar")
		fmt.Println(" x - Sair")
		a.printSeparator()

		input := a.readInput()

		switch strings.ToLower(input) {
		case "0":
			return
		case "x":
			fmt.Println("\n Até logo!")
			os.Exit(0)
		default:
			if !validator.IsYouTubeURL(input) {
				a.showError("URL inválida! Informe uma URL válida do YouTube.")
				continue
			}
			a.processVideo(input, a.ytDownloader)
		}
	}
}

func (a *App) facebookMenu() {
	for {
		a.clearScreen()
		fmt.Println(" Facebook url:")
		fmt.Println()
		fmt.Println(" 0 - Voltar")
		fmt.Println(" x - Sair")
		a.printSeparator()

		input := a.readInput()

		switch strings.ToLower(input) {
		case "0":
			return
		case "x":
			fmt.Println("\n Até logo!")
			os.Exit(0)
		default:
			if !validator.IsFacebookURL(input) {
				a.showError("URL inválida! Informe uma URL válida do Facebook.")
				continue
			}
			a.processVideo(input, a.fbDownloader)
		}
	}
}

func (a *App) instagramMenu() {
	for {
		a.clearScreen()
		fmt.Println(" Instagram url:")
		fmt.Println()
		fmt.Println(" 0 - Voltar")
		fmt.Println(" x - Sair")
		a.printSeparator()

		input := a.readInput()

		switch strings.ToLower(input) {
		case "0":
			return
		case "x":
			fmt.Println("\n Até logo!")
			os.Exit(0)
		default:
			if !validator.IsInstagramURL(input) {
				a.showError("URL inválida! Informe uma URL válida do Instagram.")
				continue
			}
			a.processVideo(input, a.igDownloader)
		}
	}
}

func (a *App) processVideo(url string, dl downloader.Downloader) {
	a.clearScreen()
	fmt.Println(" Buscando informações do vídeo...")
	a.printSeparator()

	info, err := dl.GetVideoInfo(url)
	if err != nil {
		a.showError(fmt.Sprintf("Erro ao buscar vídeo: %v", err))
		return
	}

	if len(info.Formats) == 0 {
		a.showError("Nenhum formato de vídeo disponível.")
		return
	}

	langCode := ""
	if len(info.Languages) > 1 {
		var ok bool
		langCode, ok = a.selectLanguage(info)
		if !ok {
			return
		}
	} else if len(info.Languages) == 1 {
		langCode = info.Languages[0].Code
	}

	a.selectQualityAndDownload(url, info, langCode, dl)
}

func (a *App) selectLanguage(info *downloader.VideoInfo) (string, bool) {
	for {
		a.clearScreen()
		fmt.Printf(" Vídeo: %s\n", info.Title)
		fmt.Printf(" Duração: %s\n", info.Duration)
		fmt.Println()
		fmt.Println(" Idiomas disponíveis:")

		for i, lang := range info.Languages {
			fmt.Printf(" %d - %s (%s)\n", i+1, lang.Name, lang.Code)
		}

		fmt.Println()
		fmt.Println(" 0 - Voltar")
		fmt.Println(" x - Sair")
		a.printSeparator()

		choice := a.readInput()

		switch strings.ToLower(choice) {
		case "0":
			return "", false
		case "x":
			fmt.Println("\n Até logo!")
			os.Exit(0)
		default:
			idx := a.parseChoice(choice, len(info.Languages))
			if idx < 0 {
				a.showError("Opção inválida!")
				continue
			}
			return info.Languages[idx].Code, true
		}
	}
}

func (a *App) selectQualityAndDownload(url string, info *downloader.VideoInfo, langCode string, dl downloader.Downloader) {
	for {
		a.clearScreen()
		fmt.Printf(" Vídeo: %s\n", info.Title)
		fmt.Printf(" Duração: %s\n", info.Duration)
		if langCode != "" {
			fmt.Printf(" Idioma: %s\n", langCode)
		}
		fmt.Println()
		fmt.Println(" Qualidades disponíveis:")

		for i, f := range info.Formats {
			fmt.Printf(" %d - %s\n", i+1, f.Label)
		}

		fmt.Println()
		fmt.Println(" 0 - Voltar")
		fmt.Println(" x - Sair")
		a.printSeparator()

		choice := a.readInput()

		switch strings.ToLower(choice) {
		case "0":
			return
		case "x":
			fmt.Println("\n Até logo!")
			os.Exit(0)
		default:
			idx := a.parseChoice(choice, len(info.Formats))
			if idx < 0 {
				a.showError("Opção inválida!")
				continue
			}
			a.startDownload(url, info, idx, langCode, dl)
			return
		}
	}
}

func (a *App) startDownload(url string, info *downloader.VideoInfo, formatIdx int, langCode string, dl downloader.Downloader) {
	if err := a.cfg.EnsureDownloadDir(); err != nil {
		a.showError(fmt.Sprintf("Erro ao criar pasta de download: %v", err))
		return
	}

	selectedFormat := info.Formats[formatIdx]

	a.clearScreen()
	fmt.Printf(" Baixando: %s [%s]\n", info.Title, selectedFormat.Label)
	fmt.Println()

	progress := func(current, total int64) {
		if total <= 0 {
			return
		}
		pct := float64(current) / float64(total) * 100
		barLen := 30
		filled := int(pct / 100 * float64(barLen))

		bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)

		currentMB := float64(current) / 1024 / 1024
		totalMB := float64(total) / 1024 / 1024

		fmt.Printf("\r [%s] %.0f%% - %.1fMB/%.1fMB", bar, pct, currentMB, totalMB)
	}

	result, err := dl.Download(url, selectedFormat.Height, langCode, a.cfg.DownloadDir, progress)
	fmt.Println()

	if err != nil {
		a.showError(fmt.Sprintf("Erro no download: %v", err))
		return
	}

	fmt.Println()
	fmt.Println(" Download concluído com sucesso!")
	if result.FilePath != "" {
		fmt.Printf(" Salvo em: %s\n", result.FilePath)
	} else {
		fmt.Printf(" Salvo em: %s\n", a.cfg.DownloadDir)
	}

	if result.FilePath != "" {
		a.showFileInfo(result.FilePath)
	}

	a.printFooter()
	fmt.Print("\n Pressione ENTER para continuar...")
	a.reader.ReadString('\n')
}

func (a *App) showFileInfo(filePath string) {
	probe, err := downloader.ProbeFile(filePath)
	if err != nil {
		return
	}

	fmt.Println()
	fmt.Println(" Formato do arquivo:")
	if probe.HasVideo {
		fmt.Printf("   Vídeo: %s\n", strings.ToUpper(probe.VideoCodec))
	}
	if probe.HasAudio {
		fmt.Printf("   Áudio: %s\n", strings.ToUpper(probe.AudioCodec))
	}

	if !probe.HasVideo && !probe.HasAudio {
		fmt.Println()
		fmt.Println(" [AVISO] O arquivo não possui faixas de vídeo nem áudio!")
		fmt.Println(" O download pode ter falhado. Tente novamente.")
	} else if !probe.HasVideo {
		fmt.Println()
		fmt.Println(" [AVISO] O arquivo não possui faixa de vídeo!")
		fmt.Println(" Pode ser necessário baixar o codec de vídeo ou tentar outra qualidade.")
	} else if !probe.HasAudio {
		fmt.Println()
		fmt.Println(" [AVISO] O arquivo não possui faixa de áudio!")
		fmt.Println(" O merge pode ter falhado. Verifique se o ffmpeg está instalado corretamente.")
	}
}

func (a *App) parseChoice(input string, max int) int {
	var n int
	_, err := fmt.Sscanf(input, "%d", &n)
	if err != nil || n < 1 || n > max {
		return -1
	}
	return n - 1
}

func (a *App) readInput() string {
	fmt.Print("\n >> ")
	input, _ := a.reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func (a *App) printHeader() {
	fmt.Println(" -------------------------------")
	fmt.Printf("    %s\n", a.cfg.AppName)
	fmt.Println(" -------------------------------")
}

func (a *App) printFooter() {
	fmt.Println(" -------------------------------")
	fmt.Printf(" %s\n", a.cfg.Copyright)
	fmt.Println(" -------------------------------")
}

func (a *App) printSeparator() {
	fmt.Println(" -------------------------------")
}

func (a *App) showError(msg string) {
	fmt.Printf("\n [ERRO] %s\n", msg)
	fmt.Print(" Pressione ENTER para continuar...")
	a.reader.ReadString('\n')
}

func (a *App) clearScreen() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		fmt.Print("\033[H\033[2J")
	}
}
