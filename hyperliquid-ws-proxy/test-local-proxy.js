#!/usr/bin/env node

const WebSocket = require('ws');
const fs = require('fs');

// Configuration pour tester uniquement notre proxy en mode local node
const PROXY_WS = 'ws://localhost:8080/ws';  // Ou remplacer par l'IP du VPS

// Subscriptions supportées en mode local node
const LOCAL_NODE_SUBSCRIPTIONS = [
    // Les seules subscriptions que notre local node supporte actuellement
    { type: "allMids" },
    { type: "trades", coin: "BTC" },
    { type: "trades", coin: "ETH" },
    { type: "trades", coin: "SOL" },
    { type: "trades", coin: "AVAX" },
    { type: "trades", coin: "ATOM" },
];

class LocalProxyTester {
    constructor() {
        this.results = {};
        this.testTimeout = 30000; // 30 secondes par test
        this.messageCollectionTime = 15000; // 15 secondes pour collecter les messages
    }

    async testLocalProxy() {
        console.log('🧪 TEST DU PROXY WEBSOCKET EN MODE LOCAL NODE');
        console.log('=' .repeat(60));
        console.log(`🚀 Proxy local: ${PROXY_WS}`);
        console.log('=' .repeat(60));

        for (let i = 0; i < LOCAL_NODE_SUBSCRIPTIONS.length; i++) {
            const subscription = LOCAL_NODE_SUBSCRIPTIONS[i];
            console.log(`\n[${i + 1}/${LOCAL_NODE_SUBSCRIPTIONS.length}] Test: ${JSON.stringify(subscription)}`);
            
            try {
                const result = await this.testSubscription(subscription);
                this.results[this.getSubscriptionKey(subscription)] = result;
                
                if (result.success) {
                    console.log(`✅ SUCCESS: ${result.summary}`);
                } else {
                    console.log(`❌ FAILED: ${result.error}`);
                }
            } catch (error) {
                console.log(`💥 ERROR: ${error.message}`);
                this.results[this.getSubscriptionKey(subscription)] = {
                    success: false,
                    error: error.message
                };
            }
            
            // Pause entre les tests
            if (i < LOCAL_NODE_SUBSCRIPTIONS.length - 1) {
                console.log('⏳ Pause de 2 secondes...');
                await this.sleep(2000);
            }
        }

        this.printSummary();
    }

    async testSubscription(subscription) {
        return new Promise((resolve, reject) => {
            const ws = new WebSocket(PROXY_WS);
            let messages = [];
            let connected = false;
            
            const timeout = setTimeout(() => {
                ws.close();
                if (!connected) {
                    reject(new Error('Timeout de connexion au proxy'));
                } else if (messages.length === 0) {
                    reject(new Error('Aucun message reçu dans le délai imparti'));
                } else {
                    resolve({
                        success: true,
                        messageCount: messages.length,
                        summary: `${messages.length} messages reçus`,
                        sampleMessages: messages.slice(0, 2) // Premiers messages pour debug
                    });
                }
            }, this.testTimeout);

            ws.on('open', () => {
                console.log('🔗 Connexion établie');
                connected = true;
                
                // Envoyer la subscription
                const subscribeMessage = {
                    method: "subscribe",
                    subscription: subscription
                };
                
                ws.send(JSON.stringify(subscribeMessage));
                console.log(`📤 Subscription envoyée: ${JSON.stringify(subscription)}`);
                
                // Collecter les messages pendant un certain temps
                setTimeout(() => {
                    clearTimeout(timeout);
                    ws.close();
                    
                    if (messages.length > 0) {
                        resolve({
                            success: true,
                            messageCount: messages.length,
                            summary: `${messages.length} messages reçus en ${this.messageCollectionTime/1000}s`,
                            sampleMessages: messages.slice(0, 2),
                            uniqueSymbols: this.extractUniqueSymbols(messages, subscription.type)
                        });
                    } else {
                        reject(new Error('Aucun message reçu'));
                    }
                }, this.messageCollectionTime);
            });

            ws.on('message', (data) => {
                try {
                    const message = JSON.parse(data.toString());
                    messages.push(message);
                    
                    // Log le premier message de chaque type pour debug
                    if (messages.length === 1) {
                        console.log(`📩 Premier message reçu:`, JSON.stringify(message, null, 2).substring(0, 200) + '...');
                    } else if (messages.length % 5 === 0) {
                        console.log(`📊 ${messages.length} messages reçus jusqu'à présent...`);
                    }
                } catch (error) {
                    console.log(`⚠️  Message non-JSON reçu: ${data.toString().substring(0, 100)}`);
                }
            });

            ws.on('error', (error) => {
                clearTimeout(timeout);
                reject(new Error(`Erreur WebSocket: ${error.message}`));
            });

            ws.on('close', (code, reason) => {
                console.log(`🔌 Connexion fermée (code: ${code}, raison: ${reason || 'aucune'})`);
            });
        });
    }

    extractUniqueSymbols(messages, subscriptionType) {
        const symbols = new Set();
        
        messages.forEach(msg => {
            if (subscriptionType === 'allMids' && msg.mids) {
                Object.keys(msg.mids).forEach(symbol => symbols.add(symbol));
            } else if (subscriptionType === 'trades' && msg.coin) {
                symbols.add(msg.coin);
            }
        });
        
        return Array.from(symbols);
    }

    getSubscriptionKey(subscription) {
        let key = subscription.type;
        if (subscription.coin) key += `_${subscription.coin}`;
        if (subscription.user) key += `_${subscription.user}`;
        return key;
    }

    printSummary() {
        console.log('\n' + '=' .repeat(60));
        console.log('📋 RÉSUMÉ DES TESTS');
        console.log('=' .repeat(60));

        let successfulTests = 0;
        let totalTests = 0;

        for (const [key, result] of Object.entries(this.results)) {
            totalTests++;
            if (result.success) {
                successfulTests++;
                console.log(`✅ ${key}: ${result.summary}`);
                if (result.uniqueSymbols && result.uniqueSymbols.length > 0) {
                    console.log(`   📈 Symboles détectés: ${result.uniqueSymbols.join(', ')}`);
                }
            } else {
                console.log(`❌ ${key}: ${result.error}`);
            }
        }

        console.log('\n' + '-' .repeat(60));
        console.log(`📈 TAUX DE RÉUSSITE: ${successfulTests}/${totalTests} (${Math.round(successfulTests/totalTests*100)}%)`);
        
        if (successfulTests === totalTests) {
            console.log('🎉 PARFAIT! Votre proxy local fonctionne correctement!');
            console.log('💡 Vous pouvez maintenant connecter vos applications à: ' + PROXY_WS);
        } else {
            console.log('⚠️  Des problèmes ont été détectés - vérifiez les logs du service');
        }

        // Sauvegarder les résultats
        fs.writeFileSync('test-local-results.json', JSON.stringify(this.results, null, 2));
        console.log('💾 Résultats sauvés dans test-local-results.json');
        
        console.log('\n📚 ENDPOINTS DISPONIBLES:');
        console.log(`   WebSocket: ws://YOUR_VPS_IP:8080/ws`);
        console.log(`   Health:    http://YOUR_VPS_IP:8080/health`);
        console.log(`   Stats:     http://YOUR_VPS_IP:8080/stats`);
    }

    sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }
}

// Vérifier si WebSocket est installé
try {
    require('ws');
} catch (error) {
    console.error('❌ Le module "ws" n\'est pas installé.');
    console.error('📦 Installez-le avec: npm install ws');
    process.exit(1);
}

// Lancer les tests
if (require.main === module) {
    const tester = new LocalProxyTester();
    tester.testLocalProxy().catch(error => {
        console.error('💥 Erreur fatale:', error);
        process.exit(1);
    });
}

module.exports = LocalProxyTester; 