package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

const probeClipboardFormat transfer.Format = "application/x-pdmview-mygo-probe"

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mygo-probe:", err)
		os.Exit(2)
	}

	probe := newProbeState(cfg, nativePickFiles, nativeClipboardRoundTrip)
	mygo.App.SetName("PdmView MyGo Probe")
	mygo.App.WhenReady(func() {
		probe.window = mygo.NewWindow(mygo.WindowOptions{
			Title:           "PdmView P0-02 · MyGo native probe",
			Width:           cfg.width,
			Height:          cfg.height,
			MinWidth:        900,
			MinHeight:       600,
			BackgroundColor: "#0b1220",
			Content:         ui.View(probe.view),
		})
		if cfg.smoke {
			startSmoke(probe, cfg)
		}
	})

	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mygo-probe:", err)
		os.Exit(1)
	}
	if code := probe.exitCode.Load(); code != 0 {
		os.Exit(int(code))
	}
}

type probeConfig struct {
	width, height int
	nodes         int
	bench         bool
	benchWarmup   time.Duration
	benchMeasure  time.Duration
	smoke         bool
	smokeTimeout  time.Duration
	smokeOut      string
}

func parseConfig(args []string) (probeConfig, error) {
	cfg := probeConfig{}
	fs := flag.NewFlagSet("mygo-probe", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.IntVar(&cfg.width, "width", 1400, "native window content width in DIPs")
	fs.IntVar(&cfg.height, "height", 900, "native window content height in DIPs")
	fs.IntVar(&cfg.nodes, "nodes", 0, "synthetic nodes: default 2; performance runs accept only 100, 300, or 500")
	fs.BoolVar(&cfg.bench, "bench", false, "actively redraw after warmup and print a fixed-duration FPS summary")
	fs.DurationVar(&cfg.benchWarmup, "warmup", 2*time.Second, "benchmark warmup before measurement")
	fs.DurationVar(&cfg.benchMeasure, "measure", 10*time.Second, "benchmark measurement duration")
	fs.BoolVar(&cfg.smoke, "smoke", false, "observe frames, capture a screenshot, and quit after a finite timeout")
	fs.DurationVar(&cfg.smokeTimeout, "smoke-timeout", 10*time.Second, "smoke deadline before capture and quit")
	fs.StringVar(&cfg.smokeOut, "smoke-out", "", "optional PNG path for the smoke screenshot; no file is written when empty")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		return cfg, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if cfg.width < 900 || cfg.height < 600 {
		return cfg, errors.New("width and height must be at least 900x600")
	}
	if cfg.bench && cfg.smoke {
		return cfg, errors.New("-bench and -smoke are separate modes")
	}
	if cfg.benchWarmup < 0 {
		return cfg, errors.New("-warmup cannot be negative")
	}
	if cfg.benchMeasure <= 0 {
		return cfg, errors.New("-measure must be positive")
	}
	if cfg.smokeTimeout <= 0 {
		return cfg, errors.New("-smoke-timeout must be positive")
	}

	if cfg.nodes == 0 {
		cfg.nodes = 2
	}
	if cfg.bench {
		if cfg.nodes != 100 && cfg.nodes != 300 && cfg.nodes != 500 {
			return cfg, errors.New("-bench requires -nodes 100, 300, or 500")
		}
	} else if cfg.nodes != 2 && cfg.nodes != 100 && cfg.nodes != 300 && cfg.nodes != 500 {
		return cfg, errors.New("-nodes must be 2 (default) or one of the performance sizes 100, 300, 500")
	}
	return cfg, nil
}

func nativePickFiles(parent *mygo.Window) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      parent,
		Title:       "P0-02 MyGo native file picker",
		ButtonLabel: "Choose (path discarded)",
		Message:     "The probe reports only whether a selection was made. It never reads the selection or displays its path.",
		Filters:     []mygo.FileFilter{{Name: "PowerDesigner model", Extensions: []string{"pdm"}}},
	})
}

func nativeClipboardRoundTrip() (string, error) {
	backup, err := mygo.Clipboard.Read()
	if err != nil {
		return "", fmt.Errorf("clipboard backup: %w", err)
	}
	restore := func() error {
		if len(backup.Items()) == 0 {
			mygo.Clipboard.Clear()
			return nil
		}
		if err := mygo.Clipboard.Write(backup); err != nil {
			return err
		}
		return mygo.Clipboard.Flush()
	}

	marker := []byte("pdmview-mygo-probe:" + time.Now().UTC().Format(time.RFC3339Nano))
	unicodeText := []byte("PdmView MyGo probe · 中文 · 日本語 · Кириллица")
	if err := mygo.Clipboard.Write(transfer.New(transfer.NewItem(
		transfer.Bytes(probeClipboardFormat, marker),
		transfer.Bytes(transfer.Text, unicodeText),
	)), mygo.ClipboardOptions{Concealed: true}); err != nil {
		return "", fmt.Errorf("write synthetic clipboard data: %w", err)
	}
	readBack, formatErr := mygo.Clipboard.ReadFormat(probeClipboardFormat)
	readText, textErr := mygo.Clipboard.ReadFormat(transfer.Text)
	currentMarker, currentMarkerErr := mygo.Clipboard.ReadFormat(probeClipboardFormat)
	if currentMarkerErr != nil || !bytes.Equal(currentMarker, marker) {
		err := errors.New("clipboard ownership changed during verification")
		return "Clipboard changed during verification", err
	}
	if err := restore(); err != nil {
		return "Restoring the prior clipboard failed", fmt.Errorf("restore clipboard: %w", err)
	}
	if err := errors.Join(formatErr, textErr); err != nil {
		return "Clipboard read failed; prior clipboard restored", err
	}
	if !bytes.Equal(readBack, marker) || !bytes.Equal(readText, unicodeText) {
		return "Clipboard Unicode round-trip mismatch; prior clipboard restored", errors.New("clipboard round-trip mismatch")
	}
	return "Clipboard Unicode plus scoped-format round-trip OK; prior clipboard restored", nil
}

func startSmoke(probe *probeState, cfg probeConfig) {
	if probe.window.Page() != nil || probe.window.NativeHandle() == 0 {
		fmt.Fprintln(os.Stderr, "mygo-probe: expected a native window with no web page")
		probe.exitCode.Store(1)
		mygo.App.Quit()
		return
	}
	deadline := time.Now().Add(cfg.smokeTimeout)
	time.AfterFunc(cfg.smokeTimeout+5*time.Second, mygo.App.Quit)

	go func() {
		for probe.frames.Load() < 3 && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		frames := probe.frames.Load()
		png, err := probe.window.CapturePage()
		if err != nil {
			fmt.Printf("SMOKE frames=%d scale=%.2f accelerated=%t screenshot_error=%q\n",
				frames, probe.frameScale(), probe.frameAccelerated(), err)
			probe.exitCode.Store(1)
			mygo.App.Quit()
			return
		}
		if frames == 0 || len(png) == 0 {
			fmt.Printf("SMOKE frames=%d scale=%.2f accelerated=%t screenshot_error=%q\n",
				frames, probe.frameScale(), probe.frameAccelerated(), "empty frame or screenshot")
			probe.exitCode.Store(1)
			mygo.App.Quit()
			return
		}

		sum := sha256.Sum256(png)
		wrote := ""
		if cfg.smokeOut != "" {
			if err := os.WriteFile(cfg.smokeOut, png, 0o600); err != nil {
				wrote = "write_error=" + fmt.Sprintf("%q", err)
				probe.exitCode.Store(1)
			} else {
				wrote = "path=" + cfg.smokeOut
			}
		}
		fmt.Printf("SMOKE native=true frames=%d scale=%.2f accelerated=%t png_bytes=%d sha256=%x %s\n",
			frames, probe.frameScale(), probe.frameAccelerated(), len(png), sum, wrote)
		mygo.App.Quit()
	}()
}
