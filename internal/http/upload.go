package http

import (
	"errors"
	"mime/multipart"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrUploadTooLarge is returned by ParseUploadedFile when the request body
// exceeds maxSize.
var ErrUploadTooLarge = errors.New("upload too large")

// ErrUploadMissingFile is returned by ParseUploadedFile when the named
// multipart field is absent from the request.
var ErrUploadMissingFile = errors.New("missing uploaded file")

// ParseUploadedFile enforces a maximum request-body size and extracts the
// uploaded file at fieldName from a multipart/form-data request. Callers own
// closing the returned multipart.File. Shared by every domain that accepts a
// single-file upload (branding logo/favicon, avatars, ...) so the
// MaxBytesReader + ParseMultipartForm + FormFile boilerplate lives in one
// place.
func ParseUploadedFile(c *gin.Context, maxSize int64, fieldName string) (multipart.File, *multipart.FileHeader, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
	if err := c.Request.ParseMultipartForm(maxSize); err != nil {
		return nil, nil, ErrUploadTooLarge
	}

	file, header, err := c.Request.FormFile(fieldName)
	if err != nil {
		return nil, nil, ErrUploadMissingFile
	}

	return file, header, nil
}
