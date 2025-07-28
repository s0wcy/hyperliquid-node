# Configurations Hyperliquid Node

## Fichiers conservés

### ✅ Configurations opérationnelles

1. **`docker-compose.yml`** - Configuration originale Testnet
   - Chain: Testnet
   - Dockerfile: `Dockerfile`
   - Volume: `hl-data`

2. **`docker-compose.mainnet.yml`** - Configuration Mainnet (ACTIVE)
   - Chain: Mainnet  
   - Dockerfile: `Dockerfile.mainnet`
   - Volume: `hl-data-mainnet`
   - **Status: ✅ OPÉRATIONNEL** - Traite les blocs en temps réel

### 📁 Dockerfiles

1. **`Dockerfile`** - Pour Testnet
   - Binaires: `binaries.hyperliquid-testnet.xyz`
   - Pairs: Hypurrscan (testnet)

2. **`Dockerfile.mainnet`** - Pour Mainnet
   - Binaires: `binaries.hyperliquid.xyz`
   - Pairs: ASXN, B-Harvest, Nansen x HypurrCollective

### 🛠️ Outils de gestion

1. **`manage-node.ps1`** - Script PowerShell de gestion
2. **`README-DOCKER.md`** - Documentation principale
3. **`FIREWALL-SETUP.md`** - Guide configuration pare-feu

## Commandes rapides

### Mainnet (Recommandé)
```bash
# Démarrer
docker-compose -f docker-compose.mainnet.yml up -d

# Logs
docker-compose -f docker-compose.mainnet.yml logs -f node

# Arrêter
docker-compose -f docker-compose.mainnet.yml down
```

### Testnet
```bash
# Démarrer
docker-compose up -d

# Logs
docker-compose logs -f node

# Arrêter
docker-compose down
```

## Status actuel

- **Mainnet**: ✅ ACTIF - Synchronisé et traitant les blocs
- **Testnet**: ⏸️ ARRÊTÉ - Configuration disponible mais non active 