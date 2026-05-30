package oauth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	oauthserver "github.com/go-oauth2/oauth2/v4/server"
	httpinternal "github.com/perber/wiki/internal/http"
)

func writeOAuthBadRequest(c *gin.Context, err error) {
	c.String(http.StatusBadRequest, err.Error())
}

func writeRegistrationError(c *gin.Context, description string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error":             "invalid_client_metadata",
		"error_description": description,
	})
}

func writeTokenError(c *gin.Context, server *oauthserver.Server, err error) {
	data, status, header := server.GetErrorData(err)
	for name, values := range header {
		for _, value := range values {
			c.Header(name, value)
		}
	}
	c.JSON(status, data)
}

func (r *Routes) handleApprovalDetails(ctx httpinternal.RouterContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := r.currentWebUser(c, ctx)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		details, ok := r.service.approvalDetails(c.Query("approval_token"), user.ID)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_approval"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"clientLabel": details.ClientLabel,
			"clientId":    details.ClientID,
			"redirectUri": details.RedirectURI,
			"scope":       details.Scope,
			"resource":    details.Resource,
		})
	}
}
