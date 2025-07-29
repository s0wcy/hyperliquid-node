# HyperWS - Proxy WebSocket Hyperliquid Optimisé

HyperWS est un proxy WebSocket léger et optimisé pour Hyperliquid qui lit les données directement depuis un nœud non-validateur local. Cette version épurée offre toutes les fonctionnalités de l'API WebSocket officielle d'Hyperliquid sans les limitations de débit.

## 🚀 Fonctionnalités

- ✅ **Toutes les souscriptions Hyperliquid** : `allMids`, `trades`, `l2Book`, `candle`, `bbo`, et plus
- ✅ **Pas de limites de débit** : Lecture directe depuis le nœud local
- ✅ **Données dynamiques** : Pas de données hardcodées, tout est récupéré en temps réel
- ✅ **Multi-clients** : Support de milliers de connexions simultanées
- ✅ **Container Docker** : Déploiement facile sur n'importe quel hébergeur
- ✅ **Configuration simple** : Un seul fichier YAML à configurer
- ✅ **Monitoring intégré** : Endpoints de santé et statistiques

## 📋 Prérequis

- Un nœud Hyperliquid non-validateur en fonctionnement
- Docker et Docker Compose (recommandé)
- Ou Go 1.21+ pour compilation manuelle

## 🔧 Installation et Configuration

### Méthode 1 : Docker Compose (Recommandée)

1. **Cloner ou télécharger HyperWS**
```bash
# Naviguez vers le dossier hyperws de votre projet
cd hyperws
```

2. **Configurer le chemin des données**
Éditez le fichier `docker-compose.yml` et ajustez le volume pour pointer vers vos données de nœud :
```yaml
volumes:
  # Pour mainnet
  - /var/lib/docker/volumes/node_hl-data-mainnet/_data:/data:ro
  # Pour testnet
  # - /var/lib/docker/volumes/node_hl-data-testnet/_data:/data:ro
```

3. **Optionnel : Personnaliser la configuration**
Copiez et modifiez `config.yaml` si nécessaire :
```yaml
# HyperWS - Configuration
server:
  host: "0.0.0.0"
  port: 8080

node:
  data_path: "/data"  # Chemin dans le container

proxy:
  max_clients: 1000
  heartbeat_interval: 30
  message_buffer_size: 1024

logging:
  level: "info"
  format: "text"
```

4. **Démarrer HyperWS**
```bash
docker-compose up -d
```

### Méthode 2 : Docker simple

```bash
docker build -t hyperws .

docker run -d \
  --name hyperws \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /var/lib/docker/volumes/node_hl-data-mainnet/_data:/data:ro \
  hyperws
```

### Méthode 3 : Compilation manuelle

```bash
# Installer les dépendances
go mod download

# Compiler
go build -o hyperws .

# Exécuter
./hyperws -config config.yaml
```

## 📡 Utilisation

### Endpoints Disponibles

- **WebSocket** : `ws://localhost:8080/ws`
- **Santé** : `http://localhost:8080/health`
- **Statistiques** : `http://localhost:8080/stats`

### Connexion WebSocket

Connectez-vous exactement comme avec l'API officielle Hyperliquid :

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

// Souscrire aux prix moyens de tous les assets
ws.send(JSON.stringify({
  method: "subscribe",
  subscription: {
    type: "allMids"
  }
}));

// Souscrire aux trades d'un asset spécifique
ws.send(JSON.stringify({
  method: "subscribe",
  subscription: {
    type: "trades",
    coin: "BTC"
  }
}));

// Écouter les messages
ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Reçu:', data);
};
```

### Types de Souscription Supportés

| Type | Description | Paramètres |
|------|-------------|------------|
| `allMids` | Prix moyens de tous les assets | - |
| `trades` | Trades d'un asset | `coin` |
| `l2Book` | Book de profondeur | `coin` |
| `candle` | Données de chandelier | `coin`, `interval` |
| `bbo` | Meilleure offre/demande | `coin` |
| `notification` | Notifications utilisateur | `user` |
| `webData2` | Données interface web | - |
| `orderUpdates` | Mises à jour d'ordres | `user` |
| `userEvents` | Événements utilisateur | `user` |
| `userFills` | Exécutions utilisateur | `user` |
| `userFundings` | Paiements de funding | `user` |
| `userNonFundingLedgerUpdates` | Mises à jour du ledger | `user` |
| `activeAssetCtx` | Contexte des assets | `coin` |
| `activeAssetData` | Données d'asset actif | `user`, `coin` |
| `userTwapSliceFills` | Exécutions TWAP | `user` |
| `userTwapHistory` | Historique TWAP | `user` |

## 📊 Monitoring

### Endpoint de Santé

```bash
curl http://localhost:8080/health
```

Réponse :
```json
{
  "status": "healthy",
  "timestamp": 1698765432,
  "clients": 5,
  "node_running": true,
  "subscriptions": 8,
  "version": "1.0.0"
}
```

### Statistiques Détaillées

```bash
curl http://localhost:8080/stats
```

Réponse :
```json
{
  "server": {
    "name": "HyperWS",
    "version": "1.0.0",
    "uptime": 3600
  },
  "websocket": {
    "connected_clients": 5,
    "active_subscriptions": 8
  },
  "node": {
    "running": true,
    "data_path": "/data",
    "total_coins": 25,
    "total_trades": 1250,
    "files_monitored": 12,
    "assets_loaded": 20
  }
}
```

## 🔧 Configuration Avancée

### Variables d'Environnement

```bash
# Niveau de log
export LOG_LEVEL=debug

# Format de log
export LOG_FORMAT=json

# Port personnalisé
export PORT=9090
```

### Options de Ligne de Commande

```bash
./hyperws -h

Usage:
  -config string
        Chemin vers le fichier de configuration (default "config.yaml")
  -log-level string
        Niveau de log (debug, info, warn, error)
  -version
        Afficher les informations de version
```

## 🐛 Dépannage

### Le Service ne Démarre Pas

1. **Vérifiez les logs**
```bash
docker-compose logs hyperws
```

2. **Vérifiez le chemin des données du nœud**
```bash
# Le répertoire doit exister et contenir replica_cmds/
ls -la /var/lib/docker/volumes/node_hl-data-mainnet/_data/
```

3. **Vérifiez les permissions**
```bash
# Le container doit pouvoir lire les données
sudo chmod -R 755 /var/lib/docker/volumes/node_hl-data-mainnet/_data/
```

### Pas de Données Reçues

1. **Vérifiez que le nœud génère des données**
```bash
find /var/lib/docker/volumes/node_hl-data-mainnet/_data/replica_cmds/ -name "*" -newer -1h
```

2. **Vérifiez les souscriptions actives**
```bash
curl http://localhost:8080/stats | jq '.websocket.active_subscriptions'
```

3. **Activez les logs debug**
```bash
# Dans docker-compose.yml
environment:
  - LOG_LEVEL=debug
```

### Performance

- **Ajustez `max_clients`** selon vos besoins dans la configuration
- **Montez le répertoire de données en lecture seule** (`ro`) pour la sécurité
- **Utilisez un reverse proxy** (nginx) pour la production

## 📝 Notes de Version

### v1.0.0
- Version initiale optimisée
- Support complet des souscriptions Hyperliquid
- Lecture directe du nœud non-validateur
- Container Docker léger
- Monitoring intégré

## 🤝 Support

Pour des questions ou des problèmes :
1. Vérifiez les logs avec `docker-compose logs hyperws`
2. Consultez l'endpoint `/health` pour diagnostiquer
3. Activez les logs debug si nécessaire

## 📄 Licence

Ce projet utilise la même licence que le projet parent. 