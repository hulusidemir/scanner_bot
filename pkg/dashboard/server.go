package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"scanner_bot/pkg/tracker"
)

//go:embed static
var staticFiles embed.FS

type Server struct {
	store *tracker.Store
	port  int
}

func NewServer(store *tracker.Store, port int) *Server {
	return &Server{store: store, port: port}
}

func (s *Server) Start() {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/trades", s.handleTrades)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/active", s.handleActive)

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to setup static files: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("[Dashboard] Starting on http://localhost%s", addr)

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("Dashboard server failed: %v", err)
		}
	}()
}

func (s *Server) handleTrades(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	filter := r.URL.Query().Get("filter")

	var trades interface{}
	var err error

	switch filter {
	case "active":
		trades, err = s.store.GetActiveTrades()
	case "closed":
		trades, err = s.store.GetClosedTrades()
	default:
		trades, err = s.store.GetAllTrades()
	}

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(trades)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	stats, err := s.store.GetStats()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleActive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	trades, err := s.store.GetActiveTrades()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	json.NewEncoder(w).Encode(trades)
}
