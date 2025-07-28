package proxy

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"hyperliquid-ws-proxy/client"
	"hyperliquid-ws-proxy/config"
	"hyperliquid-ws-proxy/hyperliquid"
	"hyperliquid-ws-proxy/types"
)

// Proxy orchestrates the WebSocket proxy between clients and Hyperliquid
type Proxy struct {
	config        *config.Config
	hub           *client.Hub
	hlConnector   *hyperliquid.Connector
	
	// Subscription management
	globalSubscriptions map[string]*SubscriptionInfo
	subMu              sync.RWMutex
	
	// Statistics
	stats ProxyStats
	
	// Local node integration
	localNodeReader *LocalNodeReader
}

// SubscriptionInfo tracks subscription details
type SubscriptionInfo struct {
	Subscription *types.SubscriptionRequest
	Clients      map[*client.Client]bool
	LastMessage  []byte
	LastUpdate   time.Time
}

// ProxyStats holds proxy statistics
type ProxyStats struct {
	ConnectedClients     int
	ActiveSubscriptions  int
	MessagesProcessed    int64
	MessagesForwarded    int64
	PostRequestsHandled  int64
	LastActivity         time.Time
	StartTime            time.Time
	mu                   sync.RWMutex
}

// NewProxy creates a new proxy instance
func NewProxy(cfg *config.Config) *Proxy {
	p := &Proxy{
		config:              cfg,
		hub:                 client.NewHub(),
		globalSubscriptions: make(map[string]*SubscriptionInfo),
		stats: ProxyStats{
			StartTime: time.Now(),
		},
	}
	
	// Initialize Hyperliquid connector
	p.hlConnector = hyperliquid.NewConnector(cfg.GetHyperliquidURL())
	p.hlConnector.SetEventHandlers(
		p.handleHyperliquidMessage,
		p.handleHyperliquidConnect,
		p.handleHyperliquidDisconnect,
		p.handleHyperliquidError,
	)
	
	// Initialize local node reader if enabled
	if cfg.Proxy.EnableLocalNode {
		p.localNodeReader = NewLocalNodeReader(cfg.Proxy.LocalNodeDataPath)
	}
	
	return p
}

// Start starts the proxy
func (p *Proxy) Start() error {
	logrus.Info("Starting Hyperliquid WebSocket Proxy")
	
	// Start the client hub
	go p.hub.Run()
	
	// Start client message processor
	go p.processClientMessages()
	
	// Connect to Hyperliquid
	if err := p.hlConnector.Connect(); err != nil {
		return fmt.Errorf("failed to connect to Hyperliquid: %v", err)
	}
	
	// Start statistics updater
	go p.updateStats()
	
	// Start local node reader if enabled
	if p.localNodeReader != nil {
		go p.localNodeReader.Start()
	}
	
	logrus.Info("Proxy started successfully")
	return nil
}

// Stop stops the proxy
func (p *Proxy) Stop() {
	logrus.Info("Stopping proxy...")
	
	// Disconnect from Hyperliquid
	p.hlConnector.Disconnect()
	
	// Stop local node reader
	if p.localNodeReader != nil {
		p.localNodeReader.Stop()
	}
	
	logrus.Info("Proxy stopped")
}

// GetHub returns the client hub
func (p *Proxy) GetHub() *client.Hub {
	return p.hub
}

// GetStats returns proxy statistics
func (p *Proxy) GetStats() ProxyStats {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()
	
	stats := p.stats
	stats.ConnectedClients = p.hub.GetClientCount()
	
	p.subMu.RLock()
	stats.ActiveSubscriptions = len(p.globalSubscriptions)
	p.subMu.RUnlock()
	
	return stats
}

// processClientMessages processes messages from clients
func (p *Proxy) processClientMessages() {
	for {
		select {
		case clientMsg := <-p.hub.ClientMessage:
			p.handleClientMessage(clientMsg.Client, clientMsg.Message)
		}
	}
}

// handleClientMessage handles a message from a client
func (p *Proxy) handleClientMessage(c *client.Client, data []byte) {
	p.updateStatsActivity()
	
	var msg types.WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logrus.WithError(err).Error("Failed to parse client message")
		p.sendErrorToClient(c, "Invalid message format")
		return
	}
	
	switch msg.Method {
	case "subscribe":
		p.handleSubscribe(c, msg.Subscription)
	case "unsubscribe":
		p.handleUnsubscribe(c, msg.Subscription)
	case "post":
		p.handlePostRequest(c, &msg)
	default:
		logrus.WithField("method", msg.Method).Warn("Unknown method")
		p.sendErrorToClient(c, "Unknown method: "+msg.Method)
	}
}

// handleSubscribe handles subscription requests
func (p *Proxy) handleSubscribe(c *client.Client, sub *types.SubscriptionRequest) {
	if sub == nil {
		p.sendErrorToClient(c, "Missing subscription details")
		return
	}
	
	logrus.WithFields(logrus.Fields{
		"client_id": c.ID,
		"type":      sub.Type,
		"coin":      sub.Coin,
		"user":      sub.User,
	}).Debug("Handling subscription")
	
	// Create subscription key
	key := p.createSubscriptionKey(sub)
	
	// Add client to subscription
	p.subMu.Lock()
	subInfo, exists := p.globalSubscriptions[key]
	if !exists {
		subInfo = &SubscriptionInfo{
			Subscription: sub,
			Clients:      make(map[*client.Client]bool),
			LastUpdate:   time.Now(),
		}
		p.globalSubscriptions[key] = subInfo
		
		// Subscribe to Hyperliquid (only if this is the first client for this subscription)
		go func() {
			if err := p.hlConnector.Subscribe(sub); err != nil {
				logrus.WithError(err).Error("Failed to subscribe to Hyperliquid")
				p.sendErrorToClient(c, "Failed to subscribe: "+err.Error())
				
				// Remove the subscription since it failed
				p.subMu.Lock()
				delete(p.globalSubscriptions, key)
				p.subMu.Unlock()
				return
			}
		}()
	}
	
	subInfo.Clients[c] = true
	p.subMu.Unlock()
	
	// Add subscription to client
	c.AddSubscription(key, sub)
	
	// Send subscription response
	response := types.WSMessage{
		Channel: "subscriptionResponse",
		Data:    json.RawMessage(fmt.Sprintf(`{"method":"subscribe","subscription":%s}`, p.toJSON(sub))),
	}
	c.SendMessage(response)
	
	// Send last message if available
	if subInfo.LastMessage != nil {
		c.Send <- subInfo.LastMessage
	}
}

// handleUnsubscribe handles unsubscription requests
func (p *Proxy) handleUnsubscribe(c *client.Client, sub *types.SubscriptionRequest) {
	if sub == nil {
		p.sendErrorToClient(c, "Missing subscription details")
		return
	}
	
	logrus.WithFields(logrus.Fields{
		"client_id": c.ID,
		"type":      sub.Type,
		"coin":      sub.Coin,
		"user":      sub.User,
	}).Debug("Handling unsubscription")
	
	key := p.createSubscriptionKey(sub)
	
	p.subMu.Lock()
	subInfo, exists := p.globalSubscriptions[key]
	if exists {
		delete(subInfo.Clients, c)
		
		// If no more clients, unsubscribe from Hyperliquid
		if len(subInfo.Clients) == 0 {
			delete(p.globalSubscriptions, key)
			go func() {
				if err := p.hlConnector.Unsubscribe(sub); err != nil {
					logrus.WithError(err).Error("Failed to unsubscribe from Hyperliquid")
				}
			}()
		}
	}
	p.subMu.Unlock()
	
	// Remove subscription from client
	c.RemoveSubscription(key)
	
	// Send unsubscription response
	response := types.WSMessage{
		Channel: "subscriptionResponse",
		Data:    json.RawMessage(fmt.Sprintf(`{"method":"unsubscribe","subscription":%s}`, p.toJSON(sub))),
	}
	c.SendMessage(response)
}

// handlePostRequest handles POST requests via WebSocket
func (p *Proxy) handlePostRequest(c *client.Client, msg *types.WSMessage) {
	if msg.Request == nil || msg.ID == nil {
		p.sendErrorToClient(c, "Invalid POST request format")
		return
	}
	
	logrus.WithFields(logrus.Fields{
		"client_id":    c.ID,
		"request_id":   *msg.ID,
		"request_type": msg.Request.Type,
	}).Debug("Handling POST request")
	
	// Forward request to Hyperliquid
	response, err := p.hlConnector.PostRequest(msg.Request.Type, msg.Request.Payload)
	if err != nil {
		logrus.WithError(err).Error("POST request failed")
		p.sendPostErrorToClient(c, *msg.ID, err.Error())
		return
	}
	
	// Send response back to client
	responseMsg := types.WSMessage{
		Channel: "post",
		Data:    json.RawMessage(p.toJSON(response)),
	}
	c.SendMessage(responseMsg)
	
	p.stats.mu.Lock()
	p.stats.PostRequestsHandled++
	p.stats.mu.Unlock()
}

// handleHyperliquidMessage handles messages from Hyperliquid
func (p *Proxy) handleHyperliquidMessage(data []byte) {
	p.updateStatsActivity()
	
	p.stats.mu.Lock()
	p.stats.MessagesProcessed++
	p.stats.mu.Unlock()
	
	// Parse message to determine channel/type
	var msg types.WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logrus.WithError(err).Error("Failed to parse Hyperliquid message")
		return
	}
	
	// Skip subscription responses and POST responses (handled elsewhere)
	if msg.Channel == "subscriptionResponse" || msg.Channel == "post" {
		return
	}
	
	// Find matching subscriptions and forward to clients
	p.forwardMessageToClients(msg.Channel, data)
}

// forwardMessageToClients forwards a message to relevant clients
func (p *Proxy) forwardMessageToClients(channel string, data []byte) {
	p.subMu.RLock()
	defer p.subMu.RUnlock()
	
	forwardedCount := 0
	
	for key, subInfo := range p.globalSubscriptions {
		// Match channel with subscription type
		if string(subInfo.Subscription.Type) == channel {
			// Update last message
			subInfo.LastMessage = data
			subInfo.LastUpdate = time.Now()
			
			// Forward to all clients subscribed to this
			for c := range subInfo.Clients {
				select {
				case c.Send <- data:
					forwardedCount++
				default:
					// Client channel is full, skip
				}
			}
		}
	}
	
	if forwardedCount > 0 {
		p.stats.mu.Lock()
		p.stats.MessagesForwarded += int64(forwardedCount)
		p.stats.mu.Unlock()
	}
}

// handleHyperliquidConnect handles Hyperliquid connection events
func (p *Proxy) handleHyperliquidConnect() {
	logrus.Info("Connected to Hyperliquid WebSocket")
}

// handleHyperliquidDisconnect handles Hyperliquid disconnection events
func (p *Proxy) handleHyperliquidDisconnect(err error) {
	logrus.WithError(err).Warn("Disconnected from Hyperliquid WebSocket")
}

// handleHyperliquidError handles Hyperliquid error events
func (p *Proxy) handleHyperliquidError(err error) {
	logrus.WithError(err).Error("Hyperliquid WebSocket error")
}

// sendErrorToClient sends an error message to a client
func (p *Proxy) sendErrorToClient(c *client.Client, errorMsg string) {
	response := map[string]interface{}{
		"error": errorMsg,
		"time":  time.Now().Unix(),
	}
	c.SendMessage(response)
}

// sendPostErrorToClient sends a POST error response to a client
func (p *Proxy) sendPostErrorToClient(c *client.Client, requestID int64, errorMsg string) {
	response := types.WSMessage{
		Channel: "post",
		Data: json.RawMessage(fmt.Sprintf(`{
			"id": %d,
			"response": {
				"type": "error",
				"payload": "%s"
			}
		}`, requestID, errorMsg)),
	}
	c.SendMessage(response)
}

// updateStats updates proxy statistics
func (p *Proxy) updateStats() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		stats := p.GetStats()
		logrus.WithFields(logrus.Fields{
			"clients":          stats.ConnectedClients,
			"subscriptions":    stats.ActiveSubscriptions,
			"messages_proc":    stats.MessagesProcessed,
			"messages_fwd":     stats.MessagesForwarded,
			"post_requests":    stats.PostRequestsHandled,
		}).Debug("Proxy statistics")
	}
}

// updateStatsActivity updates the last activity timestamp
func (p *Proxy) updateStatsActivity() {
	p.stats.mu.Lock()
	p.stats.LastActivity = time.Now()
	p.stats.mu.Unlock()
}

// createSubscriptionKey creates a unique key for a subscription
func (p *Proxy) createSubscriptionKey(sub *types.SubscriptionRequest) string {
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
	if sub.Dex != "" {
		key += "-" + sub.Dex
	}
	return key
}

// toJSON converts an object to JSON string
func (p *Proxy) toJSON(obj interface{}) string {
	data, err := json.Marshal(obj)
	if err != nil {
		return "{}"
	}
	return string(data)
} 