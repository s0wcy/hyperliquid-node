package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"hyperliquid-ws-proxy/client"
	"hyperliquid-ws-proxy/config"
	"hyperliquid-ws-proxy/proxy"
)

// Server represents the HTTP server
type Server struct {
	config *config.Config
	proxy  *proxy.Proxy
	server *http.Server
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, p *proxy.Proxy) *Server {
	return &Server{
		config: cfg,
		proxy:  p,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	
	// WebSocket endpoint (matches Hyperliquid's /ws path)
	mux.HandleFunc("/ws", s.handleWebSocket)
	
	// Health check endpoint
	mux.HandleFunc("/health", s.handleHealth)
	
	// Statistics endpoint
	mux.HandleFunc("/stats", s.handleStats)
	
	// Proxy info endpoint
	mux.HandleFunc("/info", s.handleInfo)
	
	// Assets endpoint
	mux.HandleFunc("/assets", s.handleAssets)
	
	// CORS middleware for web clients
	handler := s.corsMiddleware(mux)
	
	s.server = &http.Server{
		Addr:         s.config.GetServerAddress(),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	
	logrus.WithField("address", s.config.GetServerAddress()).Info("Starting HTTP server")
	
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed to start: %v", err)
	}
	
	return nil
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	if s.server != nil {
		logrus.Info("Stopping HTTP server")
		return s.server.Close()
	}
	return nil
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Log connection details
	logrus.WithFields(logrus.Fields{
		"remote_addr": r.RemoteAddr,
		"user_agent":  r.Header.Get("User-Agent"),
		"origin":      r.Header.Get("Origin"),
	}).Info("New WebSocket connection")
	
	// Check client limits
	if s.proxy.GetHub().GetClientCount() >= s.config.Proxy.MaxClients {
		http.Error(w, "Too many clients connected", http.StatusTooManyRequests)
		return
	}
	
	// Upgrade to WebSocket
	client.ServeWS(s.proxy.GetHub(), w, r)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(s.proxy.GetStats().StartTime).Seconds(),
		"version":   "1.0.0",
	}
	
	json.NewEncoder(w).Encode(health)
}

// handleStats handles statistics requests
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	stats := s.proxy.GetStats()
	
	response := map[string]interface{}{
		"connected_clients":      stats.ConnectedClients,
		"active_subscriptions":   stats.ActiveSubscriptions,
		"messages_processed":     stats.MessagesProcessed,
		"messages_forwarded":     stats.MessagesForwarded,
		"post_requests_handled":  stats.PostRequestsHandled,
		"last_activity":          stats.LastActivity.Unix(),
		"start_time":             stats.StartTime.Unix(),
		"uptime_seconds":         time.Since(stats.StartTime).Seconds(),
	}
	
	json.NewEncoder(w).Encode(response)
}

// handleInfo handles proxy information requests
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	info := map[string]interface{}{
		"name":        "Hyperliquid WebSocket Proxy",
		"version":     "1.0.0",
		"description": "A WebSocket proxy for Hyperliquid API without rate limits",
		"endpoints": map[string]string{
			"websocket":   "/ws",
			"health":      "/health",
			"stats":       "/stats",
			"info":        "/info",
			"assets":      "/assets",
		},
		"supported_subscriptions": []string{
			"allMids", "l2Book", "trades", "candle", "bbo",
			"notification", "webData2", "orderUpdates", "userEvents",
			"userFills", "userFundings", "userNonFundingLedgerUpdates",
			"activeAssetCtx", "activeAssetData", "userTwapSliceFills",
			"userTwapHistory",
		},
		"features": []string{
			"Real-time WebSocket proxy",
			"No rate limits",
			"Multiple client support",
			"Automatic reconnection",
			"Local node integration",
			"POST request support",
		},
		"config": map[string]interface{}{
			"network":            s.config.Hyperliquid.Network,
			"max_clients":        s.config.Proxy.MaxClients,
			"enable_heartbeat":   s.config.Proxy.EnableHeartbeat,
			"enable_local_node":  s.config.Proxy.EnableLocalNode,
		},
	}
	
	json.NewEncoder(w).Encode(info)
}

// handleAssets handles asset listing requests
func (s *Server) handleAssets(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	// Get asset statistics and all asset names
	stats := s.proxy.GetAssetStats()
	allAssets := s.proxy.GetAllAssetNames()
	
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"statistics": stats,
			"assets":     allAssets,
		},
		"timestamp": time.Now().Unix(),
	}
	
	json.NewEncoder(w).Encode(response)
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow all origins for WebSocket connections (adjust for production)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		
		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// logMiddleware logs HTTP requests
func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		duration := time.Since(start)
		
		logrus.WithFields(logrus.Fields{
			"method":      r.Method,
			"url":         r.URL.Path,
			"status":      wrapped.statusCode,
			"duration":    duration,
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.Header.Get("User-Agent"),
		}).Info("HTTP request")
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
} 