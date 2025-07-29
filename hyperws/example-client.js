#!/usr/bin/env node

/**
 * Exemple de client JavaScript pour tester HyperWS
 * 
 * Usage:
 *   node example-client.js
 * 
 * Prérequis:
 *   npm install ws
 */

const WebSocket = require('ws');

class HyperWSClient {
    constructor(url = 'ws://localhost:8080/ws') {
        this.url = url;
        this.ws = null;
        this.subscriptions = new Map();
    }

    connect() {
        console.log('🔗 Connexion à HyperWS:', this.url);
        
        this.ws = new WebSocket(this.url);
        
        this.ws.on('open', () => {
            console.log('✅ Connecté à HyperWS');
            this.onConnected();
        });
        
        this.ws.on('message', (data) => {
            try {
                const message = JSON.parse(data.toString());
                this.handleMessage(message);
            } catch (error) {
                console.error('❌ Erreur parsing message:', error);
            }
        });
        
        this.ws.on('close', () => {
            console.log('🔌 Connexion fermée');
        });
        
        this.ws.on('error', (error) => {
            console.error('❌ Erreur WebSocket:', error.message);
        });
    }

    onConnected() {
        console.log('\n📡 Test des souscriptions...\n');
        
        // Test 1: Souscrire aux prix moyens de tous les assets
        this.subscribe('allMids', {
            type: 'allMids'
        });
        
        // Test 2: Souscrire aux trades de BTC
        setTimeout(() => {
            this.subscribe('trades-BTC', {
                type: 'trades',
                coin: 'BTC'
            });
        }, 2000);
        
        // Test 3: Souscrire aux trades d'ETH
        setTimeout(() => {
            this.subscribe('trades-ETH', {
                type: 'trades',
                coin: 'ETH'
            });
        }, 4000);
        
        // Test 4: Désouscrire après 30 secondes
        setTimeout(() => {
            console.log('\n🔄 Test de désouscription...\n');
            this.unsubscribe('trades-BTC', {
                type: 'trades',
                coin: 'BTC'
            });
        }, 30000);
    }

    subscribe(id, subscription) {
        const message = {
            method: 'subscribe',
            subscription: subscription
        };
        
        console.log(`📥 Souscription [${id}]:`, subscription);
        this.subscriptions.set(id, subscription);
        this.send(message);
    }

    unsubscribe(id, subscription) {
        const message = {
            method: 'unsubscribe',
            subscription: subscription
        };
        
        console.log(`📤 Désouscription [${id}]:`, subscription);
        this.subscriptions.delete(id);
        this.send(message);
    }

    send(message) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
        } else {
            console.error('❌ WebSocket non connecté');
        }
    }

    handleMessage(message) {
        switch (message.channel) {
            case 'subscriptionResponse':
                this.handleSubscriptionResponse(message);
                break;
                
            case 'allMids':
                this.handleAllMids(message);
                break;
                
            case 'trades':
                this.handleTrades(message);
                break;
                
            default:
                console.log('📨 Message reçu:', message);
        }
    }

    handleSubscriptionResponse(message) {
        try {
            const data = JSON.parse(message.data);
            const method = data.method;
            const sub = data.subscription;
            
            if (method === 'subscribe') {
                console.log(`✅ Souscription confirmée: ${sub.type}${sub.coin ? ` (${sub.coin})` : ''}`);
            } else if (method === 'unsubscribe') {
                console.log(`✅ Désouscription confirmée: ${sub.type}${sub.coin ? ` (${sub.coin})` : ''}`);
            }
        } catch (error) {
            console.error('❌ Erreur parsing réponse souscription:', error);
        }
    }

    handleAllMids(message) {
        try {
            const data = typeof message.data === 'string' ? JSON.parse(message.data) : message.data;
            const pricesCount = Object.keys(data.mids || {}).length;
            
            console.log(`💰 AllMids reçu: ${pricesCount} prix`);
            
            // Afficher quelques prix pour exemple
            if (data.mids) {
                const samples = Object.entries(data.mids).slice(0, 5);
                samples.forEach(([coin, price]) => {
                    console.log(`  ${coin}: $${price}`);
                });
                if (pricesCount > 5) {
                    console.log(`  ... et ${pricesCount - 5} autres`);
                }
            }
            console.log('');
        } catch (error) {
            console.error('❌ Erreur parsing allMids:', error);
        }
    }

    handleTrades(message) {
        try {
            const data = typeof message.data === 'string' ? JSON.parse(message.data) : message.data;
            
            console.log(`🔄 Trade reçu: ${data.coin} - ${data.side} ${data.sz} @ $${data.px}`);
        } catch (error) {
            console.error('❌ Erreur parsing trade:', error);
        }
    }

    disconnect() {
        if (this.ws) {
            this.ws.close();
        }
    }
}

// Vérifier les dépendances
try {
    require('ws');
} catch (error) {
    console.error('❌ Module "ws" non installé. Exécutez: npm install ws');
    process.exit(1);
}

// Créer et connecter le client
const client = new HyperWSClient();

// Gérer l'arrêt propre
process.on('SIGINT', () => {
    console.log('\n👋 Arrêt du client...');
    client.disconnect();
    process.exit(0);
});

// Démarrer le test
console.log('🚀 Démarrage du client de test HyperWS\n');
client.connect();

// Garder le processus actif
setTimeout(() => {
    console.log('\n⏰ Test terminé après 60 secondes');
    client.disconnect();
    process.exit(0);
}, 60000); 