package pagedisplay

import "encoding/json"

// Settings holds every instance-wide page display setting. The JSON tags are
// the field names GET /api/config exposes (see ConfigFields), so a new
// setting added here reaches the frontend without touching the HTTP layer.
type Settings struct {
	// AlwaysShowToc reports whether the TOC should always be shown,
	// regardless of heading count.
	AlwaysShowToc bool `json:"alwaysShowToc"`
	// ContentWidth is the page reading-column width, one of ContentWidths().
	ContentWidth string `json:"contentWidth"`
}

// ConfigFields returns the settings as the flat key/value pairs that
// GET /api/config merges into its response, keyed by the JSON tags above.
func (s Settings) ConfigFields() map[string]any {
	// Settings holds only plain bool/string fields, so neither call can fail.
	data, _ := json.Marshal(s)
	fields := map[string]any{}
	_ = json.Unmarshal(data, &fields)
	return fields
}

// Provider is the read-only view of the page display settings that the HTTP
// layer depends on. *Service implements it; so does the value returned by
// Fixed.
type Provider interface {
	// Settings returns the current page display settings.
	Settings() Settings
}

// Fixed returns a Provider pinned to a compile-time value, for the router's
// default (no Service configured) and for tests.
func Fixed(s Settings) Provider { return fixedProvider(s) }

type fixedProvider Settings

func (f fixedProvider) Settings() Settings { return Settings(f) }
