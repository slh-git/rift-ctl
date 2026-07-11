package images

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/blacktop/go-termimg"
)

func TestDisplay(t *testing.T) {
	path := writeTestPNG(t)
	var buf bytes.Buffer

	img, err := termimg.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := img.Width(8).Protocol(termimg.Halfblocks).Scale(termimg.ScaleFit).Render()
	if err != nil {
		t.Fatal(err)
	}
	if rendered == "" {
		t.Fatal("expected non-empty render")
	}

	// Exercise Display as well (auto protocol; still must open/render without panic).
	if err := Display(&buf, path, 8); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected Display to write output")
	}
}

func writeTestPNG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "card.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 200, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
