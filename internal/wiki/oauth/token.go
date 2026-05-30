package oauth

import "github.com/gin-gonic/gin"

func (r *Routes) handleToken(c *gin.Context) {
	if err := r.service.validateRefreshRequest(c.Request); err != nil {
		writeTokenError(c, r.service.server, err)
		return
	}
	_ = r.service.server.HandleTokenRequest(c.Writer, c.Request)
}
