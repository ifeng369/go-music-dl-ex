package api

import "testing"

func TestAlternativeArtistMatches(t *testing.T) {
	cases := []struct {
		wanted    string
		candidate string
		match     bool
	}{
		{"周杰伦", "周杰伦", true},
		{"周杰伦", "林俊杰", false},
		{"周杰伦、方文山", "周杰伦", true},
		{"周杰伦", "", true},
		{"", "林俊杰", true},
		{"未知歌手", "林俊杰", true},
		{"Jay Chou", "Jay Chou", true},
		{"Jay Chou", "JJ Lin", false},
	}
	for _, item := range cases {
		if got := alternativeArtistMatches(item.wanted, item.candidate); got != item.match {
			t.Fatalf("alternativeArtistMatches(%q, %q) = %v, want %v",
				item.wanted, item.candidate, got, item.match)
		}
	}
}
