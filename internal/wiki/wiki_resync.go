package wiki

import (
	"context"
	"fmt"
	"time"

	"github.com/perber/wiki/internal/wiki/pagesave"
	wikiresync "github.com/perber/wiki/internal/wiki/resync"
)

// launchReloadWithProgress adds to reloadWG and starts reloadWithProgress in a goroutine.
// The caller is responsible for having already called resyncJob.Start().
func (w *Wiki) launchReloadWithProgress() {
	w.reloadWG.Add(1)
	go func() {
		defer w.reloadWG.Done()
		w.reloadWithProgress(w.shutdownCtx)
	}()
}

// reloadWithProgress reruns the startup-time filesystem reconciliation while
// updating resyncJob phases. reloadMu serializes it so indexes are never
// rebuilt from two goroutines at once.
// Must only be called via launchReloadWithProgress (which tracks the WG).
//
// What is reloaded: page tree, link index, tag/property indexes, search index.
// What is NOT reloaded: assets (served directly from disk), auth/users
// (SQLite-based), revisions (stored per-page, no re-baseline).
func (w *Wiki) reloadWithProgress(ctx context.Context) {
	job := w.resyncJob
	started := time.Now()
	var finishErr error

	defer func() {
		w.metrics.ObserveResyncRun(finishErr, started)
	}()

	// Catch panics so a bug in any phase always releases the job and mutex.
	defer func() {
		if r := recover(); r != nil {
			finishErr = fmt.Errorf("panic during reload: %v", r)
			w.log.Error("panic during filesystem reload", "panic", r)
			job.Finish(finishErr)
		}
	}()

	w.reloadMu.Lock()
	defer w.reloadMu.Unlock()

	w.log.Info("filesystem reload started (async)")

	finishErr = w.runReloadPhases(ctx, job)
	if finishErr == nil {
		w.log.Info("filesystem reload completed (async)")
	}
	job.Finish(finishErr)
}

// runReloadPhases rebuilds the tree and every derived index in order,
// reporting each phase on job. A failed link re-index is logged and the
// reload continues; tree and search failures and cancellation abort it.
func (w *Wiki) runReloadPhases(ctx context.Context, job *wikiresync.ResyncJob) error {
	job.SetPhase(wikiresync.PhaseTree)
	if err := w.tree.ReconstructTreeFromFSContext(ctx); err != nil {
		return fmt.Errorf("tree reconstruction failed: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	job.SetPhase(wikiresync.PhaseLinks)
	if err := w.links.IndexAllPagesContext(ctx); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		w.log.Warn("link re-index failed during reload", "error", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	job.SetPhase(wikiresync.PhaseTags)
	w.bootstrapTagsAndProperties()

	job.SetPhase(wikiresync.PhaseSearch)
	w.status.Start()
	searchEffect := pagesave.NewSearchIndexSideEffect(w.searchIndex, w.tree, w.log, w.metrics)
	if err := searchEffect.IndexAllPagesContext(ctx); err != nil {
		w.log.Warn("search re-index failed during reload", "error", err)
		w.status.Fail()
		w.status.Finish()
		return fmt.Errorf("search re-index failed: %w", err)
	}
	w.status.Success()
	w.status.Finish()
	return nil
}

// TriggerResyncAsync starts a background filesystem reload if none is already running.
// Used by the signal handler so SIGUSR1/SIGHUP also goes through the shared ResyncJob.
func (w *Wiki) TriggerResyncAsync() {
	if !w.resyncJob.Start() {
		w.log.Info("resync already running, signal ignored")
		return
	}
	w.launchReloadWithProgress()
}
