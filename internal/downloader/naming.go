package downloader

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nonIDCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func ensurePlatformFileName(filePath, platform, preferredID string) (string, string) {
	if strings.TrimSpace(filePath) == "" {
		return filePath, "não foi possível padronizar nome do arquivo (caminho vazio)"
	}

	if _, err := os.Stat(filePath); err != nil {
		return filePath, "não foi possível padronizar nome do arquivo (arquivo não encontrado)"
	}

	dir := filepath.Dir(filePath)
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		ext = ".mp4"
	}

	id := sanitizeID(preferredID)
	if id == "" {
		base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		id = extractIDFromBase(base, platform)
	}
	if id == "" {
		id = randomID(10)
	}

	target := filepath.Join(dir, fmt.Sprintf("%s_%s%s", platform, id, ext))
	if sameFilePath(filePath, target) {
		return target, ""
	}

	if _, err := os.Stat(target); err == nil {
		target = filepath.Join(dir, fmt.Sprintf("%s_%s_%s%s", platform, id, randomID(4), ext))
	}

	if err := os.Rename(filePath, target); err != nil {
		return filePath, fmt.Sprintf("não foi possível renomear para padrão %s_id (%v)", platform, err)
	}

	return target, ""
}

func extractIDFromBase(base, platform string) string {
	prefix := platform + "_"
	raw := base

	if strings.HasPrefix(strings.ToLower(base), strings.ToLower(prefix)) {
		raw = base[len(prefix):]
	}

	return sanitizeID(raw)
}

func randomID(size int) string {
	if size <= 0 {
		size = 10
	}

	b := make([]byte, (size+1)/2)
	if _, err := rand.Read(b); err != nil {
		return "randfallback"
	}

	out := hex.EncodeToString(b)
	if len(out) > size {
		out = out[:size]
	}
	return out
}

func joinWarnings(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		filtered = append(filtered, p)
	}
	return strings.Join(filtered, " | ")
}

func sanitizeID(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "[](){}")
	raw = nonIDCharsRegex.ReplaceAllString(raw, "")
	if raw == "" {
		return ""
	}
	if strings.EqualFold(raw, "na") || strings.EqualFold(raw, "none") || strings.EqualFold(raw, "null") {
		return ""
	}
	return raw
}
