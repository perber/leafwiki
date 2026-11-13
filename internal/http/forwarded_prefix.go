package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// forwardedPrefixHeader is the header gin prepends to the Location of its
// trailing-slash redirect.
const forwardedPrefixHeader = "X-Forwarded-Prefix"

// Handler returns the http.Handler the server serves engine with. It drops
// X-Forwarded-Prefix before gin sees the request: gin takes the header from
// any client and puts it in front of the trailing-slash redirect target, which
// runs before any middleware. LeafWiki's sub-path is --base-path, so nothing
// needs the header.
func Handler(engine *gin.Engine) http.Handler {
	next := engine.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del(forwardedPrefixHeader)
		next.ServeHTTP(w, r)
	})
}
