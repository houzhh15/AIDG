// ==========================================
// 应用中音频功能测试脚本  
// ==========================================
// 在 http://localhost:8000 的浏览器控制台中运行

console.log('%c=== AIDG 音频功能诊断 ===', 'color: #0071e3; font-size: 16px; font-weight: bold');

// 1. 检查基础 API
console.log('\n1. 检查浏览器 API 支持:');
console.log('  navigator.mediaDevices:', !!navigator.mediaDevices ? '✓' : '✗');
console.log('  getUserMedia:', !!navigator.mediaDevices?.getUserMedia ? '✓' : '✗');
console.log('  enumerateDevices:', !!navigator.mediaDevices?.enumerateDevices ? '✓' : '✗');
console.log('  MediaRecorder:', !!window.MediaRecorder ? '✓' : '✗');

// 2. 枚举设备
console.log('\n2. 枚举音频设备:');
(async () => {
    try {
        const devices = await navigator.mediaDevices.enumerateDevices();
        const audioInputs = devices.filter(d => d.kind === 'audioinput');
        console.log(`  找到 ${audioInputs.length} 个音频输入设备:`);
        audioInputs.forEach((d, i) => {
            console.log(`    ${i+1}. ${d.label || '(无标签 - 需要权限)'}`);
            console.log(`       ID: ${d.deviceId.slice(0, 20)}...`);
        });
        
        if (!audioInputs.some(d => d.label)) {
            console.log('%c  ⚠️ 设备无标签，需要先请求权限', 'color: #ff9500');
            console.log('%c  运行以下命令请求权限:', 'color: #0071e3');
            console.log('    const stream = await navigator.mediaDevices.getUserMedia({audio: true});');
            console.log('    stream.getTracks().forEach(t => t.stop());');
            console.log('    然后重新运行此脚本');
        }
    } catch (err) {
        console.error('  ✗ 枚举失败:', err.message);
    }
})();

// 3. 检查 React 组件状态
console.log('\n3. 检查 React DevTools 是否可用:');
if (window.__REACT_DEVTOOLS_GLOBAL_HOOK__) {
    console.log('  ✓ React DevTools 已安装');
    console.log('  提示: 按 F12 → React 标签查看组件状态');
} else {
    console.log('  ✗ React DevTools 未安装');
    console.log('  建议: 安装 React DevTools 扩展以调试');
}

// 4. 测试权限请求
console.log('\n4. 快速测试权限请求:');
console.log('  运行以下命令测试麦克风权限:');
console.log('%c  testMicrophone()', 'color: #34c759; font-weight: bold');

window.testMicrophone = async function(deviceId) {
    console.log('\n=== 开始测试麦克风 ===');
    try {
        const constraints = {
            audio: deviceId ? { deviceId: { exact: deviceId } } : true
        };
        
        console.log('请求音频流...', constraints);
        const stream = await navigator.mediaDevices.getUserMedia(constraints);
        
        console.log('✓ 成功获取音频流');
        const track = stream.getAudioTracks()[0];
        const settings = track.getSettings();
        
        console.log('音频轨道信息:');
        console.table({
            '标签': track.label,
            '设备ID': settings.deviceId,
            '采样率': settings.sampleRate + ' Hz',
            '通道数': settings.channelCount,
            '启用': track.enabled,
            '静音': track.muted,
            '状态': track.readyState
        });
        
        // 监控音频电平
        console.log('\n开始监控音频电平（10秒）...');
        console.log('请说话或播放音频...');
        
        const audioCtx = new AudioContext();
        const source = audioCtx.createMediaStreamSource(stream);
        const analyser = audioCtx.createAnalyser();
        analyser.fftSize = 2048;
        source.connect(analyser);
        
        const dataArray = new Uint8Array(analyser.frequencyBinCount);
        let maxLevel = 0;
        let checkCount = 0;
        
        const interval = setInterval(() => {
            analyser.getByteTimeDomainData(dataArray);
            let sum = 0;
            for (let i = 0; i < dataArray.length; i++) {
                sum += Math.abs(dataArray[i] - 128);
            }
            const level = sum / dataArray.length;
            if (level > maxLevel) maxLevel = level;
            checkCount++;
            
            if (checkCount % 10 === 0) {
                console.log(`电平: ${level.toFixed(2)} (最大: ${maxLevel.toFixed(2)})`);
            }
        }, 100);
        
        setTimeout(() => {
            clearInterval(interval);
            stream.getTracks().forEach(t => t.stop());
            audioCtx.close();
            
            console.log('\n=== 测试结果 ===');
            console.log('最大电平:', maxLevel.toFixed(2));
            if (maxLevel < 1) {
                console.log('%c✗ 未检测到音频信号', 'color: #ff3b30; font-weight: bold');
                console.log('可能原因:');
                console.log('  1. Loopback 未启用');
                console.log('  2. 没有音频源播放');
                console.log('  3. 设备被静音');
            } else {
                console.log('%c✓ 检测到音频信号', 'color: #34c759; font-weight: bold');
                console.log('设备工作正常！');
            }
        }, 10000);
        
    } catch (err) {
        console.error('✗ 测试失败:', err.name, err.message);
    }
};

// 5. 测试特定设备
console.log('\n5. 测试特定设备:');
console.log('  获取设备列表后，运行:');
console.log('%c  testMicrophone("device-id-here")', 'color: #34c759; font-weight: bold');

// 6. 检查 MediaRecorder 支持
console.log('\n6. 检查 MediaRecorder MIME 类型支持:');
const mimeTypes = [
    'audio/webm;codecs=opus',
    'audio/webm',
    'audio/ogg;codecs=opus',
    'audio/mp4',
    'audio/wav'
];
mimeTypes.forEach(type => {
    const supported = MediaRecorder.isTypeSupported(type);
    console.log(`  ${supported ? '✓' : '✗'} ${type}`);
});

console.log('\n%c=== 诊断完成 ===', 'color: #0071e3; font-size: 16px; font-weight: bold');
console.log('\n可用命令:');
console.log('  testMicrophone()         - 测试默认麦克风');
console.log('  testMicrophone("id")     - 测试指定设备');
console.log('\n如果需要更多帮助，请查看详细日志');
