package chatartifact

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

func generateImage(spec ArtifactSpec, kind string) ([]byte, error) {
	const width, height = 800, 500
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{248, 250, 252, 255}}, image.Point{}, draw.Src)
	title := spec.Title
	if spec.Chart != nil && spec.Chart.Title != "" {
		title = spec.Chart.Title
	}
	drawImageText(img, 30, 40, title, color.RGBA{15, 23, 42, 255})
	labels, values := []string(nil), []float64(nil)
	if spec.Chart != nil {
		labels, values = spec.Chart.Labels, spec.Chart.Values
	}
	max := 1.0
	for _, value := range values {
		max = math.Max(max, value)
	}
	for i, value := range values {
		x := 50 + i*110
		barHeight := int(value / max * 320)
		rect := image.Rect(x, 420-barHeight, x+70, 420)
		draw.Draw(img, rect, &image.Uniform{C: color.RGBA{37, 99, 235, 255}}, image.Point{}, draw.Src)
		if i < len(labels) {
			drawImageText(img, x, 445, labels[i], color.RGBA{51, 65, 85, 255})
		}
	}
	var out bytes.Buffer
	if kind == "jpeg" {
		err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 90})
		return out.Bytes(), err
	}
	err := png.Encode(&out, img)
	return out.Bytes(), err
}

func drawImageText(img *image.RGBA, x, y int, text string, c color.Color) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	d.DrawString(text)
}
