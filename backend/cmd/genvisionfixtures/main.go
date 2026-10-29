package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Vision fixtures: pixel text only, neutral filenames, no EXIF/tEXt/COM markers.
func main() {
	out := filepath.Join("debug", "ai-chat-production-readiness", "vision-fixtures")
	_ = os.MkdirAll(out, 0o755)

	must(writePNG(filepath.Join(out, "ui_screen.png"), drawUIScreen))
	must(writePNG(filepath.Join(out, "table_grid.png"), drawTable))
	must(writePNG(filepath.Join(out, "trend_chart.png"), drawChart))
	must(writeJPEG(filepath.Join(out, "doc_photo.jpg"), drawDocPhoto))
	must(writePNG(filepath.Join(out, "small_print.png"), drawSmallPrint))
	// WebP via sharp script if available; also emit PNG twin converted later.
	fmt.Println("vision fixtures in", out)
}

func drawUIScreen(img *image.RGBA) {
	fill(img, color.RGBA{R: 248, G: 250, B: 252, A: 255})
	label(img, 24, 36, "Cabinet overview", color.Black)
	drawBox(img, 24, 60, 220, 120, color.RGBA{R: 219, G: 234, B: 254, A: 255})
	label(img, 36, 90, "Revenue", color.Black)
	label(img, 36, 120, "RUB 1 284 500", color.RGBA{R: 30, G: 64, B: 175, A: 255})
	drawBox(img, 260, 60, 220, 120, color.RGBA{R: 254, G: 226, B: 226, A: 255})
	label(img, 272, 90, "Risk SKUs", color.Black)
	label(img, 272, 120, "17 items", color.RGBA{R: 153, G: 27, B: 27, A: 255})
	drawBox(img, 496, 60, 180, 48, color.RGBA{R: 37, G: 99, B: 235, A: 255})
	label(img, 520, 90, "Open Actions", color.White)
	label(img, 24, 220, "Hidden value VISION_UI_77421", color.RGBA{R: 15, G: 23, B: 42, A: 255})
}

func drawTable(img *image.RGBA) {
	fill(img, color.White)
	label(img, 20, 30, "SKU performance", color.Black)
	rows := [][]string{
		{"SKU", "Orders", "Cover"},
		{"80101", "12", "4.2"},
		{"80102", "9", "2.1"},
		{"80103", "44", "11.0"},
		{"80104", "3", "0.8"},
		{"80105", "18", "6.5"},
		{"80106", "7", "1.4"},
	}
	y := 60
	for i, row := range rows {
		x := 20
		for _, cell := range row {
			label(img, x, y, cell, color.Black)
			x += 120
		}
		if i == 4 {
			label(img, 400, y, "CTRL_CELL_33901", color.RGBA{R: 127, G: 29, B: 29, A: 255})
		}
		y += 28
	}
}

func drawChart(img *image.RGBA) {
	fill(img, color.White)
	label(img, 20, 30, "Revenue vs Orders", color.Black)
	label(img, 20, 55, "Blue=Revenue Red=Orders", color.Black)
	// axes
	hline(img, 40, 300, 560, color.Black)
	vline(img, 40, 80, 300, color.Black)
	// blue line points
	ptsB := []image.Point{{60, 240}, {140, 200}, {220, 160}, {300, 120}, {380, 100}, {460, 90}}
	ptsR := []image.Point{{60, 260}, {140, 250}, {220, 210}, {300, 180}, {380, 150}, {460, 140}}
	connect(img, ptsB, color.RGBA{R: 37, G: 99, B: 235, A: 255})
	connect(img, ptsR, color.RGBA{R: 220, G: 38, B: 38, A: 255})
	label(img, 480, 85, "MAX@P5", color.RGBA{R: 37, G: 99, B: 235, A: 255})
	label(img, 20, 330, "Peak revenue point label: CHART_PEAK_55210", color.Black)
}

func drawDocPhoto(img *image.RGBA) {
	fill(img, color.RGBA{R: 226, G: 232, B: 240, A: 255})
	// tilted-ish page rectangle
	drawBox(img, 80, 40, 480, 300, color.White)
	label(img, 110, 90, "Nakladnaya #4412", color.Black)
	label(img, 110, 130, "Poluchatel: OOO Sever", color.Black)
	label(img, 110, 170, "Summa k oplate:", color.Black)
	label(img, 110, 210, "87 640 RUB", color.RGBA{R: 127, G: 29, B: 29, A: 255})
	label(img, 110, 250, "DOC_PHOTO_CODE_91820", color.Black)
}

func drawSmallPrint(img *image.RGBA) {
	fill(img, color.White)
	label(img, 10, 20, "Fine print sample", color.Black)
	// basicfont is already small; denser lines
	for i := 0; i < 12; i++ {
		label(img, 10, 45+i*14, fmt.Sprintf("line %02d value=%d note=ok", i+1, (i+1)*13), color.RGBA{R: 51, G: 65, B: 85, A: 255})
	}
	label(img, 10, 230, "TINY_MARK_60447", color.Black)
}

func writePNG(path string, paint func(*image.RGBA)) error {
	img := image.NewRGBA(image.Rect(0, 0, 700, 360))
	paint(img)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func writeJPEG(path string, paint func(*image.RGBA)) error {
	img := image.NewRGBA(image.Rect(0, 0, 700, 360))
	paint(img)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func fill(img *image.RGBA, c color.Color) {
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func drawBox(img *image.RGBA, x, y, w, h int, c color.Color) {
	draw.Draw(img, image.Rect(x, y, x+w, y+h), &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func label(img *image.RGBA, x, y int, text string, c color.Color) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	d.DrawString(text)
}

func hline(img *image.RGBA, x1, y, x2 int, c color.Color) {
	for x := x1; x <= x2; x++ {
		img.Set(x, y, c)
	}
}

func vline(img *image.RGBA, x, y1, y2 int, c color.Color) {
	for y := y1; y <= y2; y++ {
		img.Set(x, y, c)
	}
}

func connect(img *image.RGBA, pts []image.Point, c color.Color) {
	for i := 1; i < len(pts); i++ {
		line(img, pts[i-1], pts[i], c)
		img.Set(pts[i].X, pts[i].Y, c)
	}
}

func line(img *image.RGBA, a, b image.Point, c color.Color) {
	dx := abs(b.X - a.X)
	dy := abs(b.Y - a.Y)
	sx, sy := 1, 1
	if a.X > b.X {
		sx = -1
	}
	if a.Y > b.Y {
		sy = -1
	}
	err := dx - dy
	x, y := a.X, a.Y
	for {
		img.Set(x, y, c)
		if x == b.X && y == b.Y {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
