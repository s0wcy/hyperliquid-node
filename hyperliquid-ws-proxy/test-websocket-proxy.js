#!/usr/bin/env node

const WebSocket = require('ws');
const fs = require('fs');

// Configuration des WebSockets
const HYPERLIQUID_WS = 'wss://api.hyperliquid.xyz/ws';
const PROXY_WS = 'ws://155.133.7.48:8080/ws';

// Toutes les subscriptions à tester
const TEST_SUBSCRIPTIONS = [
    // Données publiques
    { type: "allMids" },
    { type: "trades", coin: "SOL" },
    { type: "trades", coin: "BTC" },
    { type: "l2Book", coin: "SOL" },
    { type: "l2Book", coin: "BTC" },
    { type: "candle", coin: "SOL", interval: "1m" },
    { type: "candle", coin: "BTC", interval: "1m" },
    { type: "bbo", coin: "SOL" },
    { type: "bbo", coin: "BTC" },
    { type: "notification" },
    
    // Données utilisateur (nécessitent une adresse)
    // { type: "orderUpdates", user: "0x..." },
    // { type: "userEvents", user: "0x..." },
    // { type: "userFills", user: "0x..." },
    // { type: "userFundings", user: "0x..." },
    // { type: "activeAssetCtx", user: "0x..." },
    // { type: "activeAssetData", user: "0x..." },
];

class WebSocketTester {
    constructor() {
        this.results = {};
        this.connections = {};
        this.testTimeout = 45000; // 45 secondes par test (plus de temps pour collecter plusieurs messages)
        this.messageTimeout = 10000; // 10 secondes pour recevoir suffisamment de messages
    }

    async testAllSubscriptions() {
        console.log('🧪 DÉMARRAGE DU TEST DE PROXY WEBSOCKET HYPERLIQUID');
        console.log('=' .repeat(60));
        console.log(`📡 Hyperliquid officiel: ${HYPERLIQUID_WS}`);
        console.log(`🚀 Votre proxy:          ${PROXY_WS}`);
        console.log('=' .repeat(60));

        for (let i = 0; i < TEST_SUBSCRIPTIONS.length; i++) {
            const subscription = TEST_SUBSCRIPTIONS[i];
            console.log(`\n[${i + 1}/${TEST_SUBSCRIPTIONS.length}] Test: ${JSON.stringify(subscription)}`);
            
            try {
                await this.testSubscription(subscription);
            } catch (error) {
                console.log(`❌ ERREUR: ${error.message}`);
                this.results[this.getSubscriptionKey(subscription)] = {
                    success: false,
                    error: error.message
                };
            }
            
            // Délai entre les tests
            await this.sleep(2000);
        }

        this.printFinalResults();
    }

    async testSubscription(subscription) {
        const key = this.getSubscriptionKey(subscription);
        
        return new Promise(async (resolve, reject) => {
            const testData = {
                hyperliquid: { messages: [], connected: false },
                proxy: { messages: [], connected: false }
            };

            let timeoutId;
            let resolved = false;

            const cleanup = () => {
                if (timeoutId) clearTimeout(timeoutId);
                if (this.connections.hyperliquid) {
                    this.connections.hyperliquid.close();
                    delete this.connections.hyperliquid;
                }
                if (this.connections.proxy) {
                    this.connections.proxy.close();
                    delete this.connections.proxy;
                }
            };

            const finishTest = (success, reason) => {
                if (resolved) return;
                resolved = true;
                cleanup();
                
                this.results[key] = {
                    success,
                    reason,
                    hyperliquid: testData.hyperliquid,
                    proxy: testData.proxy
                };

                if (success) {
                    console.log(`✅ SUCCÈS: ${reason}`);
                    resolve();
                } else {
                    console.log(`❌ ÉCHEC: ${reason}`);
                    reject(new Error(reason));
                }
            };

            // Timeout global
            timeoutId = setTimeout(() => {
                finishTest(false, 'Timeout - aucune donnée reçue dans les 30 secondes');
            }, this.testTimeout);

            try {
                // Connexion à Hyperliquid
                this.connections.hyperliquid = new WebSocket(HYPERLIQUID_WS);
                
                this.connections.hyperliquid.on('open', () => {
                    testData.hyperliquid.connected = true;
                    console.log('📡 Connecté à Hyperliquid');
                    
                    this.connections.hyperliquid.send(JSON.stringify({
                        method: "subscribe",
                        subscription: subscription
                    }));
                });

                this.connections.hyperliquid.on('message', (data) => {
                    try {
                        const rawData = data.toString();
                        const messages = this.parseMultipleJsonMessages(rawData);
                        
                        for (const message of messages) {
                            testData.hyperliquid.messages.push(message);
                            
                            if (message.channel !== 'subscriptionResponse') {
                                console.log(`📨 Hyperliquid: ${message.channel || 'unknown'}`);
                            }
                        }
                    } catch (error) {
                        console.error(`❌ Erreur parsing Hyperliquid:`, error.message);
                        console.error(`❌ Raw data: ${data.toString().substring(0, 200)}...`);
                    }
                });

                this.connections.hyperliquid.on('error', (error) => {
                    console.error(`❌ Erreur connexion Hyperliquid: ${error.message}`);
                    finishTest(false, `Erreur Hyperliquid: ${error.message}`);
                });

                this.connections.hyperliquid.on('close', (code, reason) => {
                    console.warn(`⚠️  Connexion Hyperliquid fermée: ${code} - ${reason}`);
                });

                // Délai avant connexion au proxy
                await this.sleep(1000);

                // Connexion au proxy
                this.connections.proxy = new WebSocket(PROXY_WS);
                
                this.connections.proxy.on('open', () => {
                    testData.proxy.connected = true;
                    console.log('🚀 Connecté au proxy');
                    
                    this.connections.proxy.send(JSON.stringify({
                        method: "subscribe",
                        subscription: subscription
                    }));
                });

                this.connections.proxy.on('message', (data) => {
                    try {
                        const rawData = data.toString();
                        const messages = this.parseMultipleJsonMessages(rawData);
                        
                        for (const message of messages) {
                            testData.proxy.messages.push(message);
                            
                            if (message.channel !== 'subscriptionResponse') {
                                console.log(`📨 Proxy: ${message.channel || 'unknown'}`);
                            }
                        }
                        
                        // Vérifier si on a reçu des données des deux côtés
                        if (testData.hyperliquid.messages.length > 1 && testData.proxy.messages.length > 1) {
                            this.compareMessages(testData, subscription, finishTest);
                        }
                    } catch (error) {
                        console.error(`❌ Erreur parsing Proxy:`, error.message);
                        console.error(`❌ Raw data: ${data.toString().substring(0, 200)}...`);
                    }
                });

                this.connections.proxy.on('error', (error) => {
                    console.error(`❌ Erreur connexion proxy: ${error.message}`);
                    finishTest(false, `Erreur proxy: ${error.message}`);
                });

                this.connections.proxy.on('close', (code, reason) => {
                    console.warn(`⚠️  Connexion proxy fermée: ${code} - ${reason}`);
                });

            } catch (error) {
                finishTest(false, `Erreur de connexion: ${error.message}`);
            }
        });
    }

    compareMessages(testData, subscription, finishTest) {
        const hlMessages = testData.hyperliquid.messages.filter(m => m.channel !== 'subscriptionResponse');
        const proxyMessages = testData.proxy.messages.filter(m => m.channel !== 'subscriptionResponse');

        if (hlMessages.length === 0 || proxyMessages.length === 0) {
            return; // Pas assez de données
        }

        // Vérifier que les canaux correspondent
        const hlChannels = new Set(hlMessages.map(m => m.channel));
        const proxyChannels = new Set(proxyMessages.map(m => m.channel));

        const channelsMatch = this.setsEqual(hlChannels, proxyChannels);
        
        if (!channelsMatch) {
            finishTest(false, `Différence de canaux - HL: [${[...hlChannels]}], Proxy: [${[...proxyChannels]}]`);
            return;
        }

        // Attendre d'avoir suffisamment de messages pour matcher
        if (hlMessages.length < 3 || proxyMessages.length < 3) {
            return; // Attendre plus de données
        }

        // Synchroniser et comparer les messages correspondants
        const matchResult = this.findAndCompareMatchingMessages(hlMessages, proxyMessages, subscription.type);
        
        if (matchResult.found) {
            if (matchResult.comparison.identical) {
                finishTest(true, `✅ ÉVÉNEMENTS IDENTIQUES - ID: ${matchResult.matchId}, Canal: ${matchResult.channel}, ${matchResult.comparison.details}`);
            } else if (matchResult.comparison.structureMatch) {
                finishTest(true, `⚠️  Même événement, structure identique - ID: ${matchResult.matchId}, ${matchResult.comparison.details}`);
            } else {
                finishTest(false, `❌ Même événement mais différences importantes - ID: ${matchResult.matchId}, ${matchResult.comparison.error}`);
            }
        } else {
            finishTest(false, `❌ Aucun événement correspondant trouvé - Vérifiez ${matchResult.totalCompared} messages`);
        }
    }

    compareMessageData(msg1, msg2, subscriptionType) {
        // Vérifier les champs de base
        if (msg1.channel !== msg2.channel) {
            return { identical: false, structureMatch: false, error: `Canaux différents: ${msg1.channel} vs ${msg2.channel}` };
        }

        if (!msg1.data && !msg2.data) {
            return { identical: true, structureMatch: true, fieldsCompared: 0 };
        }

        if (!msg1.data || !msg2.data) {
            return { identical: false, structureMatch: false, error: 'Un message a des données, l\'autre non' };
        }

        const data1 = msg1.data;
        const data2 = msg2.data;

        // Comparer les structures (clés)
        const keys1 = Object.keys(data1).sort();
        const keys2 = Object.keys(data2).sort();
        
        if (JSON.stringify(keys1) !== JSON.stringify(keys2)) {
            return { 
                identical: false, 
                structureMatch: false, 
                error: `Clés différentes - HL: [${keys1.join(',')}], Proxy: [${keys2.join(',')}]` 
            };
        }

        // Comparer les valeurs selon le type de subscription
        const comparisonResult = this.compareDataValues(data1, data2, subscriptionType);
        
        return {
            identical: comparisonResult.identical,
            structureMatch: true,
            fieldsCompared: keys1.length,
            details: comparisonResult.details,
            error: comparisonResult.error
        };
    }

    compareDataValues(data1, data2, subscriptionType) {
        let identicalFields = 0;
        let differentFields = 0;
        let timeRelatedDiffs = 0;
        const differences = [];

        for (const key of Object.keys(data1)) {
            const val1 = data1[key];
            const val2 = data2[key];

            // Comparaison exacte pour les types simples
            if (val1 === val2) {
                identicalFields++;
                continue;
            }

            // Tolérance pour les timestamps (différence < 5 secondes)
            if (this.isTimeField(key) && this.isCloseInTime(val1, val2, 5000)) {
                timeRelatedDiffs++;
                identicalFields++;
                continue;
            }

            // Comparaison spéciale pour les prix (tolérance de 0.001%)
            if (this.isPriceField(key) && this.isCloseInValue(val1, val2, 0.001)) {
                identicalFields++;
                continue;
            }

            // Comparaison pour les arrays
            if (Array.isArray(val1) && Array.isArray(val2)) {
                if (this.compareArrays(val1, val2)) {
                    identicalFields++;
                } else {
                    differentFields++;
                    differences.push(`${key}: arrays différents`);
                }
                continue;
            }

            // Comparaison pour les objets
            if (typeof val1 === 'object' && typeof val2 === 'object' && val1 !== null && val2 !== null) {
                const objComparison = this.compareDataValues(val1, val2, subscriptionType);
                if (objComparison.identical) {
                    identicalFields++;
                } else {
                    differentFields++;
                    differences.push(`${key}: objets différents`);
                }
                continue;
            }

            // Valeurs différentes
            differentFields++;
            differences.push(`${key}: "${val1}" vs "${val2}"`);
        }

        const totalFields = identicalFields + differentFields;
        const similarity = totalFields > 0 ? (identicalFields / totalFields) * 100 : 0;

        return {
            identical: differentFields === 0,
            details: `${identicalFields}/${totalFields} champs identiques (${similarity.toFixed(1)}%)${timeRelatedDiffs > 0 ? `, ${timeRelatedDiffs} diffs temporelles tolérées` : ''}`,
            error: differentFields > 0 ? `Différences: ${differences.slice(0, 3).join(', ')}${differences.length > 3 ? '...' : ''}` : null
        };
    }

    isTimeField(fieldName) {
        const timeFields = ['time', 'timestamp', 'statusTimestamp', 'T', 't'];
        return timeFields.includes(fieldName.toLowerCase());
    }

    isPriceField(fieldName) {
        const priceFields = ['px', 'limitpx', 'markpx', 'oraclepx', 'startpx', 'price', 'o', 'c', 'h', 'l'];
        return priceFields.some(field => fieldName.toLowerCase().includes(field));
    }

    isCloseInTime(val1, val2, toleranceMs) {
        const num1 = parseFloat(val1);
        const num2 = parseFloat(val2);
        
        if (isNaN(num1) || isNaN(num2)) return false;
        
        // Si les valeurs sont des timestamps en millisecondes
        if (num1 > 1000000000000 && num2 > 1000000000000) {
            return Math.abs(num1 - num2) <= toleranceMs;
        }
        
        // Si les valeurs sont des timestamps en secondes
        if (num1 > 1000000000 && num2 > 1000000000) {
            return Math.abs(num1 - num2) <= (toleranceMs / 1000);
        }
        
        return false;
    }

    isCloseInValue(val1, val2, tolerancePercent) {
        const num1 = parseFloat(val1);
        const num2 = parseFloat(val2);
        
        if (isNaN(num1) || isNaN(num2)) return false;
        if (num1 === 0 && num2 === 0) return true;
        
        const avgValue = (Math.abs(num1) + Math.abs(num2)) / 2;
        const diff = Math.abs(num1 - num2);
        const percentDiff = (diff / avgValue) * 100;
        
        return percentDiff <= tolerancePercent;
    }

    compareArrays(arr1, arr2) {
        if (arr1.length !== arr2.length) return false;
        
        for (let i = 0; i < arr1.length; i++) {
            if (typeof arr1[i] === 'object' && typeof arr2[i] === 'object') {
                if (!this.compareDataValues(arr1[i], arr2[i], 'array').identical) {
                    return false;
                }
            } else if (arr1[i] !== arr2[i]) {
                return false;
            }
        }
        
        return true;
    }

    // Nouvelle méthode pour synchroniser et comparer les messages
    findAndCompareMatchingMessages(hlMessages, proxyMessages, subscriptionType) {
        let bestMatch = null;
        let totalCompared = 0;

        // Pour chaque message d'Hyperliquid, chercher le correspondant dans le proxy
        for (const hlMsg of hlMessages) {
            const hlId = this.extractMessageId(hlMsg, subscriptionType);
            if (!hlId) continue;

            // Chercher le message correspondant dans le proxy
            for (const proxyMsg of proxyMessages) {
                totalCompared++;
                const proxyId = this.extractMessageId(proxyMsg, subscriptionType);
                
                if (hlId === proxyId) {
                    // Messages correspondants trouvés !
                    const comparison = this.compareMessageData(hlMsg, proxyMsg, subscriptionType);
                    
                    return {
                        found: true,
                        matchId: hlId,
                        channel: hlMsg.channel,
                        comparison: comparison,
                        totalCompared: totalCompared
                    };
                }
            }
        }

        return {
            found: false,
            totalCompared: totalCompared
        };
    }

    // Extraire l'identifiant unique d'un message selon son type
    extractMessageId(message, subscriptionType) {
        if (!message.data) return null;

        const data = message.data;
        
        switch (subscriptionType) {
            case 'trades':
                // Pour les trades : utiliser hash + tid
                if (Array.isArray(data)) {
                    const trade = data[0];
                    return trade?.hash ? `${trade.hash}-${trade.tid}` : null;
                }
                return data.hash ? `${data.hash}-${data.tid}` : null;

            case 'l2Book':
                // Pour l'order book : utiliser timestamp + coin
                return data.time ? `${data.coin}-${data.time}` : null;

            case 'candle':
                // Pour les chandeliers : utiliser temps d'ouverture + fermeture + coin
                if (Array.isArray(data)) {
                    const candle = data[0];
                    return candle ? `${candle.s}-${candle.t}-${candle.T}` : null;
                }
                return data.t ? `${data.s}-${data.t}-${data.T}` : null;

            case 'bbo':
                // Pour BBO : utiliser timestamp + coin
                return data.time ? `${data.coin}-${data.time}` : null;

            case 'allMids':
                // Pour allMids : créer un hash des prix (les prix changent ensemble)
                if (data.mids) {
                    const pricesHash = this.hashObject(data.mids);
                    return `allmids-${pricesHash}`;
                }
                return null;

            case 'orderUpdates':
                // Pour les ordres : utiliser OID + timestamp
                if (Array.isArray(data)) {
                    const order = data[0];
                    return order?.order?.oid ? `${order.order.oid}-${order.statusTimestamp}` : null;
                }
                return data.order?.oid ? `${data.order.oid}-${data.statusTimestamp}` : null;

            case 'userEvents':
                // Pour les événements utilisateur : utiliser hash ou timestamp
                if (data.fills && data.fills.length > 0) {
                    return data.fills[0].hash;
                }
                if (data.funding) {
                    return `funding-${data.funding.time}-${data.funding.coin}`;
                }
                return null;

            case 'userFills':
                // Pour les fills utilisateur : utiliser le hash du premier fill
                if (data.fills && data.fills.length > 0) {
                    return data.fills[0].hash;
                }
                return null;

            case 'notification':
                // Pour les notifications : utiliser le contenu + timestamp approximatif
                return data.notification ? `notif-${data.notification.slice(0, 20)}-${Date.now()}` : null;

            default:
                // Fallback : essayer de trouver un identifiant générique
                return data.hash || data.id || data.time || data.timestamp || null;
        }
    }

    // Créer un hash simple d'un objet
    hashObject(obj) {
        const str = JSON.stringify(obj, Object.keys(obj).sort());
        let hash = 0;
        for (let i = 0; i < str.length; i++) {
            const char = str.charCodeAt(i);
            hash = ((hash << 5) - hash) + char;
            hash = hash & hash; // Convert to 32-bit integer
        }
        return Math.abs(hash).toString(36);
    }

    // Gérer les messages JSON multiples séparés par newlines (optimisation du proxy Go)
    parseMultipleJsonMessages(rawData) {
        const messages = [];
        
        // Le proxy Go sépare les messages par des newlines
        const lines = rawData.trim().split('\n');
        
        for (const line of lines) {
            const cleanLine = line.trim();
            if (cleanLine.length === 0) {
                continue;
            }
            
            try {
                const message = JSON.parse(cleanLine);
                messages.push(message);
            } catch (error) {
                // Si le parsing d'une ligne échoue, essayer de parser toute la ligne comme un JSON
                console.warn(`⚠️  Ligne JSON invalide: ${cleanLine.substring(0, 100)}...`);
                // Ne pas faire échouer tout le parsing pour une ligne corrompue
            }
        }
        
        // Fallback : si aucun message n'a été parsé via newlines, essayer de parser directement
        if (messages.length === 0) {
            try {
                const message = JSON.parse(rawData.trim());
                messages.push(message);
            } catch (error) {
                console.error(`❌ Impossible de parser le JSON: ${rawData.substring(0, 100)}...`);
                throw error;
            }
        }
        
        return messages;
    }

    setsEqual(set1, set2) {
        return set1.size === set2.size && [...set1].every(x => set2.has(x));
    }

    getSubscriptionKey(sub) {
        return `${sub.type}${sub.coin ? `-${sub.coin}` : ''}${sub.interval ? `-${sub.interval}` : ''}${sub.user ? `-user` : ''}`;
    }

    printFinalResults() {
        console.log('\n' + '=' .repeat(60));
        console.log('📊 RÉSULTATS FINAUX DU TEST');
        console.log('=' .repeat(60));

        let totalTests = 0;
        let successfulTests = 0;

        for (const [key, result] of Object.entries(this.results)) {
            totalTests++;
            if (result.success) {
                successfulTests++;
                console.log(`✅ ${key}: ${result.reason}`);
            } else {
                console.log(`❌ ${key}: ${result.reason}`);
            }
        }

        console.log('\n' + '-' .repeat(60));
        console.log(`📈 TAUX DE RÉUSSITE: ${successfulTests}/${totalTests} (${Math.round(successfulTests/totalTests*100)}%)`);
        
        if (successfulTests === totalTests) {
            console.log('🎉 PARFAIT! Votre proxy fonctionne identiquement à Hyperliquid!');
        } else {
            console.log('⚠️  Quelques différences détectées - voir les détails ci-dessus');
        }

        // Sauvegarder les résultats détaillés
        fs.writeFileSync('test-results.json', JSON.stringify(this.results, null, 2));
        console.log('💾 Résultats détaillés sauvés dans test-results.json');
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
const tester = new WebSocketTester();
tester.testAllSubscriptions().catch(error => {
    console.error('💥 Erreur fatale:', error);
    process.exit(1);
}); 