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
			name: "native with image",
			creative: models.Creative{CreativeName: "native", ADM: "markup", FormatType: "native",
				ImageID: &imageID, Title: &title, Description: &description},
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
	if err := validateCreativeMediaFormat("native", "video/mp4"); err == nil {
		t.Fatal("native MP4 must be rejected")
	}
	if err := validateCreativeMediaFormat("push", "image/png"); err != nil {
		t.Fatalf("push image was rejected: %v", err)
	}
}
