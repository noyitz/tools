package sandbox

import (
	"io/fs"
	"net/http"
)

// NewServer creates an HTTP server with API routes and embedded static files.
func NewServer(handlers *Handlers, webFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("POST /api/connect", handlers.HandleConnect)
	mux.HandleFunc("GET /api/health", handlers.HandleHealth)
	mux.HandleFunc("POST /api/send/request", handlers.HandleSendRequest)
	mux.HandleFunc("POST /api/send/response", handlers.HandleSendResponse)
	mux.HandleFunc("GET /api/presets", handlers.HandlePresets)
	mux.HandleFunc("GET /api/cluster", handlers.HandleClusterInfo)
	mux.HandleFunc("POST /api/cluster/login", handlers.HandleClusterLogin)
	mux.HandleFunc("POST /api/cluster/logout", handlers.HandleClusterLogout)

	// Static files
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	return mux
}
