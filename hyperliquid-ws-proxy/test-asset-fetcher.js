const axios = require('axios');

// Configuration
const SERVER_URL = 'http://localhost:8080';

// Test le nouvel endpoint /assets
async function testAssetsEndpoint() {
    console.log('🧪 Test de l\'endpoint /assets');
    
    try {
        const response = await axios.get(`${SERVER_URL}/assets`);
        
        console.log('✅ Status:', response.status);
        console.log('✅ Response:', JSON.stringify(response.data, null, 2));
        
        const data = response.data;
        
        // Vérifications basiques
        if (data.status === 'success') {
            console.log('✅ Status: success');
            
            const stats = data.data.statistics;
            const assets = data.data.assets;
            
            console.log('📊 Statistiques:');
            console.log(`   - Assets perpétuels: ${stats.perp_assets || 0}`);
            console.log(`   - Assets spot: ${stats.spot_assets || 0}`);
            console.log(`   - Total assets: ${stats.total_assets || 0}`);
            console.log(`   - Dernière MAJ: ${stats.last_updated || 'N/A'}`);
            
            console.log('📋 Assets disponibles:');
            if (Array.isArray(assets)) {
                console.log(`   - Nombre d'assets: ${assets.length}`);
                console.log(`   - Premiers assets: ${assets.slice(0, 10).join(', ')}${assets.length > 10 ? '...' : ''}`);
                
                // Vérifier la présence d'assets majeurs
                const majorAssets = ['BTC', 'ETH', 'SOL', 'ARB', 'OP'];
                const foundMajor = majorAssets.filter(asset => assets.includes(asset));
                console.log(`   - Assets majeurs trouvés: ${foundMajor.join(', ')}`);
            } else {
                console.log('   - ⚠️  Assets n\'est pas un tableau');
            }
            
        } else {
            console.log('❌ Status n\'est pas success:', data.status);
        }
        
    } catch (error) {
        console.error('❌ Erreur lors du test:', error.message);
        if (error.response) {
            console.error('❌ Status HTTP:', error.response.status);
            console.error('❌ Response:', error.response.data);
        }
    }
}

// Test principal
async function main() {
    console.log('🚀 Test du système AssetFetcher');
    console.log('================================');
    
    await testAssetsEndpoint();
    
    console.log('\n✨ Tests terminés');
}

// Exécuter les tests
if (require.main === module) {
    main().catch(console.error);
} 