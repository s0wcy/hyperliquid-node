# Configuration du Pare-feu pour Node Hyperliquid

## Problème
Le node Hyperliquid nécessite que les ports 4001 et 4002 soient accessibles publiquement pour le protocole gossip. Sans cela, le node sera déprioritisé par les pairs du réseau.

## Configuration Windows Firewall

### 1. Ouvrir les ports via PowerShell (Administrateur)

```powershell
# Ouvrir le port 4001 (TCP)
New-NetFirewallRule -DisplayName "Hyperliquid Node 4001 TCP" -Direction Inbound -Protocol TCP -LocalPort 4001 -Action Allow

# Ouvrir le port 4002 (TCP)
New-NetFirewallRule -DisplayName "Hyperliquid Node 4002 TCP" -Direction Inbound -Protocol TCP -LocalPort 4002 -Action Allow

# Ouvrir les ports 4003-4006 (optionnel, pour les futures versions)
New-NetFirewallRule -DisplayName "Hyperliquid Node 4003-4006 TCP" -Direction Inbound -Protocol TCP -LocalPort 4003-4006 -Action Allow
```

### 2. Configuration via l'interface graphique

1. Ouvrir **Pare-feu Windows Defender avec sécurité avancée**
2. Cliquer sur **Règles de trafic entrant**
3. Cliquer sur **Nouvelle règle...**
4. Sélectionner **Port** → Suivant
5. Sélectionner **TCP** et spécifier **4001** → Suivant
6. Sélectionner **Autoriser la connexion** → Suivant
7. Cocher **Domaine**, **Privé**, et **Public** → Suivant
8. Nommer la règle "Hyperliquid Node 4001" → Terminer
9. Répéter pour le port 4002

## Configuration du Routeur

### Redirection de ports (NAT)
Si votre machine est derrière un routeur, vous devez configurer la redirection de ports :

1. Accéder à l'interface d'administration de votre routeur (généralement 192.168.1.1 ou 192.168.0.1)
2. Naviguer vers **Port Forwarding** ou **Redirection de ports**
3. Ajouter les règles suivantes :
   - **Port externe** : 4001, **Port interne** : 4001, **IP** : [IP de votre machine]
   - **Port externe** : 4002, **Port interne** : 4002, **IP** : [IP de votre machine]

### Trouver l'IP locale de votre machine

```powershell
ipconfig | findstr "IPv4"
```

## Vérification

### 1. Tester l'ouverture des ports localement

```powershell
# Vérifier que Docker écoute sur les ports
netstat -an | findstr ":400"
```

### 2. Tester depuis l'extérieur

Utilisez un service comme [PortChecker.co](https://portchecker.co) pour vérifier que vos ports sont accessibles depuis Internet.

## Configuration pour différents environnements

### Environnement de développement local
- Utilisez `docker-compose.ports.yml`
- Configuration minimale du pare-feu
- Pas besoin de redirection de ports

### Node de production
- Configuration complète du pare-feu ET du routeur
- Adresse IP statique recommandée
- Monitoring de la connectivité

## Commandes de diagnostic

```powershell
# Vérifier les règles de pare-feu actives
Get-NetFirewallRule | Where-Object {$_.DisplayName -like "*Hyperliquid*"}

# Vérifier les ports en écoute
netstat -an | findstr ":400"

# Tester la connectivité vers un pair
Test-NetConnection -ComputerName 54.248.41.39 -Port 4001
```

## Dépannage

### Le node ne se connecte toujours pas
1. Vérifiez que Docker expose bien les ports : `docker port hyperliquid-ports-node-1`
2. Vérifiez les règles de pare-feu : `Get-NetFirewallRule`
3. Testez la connectivité sortante : `Test-NetConnection -ComputerName 54.248.41.39 -Port 4001`
4. Vérifiez les logs du node pour les erreurs de connexion

### Erreurs "early eof"
Ces erreurs sont normales au début. Si elles persistent après 10-15 minutes, vérifiez :
- La configuration du pare-feu
- La redirection de ports du routeur
- La stabilité de la connexion Internet

## Sécurité

⚠️ **Important** : Ouvrir des ports publiquement expose votre machine à Internet. Assurez-vous que :
- Seuls les ports nécessaires (4001-4002) sont ouverts
- Votre système est à jour avec les derniers correctifs de sécurité
- Vous surveillez les logs pour détecter toute activité suspecte 