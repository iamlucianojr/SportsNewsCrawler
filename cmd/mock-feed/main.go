package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	http.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"id":        "101",
					"headline":  "Dummy News 1",
					"content":   "This is a dummy article content from the mock server.",
					"timestamp": time.Now().Format(time.RFC3339),
				},
				{
					"id":        "102",
					"headline":  "Dummy News 2",
					"content":   "Another dummy article.",
					"timestamp": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
				},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Failed to encode response", "error", err)
		}
	})

	slog.Info("Mock feed server running on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
