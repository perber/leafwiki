package branding

import (
	"path/filepath"
	"sort"
	"strings"
)

// BrandingConfig holds the branding configuration for the wiki
type BrandingConfig struct {
	SiteName    string `json:"siteName"`
	LogoFile    string `json:"logoFile"`
	FaviconFile string `json:"faviconFile"`
	// NotFoundTitle/NotFoundMessage override the default "page not found"
	// text shown for a URL that doesn't resolve to a page — including a page
	// the caller lacks read access to, which deliberately renders identically
	// (see permissions.ErrReadDenied) so a denial never reveals a page's
	// existence. Empty means "use the built-in translated default".
	NotFoundTitle       string              `json:"notFoundTitle"`
	NotFoundMessage     string              `json:"notFoundMessage"`
	BrandingConstraints BrandingConstraints `json:"-"`
}

func (bc *BrandingConfig) ToResponse() *BrandingConfigResponse {
	return &BrandingConfigResponse{
		SiteName:            bc.SiteName,
		LogoFile:            bc.LogoFile,
		FaviconFile:         bc.FaviconFile,
		NotFoundTitle:       bc.NotFoundTitle,
		NotFoundMessage:     bc.NotFoundMessage,
		BrandingConstraints: bc.BrandingConstraints.ToResponse(),
	}
}

// AllowedLogoExt returns the allowed logo image extensions
func (bc *BrandingConfig) AllowedLogoExts() []string {
	extensions := []string{}
	for ext := range bc.BrandingConstraints.LogoExts {
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)
	return extensions
}

// AllowedLogoExtsAsString returns the allowed logo image extensions as a comma-separated string
func (bc *BrandingConfig) AllowedLogoExtsAsString() string {
	return strings.Join(bc.AllowedLogoExts(), ",")
}

// AllowedFaviconExts returns the allowed favicon extensions
func (bc *BrandingConfig) AllowedFaviconExts() []string {
	extensions := []string{}
	for ext := range bc.BrandingConstraints.FaviconExts {
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)
	return extensions
}

// BrandingConstraints holds the constraints for branding assets
type BrandingConstraints struct {
	LogoExts                 map[string]bool `json:"logoExts"`
	FaviconExts              map[string]bool `json:"faviconExts"`
	MaxLogoSize              int64           `json:"maxLogoSize"`
	MaxFaviconSize           int64           `json:"maxFaviconSize"`
	MaxSiteNameLength        int             `json:"maxSiteNameLength"`
	MaxNotFoundTitleLength   int             `json:"maxNotFoundTitleLength"`
	MaxNotFoundMessageLength int             `json:"maxNotFoundMessageLength"`
}

func (bc BrandingConstraints) ToResponse() BrandingConstraintsResponse {
	return BrandingConstraintsResponse{
		LogoExts:                 bc.getSortedLogoExts(),
		FaviconExts:              bc.getSortedFaviconExts(),
		MaxLogoSize:              bc.MaxLogoSize,
		MaxFaviconSize:           bc.MaxFaviconSize,
		MaxSiteNameLength:        bc.MaxSiteNameLength,
		MaxNotFoundTitleLength:   bc.MaxNotFoundTitleLength,
		MaxNotFoundMessageLength: bc.MaxNotFoundMessageLength,
	}
}

func (bc BrandingConstraints) getSortedLogoExts() []string {
	extensions := []string{}
	for ext := range bc.LogoExts {
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)
	return extensions
}

func (bc BrandingConstraints) getSortedFaviconExts() []string {
	extensions := []string{}
	for ext := range bc.FaviconExts {
		extensions = append(extensions, ext)
	}
	sort.Strings(extensions)
	return extensions
}

// AllowedFaviconExtsAsString returns the allowed favicon extensions as a comma-separated string
func (bc *BrandingConfig) AllowedFaviconExtsAsString() string {
	return strings.Join(bc.AllowedFaviconExts(), ",")
}

type BrandingConfigResponse struct {
	SiteName            string                      `json:"siteName"`
	LogoFile            string                      `json:"logoFile"`
	FaviconFile         string                      `json:"faviconFile"`
	NotFoundTitle       string                      `json:"notFoundTitle"`
	NotFoundMessage     string                      `json:"notFoundMessage"`
	BrandingConstraints BrandingConstraintsResponse `json:"brandingConstraints"` // for client-side validation
}

type BrandingConstraintsResponse struct {
	LogoExts                 []string `json:"logoExts"`
	FaviconExts              []string `json:"faviconExts"`
	MaxLogoSize              int64    `json:"maxLogoSize"`
	MaxFaviconSize           int64    `json:"maxFaviconSize"`
	MaxSiteNameLength        int      `json:"maxSiteNameLength"`
	MaxNotFoundTitleLength   int      `json:"maxNotFoundTitleLength"`
	MaxNotFoundMessageLength int      `json:"maxNotFoundMessageLength"`
}

// DefaultBrandingConfig returns the default branding configuration
func DefaultBrandingConfig() *BrandingConfig {
	return &BrandingConfig{
		SiteName:    "LeafWiki",
		LogoFile:    "",
		FaviconFile: "",
		BrandingConstraints: BrandingConstraints{
			LogoExts:                 map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true},
			FaviconExts:              map[string]bool{".png": true, ".gif": true, ".ico": true, ".webp": true},
			MaxLogoSize:              1 * 1024 * 1024, // 1 MB
			MaxFaviconSize:           1 * 1024 * 1024, // 1 MB
			MaxSiteNameLength:        100,
			MaxNotFoundTitleLength:   150,
			MaxNotFoundMessageLength: 1000,
		},
	}
}

func (bc *BrandingConfig) IsAllowedLogoExt(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return bc.BrandingConstraints.LogoExts[ext]
}

func (bc *BrandingConfig) IsAllowedFaviconExt(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return bc.BrandingConstraints.FaviconExts[ext]
}
