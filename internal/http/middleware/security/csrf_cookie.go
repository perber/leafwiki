package security

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/middleware/utils"
)

type CSRFCookie struct {
	AllowInsecure bool
	SameSite      http.SameSite
	TTL           time.Duration
}

func NewCSRFCookie(allowInsecure bool, ttl time.Duration) *CSRFCookie {
	return &CSRFCookie{
		AllowInsecure: allowInsecure,
		SameSite:      http.SameSiteLaxMode,
		TTL:           ttl,
	}
}

func (c *CSRFCookie) cookieName(secure bool) string {
	if secure {
		return "__Host-leafwiki_csrf"
	}
	return "leafwiki_csrf"
}

func (c *CSRFCookie) generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (c *CSRFCookie) Issue(ctx *gin.Context) (string, error) {
	secure, err := utils.RequireSecure(ctx, c.AllowInsecure)
	if err != nil {
		return "", err
	}

	if existing, err := ctx.Cookie(c.cookieName(secure)); err == nil && existing != "" {
		ctx.Header("X-CSRF-Token", existing)
		return existing, nil
	}

	token, err := c.generateToken()
	if err != nil {
		return "", err
	}

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     c.cookieName(secure),
		Value:    token,
		Path:     "/",
		HttpOnly: false, // JS should read this cookie
		Secure:   secure,
		SameSite: c.SameSite,
		MaxAge:   int(c.TTL.Seconds()),
	})

	// optional, but useful for the frontend
	ctx.Header("X-CSRF-Token", token)

	return token, nil
}

// Read reads the current CSRF token from the cookie.
func (c *CSRFCookie) Read(ctx *gin.Context) (string, error) {
	secure, err := utils.RequireSecure(ctx, c.AllowInsecure)
	if err != nil {
		return "", err
	}
	return ctx.Cookie(c.cookieName(secure))
}

// Clear clears the CSRF cookie (e.g., on logout).
func (c *CSRFCookie) Clear(ctx *gin.Context) error {
	secure, err := utils.RequireSecure(ctx, c.AllowInsecure)
	if err != nil {
		return err
	}

	http.SetCookie(ctx.Writer, &http.Cookie{
		Name:     c.cookieName(secure),
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: c.SameSite,
		MaxAge:   -1,
	})

	return nil
}
