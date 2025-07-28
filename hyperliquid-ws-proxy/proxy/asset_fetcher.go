package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AssetInfo represents metadata for an asset
type AssetInfo struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	SzDecimals  int    `json:"szDecimals"`
	MaxLeverage int    `json:"maxLeverage,omitempty"`
	IsSpot      bool   `json:"isSpot"`
	TokenIndex  int    `json:"tokenIndex,omitempty"` // For spot assets
}

// AssetFetcher manages fetching and caching of Hyperliquid assets
type AssetFetcher struct {
	mu             sync.RWMutex
	perpAssets     map[int]*AssetInfo   // Index -> AssetInfo for perps
	spotAssets     map[int]*AssetInfo   // Index -> AssetInfo for spot pairs  
	assetsByName   map[string]*AssetInfo // Name -> AssetInfo lookup
	lastUpdated    time.Time
	apiURL         string
	updateInterval time.Duration
	stopChan       chan struct{}
}

// HyperliquidMetaResponse represents the perpetuals metadata response
type HyperliquidMetaResponse struct {
	Universe []struct {
		Name        string `json:"name"`
		SzDecimals  int    `json:"szDecimals"`
		MaxLeverage int    `json:"maxLeverage"`
	} `json:"universe"`
}

// HyperliquidSpotMetaResponse represents the spot metadata response
type HyperliquidSpotMetaResponse struct {
	Tokens []struct {
		Name       string `json:"name"`
		Index      int    `json:"index"`
		SzDecimals int    `json:"szDecimals"`
	} `json:"tokens"`
	Universe []struct {
		Name      string `json:"name"`
		Tokens    []int  `json:"tokens"`
		Index     int    `json:"index"`
	} `json:"universe"`
}

// NewAssetFetcher creates a new AssetFetcher
func NewAssetFetcher() *AssetFetcher {
	return &AssetFetcher{
		perpAssets:     make(map[int]*AssetInfo),
		spotAssets:     make(map[int]*AssetInfo),
		assetsByName:   make(map[string]*AssetInfo),
		apiURL:         "https://api.hyperliquid.xyz/info",
		updateInterval: 5 * time.Minute, // Update every 5 minutes
		stopChan:       make(chan struct{}),
	}
}

// Start initializes the asset fetcher and starts periodic updates
func (af *AssetFetcher) Start() error {
	logrus.Info("Starting asset fetcher - fetching initial asset metadata from Hyperliquid API")
	
	// Initial fetch
	if err := af.fetchAssets(); err != nil {
		return fmt.Errorf("failed to fetch initial assets: %w", err)
	}
	
	// Start periodic updates
	go af.periodicUpdate()
	
	return nil
}

// Stop stops the periodic updates
func (af *AssetFetcher) Stop() {
	close(af.stopChan)
}

// periodicUpdate runs the periodic asset updates
func (af *AssetFetcher) periodicUpdate() {
	ticker := time.NewTicker(af.updateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			logrus.Debug("Periodic asset metadata update starting")
			if err := af.fetchAssets(); err != nil {
				logrus.WithError(err).Error("Failed to update assets during periodic fetch")
			} else {
				logrus.Debug("Periodic asset metadata update completed successfully")
			}
		case <-af.stopChan:
			logrus.Info("Asset fetcher stopped")
			return
		}
	}
}

// fetchAssets fetches both perpetuals and spot assets from Hyperliquid API
func (af *AssetFetcher) fetchAssets() error {
	af.mu.Lock()
	defer af.mu.Unlock()
	
	// Fetch perpetuals
	if err := af.fetchPerpetuals(); err != nil {
		return fmt.Errorf("failed to fetch perpetuals: %w", err)
	}
	
	// Fetch spot assets
	if err := af.fetchSpotAssets(); err != nil {
		return fmt.Errorf("failed to fetch spot assets: %w", err)
	}
	
	af.lastUpdated = time.Now()
	
	logrus.WithFields(logrus.Fields{
		"perp_assets": len(af.perpAssets),
		"spot_assets": len(af.spotAssets),
		"total_assets": len(af.assetsByName),
	}).Info("Successfully updated asset metadata from Hyperliquid API")
	
	return nil
}

// fetchPerpetuals fetches perpetual assets metadata
func (af *AssetFetcher) fetchPerpetuals() error {
	reqBody := map[string]interface{}{
		"type": "meta",
	}
	
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := http.Post(af.apiURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}
	
	var metaResp HyperliquidMetaResponse
	if err := json.NewDecoder(resp.Body).Decode(&metaResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Process perpetuals
	perpAssetNames := make([]string, 0)
	for i, asset := range metaResp.Universe {
		assetInfo := &AssetInfo{
			Index:       i,
			Name:        asset.Name,
			SzDecimals:  asset.SzDecimals,
			MaxLeverage: asset.MaxLeverage,
			IsSpot:      false,
		}
		
		af.perpAssets[i] = assetInfo
		af.assetsByName[asset.Name] = assetInfo
		perpAssetNames = append(perpAssetNames, asset.Name)
	}
	
	logrus.WithFields(logrus.Fields{
		"count": len(metaResp.Universe),
		"assets": perpAssetNames,
	}).Debug("Fetched perpetual assets")
	return nil
}

// fetchSpotAssets fetches spot assets metadata
func (af *AssetFetcher) fetchSpotAssets() error {
	reqBody := map[string]interface{}{
		"type": "spotMeta",
	}
	
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := http.Post(af.apiURL, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned non-200 status: %d", resp.StatusCode)
	}
	
	var spotResp HyperliquidSpotMetaResponse
	if err := json.NewDecoder(resp.Body).Decode(&spotResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Create token lookup
	tokenMap := make(map[int]string)
	for _, token := range spotResp.Tokens {
		tokenMap[token.Index] = token.Name
	}
	
	// Process spot pairs
	spotAssetNames := make([]string, 0)
	for _, pair := range spotResp.Universe {
		assetName := pair.Name
		
		// Handle special naming convention for spot
		if len(pair.Tokens) >= 2 {
			if pair.Tokens[0] == 1 && tokenMap[1] != "" { // PURR/USDC case
				assetName = fmt.Sprintf("%s/USDC", tokenMap[pair.Tokens[0]])
			} else if pair.Tokens[0] != 0 && pair.Tokens[0] != 1 { // Other spot pairs
				assetName = fmt.Sprintf("@%d", pair.Index)
			}
		}
		
		assetInfo := &AssetInfo{
			Index:      10000 + pair.Index, // Spot assets use 10000 + index
			Name:       assetName,
			SzDecimals: 0, // Will be filled from token info if needed
			IsSpot:     true,
			TokenIndex: pair.Index,
		}
		
		af.spotAssets[10000+pair.Index] = assetInfo
		af.assetsByName[assetName] = assetInfo
		spotAssetNames = append(spotAssetNames, fmt.Sprintf("%s(%d)", assetName, pair.Index))
	}
	
	// Limit assets shown in logs to avoid spam
	assetsToShow := spotAssetNames
	if len(spotAssetNames) > 20 {
		assetsToShow = spotAssetNames[:20]
	}
	
	logrus.WithFields(logrus.Fields{
		"count": len(spotResp.Universe),
		"assets": assetsToShow,
		"total": len(spotAssetNames),
	}).Debug("Fetched spot assets")
	return nil
}

// GetAssetByID returns asset info by ID (index)
func (af *AssetFetcher) GetAssetByID(id int) (*AssetInfo, bool) {
	af.mu.RLock()
	defer af.mu.RUnlock()
	
	// Check perpetuals first
	if asset, exists := af.perpAssets[id]; exists {
		return asset, true
	}
	
	// Check spot assets
	if asset, exists := af.spotAssets[id]; exists {
		return asset, true
	}
	
	return nil, false
}

// GetAssetByName returns asset info by name
func (af *AssetFetcher) GetAssetByName(name string) (*AssetInfo, bool) {
	af.mu.RLock()
	defer af.mu.RUnlock()
	
	asset, exists := af.assetsByName[name]
	return asset, exists
}

// GetAllAssetNames returns all asset names
func (af *AssetFetcher) GetAllAssetNames() []string {
	af.mu.RLock()
	defer af.mu.RUnlock()
	
	names := make([]string, 0, len(af.assetsByName))
	for name := range af.assetsByName {
		names = append(names, name)
	}
	return names
}

// GetAssetStats returns statistics about loaded assets
func (af *AssetFetcher) GetAssetStats() map[string]interface{} {
	af.mu.RLock()
	defer af.mu.RUnlock()
	
	return map[string]interface{}{
		"perp_assets":  len(af.perpAssets),
		"spot_assets":  len(af.spotAssets),
		"total_assets": len(af.assetsByName),
		"last_updated": af.lastUpdated,
	}
} 