document.addEventListener('DOMContentLoaded', function() {
    console.log('マインドマップツールが初期化されました');
    
    // ここにマインドマップの初期化コードを追加します
    const container = document.getElementById('mindmap-container');
    
    // サンプルのイベントリスナー
    container.addEventListener('click', function(e) {
        console.log('クリックされた位置:', e.clientX, e.clientY);
    });
}); 