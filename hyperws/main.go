package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

const (
	appName    = "HyperWS"
	appVersion = "1.0.0"
)

// Mise à niveau WebSocket
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Autoriser toutes les origines
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client WebSocket
type Client struct {
	ID           string
	conn         *websocket.Conn
	send         chan []byte
	subscriptions map[string]*SubscriptionRequest
	mu           sync.RWMutex
	hub          *Hub
}

// Hub gère tous les clients connectés
type Hub struct {
	clients       map[*Client]bool
	register      chan *Client
	unregister    chan *Client
	broadcast     chan []byte
	subscriptions map[string]map[*Client]bool // subscription_key -> clients
	mu            sync.RWMutex
}

// HyperWS - Serveur principal
type HyperWS struct {
	config     *Config
	hub        *Hub
	nodeReader *LocalNodeReader
	server     *http.Server
}

// NewClient crée un nouveau client
func NewClient(conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID:            fmt.Sprintf("client_%d", time.Now().UnixNano()),
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]*SubscriptionRequest),
		hub:           hub,
	}
}

// NewHub crée un nouveau hub
func NewHub() *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		broadcast:     make(chan []byte),
		subscriptions: make(map[string]map[*Client]bool),
	}
}

// Run démarre le hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			logrus.WithField("client_id", client.ID).Info("Client connecté")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				
				// Supprimer le client de toutes les souscriptions
				for key, clients := range h.subscriptions {
					if clients[client] {
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.subscriptions, key)
						}
					}
				}
			}
			h.mu.Unlock()
			logrus.WithField("client_id", client.ID).Info("Client déconnecté")

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// GetClientCount retourne le nombre de clients connectés
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// readPump lit les messages du client
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.WithError(err).Error("Erreur WebSocket")
			}
			break
		}

		c.handleMessage(message)
	}
}

// writePump écrit les messages vers le client
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Ajouter les messages en attente
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage traite un message du client
func (c *Client) handleMessage(data []byte) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logrus.WithError(err).Error("Message invalide")
		c.sendError("Format de message invalide")
		return
	}

	switch msg.Method {
	case "subscribe":
		c.handleSubscribe(msg.Subscription)
	case "unsubscribe":
		c.handleUnsubscribe(msg.Subscription)
	default:
		c.sendError("Méthode inconnue: " + msg.Method)
	}
}

// handleSubscribe traite une souscription
func (c *Client) handleSubscribe(sub *SubscriptionRequest) {
	if sub == nil {
		c.sendError("Détails de souscription manquants")
		return
	}

	logrus.WithFields(logrus.Fields{
		"client_id": c.ID,
		"type":      sub.Type,
		"coin":      sub.Coin,
	}).Info("Nouvelle souscription")

	// Créer la clé de souscription
	key := c.createSubscriptionKey(sub)

	// Ajouter aux souscriptions du client
	c.mu.Lock()
	c.subscriptions[key] = sub
	c.mu.Unlock()

	// Ajouter aux souscriptions globales
	c.hub.mu.Lock()
	if c.hub.subscriptions[key] == nil {
		c.hub.subscriptions[key] = make(map[*Client]bool)
	}
	c.hub.subscriptions[key][c] = true
	c.hub.mu.Unlock()

	// Envoyer confirmation
	response := WSMessage{
		Channel: "subscriptionResponse",
		Data:    json.RawMessage(fmt.Sprintf(`{"method":"subscribe","subscription":%s}`, c.toJSON(sub))),
	}
	c.sendMessage(response)

	// Envoyer données initiales selon le type
	c.sendInitialData(sub)
}

// handleUnsubscribe traite une désouscription
func (c *Client) handleUnsubscribe(sub *SubscriptionRequest) {
	if sub == nil {
		c.sendError("Détails de souscription manquants")
		return
	}

	key := c.createSubscriptionKey(sub)

	// Supprimer des souscriptions du client
	c.mu.Lock()
	delete(c.subscriptions, key)
	c.mu.Unlock()

	// Supprimer des souscriptions globales
	c.hub.mu.Lock()
	if clients := c.hub.subscriptions[key]; clients != nil {
		delete(clients, c)
		if len(clients) == 0 {
			delete(c.hub.subscriptions, key)
		}
	}
	c.hub.mu.Unlock()

	// Envoyer confirmation
	response := WSMessage{
		Channel: "subscriptionResponse",
		Data:    json.RawMessage(fmt.Sprintf(`{"method":"unsubscribe","subscription":%s}`, c.toJSON(sub))),
	}
	c.sendMessage(response)
}

// sendInitialData envoie les données initiales selon le type de souscription
func (c *Client) sendInitialData(sub *SubscriptionRequest) {
	switch sub.Type {
	case AllMidsType:
		prices := hyperWS.nodeReader.GetAllPrices()
		if len(prices) > 0 {
			allMids := AllMids{Mids: prices}
			data, _ := json.Marshal(allMids)
			msg := WSMessage{
				Channel: AllMidsType,
				Data:    data,
			}
			c.sendMessage(msg)
		}

	case TradesType:
		if sub.Coin != "" {
			trades := hyperWS.nodeReader.GetLatestTrades(sub.Coin, 5)
			for _, trade := range trades {
				data, _ := json.Marshal(trade)
				msg := WSMessage{
					Channel: TradesType,
					Data:    data,
				}
				c.sendMessage(msg)
			}
		}
	}
}

// createSubscriptionKey crée une clé unique pour la souscription
func (c *Client) createSubscriptionKey(sub *SubscriptionRequest) string {
	key := sub.Type
	if sub.User != "" {
		key += "-" + sub.User
	}
	if sub.Coin != "" {
		key += "-" + sub.Coin
	}
	if sub.Interval != "" {
		key += "-" + sub.Interval
	}
	return key
}

// sendMessage envoie un message au client
func (c *Client) sendMessage(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		logrus.WithError(err).Error("Erreur marshalling message")
		return
	}

	select {
	case c.send <- data:
	default:
		close(c.send)
	}
}

// sendError envoie un message d'erreur
func (c *Client) sendError(errorMsg string) {
	response := map[string]interface{}{
		"error": errorMsg,
		"time":  time.Now().Unix(),
	}
	c.sendMessage(response)
}

// toJSON convertit un objet en JSON string
func (c *Client) toJSON(obj interface{}) string {
	data, err := json.Marshal(obj)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// Variable globale pour l'instance principale
var hyperWS *HyperWS

// NewHyperWS crée une nouvelle instance du serveur
func NewHyperWS(config *Config) *HyperWS {
	return &HyperWS{
		config:     config,
		hub:        NewHub(),
		nodeReader: NewLocalNodeReader(config.Node.DataPath),
	}
}

// Start démarre le serveur
func (hw *HyperWS) Start() error {
	// Démarrer le hub
	go hw.hub.Run()

	// Démarrer le lecteur de nœud
	if err := hw.nodeReader.Start(); err != nil {
		return fmt.Errorf("erreur démarrage lecteur nœud: %v", err)
	}

	// Démarrer la génération de données périodique
	go hw.generatePeriodicData()

	// Configuration du serveur HTTP
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hw.handleWebSocket)
	mux.HandleFunc("/health", hw.handleHealth)
	mux.HandleFunc("/stats", hw.handleStats)

	hw.server = &http.Server{
		Addr:    hw.config.GetServerAddress(),
		Handler: mux,
	}

	logrus.WithField("address", hw.config.GetServerAddress()).Info("Serveur WebSocket démarré")
	return hw.server.ListenAndServe()
}

// handleWebSocket traite les connexions WebSocket
func (hw *HyperWS) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.WithError(err).Error("Erreur upgrade WebSocket")
		return
	}

	client := NewClient(conn, hw.hub)
	hw.hub.register <- client

	go client.writePump()
	go client.readPump()
}

// handleHealth endpoint de santé
func (hw *HyperWS) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":           "healthy",
		"timestamp":        time.Now().Unix(),
		"clients":          hw.hub.GetClientCount(),
		"node_running":     hw.nodeReader.IsRunning(),
		"subscriptions":    len(hw.hub.subscriptions),
		"version":          appVersion,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleStats endpoint de statistiques
func (hw *HyperWS) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"server": map[string]interface{}{
			"name":    appName,
			"version": appVersion,
			"uptime":  time.Since(time.Now()).Seconds(), // TODO: tracker le vrai uptime
		},
		"websocket": map[string]interface{}{
			"connected_clients":    hw.hub.GetClientCount(),
			"active_subscriptions": len(hw.hub.subscriptions),
		},
		"node": hw.nodeReader.GetStats(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// generatePeriodicData génère des données périodiques pour les souscriptions
func (hw *HyperWS) generatePeriodicData() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !hw.nodeReader.IsRunning() {
				continue
			}

			// Générer allMids si des clients sont souscrits
			hw.generateAllMids()
			
			// Générer des trades pour les coins avec souscriptions
			hw.generateTrades()
		}
	}
}

// generateAllMids génère les données allMids
func (hw *HyperWS) generateAllMids() {
	hw.hub.mu.RLock()
	hasAllMidsSubscribers := false
	for key := range hw.hub.subscriptions {
		if strings.HasPrefix(key, AllMidsType) {
			hasAllMidsSubscribers = true
			break
		}
	}
	hw.hub.mu.RUnlock()

	if !hasAllMidsSubscribers {
		return
	}

	prices := hw.nodeReader.GetAllPrices()
	if len(prices) == 0 {
		return
	}

	allMids := AllMids{Mids: prices}
	messageData := map[string]interface{}{
		"channel": AllMidsType,
		"data":    allMids,
	}

	data, err := json.Marshal(messageData)
	if err != nil {
		return
	}

	// Envoyer aux clients souscrits à allMids
	hw.hub.mu.RLock()
	for key, clients := range hw.hub.subscriptions {
		if strings.HasPrefix(key, AllMidsType) {
			for client := range clients {
				select {
				case client.send <- data:
				default:
					// Client déconnecté
				}
			}
		}
	}
	hw.hub.mu.RUnlock()
}

// generateTrades génère les données de trades
func (hw *HyperWS) generateTrades() {
	hw.hub.mu.RLock()
	tradesSubscriptions := make(map[string]map[*Client]bool)
	for key, clients := range hw.hub.subscriptions {
		if strings.HasPrefix(key, TradesType+"-") {
			coin := strings.TrimPrefix(key, TradesType+"-")
			tradesSubscriptions[coin] = clients
		}
	}
	hw.hub.mu.RUnlock()

	// Générer des trades pour chaque coin avec souscriptions
	for coin, clients := range tradesSubscriptions {
		trades := hw.nodeReader.GetLatestTrades(coin, 1)
		if len(trades) == 0 {
			continue
		}

		latestTrade := trades[len(trades)-1]
		messageData := map[string]interface{}{
			"channel": TradesType,
			"data":    latestTrade,
		}

		data, err := json.Marshal(messageData)
		if err != nil {
			continue
		}

		// Envoyer aux clients souscrits
		for client := range clients {
			select {
			case client.send <- data:
			default:
				// Client déconnecté
			}
		}
	}
}

// Stop arrête le serveur
func (hw *HyperWS) Stop() error {
	if hw.nodeReader != nil {
		hw.nodeReader.Stop()
	}

	if hw.server != nil {
		return hw.server.Close()
	}

	return nil
}

// setupLogging configure le système de logs
func setupLogging(level, format string) {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		logrus.WithError(err).Warn("Niveau de log invalide, utilisation de 'info'")
		lvl = logrus.InfoLevel
	}
	logrus.SetLevel(lvl)

	switch format {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	default:
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	}

	logrus.SetOutput(os.Stdout)
}

// main fonction principale
func main() {
	// Flags de ligne de commande
	var (
		configPath = flag.String("config", "config.yaml", "Chemin vers le fichier de configuration")
		logLevel   = flag.String("log-level", "", "Niveau de log (debug, info, warn, error)")
		version    = flag.Bool("version", false, "Afficher les informations de version")
	)
	flag.Parse()

	// Afficher la version
	if *version {
		fmt.Printf("%s v%s\n", appName, appVersion)
		fmt.Println("Proxy WebSocket optimisé pour Hyperliquid")
		os.Exit(0)
	}

	// Charger la configuration
	config, err := LoadConfig(*configPath)
	if err != nil {
		logrus.WithError(err).Fatal("Erreur chargement configuration")
	}

	// Remplacer le niveau de log si spécifié
	if *logLevel != "" {
		config.Logging.Level = *logLevel
	}

	// Valider la configuration
	if err := config.Validate(); err != nil {
		logrus.WithError(err).Fatal("Configuration invalide")
	}

	// Configuration des logs
	setupLogging(config.Logging.Level, config.Logging.Format)

	logrus.WithFields(logrus.Fields{
		"app":         appName,
		"version":     appVersion,
		"server_addr": config.GetServerAddress(),
		"node_path":   config.Node.DataPath,
	}).Info("Démarrage de l'application")

	// Créer et démarrer le serveur
	hyperWS = NewHyperWS(config)

	// Démarrer le serveur dans une goroutine
	go func() {
		if err := hyperWS.Start(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Fatal("Erreur serveur")
		}
	}()

	// Endpoints d'information
	logrus.Info("Endpoints disponibles:")
	logrus.Info("  WebSocket: ws://" + config.GetServerAddress() + "/ws")
	logrus.Info("  Santé:     http://" + config.GetServerAddress() + "/health")
	logrus.Info("  Stats:     http://" + config.GetServerAddress() + "/stats")

	// Attendre signal d'arrêt
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	logrus.Info("Signal d'arrêt reçu")

	// Arrêt gracieux
	if err := hyperWS.Stop(); err != nil {
		logrus.WithError(err).Error("Erreur lors de l'arrêt")
	}

	logrus.Info("Arrêt terminé")
} 