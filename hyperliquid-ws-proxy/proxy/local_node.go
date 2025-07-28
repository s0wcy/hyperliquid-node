package proxy

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"hyperliquid-ws-proxy/types"
)

// LocalNodeReader reads data from the local Hyperliquid node
type LocalNodeReader struct {
	dataPath        string
	isRunning       bool
	mu              sync.RWMutex
	
	// Channels for data
	tradesChan      chan []byte
	fillsChan       chan []byte
	
	// File watching
	lastReadTimes   map[string]time.Time
	
	// Data cache
	latestTrades    map[string][]*types.WsTrade
	latestPrices    map[string]string
	dataMu          sync.RWMutex
}

// NewLocalNodeReader creates a new local node reader
func NewLocalNodeReader(dataPath string) *LocalNodeReader {
	return &LocalNodeReader{
		dataPath:      dataPath,
		tradesChan:    make(chan []byte, 1000),
		fillsChan:     make(chan []byte, 1000),
		lastReadTimes: make(map[string]time.Time),
		latestTrades:  make(map[string][]*types.WsTrade),
		latestPrices:  make(map[string]string),
	}
}

// Start starts the local node reader
func (r *LocalNodeReader) Start() {
	r.mu.Lock()
	r.isRunning = true
	r.mu.Unlock()
	
	logrus.WithField("data_path", r.dataPath).Info("Starting local node reader")
	
	// Start file watchers
	go r.watchTradesDirectory()
	go r.watchFillsDirectory()
	go r.updatePricesFromTrades()
	
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

// watchTradesDirectory watches the trades directory for new files
func (r *LocalNodeReader) watchTradesDirectory() {
	tradesPath := filepath.Join(r.dataPath, "node_trades", "hourly")
	
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if !r.IsRunning() {
				return
			}
			
			r.scanTradesDirectory(tradesPath)
		}
	}
}

// watchFillsDirectory watches the fills directory for new files
func (r *LocalNodeReader) watchFillsDirectory() {
	fillsPath := filepath.Join(r.dataPath, "node_fills", "hourly")
	
	ticker := time.NewTicker(1 * time.Second) // Check every second
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if !r.IsRunning() {
				return
			}
			
			r.scanFillsDirectory(fillsPath)
		}
	}
}

// scanTradesDirectory scans the trades directory for new files
func (r *LocalNodeReader) scanTradesDirectory(basePath string) {
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return
	}
	
	// Get the most recent date directory
	recentDateDir := r.getMostRecentDirectory(basePath)
	if recentDateDir == "" {
		return
	}
	
	datePath := filepath.Join(basePath, recentDateDir)
	
	// Get the most recent hour file in that date directory
	recentHourFile := r.getMostRecentFile(datePath, ".json")
	if recentHourFile == "" {
		return
	}
	
	filePath := filepath.Join(datePath, recentHourFile)
	
	// Check if file has been modified since last read
	if r.isFileUpdated(filePath) {
		r.readTradesFile(filePath)
	}
}

// scanFillsDirectory scans the fills directory for new files
func (r *LocalNodeReader) scanFillsDirectory(basePath string) {
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return
	}
	
	// Similar logic to trades but for fills
	recentDateDir := r.getMostRecentDirectory(basePath)
	if recentDateDir == "" {
		return
	}
	
	datePath := filepath.Join(basePath, recentDateDir)
	recentHourFile := r.getMostRecentFile(datePath, ".json")
	if recentHourFile == "" {
		return
	}
	
	filePath := filepath.Join(datePath, recentHourFile)
	
	if r.isFileUpdated(filePath) {
		r.readFillsFile(filePath)
	}
}

// readTradesFile reads a trades file and processes the data
func (r *LocalNodeReader) readTradesFile(filePath string) {
	logrus.WithField("file", filePath).Debug("Reading trades file")
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		logrus.WithError(err).Error("Failed to read trades file")
		return
	}
	
	// Parse the trades file (format may vary - adjust as needed)
	// Assuming each line is a JSON trade object
	lines := strings.Split(string(data), "\n")
	
	r.dataMu.Lock()
	defer r.dataMu.Unlock()
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		var trade types.WsTrade
		if err := json.Unmarshal([]byte(line), &trade); err != nil {
			logrus.WithError(err).Debug("Failed to parse trade line")
			continue
		}
		
		// Store trade
		if r.latestTrades[trade.Coin] == nil {
			r.latestTrades[trade.Coin] = make([]*types.WsTrade, 0)
		}
		
		r.latestTrades[trade.Coin] = append(r.latestTrades[trade.Coin], &trade)
		
		// Keep only last 1000 trades per coin
		if len(r.latestTrades[trade.Coin]) > 1000 {
			r.latestTrades[trade.Coin] = r.latestTrades[trade.Coin][len(r.latestTrades[trade.Coin])-1000:]
		}
		
		// Update latest price
		r.latestPrices[trade.Coin] = trade.Px
	}
	
	// Update last read time
	r.lastReadTimes[filePath] = time.Now()
}

// readFillsFile reads a fills file and processes the data
func (r *LocalNodeReader) readFillsFile(filePath string) {
	logrus.WithField("file", filePath).Debug("Reading fills file")
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		logrus.WithError(err).Error("Failed to read fills file")
		return
	}
	
	// Parse fills file - similar to trades but for user fills
	// This would be used for user-specific subscriptions
	
	// Update last read time
	r.lastReadTimes[filePath] = time.Now()
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

// getMostRecentFile returns the most recent file with a given extension
func (r *LocalNodeReader) getMostRecentFile(dirPath, extension string) string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ""
	}
	
	var files []fs.FileInfo
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), extension) {
			info, err := entry.Info()
			if err == nil {
				files = append(files, info)
			}
		}
	}
	
	if len(files) == 0 {
		return ""
	}
	
	// Sort by modification time
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})
	
	return files[len(files)-1].Name() // Return the most recently modified file
}

// isFileUpdated checks if a file has been updated since last read
func (r *LocalNodeReader) isFileUpdated(filePath string) bool {
	stat, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	
	lastRead, exists := r.lastReadTimes[filePath]
	if !exists {
		return true // First time reading this file
	}
	
	return stat.ModTime().After(lastRead)
}

// updatePricesFromTrades periodically updates prices from the latest trades
func (r *LocalNodeReader) updatePricesFromTrades() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if !r.IsRunning() {
				return
			}
			
			// This could publish price updates to subscribed clients
			// For now, it just maintains the internal price cache
			r.publishPriceUpdates()
		}
	}
}

// publishPriceUpdates publishes price updates to interested subscribers
func (r *LocalNodeReader) publishPriceUpdates() {
	r.dataMu.RLock()
	prices := make(map[string]string)
	for coin, price := range r.latestPrices {
		prices[coin] = price
	}
	r.dataMu.RUnlock()
	
	if len(prices) == 0 {
		return
	}
	
	// Create an allMids message
	allMids := types.AllMids{
		Mids: prices,
	}
	
	message := types.WSMessage{
		Channel: "allMids",
	}
	
	data, err := json.Marshal(allMids)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal allMids")
		return
	}
	
	message.Data = data
	
	// This would be sent to the proxy for distribution to clients
	// For now, we just log it
	logrus.WithField("prices_count", len(prices)).Debug("Updated prices from local node")
}

// GetNodeStats returns statistics about the local node data
func (r *LocalNodeReader) GetNodeStats() map[string]interface{} {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	
	stats := map[string]interface{}{
		"total_coins":       len(r.latestPrices),
		"total_trades":      0,
		"files_monitored":   len(r.lastReadTimes),
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