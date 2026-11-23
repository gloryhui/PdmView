package renderer

import (
	"strings"

	"pdmview/internal/graph"

	"github.com/egoist/mygo/ui"
)

const (
	annotationFontSize   = 14
	annotationLineHeight = 17
	annotationMinFont    = 1
)

type annotationVisual struct {
	lines []*ui.TextLayout
	width float32
}

func prepareAnnotations(annotations []graph.Annotation) []annotationVisual {
	if len(annotations) == 0 {
		return nil
	}
	visuals := make([]annotationVisual, len(annotations))
	font := ui.Font{Size: annotationFontSize, Weight: 450}
	normalize := strings.NewReplacer("\r\n", "\n", "\r", "\n")
	for index, annotation := range annotations {
		visual := &visuals[index]
		for _, line := range strings.Split(normalize.Replace(annotation.Text), "\n") {
			layout := ui.ShapeText(line, font, 0)
			width, _ := layout.Size()
			visual.width = max(visual.width, width)
			visual.lines = append(visual.lines, layout)
		}
	}
	return visuals
}

func (c *Canvas) drawAnnotations(p *ui.Painter, camera *graph.Camera, visible graph.Rect, theme *ui.Theme) {
	for index, annotation := range c.scene.Annotations {
		bounds := annotation.Bounds
		if bounds.Max.X < visible.Min.X || bounds.Min.X > visible.Max.X ||
			bounds.Max.Y < visible.Min.Y || bounds.Min.Y > visible.Max.Y {
			continue
		}
		rect := canvasRect(camera, bounds)
		if rect.W <= 0 || rect.H <= 0 {
			continue
		}
		fill := colorFromPDM(annotation.Fill, theme.Surface)
		border := canvasOutline(colorFromPDM(annotation.Border, theme.Border), theme)
		p.Fill(rect, fill, 0)
		p.Stroke(rect, border, 0, 1)

		visual := c.annotations[index]
		text := colorFromPDM(annotation.TextColor, theme.Text)
		if contrastRatio(text, fill) < 4.5 {
			text = ui.RGB(0, 0, 0)
			white := ui.RGB(255, 255, 255)
			if contrastRatio(white, fill) > contrastRatio(text, fill) {
				text = white
			}
		}
		visual.draw(p, rect, text)
	}
}

func (v annotationVisual) draw(p *ui.Painter, rect ui.Rect, text ui.Color) {
	height := float32(len(v.lines)) * annotationLineHeight
	// A fixed inset takes up too much of a short annotation at overview zoom.
	paddingX := min(2, rect.W/8)
	paddingY := min(2, rect.H/8)
	zoom := min((rect.W-paddingX)/(v.width+12), (rect.H-paddingY)/(height+4))
	if zoom*annotationFontSize < annotationMinFont {
		return
	}
	centerX, centerY := rect.X+rect.W/2, rect.Y+rect.H/2
	p.Clip(rect, 0, func() {
		p.Transform(ui.Scaling(zoom, zoom), centerX, centerY, func() {
			for index, layout := range v.lines {
				width, lineHeight := layout.Size()
				y := centerY - height/2 + float32(index)*annotationLineHeight + (annotationLineHeight-lineHeight)/2
				p.TextLayout(layout, centerX-width/2, y, text)
			}
		})
	})
}
