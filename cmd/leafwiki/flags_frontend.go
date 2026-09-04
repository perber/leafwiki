package main

import (
	"time"

	"github.com/urfave/cli/v3"
)

// frontendOptions holds the options for frontend appearance, features and revisions.
type frontendOptions struct {
	defaultLanguage         string
	injectCodeInHeader      string
	customStylesheet        string
	hideLinkMetadataSection bool
	maxAssetUploadSize      string
	enableRevision          bool
	enableLinkRefactor      bool
	enableAPIKeyManagement  bool
	maxRevisionHistory      int
	revisionCoalesceWindow  time.Duration
}

// Flags declares them. Adding an option here is the whole change: the flag,
// its default, its environment variable and its help text are one literal,
// and --help is generated from it.
func (o *frontendOptions) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "default-language",
			Destination: &o.defaultLanguage,
			Category:    catFrontend,
			Usage:       "default UI language code (e.g. en, de); ignored unless the frontend ships it",
			Sources:     envVars("LEAFWIKI_DEFAULT_LANGUAGE"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "inject-code-in-header",
			Destination: &o.injectCodeInHeader,
			Category:    catFrontend,
			Usage:       "raw HTML/JS injected into <head> (analytics, custom CSS); trusted code only, it is not sanitized",
			Sources:     envVars("LEAFWIKI_INJECT_CODE_IN_HEADER"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "custom-stylesheet",
			Destination: &o.customStylesheet,
			Category:    catFrontend,
			Usage:       "a .css file inside the data dir, served as /custom.css (under --base-path when set)",
			Sources:     envVars("LEAFWIKI_CUSTOM_STYLESHEET"),
			Config:      trimmed,
		},
		&cli.BoolFlag{
			Name:        "hide-link-metadata-section",
			Destination: &o.hideLinkMetadataSection,
			Category:    catFrontend,
			Usage:       "hide the link metadata section in the frontend UI",
			Sources:     envBoolVars("LEAFWIKI_HIDE_LINK_METADATA_SECTION"),
		},
		&cli.StringFlag{
			Name:             "max-asset-upload-size",
			Destination:      &o.maxAssetUploadSize,
			Category:         catFrontend,
			Usage:            "maximum size for asset uploads (e.g. 50MiB, 50MB, 52428800)",
			Value:            "50MiB",
			Sources:          envVars("LEAFWIKI_MAX_ASSET_UPLOAD_SIZE"),
			Validator:        validateByteSizeValue,
			ValidateDefaults: true,
			Config:           trimmed,
		},
		&cli.BoolFlag{
			Name:        "enable-revision",
			Destination: &o.enableRevision,
			Category:    catFeatures,
			Usage:       "enable the revision / page history feature",
			Sources:     envBoolVars("LEAFWIKI_ENABLE_REVISION"),
		},
		&cli.BoolFlag{
			Name:        "enable-link-refactor",
			Destination: &o.enableLinkRefactor,
			Category:    catFeatures,
			Usage:       "enable the link refactoring dialog and rewrite flow",
			Sources:     envBoolVars("LEAFWIKI_ENABLE_LINK_REFACTOR"),
		},
		&cli.BoolFlag{
			Name:        "enable-api-key-management",
			Destination: &o.enableAPIKeyManagement,
			Category:    catFeatures,
			Usage:       "enable the experimental API key management feature",
			Sources:     envBoolVars("LEAFWIKI_ENABLE_API_KEY_MANAGEMENT"),
		},
		&cli.IntFlag{
			Name:        "max-revision-history",
			Destination: &o.maxRevisionHistory,
			Category:    catRevisions,
			Usage:       "maximum revisions kept per page; 0 = unlimited",
			Value:       100,
			Sources:     envVars("LEAFWIKI_MAX_REVISION_HISTORY"),
		},
		&cli.DurationFlag{
			Name:        "revision-coalesce-window",
			Destination: &o.revisionCoalesceWindow,
			Category:    catRevisions,
			Usage:       "coalesce rapid successive saves by the same author (e.g. 5m); 0 = disabled",
			Value:       5 * time.Minute,
			Sources:     envVars("LEAFWIKI_REVISION_COALESCE_WINDOW"),
			DefaultText: "5m",
		},
	}
}
