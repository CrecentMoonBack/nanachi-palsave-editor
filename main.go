package main

import (
	"embed"
	"net/http"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/wo420/nanachi-palsave-editor/internal/icons"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "나나치의 팰월드 세이브 에디터",
		Width:  1280,
		Height: 860,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: iconMiddleware,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// iconMiddleware serves /icons/* from the user's local icon folder, letting
// the frontend use a plain <img src="/icons/foo.webp">.
//
// Artwork is never embedded — see docs/THIRD_PARTY.md — so this quietly serves
// nothing when the folder is absent and the UI falls back to text names.
func iconMiddleware(next http.Handler) http.Handler {
	h := icons.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/icons/") {
			h.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
