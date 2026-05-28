package revisions

import "testing"

func TestDetectRevisionAssetMIMETypeFallsBackToExtensionThenOctetStream(t *testing.T) {
	t.Run("manifest value wins", func(t *testing.T) {
		if got := DetectRevisionAssetMIMEType("style.css", "text/custom"); got != "text/custom" {
			t.Fatalf("DetectRevisionAssetMIMEType = %q, want manifest MIME", got)
		}
	})

	t.Run("extension fallback", func(t *testing.T) {
		if got := DetectRevisionAssetMIMEType("style.css", ""); got != "text/css; charset=utf-8" {
			t.Fatalf("DetectRevisionAssetMIMEType = %q, want CSS MIME", got)
		}
	})

	t.Run("octet stream fallback", func(t *testing.T) {
		if got := DetectRevisionAssetMIMEType("asset.unknownext", ""); got != "application/octet-stream" {
			t.Fatalf("DetectRevisionAssetMIMEType = %q, want octet-stream fallback", got)
		}
	})
}
