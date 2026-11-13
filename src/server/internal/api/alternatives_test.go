package api

import "testing"

func TestSameArtistName(t *testing.T) {
	cases := []struct {
		wanted    string
		candidate string
		match     bool
	}{
		{"周杰伦", "周杰伦", true},
		{"周杰伦", "周杰伦、费玉清", true},
		{"周杰伦、费玉清", "周杰伦", true},
		{"G.E.M.邓紫棋", "邓紫棋", true},
		{"周杰伦", "林俊杰", false},
		{"周杰伦", "张杰", false},
		{"Jay Chou", "Jay Chou", true},
		{"Jay Chou", "JJ Lin", false},
		{"周杰伦", "Jay Chou", false},
		{"周杰伦", "", false},
	}
	for _, item := range cases {
		if got := sameArtistName(item.wanted, item.candidate); got != item.match {
			t.Fatalf("sameArtistName(%q, %q) = %v, want %v",
				item.wanted, item.candidate, got, item.match)
		}
	}
}

func TestUnknownArtist(t *testing.T) {
	for _, value := range []string{"", "  ", "未知歌手", "Unknown Artist"} {
		if !unknownArtist(value) {
			t.Fatalf("unknownArtist(%q) = false", value)
		}
	}
	for _, value := range []string{"周杰伦", "Jay Chou"} {
		if unknownArtist(value) {
			t.Fatalf("unknownArtist(%q) = true", value)
		}
	}
}
