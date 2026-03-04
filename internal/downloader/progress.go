package downloader

import (
	"bufio"
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var (
	progressWithSizeRegex = regexp.MustCompile(`\[download\]\s+([\d.]+)%\s+of\s+~?\s*([\d.]+)\s*([A-Za-z]+)`)
	progressPercentRegex  = regexp.MustCompile(`([\d.]+)%`)
	dtProgressRegex       = regexp.MustCompile(`^__DT_PROGRESS__:([^:]*):([^:]*):([^:]*):(.+)$`)
)

// parseProgressLine interpreta linhas de progresso do yt-dlp.
// Quando houver tamanho total conhecido, retorna current/total em bytes.
// Quando houver apenas percentual, retorna current=pct e total=0.
func parseProgressLine(line string) (int64, int64, bool) {
	if matches := dtProgressRegex.FindStringSubmatch(strings.TrimSpace(line)); len(matches) >= 5 {
		downloaded := parseIntToken(matches[1])
		total := parseIntToken(matches[2])
		estimated := parseIntToken(matches[3])
		percentToken := strings.TrimSpace(matches[4])

		if total > 0 {
			return downloaded, total, true
		}
		if estimated > 0 {
			return downloaded, estimated, true
		}
		if pct := parsePercentToken(percentToken); pct >= 0 {
			return int64(pct), 0, true
		}
		return 0, 0, false
	}

	if matches := progressWithSizeRegex.FindStringSubmatch(line); len(matches) >= 4 {
		pct, errPct := strconv.ParseFloat(matches[1], 64)
		totalVal, errTotal := strconv.ParseFloat(matches[2], 64)
		if errPct != nil || errTotal != nil {
			return 0, 0, false
		}

		totalBytes := toBytes(totalVal, matches[3])
		if totalBytes <= 0 {
			return 0, 0, false
		}

		currentBytes := int64(float64(totalBytes) * pct / 100)
		return currentBytes, totalBytes, true
	}

	if matches := progressPercentRegex.FindStringSubmatch(line); len(matches) >= 2 {
		pct, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			return 0, 0, false
		}
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		return int64(pct), 0, true
	}

	return 0, 0, false
}

func parseIntToken(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "none") || raw == "NA" || raw == "n/a" {
		return 0
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || f <= 0 {
		return 0
	}
	return int64(f)
}

func parsePercentToken(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "none") {
		return -1
	}
	m := progressPercentRegex.FindStringSubmatch(raw)
	if len(m) < 2 {
		return -1
	}
	p, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return -1
	}
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return int(p)
}

func toBytes(val float64, unit string) int64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "b":
		return int64(val)
	case "kb":
		return int64(val * 1000)
	case "mb":
		return int64(val * 1000 * 1000)
	case "gb":
		return int64(val * 1000 * 1000 * 1000)
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

// scanCRLF divide tokens por '\n' ou '\r' para suportar saídas que atualizam a mesma linha.
func scanCRLF(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		j := i + 1
		// Consome \r\n ou \n\r em sequência.
		for j < len(data) && (data[j] == '\r' || data[j] == '\n') {
			j++
		}
		return j, dropCRLF(data[:i]), nil
	}

	if atEOF {
		return len(data), dropCRLF(data), nil
	}

	return 0, nil, nil
}

func newProgressScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Split(scanCRLF)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return s
}

func dropCRLF(data []byte) []byte {
	return bytes.TrimRight(data, "\r\n")
}
