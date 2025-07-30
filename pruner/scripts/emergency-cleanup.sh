#!/bin/bash
DATA_PATH="/home/hluser/hl/data"

echo "$(date): EMERGENCY CLEANUP started - This will free significant disk space!" >> /proc/1/fd/1

# Get initial size
size_before=$(du -sh "$DATA_PATH" | cut -f1)
echo "$(date): Initial size: $size_before" >> /proc/1/fd/1

# === EMERGENCY: Keep only YESTERDAY for periodic_abci_states ===
echo "$(date): EMERGENCY: Cleaning periodic_abci_states (keeping only 1 day)..." >> /proc/1/fd/1
if [ -d "$DATA_PATH/periodic_abci_states" ]; then
    abci_before=$(du -sh "$DATA_PATH/periodic_abci_states" 2>/dev/null | cut -f1 || echo "0")
    
    cd "$DATA_PATH/periodic_abci_states" 2>/dev/null && {
        # Garder seulement le répertoire le plus récent
        ls -1d */ 2>/dev/null | sort | head -n -1 | while read dir; do
            if [ -n "$dir" ]; then
                echo "$(date): EMERGENCY: Removing periodic_abci_states/$dir" >> /proc/1/fd/1
                rm -rf "$dir" 2>/dev/null || true
            fi
        done
    }
    
    abci_after=$(du -sh "$DATA_PATH/periodic_abci_states" 2>/dev/null | cut -f1 || echo "0")
    echo "$(date): periodic_abci_states: $abci_before -> $abci_after" >> /proc/1/fd/1
fi

# === EMERGENCY: Keep only 2 days for daily_evm_checkpoints ===
echo "$(date): EMERGENCY: Cleaning daily_evm_checkpoints (keeping only 2 days)..." >> /proc/1/fd/1
if [ -d "$DATA_PATH/daily_evm_checkpoints" ]; then
    evm_before=$(du -sh "$DATA_PATH/daily_evm_checkpoints" 2>/dev/null | cut -f1 || echo "0")
    
    cd "$DATA_PATH/daily_evm_checkpoints" 2>/dev/null && {
        ls -1d */ 2>/dev/null | sort | head -n -2 | while read dir; do
            if [ -n "$dir" ]; then
                echo "$(date): EMERGENCY: Removing daily_evm_checkpoints/$dir" >> /proc/1/fd/1
                rm -rf "$dir" 2>/dev/null || true
            fi
        done
    }
    
    evm_after=$(du -sh "$DATA_PATH/daily_evm_checkpoints" 2>/dev/null | cut -f1 || echo "0")
    echo "$(date): daily_evm_checkpoints: $evm_before -> $evm_after" >> /proc/1/fd/1
fi

# === EMERGENCY: Aggressive replica_cmds cleanup (12h only) ===
echo "$(date): EMERGENCY: Aggressive replica_cmds cleanup (12h retention)..." >> /proc/1/fd/1
if [ -d "$DATA_PATH/replica_cmds" ]; then
    replica_before=$(du -sh "$DATA_PATH/replica_cmds" 2>/dev/null | cut -f1 || echo "0")
    
    # Supprimer tout ce qui est plus vieux que 12h (720 minutes)
    find "$DATA_PATH/replica_cmds" -mindepth 1 -depth -mmin +720 -type f ! -name "visor_child_stderr" -delete 2>/dev/null || true
    find "$DATA_PATH/replica_cmds" -mindepth 1 -depth -type d -empty -delete 2>/dev/null || true
    
    replica_after=$(du -sh "$DATA_PATH/replica_cmds" 2>/dev/null | cut -f1 || echo "0")
    echo "$(date): replica_cmds: $replica_before -> $replica_after" >> /proc/1/fd/1
fi

# Get final size
size_after=$(du -sh "$DATA_PATH" | cut -f1)
echo "$(date): EMERGENCY CLEANUP completed: $size_before -> $size_after" >> /proc/1/fd/1

# Final disk check
USAGE=$(df / | tail -1 | awk '{print $5}' | sed 's/%//')
echo "$(date): Final disk usage: ${USAGE}%" >> /proc/1/fd/1

if [ $USAGE -lt 80 ]; then
    echo "$(date): SUCCESS: Disk usage now acceptable: ${USAGE}%" >> /proc/1/fd/1
elif [ $USAGE -lt 90 ]; then
    echo "$(date): WARNING: Disk usage improved but still high: ${USAGE}%" >> /proc/1/fd/1
else
    echo "$(date): CRITICAL: Disk usage still critical: ${USAGE}% - Consider upgrading VPS" >> /proc/1/fd/1
fi 