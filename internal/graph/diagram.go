package graph

import "pdmview/internal/pdm"

// DiagramGraph is the graph projection of one selected PDM diagram.
type DiagramGraph struct {
	DiagramID   string
	Scene       Scene
	NodeSources []DiagramNodeSource
	EdgeSources []DiagramEdgeSource
	AutoLayout  bool
}

// NodeSource returns the source aligned with Scene.Nodes[offset].
func (d DiagramGraph) NodeSource(offset int) (DiagramNodeSource, bool) {
	if offset < 0 || offset >= len(d.Scene.Nodes) || offset >= len(d.NodeSources) {
		return DiagramNodeSource{}, false
	}
	return d.NodeSources[offset], true
}

// EdgeSource returns the source aligned with Scene.Edges[offset].
func (d DiagramGraph) EdgeSource(offset int) (DiagramEdgeSource, bool) {
	if offset < 0 || offset >= len(d.Scene.Edges) || offset >= len(d.EdgeSources) {
		return DiagramEdgeSource{}, false
	}
	return d.EdgeSources[offset], true
}

// DiagramNodeSource preserves one diagram symbol's exact source geometry.
type DiagramNodeSource struct {
	SymbolID string
	TableID  string
	Rect     *pdm.Rect
}

// DiagramEdgeSource preserves one relationship symbol's ordered source path.
type DiagramEdgeSource struct {
	SymbolID            string
	ReferenceID         string
	SourceSymbolID      string
	DestinationSymbolID string
	Rect                *pdm.Rect
	Points              []pdm.Point
	ControlPoints       []pdm.Point
}

const (
	autoLayoutColumns = 4
	autoGapX          = 64.0
	autoGapY          = 48.0
)

// MapDiagram maps one selected diagram without scaling or layout inference.
// The auto-layout grid uses the existing 236x144 card footprint in four
// columns; full fallback starts at (0,0), while mixed layout starts to the
// right of the valid rectangle union at its top edge.
func MapDiagram(model *pdm.Model, diagram *pdm.Diagram) DiagramGraph {
	result := DiagramGraph{}
	tables, orderedTables := collectDiagramTables(model)
	decorations := collectTableDecorations(model)

	if diagram == nil {
		if model == nil {
			return result
		}
		result.Scene.Nodes, result.NodeSources, result.AutoLayout = fallbackNodes(orderedTables, decorations)
		result.Scene.Bounds = sceneBounds(result.Scene.Nodes, nil)
		return result
	}

	result.DiagramID = diagram.ID
	if len(diagram.TableSymbols) > 0 {
		result.Scene.Nodes, result.NodeSources, result.AutoLayout =
			symbolNodes(diagram.TableSymbols, tables, decorations)
	} else if contents, ok := ownerContents(model, diagram.OwnerID); ok {
		result.Scene.Nodes, result.NodeSources, result.AutoLayout =
			fallbackNodes(contents.Tables, decorations)
	}

	symbolIndex := uniqueSymbolNodeIndex(diagram.TableSymbols)
	result.Scene.Edges, result.EdgeSources = diagramEdges(diagram.ReferenceSymbols, symbolIndex)
	result.Scene.Bounds = sceneBounds(result.Scene.Nodes, result.Scene.Edges)
	return result
}

func symbolNodes(
	symbols []pdm.TableSymbol,
	tables map[string]pdm.Table,
	decorations map[string]tableDecoration,
) ([]Node, []DiagramNodeSource, bool) {
	var validBounds Rect
	hasValid := false
	for _, symbol := range symbols {
		if !validNodeRect(symbol.Rect) {
			continue
		}
		bounds := graphRect(*symbol.Rect)
		if !hasValid {
			validBounds, hasValid = bounds, true
		} else {
			validBounds = validBounds.Union(bounds)
		}
	}

	startX, startY := 0.0, 0.0
	if hasValid {
		startX = validBounds.Max.X + autoGapX
		startY = validBounds.Min.Y
	}

	nodes := make([]Node, 0, len(symbols))
	sources := make([]DiagramNodeSource, 0, len(symbols))
	autoCount := 0
	for _, symbol := range symbols {
		bounds := Rect{}
		if validNodeRect(symbol.Rect) {
			bounds = graphRect(*symbol.Rect)
		} else {
			bounds = autoBounds(autoCount, startX, startY)
			autoCount++
		}

		table, resolved := tables[symbol.TableID]
		name := symbol.TableID
		if name == "" {
			name = symbol.ID
		}
		node := Node{
			ID:     len(nodes),
			Bounds: bounds,
			Name:   name,
		}
		if resolved {
			node.Name = tableDisplayName(table)
			node.Code = table.Code
			node.Fields = tableFields(table, decorations[table.ID])
		}
		nodes = append(nodes, node)
		sources = append(sources, DiagramNodeSource{
			SymbolID: symbol.ID,
			TableID:  symbol.TableID,
			Rect:     clonePDMRect(symbol.Rect),
		})
	}
	return nodes, sources, autoCount > 0
}

func fallbackNodes(
	tables []pdm.Table,
	decorations map[string]tableDecoration,
) ([]Node, []DiagramNodeSource, bool) {
	nodes := make([]Node, 0, len(tables))
	sources := make([]DiagramNodeSource, 0, len(tables))
	for _, table := range tables {
		nodes = append(nodes, Node{
			ID:     len(nodes),
			Bounds: autoBounds(len(nodes), 0, 0),
			Name:   tableDisplayName(table),
			Code:   table.Code,
			Fields: tableFields(table, decorations[table.ID]),
		})
		sources = append(sources, DiagramNodeSource{TableID: table.ID})
	}
	return nodes, sources, len(nodes) > 0
}

func diagramEdges(symbols []pdm.ReferenceSymbol, symbolIndex map[string]int) ([]Edge, []DiagramEdgeSource) {
	edges := make([]Edge, 0, len(symbols))
	sources := make([]DiagramEdgeSource, 0, len(symbols))
	for _, symbol := range symbols {
		edge := Edge{
			ID:     len(edges),
			From:   resolveSymbol(symbol.SourceSymbolID, symbolIndex),
			To:     resolveSymbol(symbol.DestinationSymbolID, symbolIndex),
			Points: graphPoints(symbol.Points),
		}
		edges = append(edges, edge)
		sources = append(sources, DiagramEdgeSource{
			SymbolID:            symbol.ID,
			ReferenceID:         symbol.ReferenceID,
			SourceSymbolID:      symbol.SourceSymbolID,
			DestinationSymbolID: symbol.DestinationSymbolID,
			Rect:                clonePDMRect(symbol.Rect),
			Points:              clonePDMPoints(symbol.Points),
			ControlPoints:       clonePDMPoints(symbol.ControlPoints),
		})
	}
	return edges, sources
}

func uniqueSymbolNodeIndex(symbols []pdm.TableSymbol) map[string]int {
	index := make(map[string]int)
	duplicates := make(map[string]bool)
	for offset, symbol := range symbols {
		if symbol.ID == "" {
			continue
		}
		if _, exists := index[symbol.ID]; exists {
			duplicates[symbol.ID] = true
			continue
		}
		index[symbol.ID] = offset
	}
	for id := range duplicates {
		delete(index, id)
	}
	return index
}

func resolveSymbol(id string, index map[string]int) int {
	if id == "" {
		return -1
	}
	if offset, exists := index[id]; exists {
		return offset
	}
	return -1
}

func validNodeRect(rect *pdm.Rect) bool {
	return rect != nil && rect.Max.X > rect.Min.X && rect.Max.Y > rect.Min.Y
}

func autoBounds(offset int, startX, startY float64) Rect {
	column := offset % autoLayoutColumns
	row := offset / autoLayoutColumns
	minX := startX + float64(column)*(nodeWidth+autoGapX)
	minY := startY + float64(row)*(nodeHeight+autoGapY)
	return Rect{
		Min: Point{X: minX, Y: minY},
		Max: Point{X: minX + nodeWidth, Y: minY + nodeHeight},
	}
}

func graphRect(rect pdm.Rect) Rect {
	return Rect{
		Min: Point{X: float64(rect.Min.X), Y: float64(rect.Min.Y)},
		Max: Point{X: float64(rect.Max.X), Y: float64(rect.Max.Y)},
	}
}

func graphPoints(points []pdm.Point) []Point {
	if len(points) == 0 {
		return nil
	}
	result := make([]Point, len(points))
	for offset, point := range points {
		result[offset] = Point{X: float64(point.X), Y: float64(point.Y)}
	}
	return result
}

func clonePDMRect(rect *pdm.Rect) *pdm.Rect {
	if rect == nil {
		return nil
	}
	copy := *rect
	return &copy
}

func clonePDMPoints(points []pdm.Point) []pdm.Point {
	if len(points) == 0 {
		return nil
	}
	return append([]pdm.Point(nil), points...)
}

func tableDisplayName(table pdm.Table) string {
	return firstDisplayName(table.Name, table.Code, table.ID)
}

type tableDecoration struct {
	primary map[string]struct{}
	foreign map[string]struct{}
}

func collectTableDecorations(model *pdm.Model) map[string]tableDecoration {
	decorations := make(map[string]tableDecoration)
	if model == nil {
		return decorations
	}
	owners := make(map[string]string)
	collectColumnOwners(model.Contents, owners)
	collectDecorations(model.Contents, decorations, owners)
	return decorations
}

func collectColumnOwners(contents pdm.Contents, owners map[string]string) {
	for _, table := range contents.Tables {
		for _, column := range table.Columns {
			if column.ID != "" {
				if _, exists := owners[column.ID]; !exists {
					owners[column.ID] = table.ID
				}
			}
		}
	}
	for _, pkg := range contents.Packages {
		collectColumnOwners(pkg.Contents, owners)
	}
}

func collectDecorations(
	contents pdm.Contents,
	decorations map[string]tableDecoration,
	owners map[string]string,
) {
	for _, table := range contents.Tables {
		if table.ID == "" || table.PrimaryKeyID == "" {
			continue
		}
		for _, key := range table.Keys {
			if key.ID != table.PrimaryKeyID {
				continue
			}
			decoration := decorations[table.ID]
			if decoration.primary == nil {
				decoration.primary = make(map[string]struct{})
			}
			for _, columnID := range key.ColumnIDs {
				if columnID != "" && owners[columnID] == table.ID {
					decoration.primary[columnID] = struct{}{}
				}
			}
			decorations[table.ID] = decoration
		}
	}
	for _, reference := range contents.References {
		if reference.ChildTableID == "" {
			continue
		}
		var childColumns []string
		for _, join := range reference.Joins {
			if join.ChildColumnID != "" && owners[join.ChildColumnID] == reference.ChildTableID {
				childColumns = append(childColumns, join.ChildColumnID)
			}
		}
		if len(childColumns) == 0 {
			continue
		}
		decoration := decorations[reference.ChildTableID]
		if decoration.foreign == nil {
			decoration.foreign = make(map[string]struct{})
		}
		for _, columnID := range childColumns {
			decoration.foreign[columnID] = struct{}{}
		}
		decorations[reference.ChildTableID] = decoration
	}
	for _, pkg := range contents.Packages {
		collectDecorations(pkg.Contents, decorations, owners)
	}
}

func tableFields(table pdm.Table, decoration tableDecoration) []Field {
	fields := make([]Field, len(table.Columns))
	for offset, column := range table.Columns {
		_, primary := decoration.primary[column.ID]
		_, foreign := decoration.foreign[column.ID]
		fields[offset] = Field{
			Name:       firstDisplayName(column.Name, column.Code, column.ID),
			DataType:   column.DataType.Value,
			PrimaryKey: primary,
			ForeignKey: foreign,
		}
	}
	return fields
}

func firstDisplayName(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func collectDiagramTables(model *pdm.Model) (map[string]pdm.Table, []pdm.Table) {
	tables := make(map[string]pdm.Table)
	if model == nil {
		return tables, nil
	}
	var ordered []pdm.Table
	collectTables(model.Contents, tables, &ordered)
	return tables, ordered
}

func collectTables(contents pdm.Contents, tables map[string]pdm.Table, ordered *[]pdm.Table) {
	for _, table := range contents.Tables {
		*ordered = append(*ordered, table)
		if table.ID != "" {
			if _, exists := tables[table.ID]; !exists {
				tables[table.ID] = table
			}
		}
	}
	for _, pkg := range contents.Packages {
		collectTables(pkg.Contents, tables, ordered)
	}
}

func ownerContents(model *pdm.Model, ownerID string) (pdm.Contents, bool) {
	if model == nil {
		return pdm.Contents{}, false
	}
	if model.ID == ownerID {
		return model.Contents, true
	}
	return packageContents(model.Contents.Packages, ownerID)
}

func packageContents(packages []pdm.Package, ownerID string) (pdm.Contents, bool) {
	for _, pkg := range packages {
		if pkg.ID == ownerID {
			return pkg.Contents, true
		}
		if contents, found := packageContents(pkg.Contents.Packages, ownerID); found {
			return contents, true
		}
	}
	return pdm.Contents{}, false
}

func sceneBounds(nodes []Node, edges []Edge) Rect {
	var bounds Rect
	hasBounds := false
	for _, node := range nodes {
		if !hasBounds {
			bounds, hasBounds = node.Bounds, true
		} else {
			bounds = bounds.Union(node.Bounds)
		}
	}
	for _, edge := range edges {
		if len(edge.Points) == 0 {
			continue
		}
		edgeBounds := edge.Bounds()
		if !hasBounds {
			bounds, hasBounds = edgeBounds, true
		} else {
			bounds = bounds.Union(edgeBounds)
		}
	}
	if !hasBounds {
		return Rect{}
	}
	return bounds
}
