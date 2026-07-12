package images

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"

	"github.com/blacktop/go-termimg"
	"golang.org/x/image/draw"
	"golang.org/x/term"
)

const (
	DefaultWidth   = 40
	SearchWidth    = 16 // fallback thumb width when terminal size is unknown
	SearchGap      = 1
	SearchColumns  = 5
	SearchMinWidth = 8
)

// Display renders a local image file to w using the best available terminal protocol.
func Display(w io.Writer, path string, widthCells int) error {
	if widthCells <= 0 {
		widthCells = DefaultWidth
	}
	img, err := termimg.Open(path)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	termimg.ClearResizeCache()
	return printRendered(w, img.Width(widthCells).Scale(termimg.ScaleFit))
}

// GalleryLayout picks a thumb width so SearchColumns cards fit the terminal.
// Falls back to SearchWidth when the window size can't be read.
func GalleryLayout(w io.Writer) (columns, cellWidth, gap int) {
	columns = SearchColumns
	gap = SearchGap
	cols, ok := terminalColsExact(w)
	if !ok {
		return columns, SearchWidth, gap
	}
	cellWidth = (cols - (columns-1)*gap) / columns
	if cellWidth < SearchMinWidth {
		cellWidth = SearchMinWidth
	}
	return columns, cellWidth, gap
}

// DisplayGalleryRow renders paths side by side as one terminal image.
// Empty paths become dark placeholders so numbering stays aligned.
func DisplayGalleryRow(w io.Writer, paths []string, cellWidth, gapCells int) error {
	if len(paths) == 0 {
		return nil
	}
	if cellWidth <= 0 {
		cellWidth = SearchWidth
	}
	if gapCells < 0 {
		gapCells = SearchGap
	}

	features := termimg.QueryTerminalFeatures()
	pxPerCell := features.FontWidth
	if pxPerCell <= 0 {
		pxPerCell = 8
	}
	thumbW := cellWidth * pxPerCell
	gapPx := gapCells * pxPerCell

	thumbs := make([]image.Image, len(paths))
	maxH := 0
	for i, path := range paths {
		thumb, err := loadThumb(path, thumbW)
		if err != nil {
			thumb = placeholder(thumbW, thumbW*3/2)
		}
		thumbs[i] = thumb
		if thumb.Bounds().Dy() > maxH {
			maxH = thumb.Bounds().Dy()
		}
	}

	totalW := len(thumbs)*thumbW + (len(thumbs)-1)*gapPx
	canvas := image.NewRGBA(image.Rect(0, 0, totalW, maxH))
	x := 0
	for i, thumb := range thumbs {
		y := (maxH - thumb.Bounds().Dy()) / 2
		draw.Draw(canvas, image.Rect(x, y, x+thumb.Bounds().Dx(), y+thumb.Bounds().Dy()), thumb, thumb.Bounds().Min, draw.Over)
		x += thumbW
		if i < len(thumbs)-1 {
			x += gapPx
		}
	}

	widthCells := len(thumbs)*cellWidth + (len(thumbs)-1)*gapCells

	// Avoid go-termimg ResizeImage cache (keys on path+size; same-sized rows collide).
	termimg.ClearResizeCache()
	return printRendered(w, termimg.New(canvas).Width(widthCells).Scale(termimg.ScaleNone))
}

func printRendered(w io.Writer, img *termimg.Image) error {
	rendered, err := img.Render()
	if err != nil {
		return fmt.Errorf("render image: %w", err)
	}
	if _, err := fmt.Fprint(w, rendered); err != nil {
		return err
	}
	if len(rendered) == 0 || rendered[len(rendered)-1] != '\n' {
		_, err = fmt.Fprintln(w)
	}
	return err
}

func terminalColsExact(w io.Writer) (int, bool) {
	f, ok := w.(*os.File)
	if !ok {
		return 0, false
	}
	cols, _, err := term.GetSize(int(f.Fd()))
	if err != nil || cols <= 0 {
		return 0, false
	}
	return cols, true
}

func terminalCols(w io.Writer) int {
	if cols, ok := terminalColsExact(w); ok {
		return cols
	}
	return 80
}

func loadThumb(path string, targetW int) (image.Image, error) {
	if path == "" {
		return placeholder(targetW, targetW*3/2), nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return placeholder(targetW, targetW*3/2), nil
	}
	targetH := targetW * b.Dy() / b.Dx()
	if targetH < 1 {
		targetH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst, nil
}

func placeholder(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{R: 40, G: 40, B: 48, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}
