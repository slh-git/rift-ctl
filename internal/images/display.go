package images

import (
	"fmt"
	"io"

	"github.com/blacktop/go-termimg"
)

const DefaultWidth = 40

// Display renders a local image file to w using the best available terminal protocol.
func Display(w io.Writer, path string, widthCells int) error {
	if widthCells <= 0 {
		widthCells = DefaultWidth
	}
	img, err := termimg.Open(path)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	rendered, err := img.Width(widthCells).Scale(termimg.ScaleFit).Render()
	if err != nil {
		return fmt.Errorf("render image: %w", err)
	}
	_, err = fmt.Fprint(w, rendered)
	if err != nil {
		return err
	}
	if len(rendered) == 0 || rendered[len(rendered)-1] != '\n' {
		_, err = fmt.Fprintln(w)
	}
	return err
}
