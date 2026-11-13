package api

import "testing"

func TestThumbnailCoverURL(t *testing.T) {
	cases := map[string]string{
		"https://p2.music.126.net/hG==/109951.jpg":                          "https://p2.music.126.net/hG==/109951.jpg?param=300y300",
		"https://is1-ssl.mzstatic.com/image/thumb/a/600x600bb.jpg":          "https://is1-ssl.mzstatic.com/image/thumb/a/300x300bb.jpg",
		"https://p3-luna.douyinpic.com/img/a~tplv-x-resize:960:960.png":     "https://p3-luna.douyinpic.com/img/a~tplv-x-resize:300:300.png",
		"https://img01.dmhmusic.com/0210/M00/a.jpg":                         "https://img01.dmhmusic.com/0210/M00/a.jpg?imageView2/1/w/300/h/300",
		"https://y.gtimg.cn/music/photo_new/T002R300x300M000abc.jpg":        "https://y.gtimg.cn/music/photo_new/T002R300x300M000abc.jpg",
	}
	for input, want := range cases {
		if got := thumbnailCoverURL(input); got != want {
			t.Fatalf("thumbnailCoverURL(%q) = %q, want %q", input, got, want)
		}
	}
}
