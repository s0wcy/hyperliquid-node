#!/bin/bash
DATA_PATH="/home/hluser/hl/data"

# Configuration pour VPS avec espace limité
REPLICA_CMDS_RETENTION_HOURS=24        # Garder 24h de replica_cmds
PERIODIC_ABCI_RETENTION_DAYS=2         # Garder 2 jours de periodic_abci_states
DAILY_EVM_RETENTION_DAYS=3             # Garder 3 jours de daily_evm_checkpoints

# Folders to exclude from general pruning
EXCLUDES=("visor_child_stderr" "rate_limited_ips" "node_logs")

echo "$(date): COMPLETE Prune script started" >> /proc/1/fd/1
echo "$(date): Retention: replica_cmds=${REPLICA_CMDS_RETENTION_HOURS}h, periodic_abci=${PERIODIC_ABCI_RETENTION_DAYS}d, daily_evm=${DAILY_EVM_RETENTION_DAYS}d" >> /proc/1/fd/1

# Check if data directory exists
if [ ! -d "$DATA_PATH" ]; then
    echo "$(date): Error: Data directory $DATA_PATH does not exist." >> /proc/1/fd/1
    exit 1
fi

# Get directory size before pruning
size_before=$(du -sh "$DATA_PATH" | cut -f1)
echo "$(date): Size before pruning: $size_before" >> /proc/1/fd/1

# === 1. NETTOYER REPLICA_CMDS (comme avant) ===
echo "$(date): Cleaning replica_cmds (older than ${REPLICA_CMDS_RETENTION_HOURS}h)..." >> /proc/1/fd/1
EXCLUDE_EXPR=""
for name in "${EXCLUDES[@]}"; do
    EXCLUDE_EXPR+=" ! -name \"$name\""
done

MINUTES=$((60*$REPLICA_CMDS_RETENTION_HOURS))
if [ -d "$DATA_PATH/replica_cmds" ]; then
    replica_before=$(du -sh "$DATA_PATH/replica_cmds" 2>/dev/null | cut -f1 || echo "0")
    eval "find \"$DATA_PATH/replica_cmds\" -mindepth 1 -depth -mmin +$MINUTES -type f $EXCLUDE_EXPR -delete" 2>/dev/null
    find "$DATA_PATH/replica_cmds" -mindepth 1 -depth -type d -empty -delete 2>/dev/null || true
    replica_after=$(du -sh "$DATA_PATH/replica_cmds" 2>/dev/null | cut -f1 || echo "0")
    echo "$(date): replica_cmds: $replica_before -> $replica_after" >> /proc/1/fd/1
fi

# === 2. NETTOYER PERIODIC_ABCI_STATES (NOUVEAU) ===
echo "$(date): Cleaning periodic_abci_states (keeping last ${PERIODIC_ABCI_RETENTION_DAYS} days)..." >> /proc/1/fd/1
if [ -d "$DATA_PATH/periodic_abci_states" ]; then
    abci_before=$(du -sh "$DATA_PATH/periodic_abci_states" 2>/dev/null | cut -f1 || echo "0")
    
    # Aller dans le répertoire et supprimer les anciens répertoires (par date de nom)
    cd "$DATA_PATH/periodic_abci_states" 2>/dev/null && {
        # Lister tous les répertoires, trier par nom (date), garder seulement les N derniers
        ls -1d */ 2>/dev/null | sort | head -n -${PERIODIC_ABCI_RETENTION_DAYS} | while read dir; do
            if [ -n "$dir" ] && [ "$dir" != "./" ] && [ "$dir" != "../" ]; then
                echo "$(date): Removing periodic_abci_states/$dir" >> /proc/1/fd/1
                rm -rf "$dir" 2>/dev/null || true
            fi
        done
    }
    
    abci_after=$(du -sh "$DATA_PATH/periodic_abci_states" 2>/dev/null | cut -f1 || echo "0")
    echo "$(date): periodic_abci_states: $abci_before -> $abci_after" >> /proc/1/fd/1
fi

# === 3. NETTOYER DAILY_EVM_CHECKPOINTS (NOUVEAU) ===
echo "$(date): Cleaning daily_evm_checkpoints (keeping last ${DAILY_EVM_RETENTION_DAYS} days)..." >> /proc/1/fd/1
if [ -d "$DATA_PATH/daily_evm_checkpoints" ]; then
    evm_before=$(du -sh "$DATA_PATH/daily_evm_checkpoints" 2>/dev/null | cut -f1 || echo "0")
    
    cd "$DATA_PATH/daily_evm_checkpoints" 2>/dev/null && {
        ls -1d */ 2>/dev/null | sort | head -n -${DAILY_EVM_RETENTION_DAYS} | while read dir; do
            if [ -n "$dir" ] && [ "$dir" != "./" ] && [ "$dir" != "../" ]; then
                echo "$(date): Removing daily_evm_checkpoints/$dir" >> /proc/1/fd/1
                rm -rf "$dir" 2>/dev/null || true
            fi
        done
    }
    
    evm_after=$(du -sh "$DATA_PATH/daily_evm_checkpoints" 2>/dev/null | cut -f1 || echo "0")
    echo "$(date): daily_evm_checkpoints: $evm_before -> $evm_after" >> /proc/1/fd/1
fi

# === 4. NETTOYER AUTRES FICHIERS ANCIENS ===
echo "$(date): Cleaning other old files..." >> /proc/1/fd/1
# Supprimer les autres fichiers/répertoires anciens de plus de 48h
find "$DATA_PATH" -maxdepth 1 -mindepth 1 -mtime +2 -type f -delete 2>/dev/null || true
find "$DATA_PATH" -maxdepth 1 -mindepth 1 -mtime +2 -type d ! -name "replica_cmds" ! -name "periodic_abci_states" ! -name "daily_evm_checkpoints" -exec rm -rf {} + 2>/dev/null || true

# Get final size
size_after=$(du -sh "$DATA_PATH" | cut -f1)
echo "$(date): COMPLETE Pruning finished: $size_before -> $size_after" >> /proc/1/fd/1

# Check disk usage and warn if still high
USAGE=$(df / | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $USAGE -gt 85 ]; then
    echo "$(date): WARNING: Disk usage still high: ${USAGE}%" >> /proc/1/fd/1
    if [ $USAGE -gt 95 ]; then
        echo "$(date): CRITICAL: Disk usage critical: ${USAGE}%" >> /proc/1/fd/1
    fi
else
    echo "$(date): INFO: Disk usage acceptable: ${USAGE}%" >> /proc/1/fd/1
fi

echo "$(date): Complete pruning process finished successfully" >> /proc/1/fd/1 