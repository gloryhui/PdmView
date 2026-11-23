package renderer

import (
	"math"

	"pdmview/internal/graph"
	"pdmview/internal/pdm"

	"github.com/egoist/mygo/ui"
)

const (
	tableInset     = 7
	tableHeader    = 20
	tableRowHeight = 17
)

type tableVisual struct {
	unit       float64
	heading    []*labelVisual
	comment    *labelVisual
	rows       []tableRow
	typeX      float32
	indicatorX float32
}

type tableRow struct {
	name, dataType, indicators *ui.TextLayout
	primary, foreign           bool
}

func (c *Canvas) prepareTable(node graph.Node) *tableVisual {
	content := node.Presentation
	titleFont := ui.Font{Size: 14, Weight: 650, LineHeight: 1.08}
	fieldFont := ui.Font{Size: 12.5, Weight: 400, LineHeight: 1.10}
	visual := &tableVisual{unit: c.lodWidthFactor}
	for _, title := range []string{tableStereotype(content.Stereotype), content.Title} {
		if label := c.labels.label(singleLineDisplay(title), titleFont); label != nil {
			visual.heading = append(visual.heading, label)
		}
	}
	visual.comment = c.labels.label(singleLineDisplay(content.Comment), fieldFont)
	top := visual.fieldStart()
	if count := len(content.Columns); count > 0 {
		required := float64(top) + float64(count)*tableRowHeight
		visual.unit = min(visual.unit, node.Bounds.Height()/required)
	}
	columns := content.Columns
	var showType, showIndicators bool
	for _, column := range columns {
		showType = showType || column.DataType != ""
		showIndicators = showIndicators || column.Indicators != ""
	}
	usable := max(0, float32(node.Bounds.Width()/visual.unit)-2*tableInset)
	nameWidth, typeWidth := usable, float32(0)
	if showType {
		nameWidth, typeWidth = usable*0.47, usable*0.53
	}
	if showIndicators {
		nameWidth, typeWidth = nameWidth*0.77, typeWidth*0.77
	}
	visual.typeX = tableInset + nameWidth
	visual.indicatorX = visual.typeX + typeWidth
	visual.rows = make([]tableRow, 0, len(columns))
	for _, column := range columns {
		visual.rows = append(visual.rows, tableRow{
			name:       c.tableLabel(column.Name, fieldFont, nameWidth-3),
			dataType:   c.tableLabel(column.DataType, fieldFont, typeWidth-3),
			indicators: c.tableLabel(column.Indicators, fieldFont, usable-nameWidth-typeWidth),
			primary:    column.PrimaryKey,
			foreign:    column.ForeignKey,
		})
	}
	return visual
}

func (c *Canvas) tableLabel(text string, font ui.Font, width float32) *ui.TextLayout {
	if width <= 0 {
		return nil
	}
	text = previewText(singleLineDisplay(text))
	label := c.labels.label(text, font)
	if label == nil {
		return nil
	}
	if label.fullWidth <= width {
		return label.full
	}
	return shapeLabelLayout(label.full, text, font, width, c.labels.ellipsis[font])
}

func (v *tableVisual) fieldStart() float32 {
	top := float32(len(v.heading)*tableHeader + 2)
	if v.comment != nil {
		top += tableRowHeight
	}
	return top
}

func tableStereotype(text string) string {
	if text == "" {
		return ""
	}
	return "«" + text + "»"
}

func colorFromPDM(saved *pdm.RGBColor, fallback ui.Color) ui.Color {
	if saved == nil {
		return fallback
	}
	return ui.RGB(saved.R, saved.G, saved.B)
}

func (c *Canvas) drawTable(
	p *ui.Painter, rect ui.Rect, node graph.Node, visual *nodeVisual, theme *ui.Theme,
	lod LOD, camera *graph.Camera, selected, hovered bool,
) int {
	style := node.Presentation.Style
	fill := colorFromPDM(style.Fill, theme.Surface)
	border := canvasOutline(colorFromPDM(style.Border, theme.Border), theme)
	drawCardShadow(p, rect, 0, theme)
	if !style.Transparent {
		if border == fill {
			border = canvasOutlineFallback(theme)
		}
		end := colorFromPDM(style.GradientEnd, fill)
		if (style.GradientMode == 64 || style.GradientMode == 65) && fill != end {
			p.FillGradient(rect, ui.LinearGradient{From: fill, To: end, Angle: 180}, 0)
		} else {
			p.Fill(rect, fill, 0)
		}
	}
	if selected || hovered {
		extent := float32(1.5)
		accent := theme.Border.Mix(theme.Accent, 0.70)
		if selected {
			accent = theme.Accent
		}
		p.Stroke(ui.Rect{X: rect.X - extent, Y: rect.Y - extent, W: rect.W + 2*extent, H: rect.H + 2*extent}, accent, 0, extent)
	}
	p.Stroke(rect, border, 0, 1)
	if lod == Far || visual == nil || visual.table == nil {
		return 0
	}

	table := visual.table
	zoom := tableZoom(camera, table)
	if rect.W <= 0 || rect.H <= 0 {
		return 0
	}
	text := theme.Text
	if style.Fill != nil && !style.Transparent {
		if 299*int(fill.R)+587*int(fill.G)+114*int(fill.B) >= 128000 {
			text = ui.RGB(16, 16, 16)
		} else {
			text = ui.RGB(248, 248, 248)
		}
	}
	viewport := camera.ViewportBounds()
	rows := 0
	p.Clip(rect, 0, func() {
		p.Transform(ui.Scaling(zoom, zoom), rect.X, rect.Y, func() {
			top := rect.Y
			width := rect.W / zoom
			headingColor := colorFromPDM(style.TitleColor, text)
			for _, title := range table.heading {
				if layout := title.pick(width-2*tableInset, tableHeader); layout != nil {
					labelWidth, _ := layout.Size()
					p.TextLayout(layout, rect.X+(width-labelWidth)/2, top+2, headingColor)
				}
				top += tableHeader
			}
			if len(table.heading) > 0 && (table.comment != nil || len(table.rows) > 0) {
				p.Line(rect.X, top, rect.X+width, top, 1, border)
			}
			columnColor := colorFromPDM(style.ColumnColor, text)
			if table.comment != nil {
				drawTableLabel(p, table.comment.pick(width-2*tableInset, tableRowHeight), rect.X+tableInset, top+2, columnColor, false)
			}
			start := float64(table.fieldStart())
			clipTop := float64(rect.Y)
			first := int(max(0, math.Floor(((viewport.Min.Y-clipTop)/float64(zoom)-start)/tableRowHeight)))
			last := int(min(float64(len(table.rows)), math.Ceil(((viewport.Max.Y-clipTop)/float64(zoom)-start)/tableRowHeight)))
			for index := first; index < last; index++ {
				row := table.rows[index]
				y := rect.Y + table.fieldStart() + float32(index)*tableRowHeight
				color := columnColor
				if row.primary {
					color = colorFromPDM(style.PrimaryColor, color)
				} else if row.foreign {
					color = colorFromPDM(style.ForeignColor, color)
				}
				drawTableLabel(p, row.name, rect.X+tableInset, y, color, row.primary && style.PrimaryUnderline)
				drawTableLabel(p, row.dataType, rect.X+table.typeX, y, color, false)
				drawTableLabel(p, row.indicators, rect.X+table.indicatorX, y, color, false)
				if row.name != nil || row.dataType != nil || row.indicators != nil {
					rows++
				}
			}
		})
	})
	return rows
}

func tableZoom(camera *graph.Camera, table *tableVisual) float32 {
	if camera == nil || table == nil {
		return 0
	}
	return float32(camera.Scale() * table.unit)
}

func drawTableLabel(p *ui.Painter, layout *ui.TextLayout, x, y float32, color ui.Color, underline bool) {
	if layout == nil {
		return
	}
	p.TextLayout(layout, x, y, color)
	if underline {
		span, height := layout.Size()
		p.Line(x, y+height-1, x+span, y+height-1, 1, color)
	}
}
