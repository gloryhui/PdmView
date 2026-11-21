package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"pdmview/internal/graph"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	cfg, err := ParseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "canvas-bench:", err)
		os.Exit(2)
	}
	scene, err := graph.Generate(cfg.Nodes, cfg.Seed, cfg.Density)
	if err != nil {
		fmt.Fprintln(os.Stderr, "canvas-bench:", err)
		os.Exit(2)
	}
	index := graph.NewSpatialIndex(scene)

	mygo.App.SetName("PdmView Canvas Bench")
	var stopping atomic.Bool
	var exitCode atomic.Int32
	camera := graph.NewCamera(graph.Rect{})

	mygo.App.WhenReady(func() {
		var bench *Bench
		var watchdog *time.Timer
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:          "PdmView Canvas Benchmark",
			Width:          cfg.Width,
			Height:         cfg.Height,
			UseContentSize: true,
			MinWidth:       320,
			MinHeight:      240,
			Content: ui.View(func(c *ui.Context) {
				ui.Box(c).Fill().Draw(func(p *ui.Painter, area ui.Rect) {
					if stopping.Load() || bench == nil {
						return
					}
					if bench.Frame(p, area, camera) {
						if stopping.CompareAndSwap(false, true) {
							if watchdog != nil {
								watchdog.Stop()
							}
							result, err := bench.Result()
							PrintEnd(os.Stdout, result, err)
							if err != nil {
								exitCode.Store(1)
							}
							go mygo.App.Quit()
						}
						return
					}
					p.AnimationFrame()
				})
			}),
		})
		bench = NewBench(cfg, scene, index, os.Stdout, win.NativeHandle() != 0, win.Page() == nil)

		watchdog = time.AfterFunc(cfg.Warmup+cfg.Sample+15*time.Second, func() {
			if stopping.CompareAndSwap(false, true) {
				fmt.Fprintf(os.Stdout, "CANVAS_BENCH END error=%q timeout_ms=%d\n",
					"watchdog timeout", (cfg.Warmup + cfg.Sample + 15*time.Second).Milliseconds())
				exitCode.Store(1)
				mygo.App.Quit()
			}
		})
		win.OnClosed(func() {
			if watchdog != nil {
				watchdog.Stop()
			}
			if stopping.CompareAndSwap(false, true) {
				fmt.Fprintf(os.Stdout, "CANVAS_BENCH END error=%q\n", "window closed before the benchmark completed")
				exitCode.Store(1)
			}
		})
		mygo.App.Focus()
		win.Focus()
		win.Invalidate()
	})

	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "canvas-bench:", err)
		os.Exit(1)
	}
	if code := exitCode.Load(); code != 0 {
		os.Exit(int(code))
	}
}
