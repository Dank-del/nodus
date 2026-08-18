package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Dank-del/cpm/internal/cpm"
	"github.com/schollz/progressbar/v3"
)

// CPM progress is intentionally terminal-only: redirected output stays clean
// for CI, scripts, and editors, while interactive sessions receive color and
// live progress for operations whose duration is otherwise opaque.
func supportsProgress(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type activityProgress struct {
	bar  *progressbar.ProgressBar
	done chan struct{}
	wg   sync.WaitGroup
	once sync.Once
}

func startActivityProgress(out io.Writer, description string) *activityProgress {
	if !supportsProgress(out) {
		return &activityProgress{}
	}
	bar := progressbar.NewOptions64(-1,
		progressbar.OptionSetWriter(out),
		progressbar.OptionSetDescription("[cyan]"+description+"[reset]"),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetElapsedTime(true),
		progressbar.OptionSetSpinnerChangeInterval(100*time.Millisecond),
		progressbar.OptionUseANSICodes(true),
	)
	progress := &activityProgress{bar: bar, done: make(chan struct{})}
	progress.wg.Add(1)
	go func() {
		defer progress.wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = bar.Add(1)
			case <-progress.done:
				return
			}
		}
	}()
	return progress
}

func (p *activityProgress) Stop(success bool) {
	p.once.Do(func() {
		if p.bar == nil {
			return
		}
		close(p.done)
		p.wg.Wait()
		if success {
			_ = p.bar.Finish()
			return
		}
		_ = p.bar.Exit()
	})
}

type packageProgress struct {
	bar *progressbar.ProgressBar
}

func newPackageProgress(out io.Writer, total int, description string) *packageProgress {
	if total == 0 || !supportsProgress(out) {
		return &packageProgress{}
	}
	return &packageProgress{bar: progressbar.NewOptions(total,
		progressbar.OptionSetWriter(out),
		progressbar.OptionSetDescription("[cyan]"+description+"[reset]"),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{Saucer: "[cyan]=[reset]", SaucerHead: "[cyan]>[reset]", SaucerPadding: " ", BarStart: "[", BarEnd: "]"}),
		progressbar.OptionSetWidth(24),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionUseANSICodes(true),
	)}
}

func (p *packageProgress) StartPackage(pkg cpm.Package) {
	if p.bar != nil {
		p.bar.Describe(fmt.Sprintf("[cyan]Preparing %s[reset]", pkg.Name))
	}
}

func (p *packageProgress) CompletePackage(_ cpm.Package) {
	if p.bar != nil {
		_ = p.bar.Add(1)
	}
}

func (p *packageProgress) Stop(success bool) {
	if p.bar == nil {
		return
	}
	if success {
		_ = p.bar.Finish()
		return
	}
	_ = p.bar.Exit()
}
