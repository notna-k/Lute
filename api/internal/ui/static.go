package ui

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:web
var embedded embed.FS

// Register serves the SPA for non-/api paths; call it after the /api routes. It writes
// files itself because http.FileServer redirects "/index.html" to "./", which with
// FileFromFS loops forever on GET /.
func Register(r *gin.Engine) {
	sub, err := fs.Sub(embedded, "web")
	if err != nil {
		panic("ui embed: " + err.Error())
	}

	indexBytes, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic("ui embed: missing index.html: " + err.Error())
	}

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			if !c.Writer.Written() {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			}
			return
		}
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead:
		default:
			c.Status(http.StatusNotFound)
			return
		}

		rel := strings.TrimPrefix(path.Clean("/"+c.Request.URL.Path), "/")
		if rel == "" || rel == "." {
			serveIndex(c, indexBytes)
			return
		}
		if fi, err := fs.Stat(sub, rel); err == nil && !fi.IsDir() {
			data, err := fs.ReadFile(sub, rel)
			if err == nil {
				ctype := mime.TypeByExtension(path.Ext(rel))
				if ctype == "" {
					ctype = http.DetectContentType(data)
				}
				c.Data(http.StatusOK, ctype, data)
				return
			}
		}
		serveIndex(c, indexBytes)
	})
}

func serveIndex(c *gin.Context, data []byte) {
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}
