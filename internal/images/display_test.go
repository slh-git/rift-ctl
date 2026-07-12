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

func TestDisplayGalleryRow(t *testing.T) {
	a := writeTestPNG(t)
	b := writeTestPNG(t)
	var buf bytes.Buffer
	if err := DisplayGalleryRow(&buf, []string{a, b, ""}, 8, 1); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected gallery output")
	}
}

func TestDisplayGalleryRowDistinct(t *testing.T) {
	red := writeSolidPNG(t, color.RGBA{R: 255, A: 255})
	blue := writeSolidPNG(t, color.RGBA{B: 255, A: 255})

	// Prime the broken empty-path cache, then ensure gallery rows still differ.
	termimg.ClearResizeCache()
	redImg := mustDecodePNG(t, red)
	blueImg := mustDecodePNG(t, blue)
	_ = termimg.ResizeImage(redImg, 80, 120, "")
	_ = termimg.ResizeImage(blueImg, 80, 120, "") // would return red if keyed only on size

	var first, second bytes.Buffer
	if err := DisplayGalleryRow(&first, []string{red, red}, 8, 1); err != nil {
		t.Fatal(err)
	}
	if err := DisplayGalleryRow(&second, []string{blue, blue}, 8, 1); err != nil {
		t.Fatal(err)
	}
	if first.Len() == 0 || second.Len() == 0 {
		t.Fatal("expected gallery output")
	}

	// Force halfblocks so payload reflects pixels (Kitty embeds unique image IDs).
	termimg.ClearResizeCache()
	a, err := termimg.New(mustDecodePNG(t, red)).Protocol(termimg.Halfblocks).Width(8).Scale(termimg.ScaleNone).Render()
	if err != nil {
		t.Fatal(err)
	}
	b, err := termimg.New(mustDecodePNG(t, blue)).Protocol(termimg.Halfblocks).Width(8).Scale(termimg.ScaleNone).Render()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("ScaleNone still produced identical output for different images")
	}
}

func mustDecodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestGalleryLayout(t *testing.T) {
	cols, width, gap := GalleryLayout(&bytes.Buffer{})
	if cols != SearchColumns {
		t.Fatalf("columns = %d, want %d", cols, SearchColumns)
	}
	if width != SearchWidth {
		t.Fatalf("width = %d, want default %d", width, SearchWidth)
	}
	if gap != SearchGap {
		t.Fatalf("gap = %d, want %d", gap, SearchGap)
	}
}

func TestGalleryLayoutWidth(t *testing.T) {
	// 5 across with gap 1: width = (termCols - 4) / 5
	cases := []struct {
		termCols int
		want     int
	}{
		{84, 16},  // (84-4)/5 = 16 — current default look
		{80, 15},
		{120, 23},
		{40, 8}, // floors to SearchMinWidth
	}
	for _, tc := range cases {
		got := (tc.termCols - (SearchColumns-1)*SearchGap) / SearchColumns
		if got < SearchMinWidth {
			got = SearchMinWidth
		}
		if got != tc.want {
			t.Fatalf("term=%d: width=%d, want %d", tc.termCols, got, tc.want)
		}
	}
}

func writeTestPNG(t *testing.T) string {
	t.Helper()
	return writeSolidPNG(t, color.RGBA{R: 80, G: 120, B: 200, A: 255})
}

func writeSolidPNG(t *testing.T, c color.RGBA) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "card.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 24))
	for y := 0; y < 24; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, c)
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
