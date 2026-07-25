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

	_, mimeType, extension, err := inspectImage(file)
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "image/png" || extension != "png" {
		t.Fatalf("got mime=%q extension=%q", mimeType, extension)
	}
}
