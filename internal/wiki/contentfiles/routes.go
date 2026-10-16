// Package contentfiles serves non-markdown media files that live inside the
// wiki content tree (<storageDir>/root), so pages authored on disk can embed
// them with relative Markdown paths (![alt](img/shot.png)) that resolve
// exactly like their filesystem-relative links.
//
// Wiki pages themselves are NOT served here: .md (and every other extension
// outside the media allowlist) is a 404 — the page routes own textual
// content.
package contentfiles

import (
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	coreauth "github.com/perber/wiki/internal/core/auth"
	httpinternal "github.com/perber/wiki/internal/http"
	authmw "github.com/perber/wiki/internal/http/middleware/auth"
	"github.com/perber/wiki/internal/http/middleware/security"
)

// Routes is the RouteRegistrar for content files.
type Routes struct {
	authService *coreauth.AuthService
	rootDir     string
	log         *slog.Logger
}

// RoutesConfig holds the dependencies required to build a Routes instance.
type RoutesConfig struct {
	AuthService *coreauth.AuthService
	// RootDir is the wiki content directory (<storageDir>/root).
	RootDir string
	Log     *slog.Logger
}

// NewRoutes constructs the content files RouteRegistrar.
func NewRoutes(cfg RoutesConfig) *Routes {
	return &Routes{
		authService: cfg.AuthService,
		rootDir:     cfg.RootDir,
		log:         cfg.Log,
	}
}

// serveContentExts is the allowlist of extensions /files serves, mapped to
// the Content-Type served with them. Mirrors the browser-renderable media
// from security.inlineFileExts (plus SVG, which is only ever rendered via
// <img> embedding — direct navigation downloads it, like /assets).
var serveContentExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
	".ico":  "image/x-icon",
	".pdf":  "application/pdf",
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".ogv":  "video/ogg",
	".mov":  "video/quicktime",
	".m4v":  "video/x-m4v",
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".m4a":  "audio/mp4",
	".flac": "audio/flac",
	".aac":  "audio/aac",
	".opus": "audio/opus",
}

// RegisterRoutes implements RouteRegistrar.
func (r *Routes) RegisterRoutes(ctx httpinternal.RouterContext) {
	opts := ctx.Opts
	if r.rootDir == "" {
		return
	}

	// Static /files share the read gate with /assets (public mode /
	// --disable-auth / session), on their own path and without CSRF
	// (GET-only static files) — same wiring as the /assets static group.
	group := ctx.Base.Group("/files")
	group.Use(
		contentServeHeaders(),
		authmw.InjectPublicEditor(opts.AuthDisabled),
		authmw.RequireAuthOrPublicRead(r.authService, ctx.AuthCookies, opts.AuthDisabled, opts.PublicAccess),
	)
	group.GET("/*filepath", r.handleServe)
}

// contentServeHeaders hardens the static /files responses; see
// security.SetUntrustedFileHeaders for the rules.
func contentServeHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		security.SetUntrustedFileHeaders(c.Writer.Header(), c.Request.URL.Path)
		c.Next()
	}
}

func (r *Routes) handleServe(c *gin.Context) {
	// path.Clean on the rooted form collapses "..", "." and duplicate
	// separators; a cleaned rooted path can never escape the root, so the
	// joined file always stays inside rootDir.
	cleaned := path.Clean("/" + c.Param("filepath"))
	rel := strings.TrimPrefix(cleaned, "/")
	if rel == "" {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	ext := strings.ToLower(path.Ext(rel))
	contentType, ok := serveContentExts[ext]
	if !ok {
		// Not a servable media type (includes .md — pages go through the
		// page routes, never as raw file downloads).
		c.String(http.StatusNotFound, "Not found")
		return
	}

	full := filepath.Join(r.rootDir, filepath.FromSlash(rel))
	f, err := os.Open(full)
	if err != nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			r.log.Warn("could not close content file", "path", full, "error", err)
		}
	}()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	// Fresh-but-cheap caching: always revalidate (content-tree files are
	// edited in place under stable names), 304 when the mtime is unchanged.
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Content-Type", contentType)
	http.ServeContent(c.Writer, c.Request, info.Name(), info.ModTime(), f)
}
