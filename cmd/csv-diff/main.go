package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"csv-diff/internal/browser"
	"csv-diff/internal/server"
)

func main() {
	var (
		host        = flag.String("host", "127.0.0.1", "Host address to bind HTTP server")
		port        = flag.Int("port", 8080, "Port to listen on (auto-increments if in use)")
		maxUploadMB = flag.Int64("max-upload-mb", 50, "Maximum CSV upload size in megabytes")
		noBrowser   = flag.Bool("no-browser", false, "Disable automatically opening the web browser on launch")
	)
	flag.Parse()

	cfg := server.Config{
		Host:            *host,
		Port:            *port,
		MaxUploadSizeMB: *maxUploadMB,
		SessionTTL:      2 * time.Hour,
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Fatal: Failed to initialize server: %v", err)
	}

	appURL := srv.Addr()
	fmt.Println("========================================================")
	fmt.Println("               CSV Diff Server Running                  ")
	fmt.Println("========================================================")
	fmt.Printf(" Local URL:           %s\n", appURL)
	fmt.Printf(" Max Upload Size:     %d MB\n", *maxUploadMB)
	fmt.Printf(" Bound Port:          %d\n", srv.Port())
	fmt.Println(" Press Ctrl+C to stop the server gracefully.")
	fmt.Println("========================================================")

	// Launch browser in background unless disabled
	if !*noBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := browser.Open(appURL); err != nil {
				log.Printf("Note: Could not open browser automatically: %v", err)
				log.Printf("Please open %s manually in your browser.", appURL)
			}
		}()
	}

	// Start server in background goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			serverErrChan <- err
		}
	}()

	// Listen for shutdown signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrChan:
		log.Fatalf("Server error: %v", err)
	case sig := <-stopChan:
		fmt.Printf("\nReceived signal %v. Initiating graceful shutdown...\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown error: %v", err)
		} else {
			fmt.Println("Server stopped cleanly. Goodbye!")
		}
	}
}
