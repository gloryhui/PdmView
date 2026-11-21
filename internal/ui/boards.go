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
	Boards   []DiagramBoard
	Default  int
	Prepared PreparedBoard
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
			Boards:   []DiagramBoard{board},
			Default:  0,
			Prepared: PrepareBoard(board),
		}
	}

	set := BoardSet{Boards: make([]DiagramBoard, len(sources))}
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
	return board
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
