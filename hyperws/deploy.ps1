# Script de déploiement HyperWS pour Windows
# Automatise la compilation et le déploiement

param(
    [string]$Action = "build",
    [string]$Config = "config.yaml",
    [switch]$Docker = $false,
    [switch]$Help = $false
)

function Show-Help {
    Write-Host ""
    Write-Host "Script de déploiement HyperWS" -ForegroundColor Green
    Write-Host "=============================" -ForegroundColor Green
    Write-Host ""
    Write-Host "Usage:" -ForegroundColor Yellow
    Write-Host "  .\deploy.ps1 [OPTIONS]"
    Write-Host ""
    Write-Host "Actions:" -ForegroundColor Yellow
    Write-Host "  build    - Compiler l'application (défaut)"
    Write-Host "  run      - Compiler et exécuter"
    Write-Host "  docker   - Construire l'image Docker"
    Write-Host "  deploy   - Déployer avec Docker Compose"
    Write-Host "  clean    - Nettoyer les fichiers générés"
    Write-Host ""
    Write-Host "Options:" -ForegroundColor Yellow
    Write-Host "  -Config  - Chemin vers le fichier de configuration (défaut: config.yaml)"
    Write-Host "  -Docker  - Utiliser Docker pour le déploiement"
    Write-Host "  -Help    - Afficher cette aide"
    Write-Host ""
    Write-Host "Exemples:" -ForegroundColor Yellow
    Write-Host "  .\deploy.ps1 build"
    Write-Host "  .\deploy.ps1 run -Config custom-config.yaml"
    Write-Host "  .\deploy.ps1 docker"
    Write-Host "  .\deploy.ps1 deploy"
    Write-Host ""
}

function Test-Prerequisites {
    Write-Host "Vérification des prérequis..." -ForegroundColor Blue
    
    # Vérifier Go
    try {
        $goVersion = go version 2>$null
        if ($goVersion) {
            Write-Host "✓ Go installé: $goVersion" -ForegroundColor Green
        } else {
            throw "Go non trouvé"
        }
    } catch {
        Write-Host "✗ Go n'est pas installé ou pas dans le PATH" -ForegroundColor Red
        Write-Host "  Téléchargez Go depuis: https://golang.org/dl/" -ForegroundColor Yellow
        return $false
    }
    
    # Vérifier Docker si nécessaire
    if ($Docker -or $Action -eq "docker" -or $Action -eq "deploy") {
        try {
            $dockerVersion = docker --version 2>$null
            if ($dockerVersion) {
                Write-Host "✓ Docker installé: $dockerVersion" -ForegroundColor Green
            } else {
                throw "Docker non trouvé"
            }
        } catch {
            Write-Host "✗ Docker n'est pas installé ou pas démarré" -ForegroundColor Red
            Write-Host "  Installez Docker Desktop: https://www.docker.com/products/docker-desktop" -ForegroundColor Yellow
            return $false
        }
    }
    
    return $true
}

function Build-Application {
    Write-Host "Compilation de HyperWS..." -ForegroundColor Blue
    
    try {
        # Nettoyer les dépendances
        Write-Host "Nettoyage des dépendances..."
        go mod tidy
        
        # Télécharger les dépendances
        Write-Host "Téléchargement des dépendances..."
        go mod download
        
        # Compiler
        Write-Host "Compilation..."
        if ($IsWindows -or $env:OS -eq "Windows_NT") {
            go build -o hyperws.exe .
            $executable = "hyperws.exe"
        } else {
            go build -o hyperws .
            $executable = "hyperws"
        }
        
        if (Test-Path $executable) {
            Write-Host "✓ Compilation réussie: $executable" -ForegroundColor Green
            return $executable
        } else {
            throw "L'exécutable n'a pas été créé"
        }
    } catch {
        Write-Host "✗ Erreur de compilation: $_" -ForegroundColor Red
        return $null
    }
}

function Run-Application {
    param([string]$Executable)
    
    if (-not $Executable -or -not (Test-Path $Executable)) {
        Write-Host "✗ Exécutable non trouvé" -ForegroundColor Red
        return
    }
    
    Write-Host "Démarrage de HyperWS..." -ForegroundColor Blue
    Write-Host "Configuration: $Config" -ForegroundColor Yellow
    Write-Host "Appuyez sur Ctrl+C pour arrêter" -ForegroundColor Yellow
    Write-Host ""
    
    try {
        if ($Config -and (Test-Path $Config)) {
            & ".\$Executable" -config $Config
        } else {
            & ".\$Executable"
        }
    } catch {
        Write-Host "✗ Erreur d'exécution: $_" -ForegroundColor Red
    }
}

function Build-DockerImage {
    Write-Host "Construction de l'image Docker..." -ForegroundColor Blue
    
    try {
        docker build -t hyperws:latest .
        Write-Host "✓ Image Docker construite: hyperws:latest" -ForegroundColor Green
    } catch {
        Write-Host "✗ Erreur de construction Docker: $_" -ForegroundColor Red
    }
}

function Deploy-DockerCompose {
    Write-Host "Déploiement avec Docker Compose..." -ForegroundColor Blue
    
    if (-not (Test-Path "docker-compose.yml")) {
        Write-Host "✗ docker-compose.yml non trouvé" -ForegroundColor Red
        return
    }
    
    try {
        Write-Host "Démarrage des services..."
        docker-compose up -d
        
        Write-Host ""
        Write-Host "✓ HyperWS déployé avec succès!" -ForegroundColor Green
        Write-Host ""
        Write-Host "Endpoints disponibles:" -ForegroundColor Yellow
        Write-Host "  WebSocket: ws://localhost:8080/ws"
        Write-Host "  Santé:     http://localhost:8080/health"
        Write-Host "  Stats:     http://localhost:8080/stats"
        Write-Host ""
        Write-Host "Commandes utiles:" -ForegroundColor Yellow
        Write-Host "  docker-compose logs hyperws    # Voir les logs"
        Write-Host "  docker-compose down           # Arrêter le service"
        Write-Host "  docker-compose restart hyperws # Redémarrer"
        
    } catch {
        Write-Host "✗ Erreur de déploiement: $_" -ForegroundColor Red
    }
}

function Clean-Files {
    Write-Host "Nettoyage des fichiers générés..." -ForegroundColor Blue
    
    $filesToClean = @("hyperws.exe", "hyperws", "go.sum")
    
    foreach ($file in $filesToClean) {
        if (Test-Path $file) {
            Remove-Item $file -Force
            Write-Host "✓ Supprimé: $file" -ForegroundColor Green
        }
    }
    
    Write-Host "✓ Nettoyage terminé" -ForegroundColor Green
}

# Script principal
if ($Help) {
    Show-Help
    exit 0
}

Write-Host ""
Write-Host "HyperWS - Proxy WebSocket Hyperliquid" -ForegroundColor Cyan
Write-Host "=====================================" -ForegroundColor Cyan
Write-Host ""

# Vérifier les prérequis
if (-not (Test-Prerequisites)) {
    exit 1
}

# Exécuter l'action demandée
switch ($Action.ToLower()) {
    "build" {
        $executable = Build-Application
        if ($executable) {
            Write-Host ""
            Write-Host "Pour exécuter: .\$executable" -ForegroundColor Yellow
        }
    }
    
    "run" {
        $executable = Build-Application
        if ($executable) {
            Write-Host ""
            Run-Application $executable
        }
    }
    
    "docker" {
        Build-DockerImage
    }
    
    "deploy" {
        Deploy-DockerCompose
    }
    
    "clean" {
        Clean-Files
    }
    
    default {
        Write-Host "Action inconnue: $Action" -ForegroundColor Red
        Write-Host "Utilisez -Help pour voir les options disponibles" -ForegroundColor Yellow
        exit 1
    }
} 