package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// FileProbeInfo contém informações sobre as streams do arquivo baixado.
type FileProbeInfo struct {
	VideoCodec string
	AudioCodec string
	HasVideo   bool
	HasAudio   bool
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
}

type ffprobeStream struct {
	CodecType string `json:"codec_type"`
	CodecName string `json:"codec_name"`
}

// ProbeFile usa ffprobe para inspecionar o arquivo e retornar os codecs de vídeo e áudio.
func ProbeFile(path string) (*FileProbeInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		path,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("erro ao executar ffprobe: %w", err)
	}

	var result ffprobeOutput
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("erro ao parsear saída do ffprobe: %w", err)
	}

	info := &FileProbeInfo{}
	for _, s := range result.Streams {
		switch strings.ToLower(s.CodecType) {
		case "video":
			if !info.HasVideo {
				info.VideoCodec = s.CodecName
				info.HasVideo = true
			}
		case "audio":
			if !info.HasAudio {
				info.AudioCodec = s.CodecName
				info.HasAudio = true
			}
		}
	}

	return info, nil
}
