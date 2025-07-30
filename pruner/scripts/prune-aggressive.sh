#!/bin/bash
DATA_PATH="/home/hluser/hl/data"

# Configuration agressive pour VPS avec espace limité
# Garder seulement 24h de données au lieu de 48h
RETENTION_HOURS=24

# Folders to exclude from pruning
EXCLUDES=("visor_child_stderr" "rate_limited_ips")

# Log startup for debugging
echo "$(date): AGGRESSIVE Prune script started (${RETENTION_HOURS}h retention)" >> /proc/1/fd/1

# Check if data directory exists
if [ ! -d "$DATA_PATH" ]; then
    echo "$(date): Error: Data directory $DATA_PATH does not exist." >> /proc/1/fd/1
    exit 1
fi

echo "$(date): Starting AGGRESSIVE pruning process" >> /proc/1/fd/1

# Get directory size before pruning
size_before=$(du -sh "$DATA_PATH" | cut -f1)
files_before=$(find "$DATA_PATH" -type f | wc -l)
echo "$(date): Size before pruning: $size_before with $files_before files" >> /proc/1/fd/1

# Build the exclusion part of the find command  
EXCLUDE_EXPR=""
for name in "${EXCLUDES[@]}"; do
    EXCLUDE_EXPR+=" ! -name \"$name\""
done

# Delete data older than specified hours
MINUTES=$((60*$RETENTION_HOURS))
echo "$(date): Deleting files older than $RETENTION_HOURS hours ($MINUTES minutes)" >> /proc/1/fd/1

# Supprimer les fichiers anciens
eval "find \"$DATA_PATH\" -mindepth 1 -depth -mmin +$MINUTES -type f $EXCLUDE_EXPR -delete"

# Supprimer les répertoires vides après suppression des fichiers
find "$DATA_PATH" -mindepth 1 -depth -type d -empty -delete 2>/dev/null || true

# Get directory size after pruning
size_after=$(du -sh "$DATA_PATH" | cut -f1)
files_after=$(find "$DATA_PATH" -type f | wc -l)
echo "$(date): Size after pruning: $size_after with $files_after files" >> /proc/1/fd/1

# Calculate space freed
echo "$(date): AGGRESSIVE Pruning completed." >> /proc/1/fd/1
echo "$(date): Reduced from $size_before to $size_after" >> /proc/1/fd/1
echo "$(date): Removed $(($files_before - $files_after)) files" >> /proc/1/fd/1

# Alert if disk usage is still high
USAGE=$(df / | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $USAGE -gt 90 ]; then
    echo "$(date): WARNING: Disk usage still high: ${USAGE}%" >> /proc/1/fd/1
fi 