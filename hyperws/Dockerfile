# === BUILD STAGE ===
FROM golang:1.21-alpine AS builder

# Installer les outils nécessaires
RUN apk add --no-cache git ca-certificates tzdata

# Répertoire de travail
WORKDIR /app

# Copier les fichiers de configuration Go
COPY go.mod go.sum ./

# Télécharger les dépendances
RUN go mod download

# Copier le code source
COPY . .

# Compiler l'application avec optimisations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o hyperws .

# === PRODUCTION STAGE ===
FROM alpine:latest

# Installer les certificats CA et le timezone
RUN apk --no-cache add ca-certificates tzdata wget

# Créer un utilisateur non-root pour la sécurité
RUN addgroup -g 1001 hyperws && \
    adduser -D -s /bin/sh -u 1001 -G hyperws hyperws

# Créer les répertoires nécessaires
RUN mkdir -p /app /data && \
    chown -R hyperws:hyperws /app /data

# Copier le binaire depuis le stage de build
COPY --from=builder --chown=hyperws:hyperws /app/hyperws /app/hyperws

# Copier le fichier de configuration par défaut
COPY --chown=hyperws:hyperws config.yaml /app/config.yaml

# Passer à l'utilisateur non-root
USER hyperws:hyperws

# Répertoire de travail
WORKDIR /app

# Exposer le port WebSocket
EXPOSE 8080

# Health check pour Docker
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Labels pour métadonnées
LABEL maintainer="HyperWS Team" \
      description="Proxy WebSocket optimisé pour Hyperliquid" \
      version="1.0.0"

# Point d'entrée
ENTRYPOINT ["./hyperws"]

# Arguments par défaut
CMD ["-config", "config.yaml"] 