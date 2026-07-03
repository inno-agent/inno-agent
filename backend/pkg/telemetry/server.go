package telemetry

import (
	"log"
	"net/http"
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
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("metrics server error: %v", err)
		}
	}()
}
