package auth

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
	CookiePath    string
}

func NewAuthCookies(allowInsecure bool, accessTTL, refreshTTL time.Duration, cookiePath string) *AuthCookies {
	if cookiePath == "" {
		cookiePath = "/"
	}
	return &AuthCookies{
		AllowInsecure: allowInsecure,
		SameSite:      http.SameSiteLaxMode,
		AccessTTL:     accessTTL,
		RefreshTTL:    refreshTTL,
		CookiePath:    cookiePath,
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
		Path:     a.CookiePath,
		HttpOnly: true,
		Secure:   secure,
		SameSite: a.SameSite,
		MaxAge:   int(a.AccessTTL.Seconds()),
	})

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshName,
		Value:    refreshToken,
		Path:     a.CookiePath,
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

	expire := func(name string) {
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     a.CookiePath,
			HttpOnly: true,
			Secure:   secure,
			SameSite: a.SameSite,
			MaxAge:   -1,
		})
	}

	expire(accessName)
	expire(refreshName)
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
