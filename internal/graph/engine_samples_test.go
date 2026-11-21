//go:build pdm_layout_check

package graph

import (
	"math"
	"os"
	"testing"

	"pdmview/internal/pdm"
)

func TestPhase5SampleCamera(t *testing.T) {
	model := openEngineSample(t)
	const padding = 24.0
	viewport := Rect{Min: Point{X: 37, Y: 51}, Max: Point{X: 1061, Y: 819}}
	camera := NewDiagramCamera(viewport)
	legacy := NewCamera(viewport)
	var diagrams, nodes, legacyClipped int
	for _, diagram := range sampleDiagrams(&model.Contents) {
		mapped := MapDiagram(model, diagram)
		if len(mapped.Scene.Nodes) == 0 {
			continue
		}
		legacy.Fit(mapped.Scene.Bounds, padding)
		if !sampleFitsCamera(legacy, mapped.Scene.Bounds, padding) {
			legacyClipped++
		}
		camera.FitBounds(mapped.Scene.Bounds, padding)
		if !sampleFitsCamera(camera, mapped.Scene.Bounds, padding) {
			t.Fatalf("diagram %d did not fit the viewport with padding", diagrams+1)
		}
		for _, node := range mapped.Scene.Nodes {
			screen := camera.WorldToScreen(rectMiddle(node.Bounds))
			if roundTrip := camera.WorldToScreen(camera.ScreenToWorld(screen)); math.Hypot(screen.X-roundTrip.X, screen.Y-roundTrip.Y) > 1e-7 {
				t.Fatalf("diagram %d node %d screen round trip exceeded tolerance", diagrams+1, node.ID)
			}
			nodes++
		}
		diagrams++
	}
	if diagrams == 0 {
		t.Fatal("sample has no populated diagram")
	}
	t.Logf("1024x768 DIP viewport, 24 DIP padding: fitted_diagrams=%d node_round_trips=%d prototype_clipped_diagrams=%d", diagrams, nodes, legacyClipped)
}

func openEngineSample(t *testing.T) *pdm.Model {
	t.Helper()
	path := os.Getenv("PDM_LAYOUT_SAMPLE")
	if path == "" {
		t.Skip("set PDM_LAYOUT_SAMPLE to a local XML export")
	}
	input, err := os.Open(path)
	if err != nil {
		t.Fatal("could not open the local XML export")
	}
	defer input.Close()
	document, err := pdm.Parse(input)
	if err != nil {
		t.Fatal("PDM parser rejected the local XML export")
	}
	return &document.Model
}

func sampleDiagrams(contents *pdm.Contents) []*pdm.Diagram {
	var diagrams []*pdm.Diagram
	for index := range contents.Diagrams {
		diagrams = append(diagrams, &contents.Diagrams[index])
	}
	for index := range contents.Packages {
		diagrams = append(diagrams, sampleDiagrams(&contents.Packages[index].Contents)...)
	}
	return diagrams
}

func sampleFitsCamera(camera *Camera, world Rect, padding float64) bool {
	viewport := camera.ViewportBounds()
	leftTop := camera.WorldToScreen(world.Min)
	rightBottom := camera.WorldToScreen(world.Max)
	const epsilon = 1e-7
	return leftTop.X >= viewport.Min.X+padding-epsilon &&
		leftTop.Y >= viewport.Min.Y+padding-epsilon &&
		rightBottom.X <= viewport.Max.X-padding+epsilon &&
		rightBottom.Y <= viewport.Max.Y-padding+epsilon
}
