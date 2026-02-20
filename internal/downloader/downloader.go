package downloader

// VideoInfo contém os metadados de um vídeo.
type VideoInfo struct {
	Title     string
	Duration  string
	Formats   []Format
	Languages []AudioLang
}

// Format representa uma opção de qualidade disponível.
type Format struct {
	Height int
	Label  string
}

// AudioLang representa um idioma de áudio disponível.
type AudioLang struct {
	Code string
	Name string
}

// DownloadResult contém o resultado de um download bem-sucedido.
type DownloadResult struct {
	FilePath string
}

// Downloader define a interface para qualquer plataforma de download.
type Downloader interface {
	GetVideoInfo(url string) (*VideoInfo, error)
	Download(url string, height int, langCode string, dest string, progress func(current, total int64)) (DownloadResult, error)
}
