package gateway

import (
	_ "embed"
	"log"
	"net/http"
)

// Hub favicon assets (FEATURE-515). The icon is the co-shell mark with a chat
// bubble docked in its bottom-right quarter. It ships as an SVG (preferred by
// modern browsers and crisp at any size) plus a PNG fallback for older ones;
// both are embedded so the hub keeps shipping as a single binary.
//
//go:embed static/favicon.svg
var faviconSVG []byte

//go:embed static/favicon.png
var faviconPNG []byte

// handleFavicon serves an embedded favicon with an explicit content type and a
// one-day cache lifetime (the icon only changes with a new hub build).
func handleFavicon(body []byte, contentType string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", contentType)
		rw.Header().Set("Cache-Control", "public, max-age=86400")
		if _, err := rw.Write(body); err != nil {
			log.Printf("webui: favicon write: %v", err)
		}
	}
}
