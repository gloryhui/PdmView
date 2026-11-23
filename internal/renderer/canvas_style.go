package renderer

import (
	"math"

	"github.com/egoist/mygo/ui"
)

func canvasBackground(theme *ui.Theme) ui.Color {
	if theme.Dark {
		return ui.RGB(36, 41, 47)
	}
	return ui.RGB(230, 235, 241)
}

func canvasOutline(color ui.Color, theme *ui.Theme) ui.Color {
	if contrastRatio(color, canvasBackground(theme)) >= 3 {
		return color
	}
	return canvasOutlineFallback(theme)
}

func canvasOutlineFallback(theme *ui.Theme) ui.Color {
	if theme.Dark {
		return ui.RGB(126, 138, 151)
	}
	return ui.RGB(108, 122, 137)
}

func drawCardShadow(p *ui.Painter, rect ui.Rect, radius float32, theme *ui.Theme) {
	if rect.W < 6 || rect.H < 6 {
		return
	}
	opacity := float32(0.17)
	if theme.Dark {
		opacity = 0.36
	}
	p.Shadow(rect, radius, 0, 2, 4, 0, ui.RGBA(0, 0, 0, opacity))
}

var linearSRGB = func() [256]float64 {
	var values [256]float64
	for i := range values {
		srgb := float64(i) / 255
		if srgb <= 0.04045 {
			values[i] = srgb / 12.92
		} else {
			values[i] = math.Pow((srgb+0.055)/1.055, 2.4)
		}
	}
	return values
}()

func contrastRatio(a, b ui.Color) float64 {
	luminance := func(c ui.Color) float64 {
		return 0.2126*linearSRGB[c.R] + 0.7152*linearSRGB[c.G] + 0.0722*linearSRGB[c.B]
	}
	first, second := luminance(a), luminance(b)
	return (max(first, second) + 0.05) / (min(first, second) + 0.05)
}
