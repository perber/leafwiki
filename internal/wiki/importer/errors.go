package importer

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

const (
	ErrCodeImporterNoPlan           = "importer_no_plan"
	ErrCodeImporterExecutionRunning = "importer_execution_running"
	ErrCodeImporterStateUnavailable = "importer_state_unavailable"
	ErrCodeImporterInternalError    = "importer_internal_error"
	ErrCodeImporterUploadTooLarge   = "importer_upload_too_large"
	ErrCodeImporterMissingFile      = "importer_missing_file"
	ErrCodeImporterFileOpenFailed   = "importer_file_open_failed"
)

// ImporterErrorResponse is the structured JSON error body returned by importer endpoints.
type ImporterErrorResponse struct {
	Error ImporterErrorDetail `json:"error"`
}

// ImporterErrorDetail carries the localization-ready error data.
type ImporterErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithImporterStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, ImporterErrorResponse{
		Error: ImporterErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}

// respondWithImporterError maps errors to JSON responses for importer endpoints.
func respondWithImporterError(c *gin.Context, err error) {
	if loc, ok := sharederrors.AsLocalizedError(err); ok {
		respondWithImporterStatusError(c, importerErrorStatus(loc.Code), loc.Code, loc.Message, loc.Template, loc.Args...)
		return
	}

	respondWithImporterStatusError(c, http.StatusInternalServerError, ErrCodeImporterInternalError, "Importer request failed", "importer request failed")
}

func importerErrorStatus(code string) int {
	switch code {
	case ErrCodeImporterNoPlan:
		return http.StatusNotFound
	case ErrCodeImporterExecutionRunning:
		return http.StatusConflict
	case ErrCodeImporterStateUnavailable:
		return http.StatusInternalServerError
	case ErrCodeImporterUploadTooLarge:
		return http.StatusRequestEntityTooLarge
	case ErrCodeImporterMissingFile, ErrCodeImporterFileOpenFailed:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
