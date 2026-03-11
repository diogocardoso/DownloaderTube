package downloader

import (
	"strings"
	"testing"
)

func TestBuildFormatStringWithoutLanguage(t *testing.T) {
	got := buildFormatString(720, "")

	if strings.Contains(got, "[language=") {
		t.Fatalf("nao deveria conter filtro de idioma sem selecao: %q", got)
	}
}

func TestBuildFormatStringWithSimpleLanguage(t *testing.T) {
	got := buildFormatString(720, "en")

	assertOrder(t, got,
		"ba[language=en]",
		"b[language=en][height<=720]",
		"ba[ext=m4a]",
	)
}

func TestBuildFormatStringWithRegionalLanguage(t *testing.T) {
	got := buildFormatString(1080, "es-MX")

	assertOrder(t, got,
		"ba[language=es-MX]",
		"ba[language=es]",
		"b[language=es-MX][height<=1080]",
		"b[language=es][height<=1080]",
		"ba[ext=m4a]",
	)
}

func assertOrder(t *testing.T, src string, parts ...string) {
	t.Helper()

	last := -1
	for _, p := range parts {
		idx := strings.Index(src, p)
		if idx < 0 {
			t.Fatalf("parte esperada ausente %q em %q", p, src)
		}
		if idx <= last {
			t.Fatalf("ordem invalida para %q em %q", p, src)
		}
		last = idx
	}
}

func TestYouTubeCookiesArgs(t *testing.T) {
	t.Setenv("DT_COOKIES_FROM_BROWSER", "")
	if got := youtubeCookiesArgs(); len(got) != 0 {
		t.Fatalf("esperava sem args quando env vazia, veio: %v", got)
	}

	t.Setenv("DT_COOKIES_FROM_BROWSER", "chrome")
	got := youtubeCookiesArgs()
	if len(got) != 2 || got[0] != "--cookies-from-browser" || got[1] != "chrome" {
		t.Fatalf("args inesperados: %v", got)
	}
}

func TestYouTubeExtractorArgs(t *testing.T) {
	t.Setenv("DT_YT_EXTRACTOR_ARGS", "")
	args := appendYouTubeExtractorArgs([]string{"-j"})
	if len(args) != 1 || args[0] != "-j" {
		t.Fatalf("nao deveria alterar args sem env: %v", args)
	}

	t.Setenv("DT_YT_EXTRACTOR_ARGS", "youtube:player_client=all;formats=missing_pot")
	args = appendYouTubeExtractorArgs([]string{"-j"})
	if len(args) != 3 || args[1] != "--extractor-args" || args[2] != "youtube:player_client=all;formats=missing_pot" {
		t.Fatalf("args extractor inesperados: %v", args)
	}
}

func TestCollectAudioLanguagesKeepsRegionalVariants(t *testing.T) {
	formats := []ytdlpFormat{
		{ACodec: "mp4a.40.2", Language: "en-US"},
		{ACodec: "mp4a.40.2", Language: "en"},
		{ACodec: "mp4a.40.2", Language: "pt-BR"},
		{ACodec: "none", Language: "es"},
		{ACodec: "opus", Language: ""},
		{ACodec: "mp4a.40.2", Language: "en-US"}, // duplicado
	}

	langs := collectAudioLanguages(formats)
	if len(langs) != 3 {
		t.Fatalf("esperava 3 idiomas unicos, veio %d: %+v", len(langs), langs)
	}

	got := map[string]bool{}
	for _, l := range langs {
		got[l.Code] = true
	}
	for _, code := range []string{"en-US", "en", "pt-BR"} {
		if !got[code] {
			t.Fatalf("idioma esperado ausente: %s", code)
		}
	}
}

func TestCollectAudioLanguagesFallsBackToFormatAndURL(t *testing.T) {
	formats := []ytdlpFormat{
		{ACodec: "mp4a.40.2", Language: "", Format: "95-6 - 1280x720 [pt-BR]"},
		{ACodec: "mp4a.40.2", Language: "", URL: "https://x.test/v?xtags=acont%3Ddubbed%3Alang%3Des-MX"},
		{ACodec: "mp4a.40.2", Language: "", FormatNote: "English (US) original (default), medium"},
	}

	langs := collectAudioLanguages(formats)
	if len(langs) != 2 {
		t.Fatalf("esperava 2 idiomas detectados por fallback, veio %d: %+v", len(langs), langs)
	}

	got := map[string]bool{}
	for _, l := range langs {
		got[l.Code] = true
	}
	for _, code := range []string{"pt-BR", "es-MX"} {
		if !got[code] {
			t.Fatalf("idioma esperado ausente: %s", code)
		}
	}
}
