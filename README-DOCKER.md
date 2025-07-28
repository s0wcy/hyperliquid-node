# Node Hyperliquid avec Docker

Ce projet permet de démarrer facilement un non-validating node Hyperliquid en utilisant Docker.

## Configuration

### Testnet (par défaut)
- Utilise les pairs de référence fiables pour le testnet
- Configuration optimisée pour le développement et les tests
- Données écrites dans `~/hl/data` avec buffering désactivé pour une latence minimale

### Mainnet
- Configuration séparée pour le mainnet
- Pairs de référence communautaires fiables
- Utilise `docker-compose.mainnet.yml`

## Démarrage rapide

### 1. Démarrer le node Testnet
```bash
docker-compose up -d
```

### 2. Vérifier le statut
```bash
docker-compose ps
docker-compose logs node
```

### 3. Utiliser le script de gestion (PowerShell)
```powershell
# Voir le statut
.\manage-node.ps1 status

# Démarrer
.\manage-node.ps1 start

# Voir les logs en temps réel
.\manage-node.ps1 logs

# Redémarrer
.\manage-node.ps1 restart

# Arrêter
.\manage-node.ps1 stop
```

## Configuration Mainnet (Recommandée - Actuellement Active)

Pour utiliser le mainnet (configuration opérationnelle) :

```bash
# Construire et démarrer le node mainnet
docker-compose -f docker-compose.mainnet.yml up -d

# Voir les logs en temps réel
docker-compose -f docker-compose.mainnet.yml logs -f node

# Arrêter le node mainnet
docker-compose -f docker-compose.mainnet.yml down
```

## Données générées

Le node génère plusieurs types de données dans le volume Docker `hl-data` :

- **Trades** : `~/hl/data/node_trades/hourly/{date}/{hour}`
- **Fills** : `~/hl/data/node_fills/hourly/{date}/{hour}` (si activé)
- **Order Statuses** : `~/hl/data/node_order_statuses/hourly/{date}/{hour}` (si activé)
- **Raw Book Diffs** : `~/hl/data/node_raw_book_diffs/hourly/{date}/{hour}` (si activé)
- **Replica Commands** : `~/hl/data/replica_cmds/{start_time}/{date}/{height}`

## Flags de configuration

Le node est configuré avec les flags suivants :
- `--write-trades` : Écrit les trades en temps réel
- `--disable-output-file-buffering` : Désactive le buffering pour une latence minimale
- `--replica-cmds-style recent-actions` : Conserve seulement les 2 derniers fichiers de hauteur

## Pairs de référence

### Testnet
- `13.230.78.76` (Hypurrscan)
- `54.248.41.39` (Hypurrscan)

### Mainnet
- `20.188.6.225` (ASXN)
- `74.226.182.22` (ASXN)
- `180.189.55.18` (B-Harvest)
- `46.105.222.166` (Nansen x HypurrCollective)

## Monitoring

### Logs en temps réel
```bash
docker-compose logs -f node
```

### Statut des containers
```bash
docker-compose ps
```

### Utilisation des ressources
```bash
docker stats
```

## Nettoyage

Le service `pruner` s'exécute automatiquement pour nettoyer les anciennes données et éviter l'accumulation excessive de fichiers.

## Dépannage

### Problèmes de connectivité
Si le node a des difficultés à se connecter :
1. **IMPORTANT** : Les ports 4001 et 4002 doivent être ouverts publiquement pour le gossip
2. Sur Windows, vous devrez peut-être configurer votre pare-feu et routeur
3. Utilisez `docker-compose.ports.yml` si le mode host ne fonctionne pas
4. Attendez quelques minutes pour que le node trouve des pairs fiables
5. Vérifiez les logs pour les erreurs de connexion

### Configurations disponibles
```bash
# Testnet (configuration originale)
docker-compose up -d

# Mainnet (configuration recommandée et opérationnelle)
docker-compose -f docker-compose.mainnet.yml up -d
```

**Note importante** : Pour un node de production, vous devez configurer votre pare-feu et routeur pour permettre l'accès public aux ports 4001-4002.

### Problèmes de performance
- Assurez-vous d'avoir au moins 4 CPU cores et 32 GB RAM
- Vérifiez l'espace disque disponible (200 GB recommandé)
- Pour une latence optimale, exécutez le node à Tokyo, Japon

### Redémarrage complet
```bash
docker-compose down
docker-compose build --no-cache
docker-compose up -d
```

## Liens utiles

- [Documentation Hyperliquid](https://hyperliquid.gitbook.io/hyperliquid-docs/)
- [Schémas de données L1](https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/nodes/l1-data-schemas)
- [API Hyperliquid](https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/) 