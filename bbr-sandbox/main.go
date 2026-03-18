package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/noyitz/tools/bbr-sandbox/internal/sandbox"
)

//go:embed web
var webFS embed.FS

func main() {
	port := flag.Int("port", 8888, "HTTP server port")
	noBrowser := flag.Bool("no-browser", false, "Don't auto-open browser")
	namespace := flag.String("namespace", "bbr-plugins", "Kubernetes namespace for BBR deployment")
	flag.Parse()

	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("failed to load embedded web files: %v", err)
	}

	handlers := sandbox.NewHandlers(*namespace)
	server := sandbox.NewServer(handlers, webContent)

	addr := fmt.Sprintf(":%d", *port)
	url := fmt.Sprintf("http://localhost:%d", *port)

	if !*noBrowser {
		go openBrowser(url)
	}

	log.Printf("BBR Plugin Sandbox running at %s", url)
	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}
