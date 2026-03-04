package downloader

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ensureWhatsAppCompatible tenta garantir saída em MP4 com vídeo H.264 e áudio AAC.
// Retorna um aviso quando não for possível validar/converter para o formato ideal.
func ensureWhatsAppCompatible(filePath string) (string, string) {
	if strings.TrimSpace(filePath) == "" {
		return filePath, "não foi possível determinar o arquivo final para validar compatibilidade com WhatsApp"
	}

	if _, err := os.Stat(filePath); err != nil {
		return filePath, "arquivo final não foi encontrado para validação de compatibilidade com WhatsApp"
	}

	needTranscode, targetPath, err := needsWhatsAppTranscode(filePath)
	if err != nil {
		return filePath, fmt.Sprintf("não foi possível validar codecs automaticamente (%v)", err)
	}
	if !needTranscode {
		return targetPath, ""
	}

	tempOutput := strings.TrimSuffix(targetPath, filepath.Ext(targetPath)) + " [tmp-whatsapp].mp4"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", filePath,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-profile:v", "high",
		"-level", "4.1",
		"-preset", "veryfast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		tempOutput,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return filePath, fmt.Sprintf("falha ao converter para MP4 H.264/AAC (%s)", strings.TrimSpace(string(output)))
	}

	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return filePath, fmt.Sprintf("arquivo convertido gerado, mas não foi possível substituir o destino (%v)", err)
	}

	if err := os.Rename(tempOutput, targetPath); err != nil {
		return filePath, fmt.Sprintf("arquivo convertido gerado, mas não foi possível finalizar a troca (%v)", err)
	}

	if sameFilePath(filePath, targetPath) {
		return validateWhatsAppOutput(targetPath)
	}

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return targetPath, fmt.Sprintf("arquivo convertido salvo, mas não foi possível remover o original (%v)", err)
	}

	return validateWhatsAppOutput(targetPath)
}

func needsWhatsAppTranscode(filePath string) (bool, string, error) {
	targetPath := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".mp4"

	probe, err := ProbeFile(filePath)
	if err != nil {
		return false, "", fmt.Errorf("erro ao validar codecs do arquivo baixado: %w", err)
	}

	videoCodec := strings.ToLower(probe.VideoCodec)
	audioCodec := strings.ToLower(probe.AudioCodec)
	ext := strings.ToLower(filepath.Ext(filePath))

	needVideo := !probe.HasVideo || videoCodec != "h264"
	needAudio := probe.HasAudio && audioCodec != "aac"
	needExt := ext != ".mp4"

	return needVideo || needAudio || needExt, targetPath, nil
}

func sameFilePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return strings.EqualFold(absA, absB)
}

func validateWhatsAppOutput(path string) (string, string) {
	needTranscode, _, err := needsWhatsAppTranscode(path)
	if err != nil {
		return path, fmt.Sprintf("conversão aplicada, mas não foi possível validar codecs finais (%v)", err)
	}
	if needTranscode {
		return path, "arquivo final ainda pode ser incompatível com WhatsApp (esperado MP4 H.264/AAC)"
	}
	return path, ""
}
