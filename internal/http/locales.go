package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	localeLangPattern = regexp.MustCompile(`^[a-z]{2}(-[A-Za-z]{2,8})?$`)
	localeNSPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

type localeLanguage struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type localesListResponse struct {
	Languages []localeLanguage `json:"languages"`
}

// ResolveLocalesDir returns the directory used to serve translation files.
// When localesDir is empty, it defaults to a "locales" folder next to the data directory.
func ResolveLocalesDir(dataDir, localesDir string) string {
	if strings.TrimSpace(localesDir) != "" {
		return localesDir
	}

	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		absDataDir = dataDir
	}

	return filepath.Join(filepath.Dir(absDataDir), "locales")
}

// RegisterLocalesRoutes serves runtime locale JSON files and lists available languages.
func RegisterLocalesRoutes(base *gin.RouterGroup, localesDir string) {
	localesDir = strings.TrimSpace(localesDir)
	if localesDir == "" {
		return
	}

	api := base.Group("/api")
	api.GET("/locales", func(c *gin.Context) {
		languages, err := listLocaleLanguages(localesDir)
		if err != nil {
			slog.Default().Error("failed to list locales", "error", err, "dir", localesDir)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list locales"})
			return
		}
		c.JSON(http.StatusOK, localesListResponse{Languages: languages})
	})

	base.GET("/locales/:lang/:ns", func(c *gin.Context) {
		lang := c.Param("lang")
		ns := strings.TrimSuffix(c.Param("ns"), ".json")
		if !localeLangPattern.MatchString(lang) || !localeNSPattern.MatchString(ns) {
			c.Status(http.StatusNotFound)
			return
		}

		filePath, err := resolveLocaleFilePath(localesDir, lang, ns)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		if _, err := os.Stat(filePath); err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		c.Header("Content-Type", "application/json; charset=utf-8")
		c.File(filePath)
	})
}

func resolveLocaleFilePath(localesDir, lang, ns string) (string, error) {
	filePath := filepath.Join(localesDir, lang, ns+".json")
	cleanLocalesDir := filepath.Clean(localesDir)
	cleanFilePath := filepath.Clean(filePath)

	relPath, err := filepath.Rel(cleanLocalesDir, cleanFilePath)
	if err != nil {
		return "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", os.ErrPermission
	}

	return cleanFilePath, nil
}

func listLocaleLanguages(localesDir string) ([]localeLanguage, error) {
	names := readLanguageNames(localesDir)
	entries, err := os.ReadDir(localesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []localeLanguage{{Code: "en", Name: fallbackLanguageName("en", names)}}, nil
		}
		return nil, err
	}

	seen := map[string]struct{}{}
	languages := make([]localeLanguage, 0, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		code := entry.Name()
		if !localeLangPattern.MatchString(code) {
			continue
		}
		if !localeDirHasNamespaces(localesDir, code) {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}

		seen[code] = struct{}{}
		languages = append(languages, localeLanguage{
			Code: code,
			Name: fallbackLanguageName(code, names),
		})
	}

	if len(languages) == 0 {
		return []localeLanguage{{Code: "en", Name: fallbackLanguageName("en", names)}}, nil
	}

	return languages, nil
}

func localeDirHasNamespaces(localesDir, lang string) bool {
	entries, err := os.ReadDir(filepath.Join(localesDir, lang))
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			return true
		}
	}

	return false
}

func readLanguageNames(localesDir string) map[string]string {
	names := map[string]string{}
	data, err := os.ReadFile(filepath.Join(localesDir, "languages.json"))
	if err != nil {
		return names
	}

	if err := json.Unmarshal(data, &names); err != nil {
		slog.Default().Warn("invalid languages.json", "error", err, "dir", localesDir)
	}

	return names
}

func fallbackLanguageName(code string, names map[string]string) string {
	if name := strings.TrimSpace(names[code]); name != "" {
		return name
	}

	switch code {
	case "en":
		return "English"
	case "ru":
		return "Русский"
	default:
		return code
	}
}
