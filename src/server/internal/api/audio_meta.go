package api

import (
	"bytes"
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
	case "m4a", "mp4", "m4b":
		return mp4Metadata(path, size)
	case "ogg", "opus":
		return oggMetadata(path, size)
	case "ape":
		return apeMetadata(path, size)
	case "aiff", "aif", "aifc":
		return aiffMetadata(path, size)
	default:
		return 0, 0
	}
}

// mp4Metadata walks the atom tree and reads the movie header. m4a files were
// the main gap in the local library: their duration and bitrate used to show
// as zero in the list.
func mp4Metadata(path string, size int64) (duration, bitrate int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, 0
	}
	total := info.Size()
	for offset := int64(0); offset+8 <= total; {
		var header [8]byte
		if _, err := f.ReadAt(header[:], offset); err != nil {
			return 0, 0
		}
		boxSize := int64(binary.BigEndian.Uint32(header[:4]))
		boxType := string(header[4:8])
		headerLen := int64(8)
		if boxSize == 1 {
			var large [8]byte
			if _, err := f.ReadAt(large[:], offset+8); err != nil {
				return 0, 0
			}
			boxSize = int64(binary.BigEndian.Uint64(large[:]))
			headerLen = 16
		} else if boxSize == 0 {
			boxSize = total - offset
		}
		if boxSize < headerLen {
			return 0, 0
		}
		if boxType == "moov" {
			payload := make([]byte, boxSize-headerLen)
			if _, err := f.ReadAt(payload, offset+headerLen); err != nil {
				return 0, 0
			}
			seconds := mp4MovieSeconds(payload)
			if seconds > 0 {
				duration = int(math.Round(seconds))
				if duration > 0 {
					bitrate = int(math.Round(float64(size) * 8 / float64(duration) / 1000))
				}
			}
			return duration, bitrate
		}
		offset += boxSize
	}
	return 0, 0
}

func mp4MovieSeconds(data []byte) float64 {
	for offset := 0; offset+8 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[offset:]))
		kind := string(data[offset+4 : offset+8])
		headerLen := 8
		if size == 1 && offset+16 <= len(data) {
			size = int(binary.BigEndian.Uint64(data[offset+8:]))
			headerLen = 16
		}
		if size < headerLen || offset+size > len(data) {
			return 0
		}
		body := data[offset+headerLen : offset+size]
		switch kind {
		case "mvhd", "mdhd":
			if len(body) >= 20 {
				if body[0] == 1 && len(body) >= 32 {
					timescale := binary.BigEndian.Uint32(body[20:24])
					units := binary.BigEndian.Uint64(body[24:32])
					if timescale > 0 && units > 0 {
						return float64(units) / float64(timescale)
					}
				} else if body[0] == 0 {
					timescale := binary.BigEndian.Uint32(body[12:16])
					units := binary.BigEndian.Uint32(body[16:20])
					if timescale > 0 && units > 0 {
						return float64(units) / float64(timescale)
					}
				}
			}
		case "trak", "mdia":
			if seconds := mp4MovieSeconds(body); seconds > 0 {
				return seconds
			}
		}
		offset += size
	}
	return 0
}

// oggMetadata reads the codec sample rate from the first page and the last
// granule position from the tail. Works for Opus and Vorbis streams.
func oggMetadata(path string, size int64) (duration, bitrate int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, 0
	}
	head := make([]byte, 64*1024)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	if len(head) < 4 || string(head[0:4]) != "OggS" {
		return 0, 0
	}
	sampleRate := 0
	if idx := bytes.Index(head, []byte("OpusHead")); idx >= 0 {
		sampleRate = 48000
	} else if idx := bytes.Index(head, []byte("\x01vorbis")); idx >= 0 && idx+16 <= len(head) {
		sampleRate = int(binary.LittleEndian.Uint32(head[idx+12 : idx+16]))
	}
	if sampleRate <= 0 {
		return 0, 0
	}
	tailLen := int64(64 * 1024)
	if info.Size() < tailLen {
		tailLen = info.Size()
	}
	tail := make([]byte, tailLen)
	if _, err := f.ReadAt(tail, info.Size()-tailLen); err != nil {
		return 0, 0
	}
	last := bytes.LastIndex(tail, []byte("OggS"))
	if last < 0 || last+14 > len(tail) {
		return 0, 0
	}
	granule := int64(binary.LittleEndian.Uint64(tail[last+6 : last+14]))
	if granule <= 0 {
		return 0, 0
	}
	duration = int(math.Round(float64(granule) / float64(sampleRate)))
	if duration > 0 {
		bitrate = int(math.Round(float64(size) * 8 / float64(duration) / 1000))
	}
	return duration, bitrate
}

func apeMetadata(path string, size int64) (duration, bitrate int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	header := make([]byte, 32)
	if _, err := io.ReadFull(f, header); err != nil || string(header[:4]) != "MAC " {
		return 0, 0
	}
	version := int(binary.LittleEndian.Uint16(header[4:6]))
	if version < 3980 {
		return 0, 0
	}
	blocksPerFrame := binary.LittleEndian.Uint32(header[12:16])
	finalFrameBlocks := binary.LittleEndian.Uint32(header[16:20])
	totalFrames := binary.LittleEndian.Uint32(header[20:24])
	sampleRate := binary.LittleEndian.Uint32(header[28:32])
	if sampleRate == 0 || totalFrames == 0 {
		return 0, 0
	}
	samples := int64(totalFrames-1)*int64(blocksPerFrame) + int64(finalFrameBlocks)
	duration = int(math.Round(float64(samples) / float64(sampleRate)))
	if duration > 0 {
		bitrate = int(math.Round(float64(size) * 8 / float64(duration) / 1000))
	}
	return duration, bitrate
}

func aiffMetadata(path string, size int64) (duration, bitrate int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil || string(header[:4]) != "FORM" {
		return 0, 0
	}
	for {
		chunk := make([]byte, 8)
		if _, err := io.ReadFull(f, chunk); err != nil {
			return 0, 0
		}
		chunkSize := int64(binary.BigEndian.Uint32(chunk[4:8]))
		if string(chunk[:4]) == "COMM" && chunkSize >= 18 {
			data := make([]byte, 18)
			if _, err := io.ReadFull(f, data); err != nil {
				return 0, 0
			}
			frames := binary.BigEndian.Uint32(data[2:6])
			rate := extendedFloat(data[8:18])
			if rate > 0 && frames > 0 {
				duration = int(math.Round(float64(frames) / rate))
				if duration > 0 {
					bitrate = int(math.Round(float64(size) * 8 / float64(duration) / 1000))
				}
				return duration, bitrate
			}
			return 0, 0
		}
		if _, err := f.Seek(chunkSize, io.SeekCurrent); err != nil {
			return 0, 0
		}
	}
}

// extendedFloat decodes the 80-bit IEEE 754 value used by AIFF sample rates.
func extendedFloat(b []byte) float64 {
	if len(b) < 10 {
		return 0
	}
	exponent := int(b[0]&0x7f)<<8 | int(b[1])
	mantissa := binary.BigEndian.Uint64(b[2:10])
	if exponent == 0 && mantissa == 0 {
		return 0
	}
	value := float64(mantissa) * math.Pow(2, float64(exponent-16383-63))
	if b[0]&0x80 != 0 {
		return -value
	}
	return value
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
