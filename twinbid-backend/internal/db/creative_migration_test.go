package db

import (
	"strings"
	"testing"
)

func TestBuildLegacyBannerADMImage(t *testing.T) {
	adm := buildLegacyBannerADM(
		`https://target.example/path?a=1&b="quoted"`,
		"https://twinbid.io/api/media/11111111-1111-1111-1111-111111111111",
		"image/jpg",
		300,
		250,
	)
	if !strings.Contains(adm, `<a href="https://target.example/path?a=1&amp;b=&#34;quoted&#34;"`) {
		t.Fatalf("target URL was not escaped: %s", adm)
	}
	if !strings.Contains(adm, `<img src="https://twinbid.io/api/media/11111111-1111-1111-1111-111111111111" width="300" height="250"`) {
		t.Fatalf("image markup is invalid: %s", adm)
	}
}

func TestBuildLegacyBannerADMVideo(t *testing.T) {
	adm := buildLegacyBannerADM(
		"https://target.example",
		"https://twinbid.io/api/media/22222222-2222-2222-2222-222222222222",
		"video/mp4",
		300,
		250,
	)
	if !strings.Contains(adm, `<video src="https://twinbid.io/api/media/22222222-2222-2222-2222-222222222222" width="300" height="250"`) {
		t.Fatalf("video markup is invalid: %s", adm)
	}
	if strings.Contains(adm, "<img") {
		t.Fatalf("MP4 migration produced image markup: %s", adm)
	}
}
