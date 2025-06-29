package ssr

import (
	"bytes"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
	"github.com/yuin/goldmark"
)

var mdRenderer = goldmark.New()

var tmpl *template.Template

func loadTemplate(fileSys fs.FS, environment string) {
	var err error
	if environment == "production" {
		if tmpl != nil {
			return
		}
	}

	tmpl, err = template.ParseFS(fileSys, "index.html")
	if err != nil {
		panic("could not parse template: " + err.Error())
	}
}

func IsFrontendRoute(path string) bool {
	// Define known frontend routes (match react-router-dom)
	path = strings.TrimSuffix(path, "/")

	if path == "/login" || path == "/users" || path == "/" {
		return true
	}
	if strings.HasPrefix(path, "/e/") {
		return true
	}
	// fallback route: catch-all
	return true // because "*" maps to PageViewer
}

func IsSSRPath(p string) bool {
	return !strings.HasPrefix(p, "/api/") &&
		!strings.HasPrefix(p, "/e/") &&
		!strings.HasPrefix(p, "/assets/") &&
		!strings.HasPrefix(p, "/static/") &&
		!strings.HasPrefix(p, "/favicon") &&
		!strings.HasPrefix(p, "/@vite/") &&
		!strings.HasPrefix(p, "/@react/") &&
		!strings.HasPrefix(p, "/src/") &&
		!strings.HasSuffix(p, ".js") &&
		!strings.HasSuffix(p, ".ts") &&
		!strings.HasSuffix(p, ".tsx") &&
		!strings.HasSuffix(p, ".css") &&
		!strings.HasSuffix(p, ".map") &&
		!strings.HasSuffix(p, ".ico") &&
		!strings.HasSuffix(p, ".svg")
}

func RenderSSRPage(c *gin.Context, fileSys fs.FS, wikiInstance *wiki.Wiki, environment string) {
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	path := strings.TrimPrefix(c.Request.URL.Path, "/")

	log.Print("SSRHandler called with path: " + path)

	// Redirect to first page if path is empty
	if path == "" {
		if wikiInstance.GetTree() == nil {
			log.Print("Wiki instance has no tree set")
			c.Status(http.StatusNotFound)
			return
		}
		if len(wikiInstance.GetTree().Children) == 0 {
			log.Print("Wiki instance has no pages")
			c.Status(http.StatusNotFound)
			return
		}

		slug := wikiInstance.GetTree().Children[0].Slug
		// Redirect to the first page
		log.Print("Redirecting to first page: " + slug)
		c.Redirect(http.StatusFound, "/"+slug)
		return
	}
	renderPage(c, fileSys, wikiInstance, environment, path)
}

func renderPage(c *gin.Context, fileSys fs.FS, wikiInstance *wiki.Wiki, environment string, path string) {
	// Initialize the template if not already done
	loadTemplate(fileSys, environment)

	// Render the SSR page
	page, err := wikiInstance.FindByPath(path)
	if err != nil || page == nil {
		log.Printf("Error finding page by path '%s': %v", path, err)
		RenderNotFoundSSRPage(c, fileSys, environment)
		return
	}

	var htmlBuf bytes.Buffer
	if err := mdRenderer.Convert([]byte(page.Content), &htmlBuf); err != nil {
		c.String(http.StatusInternalServerError, "Markdown error")
		return
	}

	data := TemplateData{
		Title:       page.Title + " - Leafwiki",
		Description: "",
		Content:     template.HTML(htmlBuf.String()),
	}

	if err = tmpl.Execute(c.Writer, data); err != nil {
		log.Printf("Error executing template: %v", err)
		c.String(http.StatusInternalServerError, "Template rendering error")
		return
	}
}

func RenderEmptySSRPage(c *gin.Context, fileSys fs.FS, environment string) {
	// Initialize the template if not already done
	loadTemplate(fileSys, environment)

	data := TemplateData{
		Title:       "Leafwiki",
		Description: "",
		Content:     "",
	}

	if err := tmpl.Execute(c.Writer, data); err != nil {
		log.Printf("Error executing template: %v", err)
		c.String(http.StatusInternalServerError, "Template rendering error")
		return
	}
}

func RenderNotFoundSSRPage(c *gin.Context, fileSys fs.FS, environment string) {
	// Initialize the template if not already done
	loadTemplate(fileSys, environment)

	data := TemplateData{
		Title:       "Page Not Found",
		Description: "The page you are looking for does not exist.",
		Content:     "The page you are looking for does not exist.",
	}

	if err := tmpl.Execute(c.Writer, data); err != nil {
		log.Printf("Error executing template: %v", err)
		c.String(http.StatusInternalServerError, "Template rendering error")
		return
	}
}
