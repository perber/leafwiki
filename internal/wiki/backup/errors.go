package wikibackup

import "github.com/gin-gonic/gin"

const (
	ErrCodeBackupNotEnabled  = "backup_not_enabled"
	ErrCodeBackupInternalError = "backup_internal_error"
)

// BackupErrorResponse is the structured JSON error body returned by backup endpoints.
type BackupErrorResponse struct {
	Error BackupErrorDetail `json:"error"`
}

// BackupErrorDetail carries the localization-ready error data.
type BackupErrorDetail struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Template string   `json:"template"`
	Args     []string `json:"args,omitempty"`
}

func respondWithBackupStatusError(c *gin.Context, status int, code, message, template string, args ...string) {
	c.JSON(status, BackupErrorResponse{
		Error: BackupErrorDetail{
			Code:     code,
			Message:  message,
			Template: template,
			Args:     append([]string(nil), args...),
		},
	})
}
