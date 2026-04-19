package branding

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodeBrandingConfigUnavailable   = "branding_config_unavailable"
	ErrCodeBrandingLogoInvalidType     = "branding_logo_invalid_type"
	ErrCodeBrandingLogoUploadFailed    = "branding_logo_upload_failed"
	ErrCodeBrandingLogoDeleteFailed    = "branding_logo_delete_failed"
	ErrCodeBrandingFaviconInvalidType  = "branding_favicon_invalid_type"
	ErrCodeBrandingFaviconUploadFailed = "branding_favicon_upload_failed"
	ErrCodeBrandingFaviconDeleteFailed = "branding_favicon_delete_failed"
	ErrCodeBrandingUpdateFailed        = "branding_update_failed"
	ErrCodeBrandingInternalError       = "branding_internal_error"
)

// BrandingErrorResponse is the structured JSON error body returned by branding endpoints.
type BrandingErrorResponse struct {
	Error BrandingErrorDetail `json:"error"`
}

// BrandingErrorDetail carries the localization-ready error data.
type BrandingErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithBrandingStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, BrandingErrorResponse{
		Error: BrandingErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

// respondWithBrandingError maps errors to JSON responses for branding endpoints.
func respondWithBrandingError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithBrandingStatusError(c, brandingErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
		return
	}

	var vErr *sharederrors.ValidationErrors
	if errors.As(err, &vErr) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation_error",
			"fields": vErr.Errors,
		})
		return
	}

	respondWithBrandingStatusError(c, http.StatusInternalServerError, ErrCodeBrandingInternalError, "Branding request failed", "branding request failed")
}

func brandingErrorStatus(code string) int {
	switch code {
	case ErrCodeBrandingLogoInvalidType, ErrCodeBrandingFaviconInvalidType:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
