package appui

import (
	"pdmview/internal/graph"
	"pdmview/internal/pdm"
	"pdmview/internal/renderer"
)

// DiagramBoard is one diagram source descriptor in model/package order.
type DiagramBoard struct {
	ID       string
	Name     string
	Model    *pdm.Model
	Diagram  *pdm.Diagram
	Fallback bool
	TableIDs []string
}

// PreparedBoard is the graph, BVH, and renderer for one active board.
type PreparedBoard struct {
	Board    DiagramBoard
	Graph    graph.DiagramGraph
	Index    *graph.SpatialIndex
	Renderer *renderer.Canvas
}

// BoardSet is the ordered source catalog and its prepared default.
type BoardSet struct {
	Document   *pdm.Document
	SourcePath string
	SourceXML  []byte
	Boards     []DiagramBoard
	Default    int
	Prepared   PreparedBoard
	Catalog    *modelCatalog
	Details    *tableDetailIndex
	Search     *searchIndex
}

// BuildBoardSet enumerates diagram descriptors and prepares only the selected
// default graph, BVH, and renderer. It is intended for a worker goroutine.
func BuildBoardSet(document *pdm.Document) BoardSet {
	var model *pdm.Model
	if document != nil {
		model = &document.Model
	}
	var sources []*pdm.Diagram
	if document != nil {
		collectDiagramSources(document.Model.Contents, &sources)
	}
	if len(sources) == 0 {
		board := makeDiagramBoard(model, nil, true)
		return BoardSet{
			Document: document,
			Boards:   []DiagramBoard{board},
			Default:  0,
			Prepared: PrepareBoard(board),
			Catalog:  buildModelCatalog(model, ""),
			Details:  buildTableDetailIndex(model),
			Search:   buildSearchIndex(model),
		}
	}

	set := BoardSet{Boards: make([]DiagramBoard, len(sources))}
	set.Document = document
	for offset, diagram := range sources {
		set.Boards[offset] = makeDiagramBoard(model, diagram, false)
	}
	if document != nil && document.Model.Contents.DefaultDiagramID != "" {
		for offset, diagram := range sources {
			if diagram.ID == document.Model.Contents.DefaultDiagramID {
				set.Default = offset
				break
			}
		}
	}
	set.Prepared = PrepareBoard(set.Boards[set.Default])
	set.Catalog = buildModelCatalog(model, "")
	set.Details = buildTableDetailIndex(model)
	set.Search = buildSearchIndex(model)
	return set
}

func collectDiagramSources(contents pdm.Contents, diagrams *[]*pdm.Diagram) {
	for offset := range contents.Diagrams {
		*diagrams = append(*diagrams, &contents.Diagrams[offset])
	}
	for _, pkg := range contents.Packages {
		collectDiagramSources(pkg.Contents, diagrams)
	}
}

func makeDiagramBoard(model *pdm.Model, diagram *pdm.Diagram, fallback bool) DiagramBoard {
	board := DiagramBoard{Model: model, Diagram: diagram, Fallback: fallback}
	if diagram != nil {
		board.ID = diagram.ID
		board.Name = firstNonEmpty(diagram.Name, diagram.Code, diagram.ID)
	}
	board.TableIDs = boardTableIDs(board)
	return board
}

func boardTableIDs(board DiagramBoard) []string {
	if board.Model == nil {
		return nil
	}
	if board.Diagram == nil {
		var tables []string
		collectModelTableIDs(board.Model.Contents, &tables)
		return tables
	}
	if len(board.Diagram.TableSymbols) > 0 {
		var tables []string
		seen := make(map[string]bool)
		for _, symbol := range board.Diagram.TableSymbols {
			if symbol.TableID != "" && !seen[symbol.TableID] {
				seen[symbol.TableID] = true
				tables = append(tables, symbol.TableID)
			}
		}
		return tables
	}
	contents, ok := modelContentsByOwner(*board.Model, board.Diagram.OwnerID)
	if !ok {
		return nil
	}
	tables := make([]string, 0, len(contents.Tables))
	for _, table := range contents.Tables {
		if table.ID != "" {
			tables = append(tables, table.ID)
		}
	}
	return tables
}

func collectModelTableIDs(contents pdm.Contents, tables *[]string) {
	for _, table := range contents.Tables {
		if table.ID != "" {
			*tables = append(*tables, table.ID)
		}
	}
	for _, pkg := range contents.Packages {
		collectModelTableIDs(pkg.Contents, tables)
	}
}

func modelContentsByOwner(model pdm.Model, ownerID string) (pdm.Contents, bool) {
	if model.ID == ownerID {
		return model.Contents, true
	}
	return packageContentsByOwner(model.Contents.Packages, ownerID)
}

func packageContentsByOwner(packages []pdm.Package, ownerID string) (pdm.Contents, bool) {
	for _, pkg := range packages {
		if pkg.ID == ownerID {
			return pkg.Contents, true
		}
		if found, ok := packageContentsByOwner(pkg.Contents.Packages, ownerID); ok {
			return found, true
		}
	}
	return pdm.Contents{}, false
}

// PrepareBoard builds one descriptor off the UI thread.
func PrepareBoard(board DiagramBoard) PreparedBoard {
	prepared := PreparedBoard{Board: board}
	prepared.Graph = graph.BuildDiagramGraph(board.Model, board.Diagram)
	prepared.Index = graph.NewSpatialIndex(prepared.Graph.Scene)
	prepared.Renderer = renderer.New(prepared.Graph.Scene, prepared.Index)
	return prepared
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
