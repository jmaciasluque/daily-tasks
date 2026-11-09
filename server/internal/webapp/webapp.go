// Package webapp serves the hosted web UI (built from ../../web with
// `npm run build:hosted`) under /app/. CI copies the build into dist/ before
// deploying; without it /app/ explains that the UI is not bundled.
package webapp

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the single-page app mounted at /app/.
func Handler() http.Handler {
	root, _ := fs.Sub(dist, "dist")
	files := http.FileServerFS(root)
	_, err := fs.Stat(root, "index.html")
	built := err == nil
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		// React uses inline style attributes; everything else is same-origin.
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if !built {
			http.Error(w, "web UI not bundled in this build", http.StatusNotFound)
			return
		}
		rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/app")
		rel = strings.TrimPrefix(rel, "/")
		if rel != "" && rel != "index.html" {
			if _, err := fs.Stat(root, rel); err == nil {
				if strings.HasPrefix(rel, "assets/") {
					h.Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/" + rel
				files.ServeHTTP(w, r2)
				return
			}
		}
		// SPA entry point (also for client-side routes); never cache it so
		// new deploys are picked up.
		h.Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, root, "index.html")
	})
}
