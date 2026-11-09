package seo

import "github.com/perber/wiki/internal/render"

// MaxParallelRenders is the cap on concurrent crawler body renders.
const MaxParallelRenders = maxParallelRenders

// SetRenderFunc replaces the renderer of uc, so tests can count renders.
func SetRenderFunc(uc *GetCrawlerBodyUseCase, fn func(string, render.Options) (string, error)) {
	uc.render = fn
}

// ErrRendersBusy is returned while MaxParallelRenders renders are running.
var ErrRendersBusy = errRendersBusy
