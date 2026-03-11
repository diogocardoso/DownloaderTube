# Plano de Testes - Idiomas YouTube (DownloaderTube)

Este documento ajuda a descobrir por que, em alguns momentos, o app mostra apenas:

- `English (en)`
- `English (US) (en-US)`

mesmo quando o video ja apareceu com `pt-BR` anteriormente.

## Objetivo

Isolar se o problema vem de:

- variacao de formatos retornados pelo `yt-dlp` (YouTube muda por sessao/cliente)
- versao/canal do `yt-dlp` em uso
- uso de cookies do navegador
- diferenca entre resultado no terminal e no app

## Videos de referencia

Use sempre os mesmos links para comparar:

- `https://www.youtube.com/watch?v=E14hUveM4us`
- `https://youtu.be/bO2nWVs1jqA`
- `https://www.youtube.com/watch?v=pKO9UjSeLew`

## Preparacao (PowerShell)

1) Limpar variaveis de ambiente desta sessao:

```powershell
Remove-Item Env:DT_COOKIES_FROM_BROWSER -ErrorAction SilentlyContinue
Remove-Item Env:DT_YTDLP_CHANNEL -ErrorAction SilentlyContinue
```

2) Confirmar binario e versao ativa:

```powershell
(Get-Command yt-dlp).Source
yt-dlp --version
```

3) Rodar o app uma vez para garantir deps e PATH de runtime:

```powershell
go run ./cmd
```

## Matriz de testes

Execute os blocos abaixo e salve o output em arquivo para comparacao.

### Teste A - Baseline sem cookies (terminal)

```powershell
yt-dlp -F --no-warnings "https://www.youtube.com/watch?v=E14hUveM4us"
yt-dlp --print "%(formats.:.language)j" --no-warnings "https://www.youtube.com/watch?v=E14hUveM4us"
```

Esperado:

- se retornar apenas `["en"]` ou `["en","en-US"]`, o app tambem tende a mostrar so ingles.

### Teste B - Forcar canais do yt-dlp no app

#### B1) Nightly

```powershell
$env:DT_YTDLP_CHANNEL="nightly"
go run ./cmd
```

No menu do app:

- entrar em YouTube
- colar URL de teste
- anotar idiomas exibidos

#### B2) Stable

```powershell
$env:DT_YTDLP_CHANNEL="stable"
go run ./cmd
```

Repetir o mesmo fluxo e comparar.

### Teste C - Com cookies do navegador

#### C1) Chrome

```powershell
$env:DT_COOKIES_FROM_BROWSER="chrome"
yt-dlp -F --no-warnings "https://youtu.be/bO2nWVs1jqA"
yt-dlp --print "%(formats.:.language)j" --no-warnings "https://youtu.be/bO2nWVs1jqA"
```

#### C2) Edge

```powershell
$env:DT_COOKIES_FROM_BROWSER="edge"
yt-dlp -F --no-warnings "https://youtu.be/bO2nWVs1jqA"
yt-dlp --print "%(formats.:.language)j" --no-warnings "https://youtu.be/bO2nWVs1jqA"
```

Se houver erro de cookie database:

- fechar o navegador completamente
- repetir os comandos

### Teste D - Comparar terminal vs app no mesmo ambiente

1) Definir variaveis (ex.: nightly + chrome):

```powershell
$env:DT_YTDLP_CHANNEL="nightly"
$env:DT_COOKIES_FROM_BROWSER="chrome"
```

2) Rodar no terminal:

```powershell
yt-dlp --print "%(formats.:.language)j" --no-warnings "https://youtu.be/bO2nWVs1jqA"
```

3) Rodar app:

```powershell
go run ./cmd
```

4) Conferir se os idiomas vistos no menu batem com os idiomas do terminal.

## Coleta de evidencias

### 1) Salvar output do yt-dlp

```powershell
yt-dlp -F -v --extractor-args "youtube:player_client=all;formats=missing_pot" "https://youtu.be/bO2nWVs1jqA" *> yt_test_output.txt
```

### 2) Salvar log interno do app

O app escreve log em:

- `%TEMP%\downloadertube-debug.log`

Copie esse arquivo apos o teste para nao perder historico.

## Interpretacao rapida

- `Terminal mostra so en/en-US`:
  - o app nao consegue inventar outros idiomas; limita no retorno do `yt-dlp`.
- `Terminal mostra pt-BR e app nao mostra`:
  - investigar parsing de idiomas no app (regressao).
- `Com cookies aparece pt-BR, sem cookies nao`:
  - diferenca de sessao/permissao/cliente no YouTube.
- `Nightly mostra mais idiomas que stable`:
  - manter `DT_YTDLP_CHANNEL=nightly` para esse tipo de video.

## Checklist final

- [ ] Teste A concluido e salvo
- [ ] Teste B (nightly vs stable) concluido
- [ ] Teste C (cookies) concluido
- [ ] Teste D (terminal vs app) concluido
- [ ] `yt_test_output.txt` coletado
- [ ] `%TEMP%\downloadertube-debug.log` coletado

