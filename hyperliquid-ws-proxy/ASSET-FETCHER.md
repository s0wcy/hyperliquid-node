# 📡 AssetFetcher - Fetch Dynamique des Assets Hyperliquid

## 🎯 Objectif

L'AssetFetcher remplace le mapping statique des assets par un fetch dynamique depuis l'API officielle d'Hyperliquid, assurant que tous les tokens listés et nouvellement ajoutés sont automatiquement disponibles.

## ✨ Améliorations

### ❌ **Avant (Mapping Statique)**
- ⚠️ Seulement 15-250 assets hardcodés 
- 🚫 Assets manquants si nouveaux tokens listés
- 🔄 Mise à jour manuelle nécessaire
- 📊 Prix incorrects pour assets non mappés

### ✅ **Après (Fetch Dynamique)**
- 🌐 **Fetch automatique** depuis l'API Hyperliquid
- 🔄 **Mise à jour périodique** (5 minutes)
- 📋 **Support complet** perpétuels + spot
- 🚀 **Nouveaux tokens** disponibles automatiquement
- 📊 **Prix corrects** pour tous les assets

## 🏗️ Architecture

```
AssetFetcher
├── 📡 API Calls
│   ├── POST /info {"type": "meta"}           → Perpétuels
│   └── POST /info {"type": "spotMeta"}       → Spot assets
├── 🗄️ Storage
│   ├── perpAssets: map[int]*AssetInfo        → Index → Asset info (perps) 
│   ├── spotAssets: map[int]*AssetInfo        → Index → Asset info (spot)
│   └── assetsByName: map[string]*AssetInfo   → Name → Asset lookup
└── 🔄 Auto-refresh (5 min interval)
```

## 📊 Données Récupérées

### Perpétuels
```json
{
  "name": "BTC",
  "szDecimals": 5,
  "maxLeverage": 50,
  "index": 0,
  "isSpot": false
}
```

### Spot
```json
{
  "name": "PURR/USDC",
  "index": 10000,
  "isSpot": true,
  "tokenIndex": 0
}
```

## 🔌 Nouvel Endpoint `/assets`

### **GET** `http://localhost:8080/assets`

Retourne la liste complète des assets disponibles avec statistiques.

#### Réponse
```json
{
  "status": "success",
  "data": {
    "statistics": {
      "perp_assets": 45,
      "spot_assets": 12,
      "total_assets": 57,
      "last_updated": "2025-01-28T19:30:15Z"
    },
    "assets": [
      "BTC", "ETH", "SOL", "ARB", "OP", "AVAX",
      "PURR/USDC", "@1", "@2", "..."
    ]
  },
  "timestamp": 1738094615
}
```

## 🧪 Tests

### Test des Assets
```bash
# Test de l'endpoint /assets
make test-assets

# Ou directement
npm run test:assets
node test-asset-fetcher.js
```

### Sortie Attendue
```
🚀 Test du système AssetFetcher
================================
🧪 Test de l'endpoint /assets
✅ Status: 200
✅ Status: success
📊 Statistiques:
   - Assets perpétuels: 45
   - Assets spot: 12
   - Total assets: 57
   - Dernière MAJ: 2025-01-28T19:30:15.123Z
📋 Assets disponibles:
   - Nombre d'assets: 57
   - Premiers assets: BTC, ETH, SOL, ARB, OP, AVAX, ATOM, NEAR, APT, LTC...
   - Assets majeurs trouvés: BTC, ETH, SOL, ARB, OP
✨ Tests terminés
```

## 🔄 Intégration 

### LocalNodeReader
```go
// Nouveau constructeur avec AssetFetcher
reader := NewLocalNodeReader(dataPath, assetFetcher)

// Mapping dynamique
func (r *LocalNodeReader) getAssetSymbol(assetID int) string {
    if asset, exists := r.assetFetcher.GetAssetByID(assetID); exists {
        return asset.Name
    }
    return "ASSET_" + strconv.Itoa(assetID) // Fallback
}
```

### Proxy Integration
```go
// AssetFetcher démarré en premier
func (p *Proxy) Start() error {
    // 1. Start asset fetcher
    if err := p.assetFetcher.Start(); err != nil {
        return err
    }
    
    // 2. Start local node reader (with assets available)
    go p.localNodeReader.Start()
}
```

## 📈 Métriques

L'AssetFetcher expose des métriques via `/assets`:

- **perp_assets**: Nombre d'assets perpétuels
- **spot_assets**: Nombre d'assets spot  
- **total_assets**: Total des assets disponibles
- **last_updated**: Dernière mise à jour API

## 🛠️ Configuration

### Intervalle de Refresh
```go
// Dans asset_fetcher.go
updateInterval: 5 * time.Minute // Configurable
```

### API Endpoint
```go
// URL API Hyperliquid
apiURL: "https://api.hyperliquid.xyz/info"
```

## 🔧 Dépannage

### AssetFetcher Non Initialisé
```
WARN AssetFetcher not initialized asset_id=42
```
**Solution**: Vérifier que l'AssetFetcher est démarré avant le LocalNodeReader.

### Erreur API
```
ERROR Failed to fetch initial assets: API returned non-200 status: 429
```
**Solution**: Rate limit API atteint, l'AssetFetcher retry automatiquement.

### Assets Manquants
Si des assets ne sont pas dans la liste, vérifier:
1. L'asset est listé sur Hyperliquid
2. L'AssetFetcher s'est mis à jour récemment
3. Forcer un refresh en redémarrant le service

## 🎯 Avantages

1. **📊 AllMids Complet**: Tous les prix disponibles
2. **🔄 Auto-Sync**: Nouveaux tokens automatiques  
3. **⚡ Performance**: Cache local + refresh périodique
4. **🛡️ Resilience**: Fallback sur asset ID si échec
5. **📈 Monitoring**: Endpoint `/assets` pour diagnostics

## 🚀 Déploiement

```bash
# Build et deploy avec nouvelles fonctionnalités
make compose-build compose-up

# Test complet
make test-full

# Vérifier les assets
curl http://localhost:8080/assets | jq
```

Cette amélioration garantit que votre proxy aura **toujours** accès aux derniers assets d'Hyperliquid ! 🎉 