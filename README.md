# Downloader Tube

Aplicação CLI em Go para download de vídeos do **YouTube**, **Facebook** e **Instagram** com menu interativo no terminal.

## Funcionalidades

- Menu interativo com navegação por opções numéricas
- Download de vídeos do YouTube, Facebook e Instagram
- Seleção de **idioma do áudio** (quando disponível — YouTube)
- Seleção de **qualidade/resolução** (360p, 720p, 1080p, etc.)
- **Barra de progresso** durante o download
- Merge automático de vídeo + áudio via FFmpeg
- **Thumbnail embutida** no arquivo MP4 (visível no explorador de arquivos)
- **Metadados** embutidos (título, autor, etc.)
- Auto-download de dependências (yt-dlp e FFmpeg) no primeiro uso
- Validação de URL por plataforma

## Pré-requisitos

- **Go 1.24+**

As dependências externas (**yt-dlp** e **FFmpeg**) são baixadas automaticamente na primeira execução.

## Instalação

```bash
git clone https://github.com/diogocardoso/DownloaderTube.git
cd downloadertube
go build -o downloadertube ./cmd
```

## Uso

```bash
./downloadertube
```

### Opcional: usar cookies do navegador no YouTube

Em alguns vídeos, o YouTube só expõe certas trilhas de áudio quando a requisição está autenticada.
Para esses casos, defina a variável de ambiente abaixo antes de abrir o app:

- Windows (PowerShell):

```powershell
$env:DT_COOKIES_FROM_BROWSER="chrome"
```

- Linux/macOS (bash):

```bash
export DT_COOKIES_FROM_BROWSER=chrome
```

Valores comuns: `chrome`, `firefox`, `edge`.
Se a variável não estiver definida, o comportamento padrão atual é mantido.

### Opcional: forçar estratégia de extração do YouTube

Em alguns vídeos com multi-áudio, o YouTube pode omitir formatos no resultado padrão.
Para esses casos, é possível forçar `extractor-args` no `yt-dlp` usado pelo app:

- Windows (PowerShell):

```powershell
$env:DT_YT_EXTRACTOR_ARGS="youtube:player_client=all;formats=missing_pot"
```

- Linux/macOS (bash):

```bash
export DT_YT_EXTRACTOR_ARGS="youtube:player_client=all;formats=missing_pot"
```

Observação:
- Essa configuração ajuda o menu de idiomas a exibir trilhas que não aparecem no modo padrão.
- Alguns formatos listados como `MISSING POT` podem falhar no download com `403` dependendo da sessão/token.

### Opcional: canal do yt-dlp gerenciado

O app suporta seleção do canal do `yt-dlp` gerenciado:

- `DT_YTDLP_CHANNEL=nightly` (recomendado para cenários com mudanças recentes do YouTube)
- `DT_YTDLP_CHANNEL=stable`

Exemplo (PowerShell):

```powershell
$env:DT_YTDLP_CHANNEL="nightly"
```

O menu principal será exibido:

```
----
 Downloader Tube
----
 1 - Youtube
 2 - Facebook
 3 - Instagram

 x - Sair
----
 @Copyright - https://webadvance.com.br | Diogo-dev
----
```

1. Escolha a plataforma desejada
2. Cole a URL do vídeo
3. Selecione o idioma do áudio (se disponível)
4. Selecione a qualidade/resolução
5. Aguarde o download com barra de progresso

Os arquivos são salvos em `~/Downloads/DownloaderTube/` por padrão.

## Estrutura do Projeto

```
cmd/                     → Ponto de entrada (main.go)
internal/
  cli/                   → Menus e interação com o usuário
  config/                → Configurações (pasta de destino, etc.)
  deps/                  → Auto-download de yt-dlp e FFmpeg
  downloader/            → Interface Downloader + implementações por plataforma
    downloader.go        → Interface e tipos compartilhados
    youtube.go           → YouTubeDownloader
    facebook.go          → FacebookDownloader
    instagram.go         → InstagramDownloader
    probe.go             → Análise de codecs via FFprobe
pkg/
  validator/             → Validação de URLs por plataforma
```

## Arquitetura

O projeto segue uma arquitetura modular orientada a interfaces:

- **`Downloader`** — interface central que cada plataforma implementa:
  - `GetVideoInfo(url)` → retorna metadados, formatos e idiomas disponíveis
  - `Download(url, height, lang, dest, progress)` → executa o download com callback de progresso

- **Extensível** — para adicionar uma nova plataforma (ex: Vimeo), basta criar um novo struct que implemente `Downloader` e registrá-lo no menu.

- **Dependências externas** gerenciadas automaticamente pelo pacote `internal/deps`, que baixa yt-dlp e FFmpeg no diretório local do usuário.

## Dependências Externas

| Ferramenta | Finalidade | Gerenciamento |
|---|---|---|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp) | Extração e download de vídeos | Auto-download via GitHub Releases |
| [FFmpeg](https://github.com/BtbN/FFmpeg-Builds) | Merge vídeo+áudio, thumbnail, metadados | Auto-download via FFmpeg-Builds |

Os binários são salvos em:
- **Windows:** `%LOCALAPPDATA%/DownloaderTube/bin/`
- **Linux:** `~/.cache/DownloaderTube/bin/`

## Licença

© [WebAdvance](https://webadvance.com.br) — Diogo-dev
