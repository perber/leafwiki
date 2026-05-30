package revisions

import (
	"mime"
	"path"
	"strings"
)

func DetectRevisionAssetMIMEType(assetName, manifestMIMEType string) string {
	if manifestMIMEType != "" {
		return manifestMIMEType
	}
	if mimeType := mime.TypeByExtension(strings.ToLower(path.Ext(assetName))); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}
