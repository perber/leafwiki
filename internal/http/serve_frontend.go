package http

import (
	"io/fs"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/wiki"
)

func serveFrontend(router *gin.Engine, wikiInstance *wiki.Wiki) (fs.FS, error) {

	var distFS fs.FS
	var staticFS fs.FS
	var err error

	if EmbedFrontend == "true" {
		// If embedding, use the embedded filesystem
		distFS, err = fs.Sub(frontend, "dist")
		if err != nil {
			return nil, err
		}
		staticFS, err = fs.Sub(frontend, "dist/static")
		if err != nil {
			return nil, err
		}
	} else {
		// If not embedding, use the local filesystem
		distFS = os.DirFS("../../dist")
		staticFS = os.DirFS("../../dist/static")
	}

	router.StaticFS("/assets", gin.Dir(wikiInstance.GetAssetService().GetAssetsDir(), true))

	// Serve the embedded frontend files js, css, ...
	router.StaticFS("/static", http.FS(staticFS))

	router.GET("/favicon.svg", func(c *gin.Context) {
		file, err := distFS.Open("favicon.svg")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		stat, err := file.Stat()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		c.DataFromReader(http.StatusOK, stat.Size(), "image/svg+xml", file, nil)
	})

	return distFS, nil
}
