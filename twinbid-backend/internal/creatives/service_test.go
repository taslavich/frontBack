package creatives

import (
	"encoding/json"
	"os"
	"testing"

	"twinbid-backend/internal/models"
)

func TestValidateCreativeImageRules(t *testing.T) {
	imageID := "11111111-1111-1111-1111-111111111111"
	img := "img"
	iframe := "iframe"
	w, h := 300, 250
	title, description := "title", "description"

	tests := []struct {
		name     string
		creative models.Creative
		wantErr  bool
	}{
		{
			name: "banner img requires and accepts image",
			creative: models.Creative{CreativeName: "banner", ADM: "<a><img></a>", FormatType: "banner",
				BannerType: &img, ImageID: &imageID, W: &w, H: &h},
		},
		{
			name: "banner iframe rejects image",
			creative: models.Creative{CreativeName: "iframe", ADM: "<iframe></iframe>", FormatType: "banner",
				BannerType: &iframe, ImageID: &imageID, W: &w, H: &h},
			wantErr: true,
		},
		{
			name: "banner iframe without image",
			creative: models.Creative{CreativeName: "iframe", ADM: "<iframe></iframe>", FormatType: "banner",
				BannerType: &iframe, W: &w, H: &h},
		},
		{
			name: "native requires image",
			creative: models.Creative{CreativeName: "native", ADM: "markup", FormatType: "native",
				Title: &title, Description: &description},
			wantErr: true,
		},
		{
			name: "native requires dimensions",
			creative: models.Creative{CreativeName: "native", ADM: "markup", FormatType: "native",
				ImageID: &imageID, Title: &title, Description: &description},
			wantErr: true,
		},
		{
			name: "native with image and dimensions",
			creative: models.Creative{CreativeName: "native", ADM: "markup", FormatType: "native",
				ImageID: &imageID, W: &w, H: &h, Title: &title, Description: &description},
		},
		{
			name: "push requires dimensions",
			creative: models.Creative{CreativeName: "push", ADM: "markup", FormatType: "push",
				ImageID: &imageID, Title: &title, Description: &description},
			wantErr: true,
		},
		{
			name: "push with image and dimensions",
			creative: models.Creative{CreativeName: "push", ADM: "markup", FormatType: "push",
				ImageID: &imageID, W: &w, H: &h, Title: &title, Description: &description},
		},
		{
			name: "popunder rejects image",
			creative: models.Creative{CreativeName: "pop", ADM: "markup", FormatType: "popunder",
				ImageID: &imageID},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateCreative(test.creative)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateCreative() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestPatchImageIDDistinguishesOmittedAndNull(t *testing.T) {
	var omitted PatchCreativeRequest
	if err := json.Unmarshal([]byte(`{"adm":"markup"}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.ImageID.Set {
		t.Fatal("omitted image_id must not be marked as set")
	}

	var explicitNull PatchCreativeRequest
	if err := json.Unmarshal([]byte(`{"image_id":null}`), &explicitNull); err != nil {
		t.Fatal(err)
	}
	if !explicitNull.ImageID.Set || explicitNull.ImageID.Value != nil {
		t.Fatalf("image_id:null was not decoded as explicit null: %#v", explicitNull.ImageID)
	}
}

func TestInspectImageUsesActualContentType(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "image-*.png")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}
	if _, err := file.Write(pngHeader); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	_, mimeType, extension, err := inspectCreativeMedia(file, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/png" || extension != "png" {
		t.Fatalf("got mime=%q extension=%q", mimeType, extension)
	}
}

func TestInspectCreativeMediaPreservesJPEGAlias(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "creative-*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	jpegHeader := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01}
	if _, err := file.Write(jpegHeader); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	_, mimeType, extension, err := inspectCreativeMedia(file, "image/jpg")
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/jpg" || extension != "jpg" {
		t.Fatalf("got mime=%q extension=%q", mimeType, extension)
	}
}

func TestInspectCreativeMediaAcceptsMP4(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "creative-*.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	mp4Header := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0x00, 0x00, 0x00, 0x01, 'i', 's', 'o', 'm'}
	if _, err := file.Write(mp4Header); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	_, mimeType, extension, err := inspectCreativeMedia(file, "video/mp4")
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "video/mp4" || extension != "mp4" {
		t.Fatalf("got mime=%q extension=%q", mimeType, extension)
	}
}

func TestInspectCreativeMediaEnforcesPerTypeLimits(t *testing.T) {
	t.Run("image limit", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "creative-*.png")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		pngHeader := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
		if _, err := file.Write(pngHeader); err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxCreativeImageSize + 1); err != nil {
			t.Fatal(err)
		}
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := inspectCreativeMedia(file, "image/png"); err == nil {
			t.Fatal("expected oversized image to be rejected")
		}
	})

	t.Run("video limit", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "creative-*.mp4")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		mp4Header := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}
		if _, err := file.Write(mp4Header); err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(maxCreativeVideoSize + 1); err != nil {
			t.Fatal(err)
		}
		if _, err := file.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := inspectCreativeMedia(file, "video/mp4"); err == nil {
			t.Fatal("expected oversized MP4 to be rejected")
		}
	})
}

func TestValidateCreativeMediaFormat(t *testing.T) {
	if err := validateCreativeMediaFormat("banner", "video/mp4"); err != nil {
		t.Fatalf("banner MP4 was rejected: %v", err)
	}
	if err := validateCreativeMediaFormat("video", "video/mp4"); err != nil {
		t.Fatalf("video MP4 was rejected: %v", err)
	}
	if err := validateCreativeMediaFormat("video", "image/png"); err == nil {
		t.Fatal("video image must be rejected")
	}
	if err := validateCreativeMediaFormat("native", "video/mp4"); err == nil {
		t.Fatal("native MP4 must be rejected")
	}
	if err := validateCreativeMediaFormat("push", "image/png"); err != nil {
		t.Fatalf("push image was rejected: %v", err)
	}
}

func TestTrackerMacrosUseUserParameterNames(t *testing.T) {
	macros := nonNilMacroMap(models.MacroMap{
		"site_id":      "source",
		"country_code": "geo",
		"click_id":     "subid",
	})
	if err := validateTrackerMacros(macros); err != nil {
		t.Fatalf("valid macro map rejected: %v", err)
	}
	if macros["site_id"] != "source" || macros["click_id"] != "subid" {
		t.Fatalf("unexpected normalized macros: %#v", macros)
	}
}

func TestTrackerMacrosDefaultClickIDForLegacyRequests(t *testing.T) {
	creative := models.Creative{FormatType: "popunder", TrackersMacros: models.MacroMap{"site_id": "site_id"}}
	normalizeTrackerMacrosForCreative(&creative)
	if creative.TrackersMacros["click_id"] != "click_id" {
		t.Fatalf("click_id default got %q", creative.TrackersMacros["click_id"])
	}
}

func TestTrackerMacrosRemainEmptyForIframeBanner(t *testing.T) {
	iframe := "iframe"
	creative := models.Creative{
		FormatType:     "banner",
		BannerType:     &iframe,
		TrackersMacros: models.MacroMap{"click_id": "subid", "site_id": "source"},
	}
	normalizeTrackerMacrosForCreative(&creative)
	if len(creative.TrackersMacros) != 0 {
		t.Fatalf("iframe trackers_macros must stay empty: %#v", creative.TrackersMacros)
	}
}

func TestTrackerMacrosRejectDuplicateParameterNames(t *testing.T) {
	err := validateTrackerMacros(models.MacroMap{
		"site_id":  "subid",
		"click_id": "subid",
	})
	if err == nil {
		t.Fatal("duplicate parameter names must be rejected")
	}
}

func TestTrackerMacrosRejectUnknownKeysAndInvalidNames(t *testing.T) {
	if err := validateTrackerMacros(models.MacroMap{"unknown": "value"}); err == nil {
		t.Fatal("unknown macro key must be rejected")
	}
	if err := validateTrackerMacros(models.MacroMap{"site_id": "bad&name"}); err == nil {
		t.Fatal("invalid query parameter name must be rejected")
	}
}

func TestValidateVideoCreative(t *testing.T) {
	w, h := 1920, 1080
	imageID := "11111111-1111-1111-1111-111111111111"
	videoFormat := models.VideoFormatInstream
	skippable := true
	creative := models.Creative{
		CreativeName: "video",
		ADM:          `https://advertiser.example/landing?campaign=video`,
		FormatType:   "video",
		W:            &w,
		H:            &h,
		ImageID:      &imageID,
		VideoFormat:  &videoFormat,
		VideoMetadata: &models.VideoCreativeMetadata{
			Mimes: []string{"video/mp4"}, Duration: 20, Protocols: []int{2, 3, 7},
			API: []int{2}, Bitrate: 1200, Linearity: 1, Skippable: &skippable,
			Width: 1920, Height: 1080, Codec: "h264", FileSize: 5 << 20,
		},
	}
	if err := validateCreative(creative); err != nil {
		t.Fatalf("valid VIDEO creative rejected: %v", err)
	}

	creative.VideoMetadata = nil
	if err := validateCreative(creative); err == nil {
		t.Fatal("VIDEO creative without technical metadata must be rejected")
	}
	creative.VideoMetadata = &models.VideoCreativeMetadata{
		Mimes: []string{"video/mp4"}, Duration: 20, Protocols: []int{2, 3, 7},
		Linearity: 1, Width: 1920, Height: 1080, Codec: "h264", FileSize: 5 << 20,
	}

	badFormat := "placement_5"
	creative.VideoFormat = &badFormat
	if err := validateCreative(creative); err == nil {
		t.Fatal("unknown video_format must be rejected")
	}
	creative.VideoFormat = &videoFormat
	creative.ADM = `javascript:alert(1)`
	if err := validateCreative(creative); err == nil {
		t.Fatal("non-http advertiser URL must be rejected")
	}
}

func TestParseFFProbeVideoMetadata(t *testing.T) {
	raw := []byte(`{
		"streams":[{"codec_name":"h264","width":1920,"height":1080,"bit_rate":"1250000","duration":"19.2"}],
		"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"19.2","bit_rate":"1300000"}
	}`)
	got, err := parseFFProbeVideoMetadata(raw, 5<<20)
	if err != nil {
		t.Fatal(err)
	}
	if got.Width != 1920 || got.Height != 1080 || got.Duration != 20 || got.Codec != "h264" || got.Bitrate != 1250 || got.FileSize != 5<<20 {
		t.Fatalf("unexpected ffprobe metadata: %#v", got)
	}
	if len(got.Protocols) != 3 || got.Protocols[0] != 2 || got.Protocols[1] != 3 || got.Protocols[2] != 7 {
		t.Fatalf("unexpected supported protocols: %#v", got.Protocols)
	}
}

func TestParseFFProbeVideoMetadataRejectsWrongResolutionAndMissingStream(t *testing.T) {
	wrongResolution := []byte(`{"streams":[{"codec_name":"h264","width":1280,"height":720,"duration":"10"}],"format":{}}`)
	if _, err := parseFFProbeVideoMetadata(wrongResolution, 1<<20); err == nil {
		t.Fatal("wrong VIDEO resolution must be rejected")
	}
	if _, err := parseFFProbeVideoMetadata([]byte(`{"streams":[],"format":{}}`), 1<<20); err == nil {
		t.Fatal("MP4 without a video stream must be rejected")
	}
}

func TestPatchVideoFormatDistinguishesOmittedAndNull(t *testing.T) {
	var omitted PatchCreativeRequest
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.VideoFormat.Set {
		t.Fatal("omitted video_format must not be marked as set")
	}

	var provided PatchCreativeRequest
	if err := json.Unmarshal([]byte(`{"video_format":"outstream"}`), &provided); err != nil {
		t.Fatal(err)
	}
	if !provided.VideoFormat.Set || provided.VideoFormat.Value == nil || *provided.VideoFormat.Value != "outstream" {
		t.Fatalf("video_format was not decoded: %#v", provided.VideoFormat)
	}
}

func TestPatchVideoMetadataDistinguishesOmittedAndNull(t *testing.T) {
	var omitted PatchCreativeRequest
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.VideoMetadata.Set {
		t.Fatal("omitted video_metadata must not be marked as set")
	}

	var explicitNull PatchCreativeRequest
	if err := json.Unmarshal([]byte(`{"video_metadata":null}`), &explicitNull); err != nil {
		t.Fatal(err)
	}
	if !explicitNull.VideoMetadata.Set || explicitNull.VideoMetadata.Value != nil {
		t.Fatalf("video_metadata:null was not decoded as explicit null: %#v", explicitNull.VideoMetadata)
	}
}

func TestAllowedVideoMetadataMIMEIncludesRealSSPExamples(t *testing.T) {
	for _, mimeType := range []string{"video/mp4", "video/webm", "application/javascript", "text/javascript"} {
		if !allowedVideoMetadataMIME(mimeType) {
			t.Fatalf("expected VIDEO MIME %q to be accepted", mimeType)
		}
	}
	if allowedVideoMetadataMIME("image/png") {
		t.Fatal("image/png must not be accepted as VIDEO creative MIME")
	}
}

func TestParseSingleByteRange(t *testing.T) {
	tests := []struct {
		header    string
		size      int64
		wantStart int64
		wantEnd   int64
		wantOK    bool
	}{
		{"bytes=0-99", 1000, 0, 99, true},
		{"bytes=100-", 1000, 100, 999, true},
		{"bytes=-100", 1000, 900, 999, true},
		{"bytes=900-2000", 1000, 900, 999, true},
		{"bytes=1000-", 1000, 0, 0, false},
		{"bytes=0-1,4-5", 1000, 0, 0, false},
		{"items=0-10", 1000, 0, 0, false},
	}
	for _, tc := range tests {
		start, end, ok := parseSingleByteRange(tc.header, tc.size)
		if ok != tc.wantOK || (ok && (start != tc.wantStart || end != tc.wantEnd)) {
			t.Fatalf("range %q size=%d got=(%d,%d,%v) want=(%d,%d,%v)", tc.header, tc.size, start, end, ok, tc.wantStart, tc.wantEnd, tc.wantOK)
		}
	}
}
