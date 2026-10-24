package httpadapter

import (
	_ "embed"
	"net/http"

	"github.com/swaggest/swgui"
	"github.com/swaggest/swgui/v5emb"
)

// Interactive copy of v2 OpenAPI: relative server URL, example user and numeric
// discount matching the assignment's examples and Postman. The original is kept intact.
//
//go:embed openapi.json
var openAPISpec []byte

func registerSwagger(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(openAPISpec)
	})
	mux.HandleFunc("GET /swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/", http.StatusMovedPermanently)
	})
	mux.Handle("GET /swagger/", v5emb.NewHandlerWithConfig(swgui.Config{
		Title:       "Hotels Booking System",
		SwaggerJSON: "/openapi.json",
		BasePath:    "/swagger/",
		SettingsUI: map[string]string{
			"tryItOutEnabled":        "true",
			"displayRequestDuration": "true",
		},
	}))
}
