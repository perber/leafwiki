package seo

import (
	"context"
	"html"
	"strings"
	"unicode/utf8"

	"github.com/perber/wiki/internal/core/excerpt"
	httpinternal "github.com/perber/wiki/internal/http"
)

// ─── GetPageMetaUseCase ──────────────────────────────────────────────────────

// PermalinkTarget is the page a permalink points at.
type PermalinkTarget struct {
	// RoutePath is the page's current route path, without a leading "/"
	// ("" for the wiki root).
	RoutePath string
}

// PermalinkResolver resolves a stable page ID to the page's current route
// path. It returns nil and no error when no page has the ID or the tree is
// not loaded yet.
type PermalinkResolver interface {
	ResolvePermalink(ctx context.Context, id string) (*PermalinkTarget, error)
}

// GetPageMetaInput is the input for GetPageMetaUseCase.
type GetPageMetaInput struct {
	// RoutePath is the shell request's route path, starting with "/".
	RoutePath string
	// BasePath is the URL prefix the wiki is served under ("" or e.g. "/wiki").
	BasePath string
}

// GetPageMetaOutput is the metadata of one page, all plain text.
type GetPageMetaOutput struct {
	// Title is the document title, see DocumentTitle.
	Title     string
	PageTitle string
	SiteName  string
	// Description is the "description" property or an excerpt; "" when the page has neither.
	Description string
	// CanonicalURL is absolute, or "" when --public-url is not set.
	CanonicalURL string
}

// GetPageMetaUseCase builds the title, description and canonical URL of the
// page a shell request points at.
type GetPageMetaUseCase struct {
	pages        PageReader
	permalinks   PermalinkResolver
	siteName     func() string
	publicOrigin string
}

// NewGetPageMetaUseCase constructs a GetPageMetaUseCase. publicURL is
// --public-url; only its scheme and host are used, the path comes from the
// base path. An empty or unusable publicURL means no canonical URLs.
func NewGetPageMetaUseCase(pages PageReader, permalinks PermalinkResolver, siteName func() string, publicURL string) *GetPageMetaUseCase {
	return &GetPageMetaUseCase{pages: pages, permalinks: permalinks, siteName: siteName, publicOrigin: origin(publicURL)}
}

// Execute returns the page's metadata, or nil when the route is not a page.
// A permalink route gets the metadata of the page its ID points at, with
// that page's route path as canonical URL.
func (uc *GetPageMetaUseCase) Execute(ctx context.Context, in GetPageMetaInput) (*GetPageMetaOutput, error) {
	routePath := strings.TrimPrefix(in.RoutePath, "/")
	if id, ok := permalinkID(routePath); ok {
		target, err := uc.permalinks.ResolvePermalink(ctx, id)
		if err != nil || target == nil {
			return nil, err
		}
		routePath = target.RoutePath
	}
	if routePath == "" {
		return nil, nil
	}
	page, err := uc.pages.PageByRoutePath(ctx, routePath)
	if err != nil || page == nil {
		return nil, err
	}
	siteName := uc.siteName()
	if siteName == "" {
		siteName = httpinternal.DefaultSiteName
	}
	out := &GetPageMetaOutput{
		Title:       DocumentTitle(page.Title, siteName),
		PageTitle:   page.Title,
		SiteName:    siteName,
		Description: description(page),
	}
	// The canonical URL is built from --public-url only, never from Host or
	// X-Forwarded-* headers (ADR-0018).
	if uc.publicOrigin != "" {
		if canonical, err := absoluteURL(uc.publicOrigin, in.BasePath, routePath); err == nil {
			out.CanonicalURL = canonical
		}
	}
	return out, nil
}

// DocumentTitle is the <title> of a page: "<page title> - <site name>". The
// SPA sets the same format in ui/leafwiki-ui/src/features/viewer/documentTitle.ts;
// change both together so the title doesn't flip once the app has loaded.
func DocumentTitle(pageTitle, siteName string) string {
	if pageTitle == "" {
		return siteName
	}
	return pageTitle + " - " + siteName
}

// description prefers the author's "description" property (exact key, capped
// at maxDescriptionRunes) and falls back to an excerpt like the one page lists show, built from
// the start of the body so a long page costs no more than a short one.
func description(page *Page) string {
	if d := strings.Join(strings.Fields(page.Properties["description"]), " "); d != "" {
		return capDescription(d)
	}
	// The excerpt comes out of an HTML sanitizer and is HTML-escaped; undo
	// that so the meta tag helpers escape it exactly once.
	return html.UnescapeString(excerpt.FromBodyStart(page.Body))
}

// maxDescriptionRunes caps an author-written description: long enough for any
// search snippet, short enough that a huge property can't bloat every shell.
const maxDescriptionRunes = 2 * excerpt.MaxRunes

// capDescription cuts d like excerpt.FromBody cuts an excerpt: at the cap, back
// to a word boundary in the last third, then "...".
func capDescription(d string) string {
	runes := []rune(d)
	if len(runes) <= maxDescriptionRunes {
		return d
	}
	cut := strings.TrimSpace(string(runes[:maxDescriptionRunes]))
	if lastSpace := strings.LastIndex(cut, " "); lastSpace >= 0 && utf8.RuneCountInString(cut[:lastSpace]) >= maxDescriptionRunes*2/3 {
		cut = cut[:lastSpace]
	}
	return strings.TrimSpace(cut) + "..."
}
