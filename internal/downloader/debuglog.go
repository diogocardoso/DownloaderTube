package downloader

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var debugLogMu sync.Mutex

func debugLogPath() string {
	return filepath.Join(os.TempDir(), "downloadertube-debug.log")
}

func debugLogf(format string, args ...any) {
	debugLogMu.Lock()
	defer debugLogMu.Unlock()

	f, err := os.OpenFile(debugLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	ts := time.Now().Format("2006-01-02 15:04:05.000")
	_, _ = fmt.Fprintf(f, "%s %s\n", ts, fmt.Sprintf(format, args...))
}
