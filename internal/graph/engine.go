package graph

import "pdmview/internal/pdm"

// BuildDiagramGraph maps a diagram and adds deterministic display geometry
// only for edges that do not already have an original path.
func BuildDiagramGraph(model *pdm.Model, diagram *pdm.Diagram) DiagramGraph {
	result := MapDiagram(model, diagram)
	if model != nil && (diagram == nil || len(diagram.TableSymbols) == 0) {
		addLogicalReferenceEdges(&result, logicalReferences(model, diagram))
	}
	assignEdgeDirections(&result, model)
	routeMissingEdges(&result.Scene)
	result.Scene.Bounds = sceneBounds(result.Scene.Nodes, result.Scene.Edges)
	return result
}

func assignEdgeDirections(result *DiagramGraph, model *pdm.Model) {
	if result == nil || model == nil {
		return
	}
	tables, _ := collectDiagramTables(model)
	references := make(map[string][]pdm.Reference)
	collectReferenceIndex(model.Contents, references)
	for offset := range result.Scene.Edges {
		edge := &result.Scene.Edges[offset]
		edge.Directed = false
		source, ok := result.EdgeSource(offset)
		if !ok || source.ReferenceID == "" {
			continue
		}
		candidates := references[source.ReferenceID]
		if len(candidates) != 1 {
			continue
		}
		reference := candidates[0]
		if _, resolved := tables[reference.ChildTableID]; !resolved {
			continue
		}
		if _, resolved := tables[reference.ParentTableID]; !resolved {
			continue
		}
		from, fromOK := result.NodeSource(edge.From)
		to, toOK := result.NodeSource(edge.To)
		if !fromOK || !toOK ||
			from.TableID != reference.ChildTableID ||
			to.TableID != reference.ParentTableID {
			continue
		}
		edge.Directed = true
	}
}

func collectReferenceIndex(contents pdm.Contents, references map[string][]pdm.Reference) {
	for _, reference := range contents.References {
		if reference.ID != "" {
			references[reference.ID] = append(references[reference.ID], reference)
		}
	}
	for _, pkg := range contents.Packages {
		collectReferenceIndex(pkg.Contents, references)
	}
}

func logicalReferences(model *pdm.Model, diagram *pdm.Diagram) []pdm.Reference {
	if model == nil {
		return nil
	}
	if diagram == nil {
		var references []pdm.Reference
		collectContentsReferences(model.Contents, &references)
		return references
	}
	contents, ok := ownerContents(model, diagram.OwnerID)
	if !ok {
		return nil
	}
	return contents.References
}

func collectContentsReferences(contents pdm.Contents, references *[]pdm.Reference) {
	*references = append(*references, contents.References...)
	for _, pkg := range contents.Packages {
		collectContentsReferences(pkg.Contents, references)
	}
}

func addLogicalReferenceEdges(result *DiagramGraph, references []pdm.Reference) {
	if len(references) == 0 {
		return
	}
	covered := make(map[string]bool)
	for _, source := range result.EdgeSources {
		if source.ReferenceID != "" {
			covered[source.ReferenceID] = true
		}
	}
	tables := uniqueVisibleTables(*result)
	for _, reference := range references {
		if reference.ID == "" || covered[reference.ID] {
			continue
		}
		child, childFound := tables[reference.ChildTableID]
		parent, parentFound := tables[reference.ParentTableID]
		if !childFound || !parentFound {
			continue
		}
		result.Scene.Edges = append(result.Scene.Edges, Edge{
			ID:   len(result.Scene.Edges),
			From: child,
			To:   parent,
		})
		result.EdgeSources = append(result.EdgeSources, DiagramEdgeSource{
			ReferenceID: reference.ID,
		})
		covered[reference.ID] = true
	}
}

func uniqueVisibleTables(result DiagramGraph) map[string]int {
	counts := make(map[string]int)
	offsets := make(map[string]int)
	for offset := range result.Scene.Nodes {
		source, ok := result.NodeSource(offset)
		if !ok || source.TableID == "" {
			continue
		}
		counts[source.TableID]++
		offsets[source.TableID] = offset
	}
	for id, count := range counts {
		if count != 1 {
			delete(offsets, id)
		}
	}
	return offsets
}
