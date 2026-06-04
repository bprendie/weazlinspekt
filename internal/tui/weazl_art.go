package tui

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

//go:embed assets/weazlansii.png
var weazlANSIPNG []byte

var weazlPalette struct {
	once   sync.Once
	colors [][]lipgloss.Color
	ok     bool
}

func weazlCellStyle(row, col int, fallback rune) lipgloss.Style {
	weazlPalette.once.Do(loadWeazlPalette)
	if !weazlPalette.ok || row < 0 || row >= len(weazlPalette.colors) {
		return fallbackWeazlRuneStyle(fallback)
	}
	if col < 0 || col >= len(weazlPalette.colors[row]) {
		return fallbackWeazlRuneStyle(fallback)
	}
	return lipgloss.NewStyle().Foreground(weazlPalette.colors[row][col])
}

func loadWeazlPalette() {
	img, err := png.Decode(bytes.NewReader(weazlANSIPNG))
	if err != nil {
		return
	}
	rows := strings.Split(inspektWeazlArt, "\n")
	if len(rows) == 0 {
		return
	}
	cols := maxWeazlWidth(rows)
	if cols == 0 {
		return
	}
	weazlPalette.colors = make([][]lipgloss.Color, len(rows))
	for row := range rows {
		weazlPalette.colors[row] = make([]lipgloss.Color, cols)
		for col := 0; col < cols; col++ {
			weazlPalette.colors[row][col] = sampledCellColor(img, row, col, len(rows), cols)
		}
	}
	weazlPalette.ok = true
}

func maxWeazlWidth(rows []string) int {
	width := 0
	for _, row := range rows {
		width = max(width, lipgloss.Width(row))
	}
	return width
}

func sampledCellColor(img image.Image, row, col, rows, cols int) lipgloss.Color {
	bounds := img.Bounds()
	x0 := bounds.Min.X + (col*bounds.Dx())/cols
	x1 := bounds.Min.X + ((col+1)*bounds.Dx())/cols
	y0 := bounds.Min.Y + (row*bounds.Dy())/rows
	y1 := bounds.Min.Y + ((row+1)*bounds.Dy())/rows
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	return averageColor(img, x0, y0, min(x1, bounds.Max.X), min(y1, bounds.Max.Y))
}

func averageColor(img image.Image, x0, y0, x1, y1 int) lipgloss.Color {
	var rsum, gsum, bsum, count uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, a := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).RGBA()
			if a == 0 {
				continue
			}
			rsum += uint64(r >> 8)
			gsum += uint64(g >> 8)
			bsum += uint64(b >> 8)
			count++
		}
	}
	if count == 0 {
		return lipgloss.Color("#000000")
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", rsum/count, gsum/count, bsum/count))
}
