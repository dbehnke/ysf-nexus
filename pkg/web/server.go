package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/dbehnke/ysf-nexus/pkg/bridge"
	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/dbehnke/ysf-nexus/pkg/repeater"
)

//go:embed dist
var staticFiles embed.FS

// maskIPAddress masks the last two octets of an IP address for privacy
// Example: 192.168.1.100:42000 -> 192.168.**:42000
func maskIPAddress(address string) string {
	re := regexp.MustCompile(`^(\d+\.\d+\.)\d+\.\d+(:\d+)?$`)
	return re.ReplaceAllString(address, "${1}**${2}")
}

// Server represents the web dashboard server
type Server struct {
	config          *config.Config
	logger          *logger.Logger
	httpServer      *http.Server
	repeaterManager *repeater.Manager
	bridgeManager   interface{}
	reflector       interface{}
	eventChan       <-chan repeater.Event
	talkLogs        []TalkLogEntry
	websocketHub    *WebSocketHub
	startTime       time.Time
	version         string
	buildTime       string
	mu              sync.RWMutex
	running         bool
	sessions        map[string]time.Time // session token -> expiry time
	sessionsMu      sync.RWMutex
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
	logger     *logger.Logger
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
func NewServer(cfg *config.Config, log *logger.Logger, manager *repeater.Manager, eventChan <-chan repeater.Event, bridgeManager interface{}, reflector interface{}, version, buildTime string) *Server {
	hub := &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
	// Assign logger to hub for internal logging
	hub.logger = log.WithComponent("web.hub")

	return &Server{
		config:          cfg,
		logger:          log.WithComponent("web"),
		repeaterManager: manager,
		bridgeManager:   bridgeManager,
		reflector:       reflector,
		eventChan:       eventChan,
		talkLogs:        make([]TalkLogEntry, 0),
		websocketHub:    hub,
		startTime:       time.Now(),
		version:         version,
		buildTime:       buildTime,
		sessions:        make(map[string]time.Time),
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

	// Start session cleanup if auth is enabled
	if s.config.Web.AuthRequired {
		go s.startSessionCleanup(ctx)
	}

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
	api.HandleFunc("/bridges", s.handleBridges).Methods("GET")
	api.HandleFunc("/logs/talk", s.handleTalkLogs).Methods("GET")
	api.HandleFunc("/current-talker", s.handleCurrentTalker).Methods("GET")

	// System endpoints
	api.HandleFunc("/system/info", s.handleSystemInfo).Methods("GET")

	// Authentication endpoints
	api.HandleFunc("/auth/login", s.handleLogin).Methods("POST")
	api.HandleFunc("/auth/logout", s.handleLogout).Methods("POST")
	api.HandleFunc("/auth/status", s.handleAuthStatus).Methods("GET")

	// Protected configuration endpoints
	protectedAPI := api.PathPrefix("/config").Subrouter()
	protectedAPI.Use(s.authMiddleware)
	protectedAPI.HandleFunc("/server", s.handleGetServerConfig).Methods("GET")
	protectedAPI.HandleFunc("/server", s.handleUpdateServerConfig).Methods("PUT")
	protectedAPI.HandleFunc("/blocklist", s.handleGetBlocklistConfig).Methods("GET")
	protectedAPI.HandleFunc("/blocklist", s.handleUpdateBlocklistConfig).Methods("PUT")
	protectedAPI.HandleFunc("/logging", s.handleGetLoggingConfig).Methods("GET")
	protectedAPI.HandleFunc("/logging", s.handleUpdateLoggingConfig).Methods("PUT")

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
			if err := indexFile.Close(); err != nil {
				s.logger.Warn("indexFile.Close failed", logger.Error(err))
			}
			w.Header().Set("Content-Type", "text/html")
			if _, err := io.Copy(w, indexFile); err != nil {
				// Client disconnects are common; log at debug level
				s.logger.Debug("io.Copy failed while serving index.html", logger.Error(err))
			}
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
	s.logger.Info("handleEvent ENTRY",
		logger.String("type", event.Type),
		logger.String("callsign", event.Callsign),
		logger.String("address", event.Address),
		logger.Duration("duration", event.Duration))

	defer func() {
		s.logger.Info("handleEvent EXIT",
			logger.String("type", event.Type),
			logger.String("callsign", event.Callsign))
	}()

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
			"callsign":  event.Callsign,
			"timestamp": event.Timestamp,
		})

	case repeater.EventConnect:
		s.broadcastWebSocketMessage("repeater_connect", map[string]interface{}{
			"callsign": event.Callsign,
			"address":  maskIPAddress(event.Address),
		})

	case repeater.EventDisconnect:
		s.broadcastWebSocketMessage("repeater_disconnect", map[string]interface{}{
			"callsign": event.Callsign,
			"address":  maskIPAddress(event.Address),
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
				if err := client.Close(); err != nil {
					if hub.logger != nil {
						hub.logger.Warn("failed to close websocket client", logger.Error(err))
					}
				}
			}
			hub.mu.Unlock()

		case message := <-hub.broadcast:
			hub.mu.RLock()
			for client := range hub.clients {
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					delete(hub.clients, client)
					if err := client.Close(); err != nil {
						if hub.logger != nil {
							hub.logger.Warn("failed to close websocket client", logger.Error(err))
						}
					}
				}
			}
			hub.mu.RUnlock()
		}
	}
}

// broadcastWebSocketMessage broadcasts a message to all WebSocket clients
func (s *Server) broadcastWebSocketMessage(messageType string, data interface{}) {
	s.logger.Info("broadcastWebSocketMessage ENTRY",
		logger.String("message_type", messageType),
		logger.Any("data", data))

	message := WebSocketMessage{
		Type: messageType,
		Data: data,
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		s.logger.Error("Failed to marshal WebSocket message", logger.Error(err))
		return
	}

	s.logger.Info("broadcastWebSocketMessage: attempting to send to hub",
		logger.String("message_type", messageType),
		logger.Int("broadcast_channel_len", len(s.websocketHub.broadcast)),
		logger.Int("broadcast_channel_cap", cap(s.websocketHub.broadcast)))

	select {
	case s.websocketHub.broadcast <- jsonData:
		s.logger.Info("broadcastWebSocketMessage: message sent to broadcast channel",
			logger.String("message_type", messageType))
	default:
		// Don't block if broadcast channel is full
		s.logger.Warn("WebSocket broadcast channel full, dropping message",
			logger.String("message_type", messageType))
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

// authMiddleware checks for valid authentication when auth is required
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If auth is not required, allow all requests
		if !s.config.Web.AuthRequired {
			next.ServeHTTP(w, r)
			return
		}

		// Check for session token
		token := r.Header.Get("Authorization")
		if token == "" {
			// Try cookie
			if cookie, err := r.Cookie("session_token"); err == nil {
				token = cookie.Value
			}
		} else {
			// Remove "Bearer " prefix if present
			token = strings.TrimPrefix(token, "Bearer ")
		}

		if token == "" {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		// Check if session is valid
		s.sessionsMu.RLock()
		expiry, exists := s.sessions[token]
		s.sessionsMu.RUnlock()

		if !exists || time.Now().After(expiry) {
			// Clean up expired session
			if exists {
				s.sessionsMu.Lock()
				delete(s.sessions, token)
				s.sessionsMu.Unlock()
			}
			http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// generateSessionToken generates a secure random session token
func (s *Server) generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// cleanupExpiredSessions removes expired sessions (should be called periodically)
func (s *Server) cleanupExpiredSessions() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	now := time.Now()
	for token, expiry := range s.sessions {
		if now.After(expiry) {
			delete(s.sessions, token)
		}
	}
}

// startSessionCleanup runs periodic session cleanup
func (s *Server) startSessionCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour) // Clean up every hour
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupExpiredSessions()
		}
	}
}

// API Handlers
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.repeaterManager.GetStats()

	response := map[string]interface{}{
		"uptime":           int(time.Since(s.startTime).Seconds()),
		"activeRepeaters":  stats.ActiveRepeaters,
		"totalConnections": stats.TotalConnections,
		"totalPackets":     stats.TotalPackets,
		"bytesReceived":    stats.TotalBytesReceived,
		"bytesSent":        stats.TotalBytesTransmitted,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleRepeaters(w http.ResponseWriter, r *http.Request) {
	stats := s.repeaterManager.GetStats()
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"repeaters": stats.Repeaters,
	}); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleBridges(w http.ResponseWriter, r *http.Request) {
	bridges := make(map[string]interface{})

	// Check if bridge manager is available and has GetStatus method
	if s.bridgeManager != nil {
		// Use type assertion to check if it has GetStatus method with correct return type
		if bm, ok := s.bridgeManager.(interface {
			GetStatus() map[string]bridge.BridgeStatus
		}); ok {
			// Run GetStatus with a short timeout to avoid blocking the HTTP handler
			type result struct {
				status map[string]bridge.BridgeStatus
			}
			ch := make(chan result, 1)

			go func() {
				ch <- result{status: bm.GetStatus()}
			}()

			select {
			case res := <-ch:
				for name, bridgeStatus := range res.status {
					bridges[name] = bridgeStatus
				}
			case <-time.After(500 * time.Millisecond):
				s.logger.Warn("bridge manager GetStatus timed out, returning partial/empty result")
				// leave bridges empty
			}
		}
	}

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"bridges": bridges,
	}); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleCurrentTalker(w http.ResponseWriter, r *http.Request) {
	// First check for regular repeater talkers
	stats := s.repeaterManager.GetStats()
	for _, repeater := range stats.Repeaters {
		if repeater.IsTalking {
			// Found a regular repeater that's talking
			response := map[string]interface{}{
				"current_talker": map[string]interface{}{
					"callsign":      repeater.Callsign,
					"address":       repeater.Address,
					"type":          "repeater",
					"is_talking":    true,
					"talk_duration": repeater.TalkDuration,
				},
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				s.logger.Error("failed to encode JSON response", logger.Error(err))
			}
			return
		}
	}

	// No regular repeater talking, check for bridge talkers
	if s.reflector != nil {
		if refl, ok := s.reflector.(interface{ GetCurrentBridgeTalker() interface{} }); ok {
			// Protect reflector call with a short timeout so slow reflector doesn't hang HTTP
			type reflResult struct {
				talker interface{}
			}
			ch := make(chan reflResult, 1)

			go func() {
				ch <- reflResult{talker: refl.GetCurrentBridgeTalker()}
			}()

			select {
			case res := <-ch:
				bridgeTalker := res.talker
				if bridgeTalker != nil {
					// Use type assertion to extract bridge talker information
					if bt, ok := bridgeTalker.(interface {
						GetCallsign() string
						GetBridgeName() string
						GetTalkDuration() time.Duration
					}); ok {
						response := map[string]interface{}{
							"current_talker": map[string]interface{}{
								"callsign":      bt.GetCallsign(),
								"address":       bt.GetBridgeName(), // Show bridge name as "address"
								"type":          "bridge",
								"is_talking":    true,
								"talk_duration": int(bt.GetTalkDuration().Seconds()),
							},
						}
						if err := json.NewEncoder(w).Encode(response); err != nil {
							s.logger.Error("failed to encode JSON response", logger.Error(err))
						}
						return
					}
				}
			case <-time.After(500 * time.Millisecond):
				s.logger.Warn("reflector GetCurrentBridgeTalker timed out, returning null")
				// fall through to return null
			}
		}
	}

	// No one is talking
	response := map[string]interface{}{
		"current_talker": nil,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
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

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	}); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"name":           s.config.Server.Name,
		"description":    s.config.Server.Description,
		"version":        s.version,
		"buildTime":      s.buildTime,
		"host":           s.config.Server.Host,
		"port":           s.config.Server.Port,
		"maxConnections": s.config.Server.MaxConnections,
		"timeout":        s.config.Server.Timeout.String(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
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
		"name":           s.config.Server.Name,
		"description":    s.config.Server.Description,
		"maxConnections": s.config.Server.MaxConnections,
		"timeoutMinutes": int(s.config.Server.Timeout.Minutes()),
	}
	if err := json.NewEncoder(w).Encode(config); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleUpdateServerConfig(w http.ResponseWriter, r *http.Request) {
	// This would update the server configuration
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "not implemented"}); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleGetBlocklistConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"enabled":   s.config.Blocklist.Enabled,
		"callsigns": s.config.Blocklist.Callsigns,
	}
	if err := json.NewEncoder(w).Encode(config); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleUpdateBlocklistConfig(w http.ResponseWriter, r *http.Request) {
	// This would update the blocklist configuration
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "not implemented"}); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleGetLoggingConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"level":   s.config.Logging.Level,
		"format":  s.config.Logging.Format,
		"file":    s.config.Logging.File,
		"maxSize": s.config.Logging.MaxSize,
	}
	if err := json.NewEncoder(w).Encode(config); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleUpdateLoggingConfig(w http.ResponseWriter, r *http.Request) {
	// This would update the logging configuration
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "not implemented"}); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

// Authentication handlers
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// If auth is not required, deny login attempts
	if !s.config.Web.AuthRequired {
		http.Error(w, "Authentication not configured", http.StatusBadRequest)
		return
	}

	var loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&loginRequest); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Use constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare([]byte(loginRequest.Username), []byte(s.config.Web.Username)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(loginRequest.Password), []byte(s.config.Web.Password)) == 1

	if !usernameMatch || !passwordMatch {
		s.logger.Warn("Failed login attempt", logger.String("username", loginRequest.Username), logger.String("remote_addr", r.RemoteAddr))
		time.Sleep(time.Second) // Add delay to slow down brute force attacks
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate session token
	token, err := s.generateSessionToken()
	if err != nil {
		s.logger.Error("Failed to generate session token", logger.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store session with 24-hour expiry
	expiry := time.Now().Add(24 * time.Hour)
	s.sessionsMu.Lock()
	s.sessions[token] = expiry
	s.sessionsMu.Unlock()

	s.logger.Info("Successful login", logger.String("username", loginRequest.Username), logger.String("remote_addr", r.RemoteAddr))

	// Set cookie and return token
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Expires:  expiry,
		HttpOnly: true,
		Secure:   r.TLS != nil, // Only secure if HTTPS
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	})

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"expires": expiry.Format(time.RFC3339),
	}); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Get token from header or cookie
	token := r.Header.Get("Authorization")
	if token == "" {
		if cookie, err := r.Cookie("session_token"); err == nil {
			token = cookie.Value
		}
	} else {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	if token != "" {
		// Remove session
		s.sessionsMu.Lock()
		delete(s.sessions, token)
		s.sessionsMu.Unlock()

		// Clear cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    "",
			Expires:  time.Now().Add(-time.Hour),
			HttpOnly: true,
			Path:     "/",
		})
	}

	if err := json.NewEncoder(w).Encode(map[string]string{"success": "logged out"}); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"auth_required": s.config.Web.AuthRequired,
		"authenticated": false,
	}

	if s.config.Web.AuthRequired {
		// Check if currently authenticated
		token := r.Header.Get("Authorization")
		if token == "" {
			if cookie, err := r.Cookie("session_token"); err == nil {
				token = cookie.Value
			}
		} else {
			token = strings.TrimPrefix(token, "Bearer ")
		}

		if token != "" {
			s.sessionsMu.RLock()
			expiry, exists := s.sessions[token]
			s.sessionsMu.RUnlock()

			if exists && time.Now().Before(expiry) {
				response["authenticated"] = true
				response["expires"] = expiry.Format(time.RFC3339)
			}
		}
	} else {
		// If auth not required, consider always authenticated
		response["authenticated"] = true
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode JSON response", logger.Error(err))
	}
}
