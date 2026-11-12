package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/guohuiyuan/go-music-dl/core"
)

// AudioInfo describes the file behind a resolved playback URL. A truncated
// preview keeps the original container header, so its average bitrate comes out
// far below any real music stream — that is what makes previews detectable.
type AudioInfo struct {
	Size      int64   `json:"size"`
	Duration  float64 `json:"duration"`
	Bitrate   float64 `json:"bitrate"`
	Truncated bool    `json:"truncated"`
}

const audioProbeBytes = 128 * 1024

// fetchAudioInfo reads the head of the media file once and derives size,
// container duration and the average bitrate.
func fetchAudioInfo(rawURL, source string) (*AudioInfo, error) {
	req, err := core.BuildSourceRequest(http.MethodGet, rawURL, source, fmt.Sprintf("bytes=0-%d", audioProbeBytes-1))
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("probe status %d", resp.StatusCode)
	}
	head, err := io.ReadAll(io.LimitReader(resp.Body, audioProbeBytes))
	if err != nil {
		return nil, err
	}

	info := &AudioInfo{}
	if total, ok := parseContentRangeTotal(resp.Header.Get("Content-Range")); ok && total > 0 {
		info.Size = total
	} else if resp.ContentLength > 0 && resp.StatusCode == http.StatusOK {
		info.Size = resp.ContentLength
	}

	ext := strings.ToLower(core.DetectAudioExtBySignature(head))
	switch ext {
	case "flac":
		info.Duration = flacDuration(head)
	default:
		info.Duration = mp3Duration(head)
	}

	if info.Size > 0 && info.Duration > 0 {
		info.Bitrate = float64(info.Size) * 8 / info.Duration / 1000
		// A real stream never averages below ~64 kbps; anything lower means the
		// bytes stop long before the duration the header claims.
		info.Truncated = info.Bitrate < 64
	}
	return info, nil
}

func parseContentRangeTotal(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	slash := strings.LastIndex(value, "/")
	if slash < 0 || slash == len(value)-1 {
		return 0, false
	}
	var total int64
	if _, err := fmt.Sscanf(value[slash+1:], "%d", &total); err != nil {
		return 0, false
	}
	return total, total > 0
}

func mp3Duration(data []byte) float64 {
	if len(data) < 4 {
		return 0
	}
	offset := 0
	if string(data[0:3]) == "ID3" && len(data) > 10 {
		size := int(data[6]&0x7f)<<21 | int(data[7]&0x7f)<<14 | int(data[8]&0x7f)<<7 | int(data[9]&0x7f)
		offset = 10 + size
	}
	if offset >= len(data) {
		return 0
	}
	// Find the first frame header.
	for i := offset; i+4 < len(data); i++ {
		if data[i] != 0xFF || data[i+1]&0xE0 != 0xE0 {
			continue
		}
		version := (data[i+1] >> 3) & 0x03
		sampleRates := []int{44100, 48000, 32000}
		srIndex := int(data[i+2] >> 2 & 0x03)
		if srIndex >= len(sampleRates) || version == 1 {
			continue
		}
		sampleRate := sampleRates[srIndex]
		if version == 2 || version == 0 {
			sampleRate /= (2 + boolToInt(version == 0))
		}
		// Xing/Info lives right after the side information.
		side := 32
		if version == 2 || version == 0 {
			side = 17
		}
		tagStart := i + 4 + side
		if tagStart+8 < len(data) {
			tag := string(data[tagStart : tagStart+4])
			if tag == "Xing" || tag == "Info" {
				flags := int(data[tagStart+4])<<24 | int(data[tagStart+5])<<16 | int(data[tagStart+6])<<8 | int(data[tagStart+7])
				pos := tagStart + 8
				if flags&0x01 != 0 && pos+4 <= len(data) {
					frames := int(data[pos])<<24 | int(data[pos+1])<<16 | int(data[pos+2])<<8 | int(data[pos+3])
					samples := 1152
					if version == 2 || version == 0 {
						samples = 576
					}
					if frames > 0 && sampleRate > 0 {
						return float64(frames) * float64(samples) / float64(sampleRate)
					}
				}
			}
		}
		// Fall back to the TLEN tag when the frame header has no Xing block.
		break
	}
	if idx := strings.Index(string(data[:minInt(len(data), 8192)]), "TLEN"); idx >= 0 && idx+8 < len(data) {
		var ms int
		if _, err := fmt.Sscanf(string(data[idx+4:idx+12]), "%d", &ms); err == nil && ms > 0 {
			return float64(ms) / 1000
		}
	}
	return 0
}

func flacDuration(data []byte) float64 {
	if len(data) < 42 || string(data[0:4]) != "fLaC" {
		return 0
	}
	// First metadata block must be STREAMINFO (type 0).
	pos := 4
	for pos+4 <= len(data) {
		blockType := data[pos] & 0x7f
		length := int(data[pos+1])<<16 | int(data[pos+2])<<8 | int(data[pos+3])
		body := pos + 4
		if blockType == 0 && body+18 <= len(data) {
			stream := data[body : body+18]
			sampleRate := int(stream[10])<<12 | int(stream[11])<<4 | int(stream[12])>>4
			totalSamples := int64(stream[13]&0x0f)<<32 | int64(stream[14])<<24 | int64(stream[15])<<16 |
				int64(stream[16])<<8 | int64(stream[17])
			if sampleRate > 0 && totalSamples > 0 {
				return float64(totalSamples) / float64(sampleRate)
			}
			return 0
		}
		pos = body + length
		if data[pos-4]&0x80 != 0 {
			break
		}
	}
	return 0
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
