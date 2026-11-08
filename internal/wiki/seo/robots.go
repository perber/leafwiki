package seo

import "context"

// PrivateRobots is robots.txt while public access is off: the wiki asks every
// crawler to stay away, since anonymous visitors only see the login page.
const PrivateRobots = "User-agent: *\nDisallow: /\n"

// ─── GetRobotsUseCase ────────────────────────────────────────────────────────

// GetRobotsInput is the input for GetRobotsUseCase.
type GetRobotsInput struct {
	// BasePath is the URL prefix the wiki is served under ("" or e.g. "/wiki").
	BasePath string
}

// GetRobotsOutput is the robots.txt of a public wiki.
type GetRobotsOutput struct {
	Text string
}

// GetRobotsUseCase builds robots.txt for a public wiki: crawling is allowed
// and, with --public-url, the sitemap is announced. It never disallows /api:
// search engines render the SPA, which reads pages through it.
type GetRobotsUseCase struct {
	publicOrigin string
}

// NewGetRobotsUseCase constructs a GetRobotsUseCase. publicURL is --public-url;
// an empty or unusable value means no Sitemap line.
func NewGetRobotsUseCase(publicURL string) *GetRobotsUseCase {
	return &GetRobotsUseCase{publicOrigin: origin(publicURL)}
}

// Execute returns robots.txt. It reads nothing; the error is always nil.
func (uc *GetRobotsUseCase) Execute(_ context.Context, in GetRobotsInput) (*GetRobotsOutput, error) {
	text := "User-agent: *\nAllow: /\n"
	if uc.publicOrigin != "" {
		if sitemap, err := absoluteURL(uc.publicOrigin, in.BasePath, "sitemap.xml"); err == nil {
			text += "\nSitemap: " + sitemap + "\n"
		}
	}
	return &GetRobotsOutput{Text: text}, nil
}
