package http

import (
	"fmt"
	"net/http"

	"github.com/SportsNewsCrawler/pkg/config"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewHTTPServer(cfg *config.Config) *http.Server {
	r := mux.NewRouter()
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintf(w, "OK"); err != nil {
			// Log error but don't fail health check
			_ = err
		}
	}).Methods("GET")
	r.Handle("/metrics", promhttp.Handler())

	return &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}
}
