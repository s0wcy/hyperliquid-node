package hyperliquid

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"hyperliquid-ws-proxy/types"
)

// Connector manages the connection to Hyperliquid WebSocket API
type Connector struct {
	URL         string
	conn        *websocket.Conn
	mu          sync.RWMutex
	isConnected bool
	
	// Channels for communication
	incomingMessages chan []byte
	outgoingMessages chan []byte
	
	// Subscription management
	subscriptions    map[string]*types.SubscriptionRequest
	subMu           sync.RWMutex
	
	// Post request management
	postRequests    map[int64]chan *types.PostResponse
	postMu          sync.RWMutex
	nextRequestID   int64
	
	// Reconnection settings
	maxRetries      int
	retryInterval   time.Duration
	currentRetries  int
	
	// Heartbeat
	enableHeartbeat bool
	heartbeatInterval time.Duration
	lastPong        time.Time
	
	// Event handlers
	onMessage       func([]byte)
	onConnect       func()
	onDisconnect    func(error)
	onError         func(error)
}

// NewConnector creates a new Hyperliquid connector
func NewConnector(url string) *Connector {
	return &Connector{
		URL:               url,
		incomingMessages:  make(chan []byte, 1000),
		outgoingMessages:  make(chan []byte, 1000),
		subscriptions:     make(map[string]*types.SubscriptionRequest),
		postRequests:      make(map[int64]chan *types.PostResponse),
		maxRetries:        5,
		retryInterval:     5 * time.Second,
		enableHeartbeat:   true,
		heartbeatInterval: 30 * time.Second,
		nextRequestID:     1,
	}
}

// SetEventHandlers sets the event handlers
func (c *Connector) SetEventHandlers(
	onMessage func([]byte),
	onConnect func(),
	onDisconnect func(error),
	onError func(error),
) {
	c.onMessage = onMessage
	c.onConnect = onConnect
	c.onDisconnect = onDisconnect
	c.onError = onError
}

// Connect establishes connection to Hyperliquid WebSocket
func (c *Connector) Connect() error {
	logrus.WithField("url", c.URL).Info("Connecting to Hyperliquid WebSocket")
	
	conn, _, err := websocket.DefaultDialer.Dial(c.URL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Hyperliquid: %v", err)
	}
	
	c.mu.Lock()
	c.conn = conn
	c.isConnected = true
	c.currentRetries = 0
	c.lastPong = time.Now()
	c.mu.Unlock()
	
	logrus.Info("Connected to Hyperliquid WebSocket")
	
	// Start goroutines
	go c.readPump()
	go c.writePump()
	// Note: JSON heartbeats are now sent directly in writePump() every 50 seconds
	// This is compatible with Hyperliquid's requirement for activity every 60 seconds
	
	// Resubscribe to existing subscriptions
	go c.resubscribeAll()
	
	if c.onConnect != nil {
		c.onConnect()
	}
	
	return nil
}

// Disconnect closes the connection
func (c *Connector) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil && c.isConnected {
		c.isConnected = false
		c.conn.Close()
		logrus.Info("Disconnected from Hyperliquid WebSocket")
	}
}

// IsConnected returns the connection status
func (c *Connector) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected
}

// Subscribe sends a subscription request to Hyperliquid
func (c *Connector) Subscribe(subscription *types.SubscriptionRequest) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Hyperliquid")
	}
	
	// Create subscription key
	key := c.createSubscriptionKey(subscription)
	
	// Store subscription
	c.subMu.Lock()
	c.subscriptions[key] = subscription
	c.subMu.Unlock()
	
	// Send subscription message
	message := types.WSMessage{
		Method:       "subscribe",
		Subscription: subscription,
	}
	
	return c.sendMessage(message)
}

// Unsubscribe sends an unsubscription request to Hyperliquid
func (c *Connector) Unsubscribe(subscription *types.SubscriptionRequest) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Hyperliquid")
	}
	
	// Create subscription key
	key := c.createSubscriptionKey(subscription)
	
	// Remove subscription
	c.subMu.Lock()
	delete(c.subscriptions, key)
	c.subMu.Unlock()
	
	// Send unsubscription message
	message := types.WSMessage{
		Method:       "unsubscribe",
		Subscription: subscription,
	}
	
	return c.sendMessage(message)
}

// PostRequest sends a POST request via WebSocket
func (c *Connector) PostRequest(requestType string, payload json.RawMessage) (*types.PostResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to Hyperliquid")
	}
	
	// Generate request ID
	c.postMu.Lock()
	requestID := c.nextRequestID
	c.nextRequestID++
	
	// Create response channel
	responseChan := make(chan *types.PostResponse, 1)
	c.postRequests[requestID] = responseChan
	c.postMu.Unlock()
	
	// Clean up channel after timeout
	defer func() {
		c.postMu.Lock()
		delete(c.postRequests, requestID)
		c.postMu.Unlock()
		close(responseChan)
	}()
	
	// Send request
	message := types.WSMessage{
		Method: "post",
		ID:     &requestID,
		Request: &types.PostRequest{
			Type:    requestType,
			Payload: payload,
		},
	}
	
	if err := c.sendMessage(message); err != nil {
		return nil, err
	}
	
	// Wait for response with timeout
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	}
}

// readPump handles incoming messages from Hyperliquid
func (c *Connector) readPump() {
	defer func() {
		c.handleDisconnect(nil)
	}()
	
	for {
		c.mu.RLock()
		conn := c.conn
		connected := c.isConnected
		c.mu.RUnlock()
		
		if !connected || conn == nil {
			break
		}
		
		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		
			// Set pong handler for WebSocket pings (backup)
	conn.SetPongHandler(func(string) error {
		c.lastPong = time.Now()
		logrus.Debug("Received WebSocket pong from Hyperliquid")
		return nil
	})
		
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logrus.WithError(err).Error("WebSocket read error")
			}
			c.handleDisconnect(err)
			break
		}
		
		// Process message
		c.processMessage(message)
	}
}

// writePump handles outgoing messages to Hyperliquid
func (c *Connector) writePump() {
	ticker := time.NewTicker(50 * time.Second) // Send JSON heartbeat every 50 seconds (safely under 60s limit)
	defer ticker.Stop()
	
	for {
		select {
		case message := <-c.outgoingMessages:
			c.mu.RLock()
			conn := c.conn
			connected := c.isConnected
			c.mu.RUnlock()
			
			if !connected || conn == nil {
				return
			}
			
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				logrus.WithError(err).Error("Write error")
				c.handleDisconnect(err)
				return
			}
			
		case <-ticker.C:
			c.mu.RLock()
			conn := c.conn
			connected := c.isConnected
			c.mu.RUnlock()
			
			if !connected || conn == nil {
				return
			}
			
			// Send JSON heartbeat message instead of WebSocket ping
			heartbeat := []byte(`{"method":"ping"}`)
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, heartbeat); err != nil {
				logrus.WithError(err).Error("Heartbeat error")
				c.handleDisconnect(err)
				return
			} else {
				logrus.Debug("Sent JSON heartbeat to Hyperliquid")
			}
		}
	}
}

// heartbeatLoop is no longer needed - JSON heartbeats are sent in writePump()
// This function is kept for backwards compatibility but does nothing
func (c *Connector) heartbeatLoop() {
	// JSON heartbeats are now handled directly in writePump() every 50 seconds
	// to comply with Hyperliquid's 60-second activity requirement
}

// processMessage processes incoming messages from Hyperliquid
func (c *Connector) processMessage(data []byte) {
	// Check for heartbeat response (pong) - ignore it
	if string(data) == `{"method":"pong"}` || string(data) == `{"status":"pong"}` {
		logrus.Debug("Received JSON pong from Hyperliquid")
		c.lastPong = time.Now()
		return
	}
	
	// Try to parse as a general message first
	var msg types.WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		logrus.WithError(err).WithField("raw_message", string(data)).Error("Failed to parse message")
		return
	}
	
	// Handle POST responses
	if msg.Channel == "post" && msg.ID != nil {
		c.handlePostResponse(msg.ID, data)
		return
	}
	
	// Forward message to handlers
	if c.onMessage != nil {
		c.onMessage(data)
	}
}

// handlePostResponse handles POST request responses
func (c *Connector) handlePostResponse(requestID *int64, data []byte) {
	if requestID == nil {
		return
	}
	
	c.postMu.RLock()
	responseChan, exists := c.postRequests[*requestID]
	c.postMu.RUnlock()
	
	if !exists {
		return
	}
	
	// Parse response
	var response types.PostResponse
	if err := json.Unmarshal(data, &response); err != nil {
		logrus.WithError(err).Error("Failed to parse POST response")
		return
	}
	
	// Send response to waiting goroutine
	select {
	case responseChan <- &response:
	default:
		// Channel might be closed or full
	}
}

// sendMessage sends a message to Hyperliquid
func (c *Connector) sendMessage(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	
	select {
	case c.outgoingMessages <- data:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout")
	}
}

// handleDisconnect handles disconnection and potential reconnection
func (c *Connector) handleDisconnect(err error) {
	c.mu.Lock()
	wasConnected := c.isConnected
	c.isConnected = false
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
	
	if wasConnected {
		logrus.WithError(err).Warn("Disconnected from Hyperliquid")
		
		if c.onDisconnect != nil {
			c.onDisconnect(err)
		}
		
		// Attempt reconnection
		go c.attemptReconnect()
	}
}

// attemptReconnect attempts to reconnect with exponential backoff
func (c *Connector) attemptReconnect() {
	for c.currentRetries < c.maxRetries {
		c.currentRetries++
		
		delay := time.Duration(c.currentRetries) * c.retryInterval
		logrus.WithFields(logrus.Fields{
			"attempt": c.currentRetries,
			"delay":   delay,
		}).Info("Attempting to reconnect...")
		
		time.Sleep(delay)
		
		if err := c.Connect(); err != nil {
			logrus.WithError(err).Error("Reconnection failed")
			if c.onError != nil {
				c.onError(err)
			}
		} else {
			logrus.Info("Reconnected successfully")
			return
		}
	}
	
	logrus.Error("Max reconnection attempts reached")
}

// resubscribeAll resubscribes to all active subscriptions
func (c *Connector) resubscribeAll() {
	// Wait a bit for connection to stabilize
	time.Sleep(1 * time.Second)
	
	c.subMu.RLock()
	subs := make([]*types.SubscriptionRequest, 0, len(c.subscriptions))
	for _, sub := range c.subscriptions {
		subs = append(subs, sub)
	}
	c.subMu.RUnlock()
	
	for _, sub := range subs {
		if err := c.Subscribe(sub); err != nil {
			logrus.WithError(err).Error("Failed to resubscribe")
		} else {
			logrus.WithField("type", sub.Type).Debug("Resubscribed")
		}
		
		// Small delay between subscriptions
		time.Sleep(100 * time.Millisecond)
	}
	
	logrus.WithField("count", len(subs)).Info("Resubscribed to all subscriptions")
}

// createSubscriptionKey creates a unique key for a subscription
func (c *Connector) createSubscriptionKey(sub *types.SubscriptionRequest) string {
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

// GetSubscriptions returns a copy of all active subscriptions
func (c *Connector) GetSubscriptions() map[string]*types.SubscriptionRequest {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	
	subs := make(map[string]*types.SubscriptionRequest)
	for k, v := range c.subscriptions {
		subs[k] = v
	}
	return subs
} 