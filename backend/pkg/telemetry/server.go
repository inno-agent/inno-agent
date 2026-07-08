package telemetry

import (
	"log"
	"net/http"
	"time"
)

// ListenAndServe starts a dedicated metrics HTTP server (for workers without a public API).
func ListenAndServe(addr, serviceName string) {
	Init(serviceName)
	mux := http.NewServeMux()
	mux.Handle("/metrics", Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	go func() {
		log.Printf("metrics listening on %s", addr)
		server := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil {
			log.Printf("metrics server error: %v", err)
		}
	}()
}
