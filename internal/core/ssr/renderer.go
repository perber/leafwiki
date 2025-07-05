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

var mdRenderer goldmark.Markdown
var breadcrumbsRenderer *BreadcrumbsRenderer
var sidebarRenderer *SidebarRenderer

var spaTemplate *template.Template
var publicTemplate *template.Template

func init() {
	mdRenderer = goldmark.New(goldmark.WithExtensions())
	breadcrumbsRenderer = NewBreadcrumbsRenderer()
	sidebarRenderer = NewSidebarRenderer()
}

func loadPublicTemplate(fileSys fs.FS, environment string) {
	var err error
	if environment == "production" {
		if publicTemplate != nil {
			return
		}
	}

	publicTemplate, err = template.ParseFS(fileSys, "index.public.html")
	if err != nil {
		panic("could not parse template: " + err.Error())
	}
}

func loadSPATemplate(fileSys fs.FS, environment string) {
	var err error
	if environment == "production" {
		if spaTemplate != nil {
			return
		}
	}

	spaTemplate, err = template.ParseFS(fileSys, "index.html")
	if err != nil {
		panic("could not parse template: " + err.Error())
	}
}

// IsInteractiveRoute checks if the given path is an interactive route
// (i.e., a route that should be handled by the frontend).
func IsInteractiveRoute(path string) bool {
	// Define known frontend routes (match react-router-dom)
	path = strings.TrimSuffix(path, "/")

	if strings.HasPrefix(path, "/users") {
		return true
	}
	if strings.HasPrefix(path, "/e/") {
		return true
	}
	// fallback route: catch-all
	return false
}

// IsAuthPath checks if the page is an authentication-related path.
func IsAuthPath(path string) bool {
	path = strings.TrimSuffix(path, "/")
	return strings.HasPrefix(path, "/login")
}

func IsApiPath(p string) bool {
	return strings.HasPrefix(p, "/api/")
}

func RenderPublicPage(c *gin.Context, fileSys fs.FS, wikiInstance *wiki.Wiki, environment string) {
	path := strings.TrimPrefix(c.Request.URL.Path, "/")

	log.Print("SSRHandler called with path: " + path)

	// Redirect to first page if path is empty
	if path == "" {
		if wikiInstance.GetTree() == nil {
			log.Print("Wiki instance has no tree set")
			RenderNotFoundPublicPage(c, fileSys, environment)
			return
		}
		if len(wikiInstance.GetTree().Children) == 0 {
			log.Print("Wiki instance has no pages")
			RenderNotFoundPublicPage(c, fileSys, environment)
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
	loadPublicTemplate(fileSys, environment)

	// Render the SSR page
	page, err := wikiInstance.FindByPath(path)
	if err != nil || page == nil {
		log.Printf("Error finding page by path '%s': %v", path, err)
		RenderNotFoundPublicPage(c, fileSys, environment)
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
		Breadcrumbs: template.HTML(buildBreadcrumbs(path, wikiInstance)),
		Sidebar:     template.HTML(buildSidebar()),
	}

	var rendered bytes.Buffer
	if err := publicTemplate.Execute(&rendered, data); err != nil {
		log.Printf("Error executing template: %v", err)
		c.String(http.StatusInternalServerError, "Template rendering error")
		return
	}

	c.Status(http.StatusOK)
	c.Writer.Write(rendered.Bytes())
}

func buildBreadcrumbs(path string, wikiInstance *wiki.Wiki) string {
	crumbs := []Breadcrumb{}
	if path == "" {
		return breadcrumbsRenderer.Render(crumbs)
	}

	// Split the path into segments
	segments := strings.Split(path, "/")
	currentPath := ""

	for _, segment := range segments {
		if segment == "" {
			continue // Skip empty segments
		}
		currentPath += segment + "/"

		page, err := wikiInstance.FindByPath(strings.TrimSuffix(currentPath, "/"))
		if err != nil || page == nil {
			continue // Skip if page not found
		}

		crumbs = append(crumbs, Breadcrumb{
			Title: page.Title,
			URL:   strings.TrimSuffix(currentPath, "/"),
		})
	}

	return breadcrumbsRenderer.Render(crumbs)
}

func buildSidebar() string {
	tabs := []Tabs{
		{
			title: "Tree",
			svg:   `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-folder-tree" aria-hidden="true"><path d="M20 10a1 1 0 0 0 1-1V6a1 1 0 0 0-1-1h-2.5a1 1 0 0 1-.8-.4l-.9-1.2A1 1 0 0 0 15 3h-2a1 1 0 0 0-1 1v5a1 1 0 0 0 1 1Z"></path><path d="M20 21a1 1 0 0 0 1-1v-3a1 1 0 0 0-1-1h-2.9a1 1 0 0 1-.88-.55l-.42-.85a1 1 0 0 0-.92-.6H13a1 1 0 0 0-1 1v5a1 1 0 0 0 1 1Z"></path><path d="M3 5a2 2 0 0 0 2 2h3"></path><path d="M3 3v13a2 2 0 0 0 2 2h3"></path></svg>`,
		},
		{
			title: "Search",
			svg:   `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-search" aria-hidden="true"><path d="m21 21-4.34-4.34"></path><circle cx="11" cy="11" r="8"></circle></svg>`,
		},
	}

	return sidebarRenderer.Render(tabs)
}

func RenderNotFoundPublicPage(c *gin.Context, fileSys fs.FS, environment string) {
	// Initialize the template if not already done
	loadPublicTemplate(fileSys, environment)

	data := TemplateData{
		Title:       "Page Not Found",
		Description: "The page you are looking for does not exist.",
		Content:     "The page you are looking for does not exist.",
	}

	var rendered bytes.Buffer
	if err := publicTemplate.Execute(&rendered, data); err != nil {
		log.Printf("Error executing template: %v", err)
		c.String(http.StatusInternalServerError, "Template rendering error")
		return
	}

	c.Status(http.StatusNotFound)
	c.Writer.Write(rendered.Bytes())
}

func RenderForbiddenPage(c *gin.Context, fileSys fs.FS, environment string) {
	// Initialize the template if not already done
	loadPublicTemplate(fileSys, environment)

	data := TemplateData{
		Title:       "Forbidden",
		Description: "You do not have permission to access this page.",
		Content:     "You do not have permission to access this page.",
	}

	var rendered bytes.Buffer
	if err := publicTemplate.Execute(&rendered, data); err != nil {
		log.Printf("Error executing template: %v", err)
		c.String(http.StatusInternalServerError, "Template rendering error")
		return
	}

	c.Status(http.StatusForbidden)
	c.Writer.Write(rendered.Bytes())
}

func RenderSPAPage(c *gin.Context, fileSys fs.FS, environment string) {
	// Initialize the template if not already done
	loadSPATemplate(fileSys, environment)

	data := TemplateData{
		Title:       "Leafwiki",
		Description: "A modern wiki platform",
		Content:     "",
	}

	var rendered bytes.Buffer
	if err := spaTemplate.Execute(&rendered, data); err != nil {
		log.Printf("Error executing template: %v", err)
		c.String(http.StatusInternalServerError, "Template rendering error")
		return
	}

	c.Status(http.StatusOK)
	c.Writer.Write(rendered.Bytes())
}
