package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// LocalNodeReader lit les données depuis le nœud Hyperliquid local
type LocalNodeReader struct {
	dataPath     string
	isRunning    bool
	mu           sync.RWMutex

	// Cache des données
	latestPrices map[string]string
	latestTrades map[string][]*WsTrade
	assetNames   map[int]string
	dataMu       sync.RWMutex

	// Surveillance des fichiers
	lastReadFiles map[string]int64
	
	// Canal pour arrêter les goroutines
	stopChan chan struct{}
}

// Block structure depuis le nœud Hyperliquid
type HLBlock struct {
	ABCIBlock struct {
		Time                string        `json:"time"`
		SignedActionBundles []interface{} `json:"signed_action_bundles"`
		Round               int64         `json:"round"`
	} `json:"abci_block"`
}

// Structures pour les actions
type ActionBundle struct {
	SignedActions []SignedAction `json:"signed_actions"`
}

type SignedAction struct {
	Action ActionData `json:"action"`
}

type ActionData struct {
	Type   string  `json:"type"`
	Orders []Order `json:"orders,omitempty"`
}

type Order struct {
	Asset int    `json:"a"`      // asset ID
	IsBuy bool   `json:"b"`      // is buy order
	Price string `json:"p"`      // price
	Size  string `json:"s"`      // size
}

// NewLocalNodeReader crée un nouveau lecteur de nœud local
func NewLocalNodeReader(dataPath string) *LocalNodeReader {
	return &LocalNodeReader{
		dataPath:      dataPath,
		latestPrices:  make(map[string]string),
		latestTrades:  make(map[string][]*WsTrade),
		assetNames:    make(map[int]string),
		lastReadFiles: make(map[string]int64),
		stopChan:      make(chan struct{}),
	}
}

// Start démarre la lecture du nœud local
func (r *LocalNodeReader) Start() error {
	r.mu.Lock()
	r.isRunning = true
	r.mu.Unlock()

	logrus.WithField("data_path", r.dataPath).Info("Démarrage du lecteur de nœud local")

	// Charger les métadonnées des assets
	if err := r.loadAssetMetadata(); err != nil {
		logrus.WithError(err).Warn("Impossible de charger les métadonnées des assets, utilisation des IDs")
	}

	// Démarrer la surveillance des fichiers
	go r.watchFiles()

	return nil
}

// Stop arrête le lecteur
func (r *LocalNodeReader) Stop() {
	r.mu.Lock()
	r.isRunning = false
	r.mu.Unlock()

	close(r.stopChan)
	logrus.Info("Lecteur de nœud local arrêté")
}

// IsRunning retourne l'état du lecteur
func (r *LocalNodeReader) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRunning
}

// loadAssetMetadata charge les métadonnées des assets depuis l'API Hyperliquid
func (r *LocalNodeReader) loadAssetMetadata() error {
	// Pour cette version simplifiée, nous utilisons une liste basique d'assets
	// Dans une version complète, ceci pourrait récupérer depuis l'API info
	basicAssets := map[int]string{
		0:  "BTC",
		1:  "ETH",
		2:  "DOGE",
		3:  "SOL",
		4:  "WIF",
		5:  "BONK",
		6:  "PEPE",
		7:  "ARB",
		8:  "AVAX",
		9:  "MATIC",
		10: "LINK",
		11: "UNI",
		12: "LTC",
		13: "BCH",
		14: "XRP",
		15: "ADA",
		16: "DOT",
		17: "NEAR",
		18: "FTM",
		19: "ATOM",
	}

	r.dataMu.Lock()
	for id, name := range basicAssets {
		r.assetNames[id] = name
	}
	r.dataMu.Unlock()

	logrus.WithField("loaded_assets", len(basicAssets)).Info("Métadonnées des assets chargées")
	return nil
}

// getAssetName retourne le nom d'un asset depuis son ID
func (r *LocalNodeReader) getAssetName(assetID int) string {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()

	if name, exists := r.assetNames[assetID]; exists {
		return name
	}

	// Fallback pour les assets non reconnus
	if assetID >= 10000 {
		// Assets spot utilisent le format @X
		return "@" + strconv.Itoa(assetID-10000)
	}

	return "ASSET_" + strconv.Itoa(assetID)
}

// watchFiles surveille les fichiers de données du nœud
func (r *LocalNodeReader) watchFiles() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopChan:
			return
		case <-ticker.C:
			if !r.IsRunning() {
				return
			}
			r.scanForNewData()
		}
	}
}

// scanForNewData recherche de nouvelles données
func (r *LocalNodeReader) scanForNewData() {
	replicaCmdsPath := filepath.Join(r.dataPath, "replica_cmds")

	// Vérifier que le répertoire existe
	if _, err := os.Stat(replicaCmdsPath); os.IsNotExist(err) {
		return
	}

	// Trouver le répertoire timestamp le plus récent
	timestampDir := r.getMostRecentDir(replicaCmdsPath)
	if timestampDir == "" {
		return
	}

	// Trouver le répertoire date le plus récent
	timestampPath := filepath.Join(replicaCmdsPath, timestampDir)
	dateDir := r.getMostRecentDir(timestampPath)
	if dateDir == "" {
		return
	}

	// Scanner les fichiers de bloc
	datePath := filepath.Join(timestampPath, dateDir)
	r.scanBlockFiles(datePath)
}

// getMostRecentDir retourne le répertoire le plus récent
func (r *LocalNodeReader) getMostRecentDir(basePath string) string {
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
	return dirs[len(dirs)-1]
}

// scanBlockFiles scanne les fichiers de blocs
func (r *LocalNodeReader) scanBlockFiles(dirPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	// Trier les fichiers par nom
	var fileNames []string
	for _, entry := range entries {
		if !entry.IsDir() {
			fileNames = append(fileNames, entry.Name())
		}
	}
	sort.Strings(fileNames)

	// Traiter les fichiers dans l'ordre
	for _, fileName := range fileNames {
		filePath := filepath.Join(dirPath, fileName)
		r.processBlockFile(filePath)
	}
}

// processBlockFile traite un fichier de bloc
func (r *LocalNodeReader) processBlockFile(filePath string) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return
	}

	// Vérifier si nous avons déjà lu ce fichier
	lastPos, exists := r.lastReadFiles[filePath]
	if exists && stat.Size() <= lastPos {
		return
	}

	// Lire depuis la dernière position
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()

	if lastPos > 0 {
		file.Seek(lastPos, 0)
	}

	// Lire le contenu restant
	remainingSize := stat.Size() - lastPos
	if remainingSize > 100*1024*1024 { // Limite à 100MB
		remainingSize = 100 * 1024 * 1024
	}

	buffer := make([]byte, remainingSize)
	bytesRead, err := file.Read(buffer)
	if err != nil && bytesRead == 0 {
		return
	}

	content := string(buffer[:bytesRead])
	lines := strings.Split(content, "\n")

	newPos := lastPos
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Traiter la ligne comme un bloc
		r.processBlockLine(line)

		// Mettre à jour la position
		if i < len(lines)-1 {
			newPos += int64(len(line) + 1)
		}
	}

	// Sauvegarder la nouvelle position
	r.lastReadFiles[filePath] = newPos
}

// processBlockLine traite une ligne de bloc (format NDJSON)
func (r *LocalNodeReader) processBlockLine(line string) {
	var block HLBlock
	if err := json.Unmarshal([]byte(line), &block); err != nil {
		return
	}

	// Traiter chaque bundle d'actions
	for _, bundleInterface := range block.ABCIBlock.SignedActionBundles {
		r.processActionBundle(bundleInterface, block.ABCIBlock.Time)
	}
}

// processActionBundle traite un bundle d'actions
func (r *LocalNodeReader) processActionBundle(bundleInterface interface{}, blockTime string) {
	// Les bundles sont des arrays [hash, bundle_data]
	bundleArray, ok := bundleInterface.([]interface{})
	if !ok || len(bundleArray) < 2 {
		return
	}

	// Extraire les données du bundle
	bundleDataBytes, err := json.Marshal(bundleArray[1])
	if err != nil {
		return
	}

	var bundle ActionBundle
	if err := json.Unmarshal(bundleDataBytes, &bundle); err != nil {
		return
	}

	// Traiter chaque action signée
	for _, signedAction := range bundle.SignedActions {
		r.processAction(&signedAction, blockTime)
	}
}

// processAction traite une action individuelle
func (r *LocalNodeReader) processAction(action *SignedAction, blockTime string) {
	if action.Action.Type == "order" {
		r.processOrders(action.Action.Orders, blockTime)
	}
}

// processOrders traite les ordres et met à jour les prix/trades
func (r *LocalNodeReader) processOrders(orders []Order, blockTime string) {
	timestamp := r.parseBlockTime(blockTime)

	for _, order := range orders {
		assetName := r.getAssetName(order.Asset)

		// Mettre à jour le prix
		r.dataMu.Lock()
		r.latestPrices[assetName] = order.Price

		// Créer un trade
		trade := &WsTrade{
			Coin: assetName,
			Side: "buy",
			Px:   order.Price,
			Sz:   order.Size,
			Time: timestamp,
			TID:  time.Now().UnixNano(),
			Hash: strconv.FormatInt(time.Now().UnixNano(), 16),
		}

		if !order.IsBuy {
			trade.Side = "sell"
		}

		// Ajouter le trade
		if r.latestTrades[assetName] == nil {
			r.latestTrades[assetName] = make([]*WsTrade, 0)
		}

		r.latestTrades[assetName] = append(r.latestTrades[assetName], trade)

		// Garder seulement les 100 derniers trades
		if len(r.latestTrades[assetName]) > 100 {
			r.latestTrades[assetName] = r.latestTrades[assetName][len(r.latestTrades[assetName])-100:]
		}

		r.dataMu.Unlock()
	}
}

// parseBlockTime parse le timestamp du bloc
func (r *LocalNodeReader) parseBlockTime(timeStr string) int64 {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Now().UnixMilli()
	}
	return t.UnixMilli()
}

// GetAllPrices retourne tous les prix actuels
func (r *LocalNodeReader) GetAllPrices() map[string]string {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()

	prices := make(map[string]string)
	for coin, price := range r.latestPrices {
		prices[coin] = price
	}
	return prices
}

// GetLatestTrades retourne les derniers trades pour une coin
func (r *LocalNodeReader) GetLatestTrades(coin string, limit int) []*WsTrade {
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

// GetStats retourne les statistiques du lecteur
func (r *LocalNodeReader) GetStats() map[string]interface{} {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()

	totalTrades := 0
	for _, trades := range r.latestTrades {
		totalTrades += len(trades)
	}

	return map[string]interface{}{
		"running":          r.IsRunning(),
		"data_path":        r.dataPath,
		"total_coins":      len(r.latestPrices),
		"total_trades":     totalTrades,
		"files_monitored":  len(r.lastReadFiles),
		"assets_loaded":    len(r.assetNames),
	}
} 