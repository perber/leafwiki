package htmlutil

import (
	"html"
	"strings"
)

type AllowedMarker struct {
	Marker string
	HTML   string
}

func EscapeText(text string) string {
	return html.EscapeString(text)
}

func EscapeTextWithAllowedMarkers(text string, markers ...AllowedMarker) string {
	escaped := EscapeText(text)
	for _, marker := range markers {
		if marker.Marker == "" {
			continue
		}
		escaped = strings.ReplaceAll(escaped, marker.Marker, marker.HTML)
	}
	return escaped
}
