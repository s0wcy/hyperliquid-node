package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"hyperliquid-ws-proxy/types"
)

// HyperliquidNodeBlock represents an ABCI block from the Hyperliquid node
type HyperliquidNodeBlock struct {
	ABCIBlock struct {
		Time                string                    `json:"time"`
		SignedActionBundles [][]interface{}           `json:"signed_action_bundles"`
		Round               int64                     `json:"round"`
		ParentRound         int64                     `json:"parent_round"`
		Hardfork            map[string]interface{}    `json:"hardfork"`
		Proposer            string                    `json:"proposer"`
	} `json:"abci_block"`
	Resps interface{} `json:"resps"`
}

// SignedActionBundle represents a bundle of signed actions
type SignedActionBundle struct {
	Hash           string         `json:"hash,omitempty"`
	SignedActions  []SignedAction `json:"signed_actions"`
	Broadcaster    string         `json:"broadcaster"`
	BroadcasterNonce int64        `json:"broadcaster_nonce"`
}

// SignedAction represents a signed action within a bundle
type SignedAction struct {
	Signature struct {
		R string `json:"r"`
		S string `json:"s"`
		V int    `json:"v"`
	} `json:"signature"`
	VaultAddress string      `json:"vaultAddress,omitempty"`
	Action       ActionData  `json:"action"`
	Nonce        int64       `json:"nonce"`
}

// ActionData represents the action data
type ActionData struct {
	Type     string      `json:"type"`
	Orders   []Order     `json:"orders,omitempty"`
	Cancels  []Cancel    `json:"cancels,omitempty"`
	Grouping string      `json:"grouping,omitempty"`
	Time     int64       `json:"time,omitempty"` 
}

// Order represents a trading order
type Order struct {
	Asset    int    `json:"a"`          // asset ID
	IsBuy    bool   `json:"b"`          // is buy order
	Price    string `json:"p"`          // price
	Size     string `json:"s"`          // size
	ReduceOnly bool `json:"r"`          // reduce only
	OrderType struct {
		Limit struct {
			TIF string `json:"tif"`    // time in force
		} `json:"limit"`
	} `json:"t"`
	ClientOrderID string `json:"c"`      // client order ID
}

// Cancel represents an order cancellation
type Cancel struct {
	Asset int    `json:"asset"`
	Cloid string `json:"cloid"`      // client order ID to cancel
}

// LocalNodeReader reads data from the local Hyperliquid node
type LocalNodeReader struct {
	dataPath        string
	isRunning       bool
	mu              sync.RWMutex
	
	// Channels for data
	blocksChan      chan *HyperliquidNodeBlock
	tradesChan      chan []byte
	ordersChan      chan []byte
	
	// File watching
	lastReadFiles   map[string]int64  // filename -> last read position
	watchedDirs     []string
	
	// Data cache
	latestBlocks    []*HyperliquidNodeBlock
	latestTrades    map[string][]*types.WsTrade
	latestPrices    map[string]string
	dataMu          sync.RWMutex
	
	// Asset fetcher for dynamic asset metadata
	assetFetcher    *AssetFetcher
}

// NewLocalNodeReader creates a new local node reader
func NewLocalNodeReader(dataPath string, assetFetcher *AssetFetcher) *LocalNodeReader {
	return &LocalNodeReader{
		dataPath:      dataPath,
		blocksChan:    make(chan *HyperliquidNodeBlock, 1000),
		tradesChan:    make(chan []byte, 1000),
		ordersChan:    make(chan []byte, 1000),
		lastReadFiles: make(map[string]int64),
		latestBlocks:  make([]*HyperliquidNodeBlock, 0),
		latestTrades:  make(map[string][]*types.WsTrade),
		latestPrices:  make(map[string]string),
		assetFetcher:  assetFetcher,
	}
}

// Start starts the local node reader
func (r *LocalNodeReader) Start() {
	r.mu.Lock()
	r.isRunning = true
	r.mu.Unlock()
	
	logrus.WithField("data_path", r.dataPath).Info("Starting local node reader for Hyperliquid replica_cmds")
	
	// AssetFetcher is expected to be already initialized and started by the caller
	
	// Start file watchers
	go r.watchReplicaCmdsDirectory()
	go r.processBlocks()
	
	logrus.Info("Local node reader started")
}

// Stop stops the local node reader
func (r *LocalNodeReader) Stop() {
	r.mu.Lock()
	r.isRunning = false
	r.mu.Unlock()
	
	logrus.Info("Local node reader stopped")
}

// IsRunning returns whether the reader is running
func (r *LocalNodeReader) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRunning
}



// getAssetSymbol returns the symbol for an asset ID using the AssetFetcher
func (r *LocalNodeReader) getAssetSymbol(assetID int) string {
	if r.assetFetcher == nil {
		logrus.WithField("asset_id", assetID).Warn("AssetFetcher not initialized")
		return "ASSET_" + strconv.Itoa(assetID)
	}
	
	// First try direct asset ID lookup (for perpetuals)
	if asset, exists := r.assetFetcher.GetAssetByID(assetID); exists {
		return asset.Name
	}
	
	// Then try spot asset lookup (spot assets use 10000 + index)
	if asset, exists := r.assetFetcher.GetAssetByID(10000 + assetID); exists {
		return asset.Name
	}
	
	// For spot assets that don't have names in the fetcher, use @X format
	// This matches Hyperliquid's convention for spot assets
	if assetID > 0 && assetID < 1000 { // Reasonable range for spot asset indices
		spotName := fmt.Sprintf("@%d", assetID)
		logrus.WithFields(logrus.Fields{
			"asset_id": assetID,
			"spot_name": spotName,
		}).Debug("Using spot asset name format")
		return spotName
	}
	
	// Return asset ID as string if not found
	logrus.WithField("asset_id", assetID).Debug("Asset not found in fetcher, using fallback name")
	return "ASSET_" + strconv.Itoa(assetID)
}

// watchReplicaCmdsDirectory watches the replica_cmds directory for new files
func (r *LocalNodeReader) watchReplicaCmdsDirectory() {
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if !r.IsRunning() {
				return
			}
			
			r.scanReplicaCmdsDirectory()
		}
	}
}

// scanReplicaCmdsDirectory scans the replica_cmds directory for new files
func (r *LocalNodeReader) scanReplicaCmdsDirectory() {
	// Look for replica_cmds directory
	replicaCmdsPath := filepath.Join(r.dataPath, "replica_cmds")
	
	if _, err := os.Stat(replicaCmdsPath); os.IsNotExist(err) {
		logrus.WithField("path", replicaCmdsPath).Debug("replica_cmds directory not found")
		return
	}
	
	// Get the most recent timestamp directory
	recentTimestampDir := r.getMostRecentDirectory(replicaCmdsPath)
	if recentTimestampDir == "" {
		return
	}
	
	timestampPath := filepath.Join(replicaCmdsPath, recentTimestampDir)
	
	// Get the most recent date directory within the timestamp
	recentDateDir := r.getMostRecentDirectory(timestampPath)
	if recentDateDir == "" {
		return
	}
	
	datePath := filepath.Join(timestampPath, recentDateDir)
	
	// Get all files in the date directory
	r.scanBlockFiles(datePath)
}

// scanBlockFiles scans for block files and reads new data
func (r *LocalNodeReader) scanBlockFiles(dirPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		logrus.WithError(err).Debug("Failed to read directory")
		return
	}
	
	// Sort files by name (which should be block numbers)
	var fileNames []string
	for _, entry := range entries {
		if !entry.IsDir() {
			fileNames = append(fileNames, entry.Name())
		}
	}
	sort.Strings(fileNames)
	
	// Process files in order
	for _, fileName := range fileNames {
		filePath := filepath.Join(dirPath, fileName)
		
		// Check if we need to read this file (or more of it)
		stat, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		
		lastReadPos, exists := r.lastReadFiles[filePath]
		if !exists || stat.Size() > lastReadPos {
			r.readBlockFile(filePath, lastReadPos)
		}
	}
}

// readBlockFile reads a block file from a given position
func (r *LocalNodeReader) readBlockFile(filePath string, fromPos int64) {
	logrus.WithFields(logrus.Fields{
		"file":     filePath,
		"from_pos": fromPos,
	}).Info("NEW VERSION - Reading block file with chunk method")
	
	file, err := os.Open(filePath)
	if err != nil {
		logrus.WithError(err).Error("Failed to open block file")
		return
	}
	defer file.Close()
	
	// Get file size
	stat, err := file.Stat()
	if err != nil {
		logrus.WithError(err).Error("Failed to get file stats")
		return
	}
	
	// If file is smaller than our last position, reset
	if stat.Size() <= fromPos {
		return
	}
	
	// Seek to the last read position
	if fromPos > 0 {
		_, err = file.Seek(fromPos, 0)
		if err != nil {
			logrus.WithError(err).Error("Failed to seek in file")
			return
		}
	}
	
	// Read the entire remaining file content
	remainingSize := stat.Size() - fromPos
	if remainingSize > 100*1024*1024 { // Limit to 100MB per read to avoid memory issues
		remainingSize = 100 * 1024 * 1024
	}
	
	buffer := make([]byte, remainingSize)
	bytesRead, err := file.Read(buffer)
	if err != nil && bytesRead == 0 {
		logrus.WithError(err).Error("Failed to read file")
		return
	}
	
	content := string(buffer[:bytesRead])
	lines := strings.Split(content, "\n")
	
	newPos := fromPos
	
	// Process each line
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Update position (except for the last line which might be incomplete)
		if i < len(lines)-1 {
			newPos += int64(len(line) + 1) // +1 for newline
		}
		
		// Skip incomplete last line if we didn't read the entire file
		if i == len(lines)-1 && bytesRead == int(remainingSize) && fromPos+int64(bytesRead) < stat.Size() {
			continue
		}
		
		// Parse the NDJSON line as a block
		var block HyperliquidNodeBlock
		if err := json.Unmarshal([]byte(line), &block); err != nil {
			logrus.WithError(err).WithField("line_length", len(line)).Debug("Failed to parse block line")
			continue
		}
		
		// Process the block
		r.processBlock(&block)
		
		// Update position for complete lines
		if i == len(lines)-1 && (bytesRead < int(remainingSize) || fromPos+int64(bytesRead) >= stat.Size()) {
			newPos += int64(len(line))
		}
	}
	
	// Update last read position
	r.lastReadFiles[filePath] = newPos
	
	logrus.WithFields(logrus.Fields{
		"file":        filePath,
		"bytes_read":  bytesRead,
		"lines_processed": len(lines),
		"new_pos":     newPos,
	}).Debug("Block file read completed")
}

// processBlock processes a single block
func (r *LocalNodeReader) processBlock(block *HyperliquidNodeBlock) {
	logrus.WithFields(logrus.Fields{
		"time":         block.ABCIBlock.Time,
		"round":        block.ABCIBlock.Round,
		"bundles_count": len(block.ABCIBlock.SignedActionBundles),
	}).Debug("Processing block")
	
	// Store the block
	r.dataMu.Lock()
	r.latestBlocks = append(r.latestBlocks, block)
	
	// Keep only last 100 blocks in memory
	if len(r.latestBlocks) > 100 {
		r.latestBlocks = r.latestBlocks[len(r.latestBlocks)-100:]
	}
	r.dataMu.Unlock()
	
	// Process each signed action bundle
	bundleProcessed := 0
	for i, bundleInterface := range block.ABCIBlock.SignedActionBundles {
		logrus.WithField("bundle_index", i).Debug("Processing signed action bundle")
		r.processSignedActionBundle(bundleInterface, block.ABCIBlock.Time)
		bundleProcessed++
	}
	
	logrus.WithFields(logrus.Fields{
		"round": block.ABCIBlock.Round,
		"bundles_processed": bundleProcessed,
	}).Debug("Block processing completed")
	
	// Send block to processing channel
	select {
	case r.blocksChan <- block:
	default:
		// Channel full, drop oldest
	}
}

// processSignedActionBundle processes a signed action bundle
func (r *LocalNodeReader) processSignedActionBundle(bundleInterface interface{}, blockTime string) {
	// SignedActionBundles are arrays of [hash, bundle_data]
	bundleArray, ok := bundleInterface.([]interface{})
	if !ok {
		logrus.WithField("type", fmt.Sprintf("%T", bundleInterface)).Debug("Bundle is not an array")
		return
	}
	
	if len(bundleArray) < 2 {
		logrus.WithField("length", len(bundleArray)).Debug("Bundle array too short")
		return
	}
	
	// Extract the bundle data (second element)
	bundleDataInterface := bundleArray[1]
	bundleDataBytes, err := json.Marshal(bundleDataInterface)
	if err != nil {
		logrus.WithError(err).Debug("Failed to marshal bundle data")
		return
	}
	
	logrus.WithField("bundle_data_size", len(bundleDataBytes)).Debug("Marshaled bundle data")
	
	var bundle SignedActionBundle
	if err := json.Unmarshal(bundleDataBytes, &bundle); err != nil {
		logrus.WithError(err).WithField("bundle_json", string(bundleDataBytes[:min(200, len(bundleDataBytes))])).Debug("Failed to unmarshal signed action bundle")
		return
	}
	
	logrus.WithFields(logrus.Fields{
		"signed_actions_count": len(bundle.SignedActions),
		"broadcaster": bundle.Broadcaster,
	}).Debug("Successfully parsed signed action bundle")
	
	// Process each signed action in the bundle
	for i, signedAction := range bundle.SignedActions {
		logrus.WithFields(logrus.Fields{
			"action_index": i,
			"action_type": signedAction.Action.Type,
		}).Debug("Processing signed action")
		r.processSignedAction(&signedAction, blockTime)
	}
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// processSignedAction processes a single signed action
func (r *LocalNodeReader) processSignedAction(action *SignedAction, blockTime string) {
	switch action.Action.Type {
	case "order":
		r.processOrders(action.Action.Orders, blockTime, action.VaultAddress)
	case "cancelByCloid":
		r.processCancellations(action.Action.Cancels, blockTime, action.VaultAddress)
	case "scheduleCancel":
		// Handle scheduled cancellations
		logrus.Debug("Scheduled cancel action")
	case "noop":
		// No operation - ignore
	default:
		logrus.WithField("type", action.Action.Type).Debug("Unknown action type")
	}
}

// processOrders processes order actions and generates trade-like data
func (r *LocalNodeReader) processOrders(orders []Order, blockTime string, userAddress string) {
	if len(orders) == 0 {
		logrus.Debug("No orders to process")
		return
	}
	
	logrus.WithField("orders_count", len(orders)).Debug("Processing orders")
	
	ordersProcessed := 0
	for _, order := range orders {
		symbol := r.getAssetSymbol(order.Asset)
		
		// Log asset mapping for debugging
		logrus.WithFields(logrus.Fields{
			"asset_id": order.Asset,
			"symbol": symbol,
			"price": order.Price,
			"size": order.Size,
		}).Debug("Processing order - asset mapping")
		
		// Skip if we couldn't map the asset
		if strings.HasPrefix(symbol, "ASSET_") {
			logrus.WithFields(logrus.Fields{
				"asset_id": order.Asset,
				"symbol": symbol,
			}).Debug("Unknown asset ID, using fallback name")
		}
		
		// Convert to WsTrade format for compatibility
		trade := &types.WsTrade{
			Coin: symbol,
			Side: "buy",
			Px:   order.Price,
			Sz:   order.Size,
			Time: r.parseBlockTime(blockTime),
			Hash: order.ClientOrderID,
			TID:  time.Now().UnixNano(), // Generate a TID
			Users: [2]string{userAddress, ""}, // User placing the order
		}
		
		if !order.IsBuy {
			trade.Side = "sell"
		}
		
		// Store the trade
		r.dataMu.Lock()
		if r.latestTrades[symbol] == nil {
			r.latestTrades[symbol] = make([]*types.WsTrade, 0)
		}
		
		r.latestTrades[symbol] = append(r.latestTrades[symbol], trade)
		
		// Keep only last 1000 trades per symbol
		if len(r.latestTrades[symbol]) > 1000 {
			r.latestTrades[symbol] = r.latestTrades[symbol][len(r.latestTrades[symbol])-1000:]
		}
		
		// Update latest price
		oldPrice, hadPrice := r.latestPrices[symbol]
		r.latestPrices[symbol] = order.Price
		totalPrices := len(r.latestPrices)
		r.dataMu.Unlock()
		
		logrus.WithFields(logrus.Fields{
			"symbol":    symbol,
			"asset_id":  order.Asset,
			"side":      trade.Side,
			"price":     order.Price,
			"old_price": oldPrice,
			"had_price": hadPrice,
			"size":      order.Size,
			"user":      userAddress,
			"total_prices": totalPrices,
		}).Debug("Processed order as trade")
		
		ordersProcessed++
	}
	
	logrus.WithField("orders_processed", ordersProcessed).Debug("Completed processing orders")
}

// processCancellations processes cancellation actions
func (r *LocalNodeReader) processCancellations(cancels []Cancel, blockTime string, userAddress string) {
	for _, cancel := range cancels {
		symbol := r.getAssetSymbol(cancel.Asset)
		
		logrus.WithFields(logrus.Fields{
			"symbol":  symbol,
			"cloid":   cancel.Cloid,
			"user":    userAddress,
		}).Debug("Processed cancellation")
	}
}

// processBlocks processes blocks from the channel
func (r *LocalNodeReader) processBlocks() {
	for {
		select {
		case block := <-r.blocksChan:
			if !r.IsRunning() {
				return
			}
			
			// Here we could generate WebSocket messages for subscribed clients
			r.generateWebSocketMessages(block)
		}
	}
}

// generateWebSocketMessages generates WebSocket messages for various subscription types
func (r *LocalNodeReader) generateWebSocketMessages(block *HyperliquidNodeBlock) {
	// Generate allMids message if we have price data
	r.dataMu.RLock()
	pricesCount := len(r.latestPrices)
	r.dataMu.RUnlock()
	
	if pricesCount > 0 {
		r.generateAllMidsMessage()
	} else {
		logrus.Debug("No prices available yet for allMids generation")
	}
}

// generateAllMidsMessage generates and sends allMids message
func (r *LocalNodeReader) generateAllMidsMessage() {
	r.dataMu.RLock()
	if len(r.latestPrices) == 0 {
		r.dataMu.RUnlock()
		return
	}
	
	allMids := types.AllMids{
		Mids: make(map[string]string),
	}
	for symbol, price := range r.latestPrices {
		allMids.Mids[symbol] = price
	}
	r.dataMu.RUnlock()
	
	_, err := json.Marshal(allMids)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal allMids message")
		return
	}
	
	logrus.WithField("symbols_count", len(allMids.Mids)).Debug("Generated allMids message")
	
	// TODO: This should be sent to the proxy for distribution to clients
	// For now, we just log that it was generated
}

// parseBlockTime parses block time to Unix timestamp
func (r *LocalNodeReader) parseBlockTime(timeStr string) int64 {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Now().UnixMilli()
	}
	return t.UnixMilli()
}

// GetLatestPrice returns the latest price for a coin
func (r *LocalNodeReader) GetLatestPrice(coin string) (string, bool) {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	
	price, exists := r.latestPrices[coin]
	return price, exists
}

// GetLatestTrades returns the latest trades for a coin
func (r *LocalNodeReader) GetLatestTrades(coin string, limit int) []*types.WsTrade {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	
	trades, exists := r.latestTrades[coin]
	if !exists {
		return nil
	}
	
	if limit > 0 && len(trades) > limit {
		return trades[len(trades)-limit:]
	}
	
	return trades
}

// GetAllLatestPrices returns all available prices
func (r *LocalNodeReader) GetAllLatestPrices() map[string]string {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	
	// Create a copy to avoid race conditions
	allPrices := make(map[string]string)
	for symbol, price := range r.latestPrices {
		allPrices[symbol] = price
	}
	return allPrices
}

// getMostRecentDirectory returns the most recent directory in a path
func (r *LocalNodeReader) getMostRecentDirectory(basePath string) string {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}
	
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	
	if len(dirs) == 0 {
		return ""
	}
	
	sort.Strings(dirs)
	return dirs[len(dirs)-1] // Return the last (most recent) directory
}

// GetNodeStats returns statistics about the local node data
func (r *LocalNodeReader) GetNodeStats() map[string]interface{} {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	
	stats := map[string]interface{}{
		"total_coins":       len(r.latestPrices),
		"total_trades":      0,
		"files_monitored":   len(r.lastReadFiles),
		"blocks_processed":  len(r.latestBlocks),
		"data_path":         r.dataPath,
		"running":           r.IsRunning(),
	}
	
	totalTrades := 0
	for _, trades := range r.latestTrades {
		totalTrades += len(trades)
	}
	stats["total_trades"] = totalTrades
	
	return stats
} 