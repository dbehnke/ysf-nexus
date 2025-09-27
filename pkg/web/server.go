package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/dbehnke/ysf-nexus/pkg/repeater"
)

//go:embed dist
var staticFiles embed.FS

// Server represents the web dashboard server
type Server struct {
	config          *config.Config
	logger          *logger.Logger
	httpServer      *http.Server
	repeaterManager *repeater.Manager
	eventChan       <-chan repeater.Event
	talkLogs        []TalkLogEntry
	websocketHub    *WebSocketHub
	mu              sync.RWMutex
	running         bool
}

// TalkLogEntry represents a talk log entry
type TalkLogEntry struct {
	ID        int64     `json:"id"`
	Callsign  string    `json:"callsign"`
	Duration  int       `json:"duration"` // in seconds
	Timestamp time.Time `json:"timestamp"`
}

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// NewServer creates a new web server
func NewServer(cfg *config.Config, log *logger.Logger, manager *repeater.Manager, eventChan <-chan repeater.Event) *Server {
	hub := &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}

	return &Server{
		config:          cfg,
		logger:          log.WithComponent("web"),
		repeaterManager: manager,
		eventChan:       eventChan,
		talkLogs:        make([]TalkLogEntry, 0),
		websocketHub:    hub,
	}
}

// Start starts the web server
func (s *Server) Start(ctx context.Context) error {
	if !s.config.Web.Enabled {
		s.logger.Info("Web server disabled")
		return nil
	}

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("web server already running")
	}
	s.running = true
	s.mu.Unlock()

	// Start WebSocket hub
	go s.websocketHub.run()

	// Start event processor
	go s.processEvents(ctx)

	// Setup routes
	router := s.setupRoutes()

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", s.config.Web.Host, s.config.Web.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: router,
	}

	s.logger.Info("Starting web server", logger.String("address", addr))

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		s.logger.Info("Shutting down web server")
		return s.Stop()
	}
}

// Stop stops the web server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

// setupRoutes configures HTTP routes
func (s *Server) setupRoutes() *mux.Router {
	router := mux.NewRouter()

	// API routes
	api := router.PathPrefix("/api").Subrouter()
	api.Use(s.corsMiddleware)
	api.Use(s.jsonMiddleware)

	// Stats endpoints
	api.HandleFunc("/stats", s.handleStats).Methods("GET")
	api.HandleFunc("/repeaters", s.handleRepeaters).Methods("GET")
	api.HandleFunc("/logs/talk", s.handleTalkLogs).Methods("GET")

	// System endpoints
	api.HandleFunc("/system/info", s.handleSystemInfo).Methods("GET")

	// Configuration endpoints (placeholder)
	api.HandleFunc("/config/server", s.handleGetServerConfig).Methods("GET")
	api.HandleFunc("/config/server", s.handleUpdateServerConfig).Methods("PUT")
	api.HandleFunc("/config/blocklist", s.handleGetBlocklistConfig).Methods("GET")
	api.HandleFunc("/config/blocklist", s.handleUpdateBlocklistConfig).Methods("PUT")
	api.HandleFunc("/config/logging", s.handleGetLoggingConfig).Methods("GET")
	api.HandleFunc("/config/logging", s.handleUpdateLoggingConfig).Methods("PUT")

	// Health check
	api.HandleFunc("/health", s.handleHealth).Methods("GET")

	// WebSocket endpoint
	router.HandleFunc("/ws", s.handleWebSocket)

	// Static files (embedded frontend)
	s.setupStaticRoutes(router)

	return router
}

// setupStaticRoutes configures static file serving
func (s *Server) setupStaticRoutes(router *mux.Router) {
	// Extract the embedded filesystem
	distFS, err := fs.Sub(staticFiles, "dist")
	if err != nil {
		s.logger.Error("Failed to setup static files", logger.Error(err))
		// Fallback to basic handler
		router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Frontend not available", http.StatusNotFound)
		})
		return
	}

	// Serve static files
	fileServer := http.FileServer(http.FS(distFS))

	// Handle SPA routing
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file
		if r.URL.Path != "/" {
			// Check if file exists
			if _, err := distFS.Open(r.URL.Path[1:]); err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fallback to index.html for SPA routing
		if indexFile, err := distFS.Open("index.html"); err == nil {
			defer indexFile.Close()
			w.Header().Set("Content-Type", "text/html")
			io.Copy(w, indexFile)
		} else {
			http.Error(w, "Frontend not available", http.StatusNotFound)
		}
	})
}

// processEvents processes repeater events and broadcasts them via WebSocket
func (s *Server) processEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.eventChan:
			s.handleEvent(event)
		}
	}
}

// handleEvent processes a repeater event
func (s *Server) handleEvent(event repeater.Event) {
	switch event.Type {
	case repeater.EventTalkEnd:
		// Add to talk logs
		s.mu.Lock()
		entry := TalkLogEntry{
			ID:        time.Now().UnixNano(),
			Callsign:  event.Callsign,
			Duration:  int(event.Duration.Seconds()),
			Timestamp: event.Timestamp,
		}
		s.talkLogs = append([]TalkLogEntry{entry}, s.talkLogs...)

		// Keep only last 1000 entries
		if len(s.talkLogs) > 1000 {
			s.talkLogs = s.talkLogs[:1000]
		}
		s.mu.Unlock()

		// Broadcast via WebSocket
		s.broadcastWebSocketMessage("talk_end", map[string]interface{}{
			"callsign": event.Callsign,
			"duration": int(event.Duration.Seconds()),
		})

	case repeater.EventTalkStart:
		s.broadcastWebSocketMessage("talk_start", map[string]interface{}{
			"callsign": event.Callsign,
		})

	case repeater.EventConnect:
		s.broadcastWebSocketMessage("repeater_connect", map[string]interface{}{
			"callsign": event.Callsign,
			"address":  event.Address,
		})

	case repeater.EventDisconnect:
		s.broadcastWebSocketMessage("repeater_disconnect", map[string]interface{}{
			"callsign": event.Callsign,
			"address":  event.Address,
		})
	}

	// Always broadcast the raw event
	s.broadcastWebSocketMessage("event", event)
}

// WebSocket hub run loop
func (hub *WebSocketHub) run() {
	for {
		select {
		case client := <-hub.register:
			hub.mu.Lock()
			hub.clients[client] = true
			hub.mu.Unlock()

		case client := <-hub.unregister:
			hub.mu.Lock()
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				client.Close()
			}
			hub.mu.Unlock()

		case message := <-hub.broadcast:
			hub.mu.RLock()
			for client := range hub.clients {
				select {
				case client := <-hub.unregister:
					delete(hub.clients, client)
					client.Close()
				default:
					if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
						delete(hub.clients, client)
						client.Close()
					}
				}
			}
			hub.mu.RUnlock()
		}
	}
}

// broadcastWebSocketMessage broadcasts a message to all WebSocket clients
func (s *Server) broadcastWebSocketMessage(messageType string, data interface{}) {
	message := WebSocketMessage{
		Type: messageType,
		Data: data,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		s.logger.Error("Failed to marshal WebSocket message", logger.Error(err))
		return
	}

	select {
	case s.websocketHub.broadcast <- jsonData:
	default:
		// Don't block if broadcast channel is full
		s.logger.Warn("WebSocket broadcast channel full, dropping message")
	}
}

// Middleware
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// API Handlers
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.repeaterManager.GetStats()

	response := map[string]interface{}{
		"uptime":            time.Since(time.Now().Add(-24*time.Hour)).Seconds(), // Placeholder
		"activeRepeaters":   stats.ActiveRepeaters,
		"totalConnections":  stats.TotalConnections,
		"totalPackets":      stats.TotalPackets,
		"bytesReceived":     stats.TotalBytesReceived,
		"bytesSent":         stats.TotalBytesTransmitted,
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleRepeaters(w http.ResponseWriter, r *http.Request) {
	stats := s.repeaterManager.GetStats()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"repeaters": stats.Repeaters,
	})
}

func (s *Server) handleTalkLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // default

	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	s.mu.RLock()
	logs := s.talkLogs
	if len(logs) > limit {
		logs = logs[:limit]
	}
	s.mu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	})
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"version":        "dev", // This would come from build info
		"buildTime":      "unknown",
		"host":           s.config.Server.Host,
		"port":           s.config.Server.Port,
		"maxConnections": s.config.Server.MaxConnections,
		"timeout":        s.config.Server.Timeout.String(),
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", logger.Error(err))
		return
	}

	s.logger.Debug("New WebSocket connection", logger.String("remote", r.RemoteAddr))

	// Register client
	s.websocketHub.register <- conn

	// Handle client disconnect
	defer func() {
		s.websocketHub.unregister <- conn
	}()

	// Send initial data
	s.sendInitialData(conn)

	// Keep connection alive and handle client messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error("WebSocket error", logger.Error(err))
			}
			break
		}
		// Handle client messages if needed
	}
}

func (s *Server) sendInitialData(conn *websocket.Conn) {
	// Send current stats
	stats := s.repeaterManager.GetStats()
	s.sendWebSocketMessage(conn, "stats_update", map[string]interface{}{
		"activeRepeaters": stats.ActiveRepeaters,
		"totalPackets":    stats.TotalPackets,
	})

	// Send current repeaters
	s.sendWebSocketMessage(conn, "repeaters_update", map[string]interface{}{
		"repeaters": stats.Repeaters,
	})
}

func (s *Server) sendWebSocketMessage(conn *websocket.Conn, messageType string, data interface{}) {
	message := WebSocketMessage{
		Type: messageType,
		Data: data,
	}

	if err := conn.WriteJSON(message); err != nil {
		s.logger.Error("Failed to send WebSocket message", logger.Error(err))
	}
}

// Configuration handlers (placeholder implementations)
func (s *Server) handleGetServerConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"name":              s.config.Server.Name,
		"description":       s.config.Server.Description,
		"maxConnections":    s.config.Server.MaxConnections,
		"timeoutMinutes":    int(s.config.Server.Timeout.Minutes()),
	}
	json.NewEncoder(w).Encode(config)
}

func (s *Server) handleUpdateServerConfig(w http.ResponseWriter, r *http.Request) {
	// This would update the server configuration
	json.NewEncoder(w).Encode(map[string]string{"status": "not implemented"})
}

func (s *Server) handleGetBlocklistConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"enabled":   s.config.Blocklist.Enabled,
		"callsigns": s.config.Blocklist.Callsigns,
	}
	json.NewEncoder(w).Encode(config)
}

func (s *Server) handleUpdateBlocklistConfig(w http.ResponseWriter, r *http.Request) {
	// This would update the blocklist configuration
	json.NewEncoder(w).Encode(map[string]string{"status": "not implemented"})
}

func (s *Server) handleGetLoggingConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"level":   s.config.Logging.Level,
		"format":  s.config.Logging.Format,
		"file":    s.config.Logging.File,
		"maxSize": s.config.Logging.MaxSize,
	}
	json.NewEncoder(w).Encode(config)
}

func (s *Server) handleUpdateLoggingConfig(w http.ResponseWriter, r *http.Request) {
	// This would update the logging configuration
	json.NewEncoder(w).Encode(map[string]string{"status": "not implemented"})
}