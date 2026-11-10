package api

import (
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// audioMetadata returns duration in seconds and bitrate in kbps for the most
// common local audio formats. It is intentionally lightweight: MP3 uses the
// first frame's bitrate (CBR estimate), FLAC reads STREAMINFO, and WAV uses the
// fmt/data chunks.
func audioMetadata(path string, size int64) (duration, bitrate int) {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "mp3":
		return mp3Metadata(path, size)
	case "flac":
		return flacMetadata(path, size)
	case "wav":
		return wavMetadata(path)
	default:
		return 0, 0
	}
}

func mp3Metadata(path string, size int64) (duration, bitrate int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	head := make([]byte, 256*1024)
	n, _ := io.ReadFull(f, head)
	head = head[:n]

	offset := 0
	if len(head) >= 10 && string(head[:3]) == "ID3" {
		tagSize := int(head[6]&0x7f)<<21 | int(head[7]&0x7f)<<14 | int(head[8]&0x7f)<<7 | int(head[9]&0x7f)
		offset = 10 + tagSize
	}
	if offset >= len(head) {
		return 0, 0
	}

	for i := offset; i+4 <= len(head); i++ {
		if head[i] != 0xFF || head[i+1]&0xE0 != 0xE0 {
			continue
		}
		b1, b2 := head[i+1], head[i+2]
		version := (b1 >> 3) & 3 // 3=MPEG1, 2=MPEG2, 0=MPEG2.5
		layer := (b1 >> 1) & 3   // 1=Layer III
		bitrateIndex := int(b2 >> 4)
		sampleRateIndex := int((b2 >> 2) & 3)
		if layer != 1 || bitrateIndex == 0 || bitrateIndex == 15 || sampleRateIndex == 3 {
			continue
		}

		var table []int
		if version == 3 {
			table = []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320}
		} else {
			table = []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160}
		}
		bitrate = table[bitrateIndex]
		if bitrate > 0 && size > 0 {
			duration = int(math.Round(float64(size) * 8 / float64(bitrate*1000)))
		}
		return duration, bitrate
	}
	return 0, 0
}

func flacMetadata(path string, size int64) (duration, bitrate int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil || string(magic) != "fLaC" {
		return 0, 0
	}

	for {
		header := make([]byte, 4)
		if _, err := io.ReadFull(f, header); err != nil {
			return 0, 0
		}
		last := header[0]&0x80 != 0
		blockType := header[0] & 0x7f
		length := int(header[1])<<16 | int(header[2])<<8 | int(header[3])

		if blockType == 0 && length >= 34 {
			data := make([]byte, 34)
			if _, err := io.ReadFull(f, data); err != nil {
				return 0, 0
			}
			packed := binary.BigEndian.Uint64(data[10:18])
			sampleRate := int(packed >> 44)
			totalSamples := int64(packed & ((1 << 36) - 1))
			if sampleRate > 0 && totalSamples > 0 {
				duration = int(math.Round(float64(totalSamples) / float64(sampleRate)))
				if duration > 0 {
					bitrate = int(math.Round(float64(size) * 8 / float64(duration) / 1000))
				}
				return duration, bitrate
			}
			return 0, 0
		}

		if _, err := f.Seek(int64(length), io.SeekCurrent); err != nil {
			return 0, 0
		}
		if last {
			return 0, 0
		}
	}
}

func wavMetadata(path string) (duration, bitrate int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil || string(header[:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return 0, 0
	}

	byteRate := 0
	dataSize := int64(0)
	for {
		chunk := make([]byte, 8)
		if _, err := io.ReadFull(f, chunk); err != nil {
			break
		}
		chunkID := string(chunk[:4])
		chunkSize := int64(binary.LittleEndian.Uint32(chunk[4:8]))

		switch chunkID {
		case "fmt ":
			data := make([]byte, chunkSize)
			if _, err := io.ReadFull(f, data); err != nil {
				return 0, 0
			}
			if len(data) >= 12 {
				byteRate = int(binary.LittleEndian.Uint32(data[8:12]))
			}
			if chunkSize%2 == 1 {
				_, _ = f.Seek(1, io.SeekCurrent)
			}
		case "data":
			dataSize = chunkSize
			goto done
		default:
			if _, err := f.Seek(chunkSize, io.SeekCurrent); err != nil {
				return 0, 0
			}
			if chunkSize%2 == 1 {
				_, _ = f.Seek(1, io.SeekCurrent)
			}
		}
	}

done:
	if byteRate > 0 && dataSize > 0 {
		duration = int(math.Round(float64(dataSize) / float64(byteRate)))
		bitrate = int(math.Round(float64(byteRate) * 8 / 1000))
	}
	return duration, bitrate
}
