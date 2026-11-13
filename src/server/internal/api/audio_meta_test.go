package api

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func mp4Box(kind string, payload []byte) []byte {
	box := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(box[:4], uint32(len(box)))
	copy(box[4:8], kind)
	copy(box[8:], payload)
	return box
}

func TestMP4MetadataReadsMovieHeader(t *testing.T) {
	mvhd := make([]byte, 20)
	mvhd[0] = 0
	binary.BigEndian.PutUint32(mvhd[12:16], 1000)  // timescale
	binary.BigEndian.PutUint32(mvhd[16:20], 123456) // duration in timescale units

	data := append(
		mp4Box("ftyp", []byte("M4A \x00\x00\x00\x00")),
		mp4Box("moov", mp4Box("mvhd", mvhd))...,
	)
	path := filepath.Join(t.TempDir(), "sample.m4a")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	duration, _ := audioMetadata(path, int64(len(data)))
	if duration != 123 {
		t.Fatalf("duration = %d, want 123", duration)
	}
}

func TestAudioMetadataIgnoresUnknownFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.wma")
	if err := os.WriteFile(path, []byte("not audio"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	if duration, bitrate := audioMetadata(path, 9); duration != 0 || bitrate != 0 {
		t.Fatalf("unexpected metadata: %d %d", duration, bitrate)
	}
}
