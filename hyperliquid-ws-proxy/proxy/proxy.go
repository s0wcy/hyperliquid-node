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
	useLocalNode    bool
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
		useLocalNode:        cfg.Proxy.EnableLocalNode,
		stats: ProxyStats{
			StartTime: time.Now(),
		},
	}
	
	// Initialize local node reader if enabled
	if cfg.Proxy.EnableLocalNode {
		logrus.Info("Local node mode enabled - will read data from local node instead of WebSocket API")
		p.localNodeReader = NewLocalNodeReader(cfg.Proxy.LocalNodeDataPath)
	} else {
		// Initialize Hyperliquid connector for remote API
		logrus.Info("Remote API mode - will connect to Hyperliquid WebSocket API")
		p.hlConnector = hyperliquid.NewConnector(cfg.GetHyperliquidURL())
		p.hlConnector.SetEventHandlers(
			p.handleHyperliquidMessage,
			p.handleHyperliquidConnect,
			p.handleHyperliquidDisconnect,
			p.handleHyperliquidError,
		)
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
	
	if p.useLocalNode && p.localNodeReader != nil {
		// Start local node reader
		go p.localNodeReader.Start()
		
		// Start local data processor
		go p.processLocalNodeData()
		
		logrus.Info("Local node reader started successfully")
	} else if p.hlConnector != nil {
		// Connect to Hyperliquid WebSocket API
		if err := p.hlConnector.Connect(); err != nil {
			return fmt.Errorf("failed to connect to Hyperliquid: %v", err)
		}
		
		logrus.Info("Connected to Hyperliquid WebSocket API")
	} else {
		return fmt.Errorf("neither local node reader nor Hyperliquid connector is available")
	}
	
	// Start statistics updater
	go p.updateStats()
	
	logrus.Info("Proxy started successfully")
	return nil
}

// Stop stops the proxy
func (p *Proxy) Stop() {
	logrus.Info("Stopping proxy...")
	
	if p.hlConnector != nil {
		// Disconnect from Hyperliquid
		p.hlConnector.Disconnect()
	}
	
	// Stop local node reader
	if p.localNodeReader != nil {
		p.localNodeReader.Stop()
	}
	
	logrus.Info("Proxy stopped")
}

// processLocalNodeData processes data from the local node reader
func (p *Proxy) processLocalNodeData() {
	ticker := time.NewTicker(1 * time.Second) // Generate updates every second
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if p.localNodeReader == nil || !p.localNodeReader.IsRunning() {
				return
			}
			
			// Generate WebSocket messages from local node data
			p.generateLocalNodeMessages()
		}
	}
}

// generateLocalNodeMessages generates WebSocket messages from local node data
func (p *Proxy) generateLocalNodeMessages() {
	// Generate allMids messages
	p.generateAllMidsFromLocalNode()
	
	// Generate trades messages for each coin
	p.generateTradesFromLocalNode()
}

// generateAllMidsFromLocalNode generates allMids messages from local node data
func (p *Proxy) generateAllMidsFromLocalNode() {
	// Check if anyone is subscribed to allMids
	hasAllMidsSubscribers := false
	p.subMu.RLock()
	for _, subInfo := range p.globalSubscriptions {
		if subInfo.Subscription.Type == "allMids" && len(subInfo.Clients) > 0 {
			hasAllMidsSubscribers = true
			break
		}
	}
	p.subMu.RUnlock()
	
	if !hasAllMidsSubscribers {
		return
	}
	
	// Get all latest prices from local node
	allPrices := make(map[string]string)
	coins := []string{"BTC", "ETH", "SOL", "MATIC", "ARB", "OP", "AVAX", "ATOM", "NEAR", "APT", "LTC", "BCH", "XRP", "SUI", "SEI"}
	
	for _, coin := range coins {
		if price, exists := p.localNodeReader.GetLatestPrice(coin); exists {
			allPrices[coin] = price
		}
	}
	
	if len(allPrices) == 0 {
		return
	}
	
	// Create allMids message
	allMids := types.AllMids{
		Mids: allPrices,
	}
	
	data, err := json.Marshal(allMids)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal allMids from local node")
		return
	}
	
	message := types.WSMessage{
		Channel: "allMids",
		Data:    data,
	}
	
	messageBytes, err := json.Marshal(message)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal allMids message")
		return
	}
	
	// Forward to clients subscribed to allMids
	p.forwardMessageToClients("allMids", messageBytes)
	
	logrus.WithField("prices_count", len(allPrices)).Debug("Generated allMids from local node")
}

// generateTradesFromLocalNode generates trades messages from local node data
func (p *Proxy) generateTradesFromLocalNode() {
	// Check which coins have trade subscribers
	coinsWithSubscribers := make(map[string]bool)
	p.subMu.RLock()
	for _, subInfo := range p.globalSubscriptions {
		if subInfo.Subscription.Type == "trades" && subInfo.Subscription.Coin != "" && len(subInfo.Clients) > 0 {
			coinsWithSubscribers[subInfo.Subscription.Coin] = true
		}
	}
	p.subMu.RUnlock()
	
	// Generate trades for subscribed coins
	for coin := range coinsWithSubscribers {
		trades := p.localNodeReader.GetLatestTrades(coin, 10) // Get last 10 trades
		if len(trades) == 0 {
			continue
		}
		
		// Send the most recent trade as a trades message
		latestTrade := trades[len(trades)-1]
		
		tradesMessage := map[string]interface{}{
			"channel": "trades",
			"data": map[string]interface{}{
				"coin":  latestTrade.Coin,
				"side":  latestTrade.Side,
				"px":    latestTrade.Px,
				"sz":    latestTrade.Sz,
				"time":  latestTrade.Time,
				"hash":  latestTrade.Hash,
				"tid":   latestTrade.TID,
				"users": latestTrade.Users,
			},
		}
		
		messageBytes, err := json.Marshal(tradesMessage)
		if err != nil {
			logrus.WithError(err).Error("Failed to marshal trades message")
			continue
		}
		
		// Forward to clients subscribed to this coin's trades
		p.forwardMessageToClients("trades", messageBytes)
		
		logrus.WithFields(logrus.Fields{
			"coin":  coin,
			"side":  latestTrade.Side,
			"price": latestTrade.Px,
		}).Debug("Generated trade from local node")
	}
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
		"local_node": p.useLocalNode,
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
		
		// Subscribe to Hyperliquid only if not using local node
		if !p.useLocalNode && p.hlConnector != nil {
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
		} else {
			logrus.WithField("subscription_type", sub.Type).Debug("Using local node data for subscription")
		}
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
	
	// Send initial data if using local node
	if p.useLocalNode && p.localNodeReader != nil {
		p.sendInitialLocalNodeData(c, sub)
	} else if subInfo.LastMessage != nil {
		// Send last message if available from remote API
		c.Send <- subInfo.LastMessage
	}
}

// sendInitialLocalNodeData sends initial data from local node to a newly subscribed client
func (p *Proxy) sendInitialLocalNodeData(c *client.Client, sub *types.SubscriptionRequest) {
	switch sub.Type {
	case "allMids":
		// Send current prices
		allPrices := make(map[string]string)
		coins := []string{"BTC", "ETH", "SOL", "MATIC", "ARB", "OP", "AVAX", "ATOM", "NEAR", "APT", "LTC", "BCH", "XRP", "SUI", "SEI"}
		
		for _, coin := range coins {
			if price, exists := p.localNodeReader.GetLatestPrice(coin); exists {
				allPrices[coin] = price
			}
		}
		
		if len(allPrices) > 0 {
			allMids := types.AllMids{Mids: allPrices}
			data, err := json.Marshal(allMids)
			if err == nil {
				message := types.WSMessage{
					Channel: "allMids",
					Data:    data,
				}
				c.SendMessage(message)
				logrus.WithField("client_id", c.ID).Debug("Sent initial allMids from local node")
			}
		}
		
	case "trades":
		if sub.Coin != "" {
			// Send recent trades for the specific coin
			trades := p.localNodeReader.GetLatestTrades(sub.Coin, 5) // Send last 5 trades
			for _, trade := range trades {
				tradesMessage := map[string]interface{}{
					"channel": "trades",
					"data": map[string]interface{}{
						"coin":  trade.Coin,
						"side":  trade.Side,
						"px":    trade.Px,
						"sz":    trade.Sz,
						"time":  trade.Time,
						"hash":  trade.Hash,
						"tid":   trade.TID,
						"users": trade.Users,
					},
				}
				
				messageBytes, err := json.Marshal(tradesMessage)
				if err == nil {
					c.Send <- messageBytes
				}
			}
			logrus.WithFields(logrus.Fields{
				"client_id": c.ID,
				"coin":      sub.Coin,
				"trades_count": len(trades),
			}).Debug("Sent initial trades from local node")
		}
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
		
		// If no more clients, unsubscribe from Hyperliquid (only if not using local node)
		if len(subInfo.Clients) == 0 {
			delete(p.globalSubscriptions, key)
			if !p.useLocalNode && p.hlConnector != nil {
				go func() {
					if err := p.hlConnector.Unsubscribe(sub); err != nil {
						logrus.WithError(err).Error("Failed to unsubscribe from Hyperliquid")
					}
				}()
			}
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
		"local_node":   p.useLocalNode,
	}).Debug("Handling POST request")
	
	if p.useLocalNode {
		// For local node mode, we can't handle POST requests as they require the Hyperliquid API
		p.sendPostErrorToClient(c, *msg.ID, "POST requests not supported in local node mode")
		return
	}
	
	if p.hlConnector == nil {
		p.sendPostErrorToClient(c, *msg.ID, "Hyperliquid connector not available")
		return
	}
	
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

// handleHyperliquidMessage handles messages from Hyperliquid (only used when not in local node mode)
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
	
	// Forward message to clients
	p.forwardMessageToClients(msg.Channel, data)
}

// forwardMessageToClients forwards a message to relevant clients
func (p *Proxy) forwardMessageToClients(channel string, data []byte) {
	p.subMu.Lock()
	defer p.subMu.Unlock()
	
	forwardedCount := 0
	clientsToRemove := make(map[*client.Client][]string) // client -> list of subscription keys to remove
	
	for key, subInfo := range p.globalSubscriptions {
		// Match channel with subscription type
		if string(subInfo.Subscription.Type) == channel {
			// Update last message
			subInfo.LastMessage = data
			subInfo.LastUpdate = time.Now()
			
			// Forward to all clients subscribed to this
			for c := range subInfo.Clients {
				// Try to send message to client
				select {
				case c.Send <- data:
					forwardedCount++
				default:
					// Client channel is full or closed - mark for removal
					logrus.WithField("client_id", c.ID).Debug("Client channel closed, removing from subscription")
					if clientsToRemove[c] == nil {
						clientsToRemove[c] = make([]string, 0)
					}
					clientsToRemove[c] = append(clientsToRemove[c], key)
				}
			}
		}
	}
	
	// Clean up disconnected clients
	for client, subscriptionKeys := range clientsToRemove {
		for _, key := range subscriptionKeys {
			if subInfo, exists := p.globalSubscriptions[key]; exists {
				delete(subInfo.Clients, client)
				
				// If no more clients for this subscription, remove the subscription entirely
				if len(subInfo.Clients) == 0 {
					delete(p.globalSubscriptions, key)
					logrus.WithField("subscription_key", key).Debug("Removed empty subscription")
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
			"local_node":       p.useLocalNode,
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