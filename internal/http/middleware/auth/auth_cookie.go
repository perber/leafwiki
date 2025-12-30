package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/http/middleware/utils"
)

type AuthCookies struct {
	AllowInsecure bool
	SameSite      http.SameSite
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

func NewAuthCookies(allowInsecure bool, accessTTL, refreshTTL time.Duration) *AuthCookies {
	return &AuthCookies{
		AllowInsecure: allowInsecure,
		SameSite:      http.SameSiteStrictMode,
		AccessTTL:     accessTTL,
		RefreshTTL:    refreshTTL,
	}
}

func (a *AuthCookies) cookieNames(secure bool) (access, refresh string) {
	if secure {
		return "__Host-leafwiki_at", "__Host-leafwiki_rt"
	}
	return "leafwiki_at", "leafwiki_rt"
}

func (a *AuthCookies) Set(c *gin.Context, accessToken, refreshToken string) error {
	secure, err := utils.RequireSecure(c, a.AllowInsecure)
	if err != nil {
		return err
	}

	accessName, refreshName := a.cookieNames(secure)

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     accessName,
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: a.SameSite,
		MaxAge:   int(a.AccessTTL.Seconds()),
	})

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshName,
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: a.SameSite,
		MaxAge:   int(a.RefreshTTL.Seconds()),
	})

	return nil
}

func (a *AuthCookies) Clear(c *gin.Context) error {
	secure, err := utils.RequireSecure(c, a.AllowInsecure)
	if err != nil {
		return err
	}

	accessName, refreshName := a.cookieNames(secure)

	expire := func(name, path string) {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     path,
			HttpOnly: true,
			Secure:   secure,
			SameSite: a.SameSite,
			MaxAge:   -1,
		})
	}

	expire(accessName, "/")
	expire(refreshName, "/")
	return nil
}

func (a *AuthCookies) ReadAccess(c *gin.Context) (string, error) {
	secure, err := utils.RequireSecure(c, a.AllowInsecure)
	if err != nil {
		return "", err
	}
	accessName, _ := a.cookieNames(secure)
	return c.Cookie(accessName)
}

func (a *AuthCookies) ReadRefresh(c *gin.Context) (string, error) {
	secure, err := utils.RequireSecure(c, a.AllowInsecure)
	if err != nil {
		return "", err
	}
	_, refreshName := a.cookieNames(secure)
	return c.Cookie(refreshName)
}
