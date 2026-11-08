package seo

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	httpinternal "github.com/perber/wiki/internal/http"
)

// Routes registers the seo slice's own routes: robots.txt and sitemap.xml.
// Crawlers honour robots.txt only at the host root, so under --base-path it
// has no effect; the sitemap works there (its scope is its directory).
type Routes struct {
	public  PublicOnly
	robots  *GetRobotsUseCase
	sitemap *BuildSitemapUseCase
	log     *slog.Logger
	// truncatedOnce keeps a wiki above MaxSitemapURLs from logging on every
	// crawler fetch.
	truncatedOnce sync.Once
}

// RoutesConfig holds the dependencies of Routes.
type RoutesConfig struct {
	Public  PublicOnly
	Robots  *GetRobotsUseCase
	Sitemap *BuildSitemapUseCase
	Log     *slog.Logger
}

// NewRoutes constructs the seo routes.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{public: cfg.Public, robots: cfg.Robots, sitemap: cfg.Sitemap, log: cfg.Log}
}

// RegisterRoutes registers GET and HEAD for both files. While public access is
// off, robots.txt disallows everything and sitemap.xml is 404; neither reads a
// page.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	basePath := ctx.Opts.BasePath
	ctx.Base.Match(httpinternal.GetAndHead, "/robots.txt", gin.WrapF(r.public.Handler(r.handleRobots(basePath), handlePrivateRobots)))
	ctx.Base.Match(httpinternal.GetAndHead, "/sitemap.xml", gin.WrapF(r.public.Handler(r.handleSitemap(basePath), handleNotFound)))
}

// noCache: both answers change when public access is toggled or pages
// change, so caches must revalidate.
func noCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache")
}

func writeRobots(w http.ResponseWriter, text string) {
	noCache(w)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(text))
}

func handlePrivateRobots(w http.ResponseWriter, _ *http.Request) {
	writeRobots(w, PrivateRobots)
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	noCache(w)
	http.NotFound(w, r)
}

// failed answers a use case error with a bare 500. Swallowed here: the
// crawler retries later, and the details stay in the log.
func (r *Routes) failed(w http.ResponseWriter, what string, err error) bool {
	if err == nil {
		return false
	}
	r.log.Error(what+" failed", "error", err)
	noCache(w)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	return true
}

func (r *Routes) handleRobots(basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		out, err := r.robots.Execute(req.Context(), GetRobotsInput{BasePath: basePath})
		if !r.failed(w, "robots.txt", err) {
			writeRobots(w, out.Text)
		}
	}
}

func (r *Routes) handleSitemap(basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		out, err := r.sitemap.Execute(req.Context(), BuildSitemapInput{BasePath: basePath})
		if r.failed(w, "sitemap", err) {
			return
		}
		if out == nil {
			handleNotFound(w, req)
			return
		}
		if out.Truncated {
			r.truncatedOnce.Do(func() {
				r.log.Warn("sitemap lists only the first pages in navigation order", "limit", MaxSitemapURLs)
			})
		}
		noCache(w)
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		if err := out.WriteXML(w); err != nil {
			// The status is already sent; the client hung up or the write failed.
			r.log.Debug("sitemap write aborted", "error", err)
		}
	}
}
