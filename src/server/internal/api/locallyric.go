package api

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// localLyric returns lyrics for a file inside the user's local music folder.
// A sidecar .lrc file wins; otherwise embedded tags (ID3 USLT / FLAC Vorbis
// comment) are used.
func (s *Server) localLyric(user *AppUser, name string) string {
	if user == nil || strings.TrimSpace(name) == "" {
		return ""
	}
	name = cleanUploadName(name)
	if !isAudioFile(name) {
		return ""
	}
	dir := s.userLocalDir(user)
	if lrc := readSidecarLRC(dir, name); lrc != "" {
		return lrc
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return embeddedLyric(data)
}

func readSidecarLRC(dir, name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	candidates := []string{
		filepath.Join(dir, base+".lrc"),
		filepath.Join(dir, base+".LRC"),
		filepath.Join(dir, name+".lrc"),
	}
	for _, path := range candidates {
		if data, err := os.ReadFile(path); err == nil && len(bytes.TrimSpace(data)) > 0 {
			return string(data)
		}
	}
	return ""
}

func embeddedLyric(data []byte) string {
	if len(data) < 10 {
		return ""
	}
	if string(data[:3]) == "ID3" {
		return id3Lyric(data)
	}
	if string(data[:4]) == "fLaC" {
		return flacLyric(data)
	}
	return ""
}

func id3Lyric(data []byte) string {
	version := data[3]
	size := int(data[6]&0x7f)<<21 | int(data[7]&0x7f)<<14 | int(data[8]&0x7f)<<7 | int(data[9]&0x7f)
	end := 10 + size
	if end > len(data) {
		end = len(data)
	}
	offset := 10
	for offset+10 <= end {
		id := string(data[offset : offset+4])
		if !isFrameID(id) {
			break
		}
		var frameSize int
		if version >= 4 {
			frameSize = int(data[offset+4]&0x7f)<<21 | int(data[offset+5]&0x7f)<<14 | int(data[offset+6]&0x7f)<<7 | int(data[offset+7]&0x7f)
		} else {
			frameSize = int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
		}
		start := offset + 10
		if frameSize <= 0 || start+frameSize > end {
			break
		}
		if id == "USLT" {
			if text := decodeID3Text(data[start : start+frameSize]); text != "" {
				return text
			}
		}
		offset = start + frameSize
	}
	return ""
}

func isFrameID(id string) bool {
	if len(id) != 4 {
		return false
	}
	for _, r := range id {
		if !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// decodeID3Text handles a USLT frame: encoding byte, 3-byte language, a
// null-terminated content descriptor and then the lyric text.
func decodeID3Text(frame []byte) string {
	if len(frame) < 5 {
		return ""
	}
	enc := frame[0]
	body := frame[4:]
	var text []byte
	switch enc {
	case 1, 2:
		i := 0
		for i+1 < len(body) {
			if body[i] == 0 && body[i+1] == 0 {
				i += 2
				break
			}
			i += 2
		}
		text = body[i:]
	default:
		i := bytes.IndexByte(body, 0)
		if i < 0 {
			i = 0
		} else {
			i++
		}
		text = body[i:]
	}
	return strings.TrimSpace(decodeID3String(enc, text))
}

func decodeID3String(enc byte, raw []byte) string {
	switch enc {
	case 1:
		if len(raw)%2 == 1 {
			raw = raw[:len(raw)-1]
		}
		units := make([]uint16, 0, len(raw)/2)
		for i := 0; i+1 < len(raw); i += 2 {
			units = append(units, binary.LittleEndian.Uint16(raw[i:i+2]))
		}
		return string(utf16.Decode(units))
	case 2:
		if len(raw)%2 == 1 {
			raw = raw[:len(raw)-1]
		}
		units := make([]uint16, 0, len(raw)/2)
		for i := 0; i+1 < len(raw); i += 2 {
			units = append(units, binary.BigEndian.Uint16(raw[i:i+2]))
		}
		return string(utf16.Decode(units))
	default:
		return strings.TrimRight(string(raw), "\x00")
	}
}

// flacLyric reads a LYRICS / UNSYNCEDLYRICS entry from the Vorbis comment block.
func flacLyric(data []byte) string {
	offset := 4
	for offset+4 <= len(data) {
		header := data[offset]
		last := header&0x80 != 0
		blockType := header & 0x7f
		length := int(data[offset+1])<<16 | int(data[offset+2])<<8 | int(data[offset+3])
		start := offset + 4
		if start+length > len(data) {
			return ""
		}
		if blockType == 4 {
			if text := vorbisLyric(data[start : start+length]); text != "" {
				return text
			}
		}
		if last {
			break
		}
		offset = start + length
	}
	return ""
}

func vorbisLyric(block []byte) string {
	if len(block) < 8 {
		return ""
	}
	pos := 4
	readUint32 := func() (int, bool) {
		if pos+4 > len(block) {
			return 0, false
		}
		v := int(binary.LittleEndian.Uint32(block[pos : pos+4]))
		pos += 4
		return v, true
	}
	count, ok := readUint32()
	if !ok {
		return ""
	}
	// Skip the vendor string.
	vendorLen, ok := readUint32()
	if !ok || pos+vendorLen > len(block) {
		return ""
	}
	pos += vendorLen
	for i := 0; i < count; i++ {
		size, ok := readUint32()
		if !ok || pos+size > len(block) {
			return ""
		}
		entry := string(block[pos : pos+size])
		pos += size
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(key)) {
		case "LYRICS", "UNSYNCEDLYRICS", "UNSYNCED LYRICS", "LYRIC":
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}
