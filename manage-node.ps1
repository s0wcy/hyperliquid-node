# Script de gestion du node Hyperliquid
param(
    [Parameter(Mandatory=$false)]
    [ValidateSet("start", "stop", "restart", "logs", "status", "testnet", "mainnet")]
    [string]$Action = "status"
)

Write-Host "=== Gestionnaire de Node Hyperliquid ===" -ForegroundColor Green

switch ($Action) {
    "start" {
        Write-Host "Démarrage du node Hyperliquid..." -ForegroundColor Yellow
        docker-compose up -d
        Write-Host "Node démarré avec succès!" -ForegroundColor Green
    }
    
    "stop" {
        Write-Host "Arrêt du node Hyperliquid..." -ForegroundColor Yellow
        docker-compose down
        Write-Host "Node arrêté!" -ForegroundColor Green
    }
    
    "restart" {
        Write-Host "Redémarrage du node Hyperliquid..." -ForegroundColor Yellow
        docker-compose down
        docker-compose up -d
        Write-Host "Node redémarré avec succès!" -ForegroundColor Green
    }
    
    "logs" {
        Write-Host "Affichage des logs du node..." -ForegroundColor Yellow
        docker-compose logs -f node
    }
    
    "status" {
        Write-Host "Statut des containers:" -ForegroundColor Yellow
        docker-compose ps
        
        Write-Host "`nLogs récents:" -ForegroundColor Yellow
        docker-compose logs --tail=20 node
    }
    
    "testnet" {
        Write-Host "Configuration pour Testnet..." -ForegroundColor Yellow
        docker-compose -f docker-compose.mainnet.yml down
        docker-compose down
        docker-compose build --no-cache
        docker-compose up -d
        Write-Host "Node configuré pour Testnet!" -ForegroundColor Green
    }
    
    "mainnet" {
        Write-Host "Configuration pour Mainnet (Recommandée)..." -ForegroundColor Yellow
        docker-compose down
        docker-compose -f docker-compose.mainnet.yml down
        docker-compose -f docker-compose.mainnet.yml build --no-cache
        docker-compose -f docker-compose.mainnet.yml up -d
        Write-Host "Node configuré pour Mainnet!" -ForegroundColor Green
    }
}

Write-Host "`nUtilisation:" -ForegroundColor Cyan
Write-Host "  .\manage-node.ps1 start     - Démarrer le node" -ForegroundColor White
Write-Host "  .\manage-node.ps1 stop      - Arrêter le node" -ForegroundColor White
Write-Host "  .\manage-node.ps1 restart   - Redémarrer le node" -ForegroundColor White
Write-Host "  .\manage-node.ps1 logs      - Voir les logs en temps réel" -ForegroundColor White
Write-Host "  .\manage-node.ps1 status    - Voir le statut actuel" -ForegroundColor White
Write-Host "  .\manage-node.ps1 testnet   - Configurer pour Testnet" -ForegroundColor White
Write-Host "  .\manage-node.ps1 mainnet   - Configurer pour Mainnet" -ForegroundColor White 