// ==========================================
// 浏览器控制台设备测试脚本
// ==========================================
// 在 http://localhost:8000 的浏览器控制台中运行此脚本

console.log('=== 音频设备测试脚本 ===');

// 1. 枚举所有设备
async function testEnumerateDevices() {
    console.log('\n1. 枚举音频设备:');
    try {
        const devices = await navigator.mediaDevices.enumerateDevices();
        const audioInputs = devices.filter(d => d.kind === 'audioinput');
        
        console.log(`   总设备数: ${devices.length}`);
        console.log(`   音频输入设备: ${audioInputs.length}`);
        
        audioInputs.forEach((device, index) => {
            console.log(`   ${index + 1}. ${device.label || '(未授权)'}`);
            console.log(`      ID: ${device.deviceId}`);
            console.log(`      Group: ${device.groupId}`);
        });
        
        return audioInputs;
    } catch (err) {
        console.error('   ✗ 枚举失败:', err);
        return [];
    }
}

// 2. 测试特定设备
async function testSpecificDevice(deviceId, deviceLabel) {
    console.log(`\n2. 测试设备: ${deviceLabel}`);
    console.log(`   Device ID: ${deviceId}`);
    
    try {
        // 请求该设备
        const stream = await navigator.mediaDevices.getUserMedia({
            audio: {
                deviceId: { exact: deviceId },
                echoCancellation: true,
                noiseSuppression: true,
                autoGainControl: true
            }
        });
        
        const track = stream.getAudioTracks()[0];
        const settings = track.getSettings();
        
        console.log('   ✓ 成功获取音频流');
        console.log('   轨道信息:');
        console.log('     - Label:', track.label);
        console.log('     - Enabled:', track.enabled);
        console.log('     - Muted:', track.muted);
        console.log('     - ReadyState:', track.readyState);
        console.log('   设置:');
        console.log('     - Device ID:', settings.deviceId);
        console.log('     - Sample Rate:', settings.sampleRate, 'Hz');
        console.log('     - Channel Count:', settings.channelCount);
        console.log('     - Echo Cancellation:', settings.echoCancellation);
        
        // 验证设备匹配
        if (settings.deviceId === deviceId) {
            console.log('   ✓ 设备ID匹配 - 使用了正确的设备');
        } else {
            console.warn('   ⚠ 设备ID不匹配!');
            console.warn('     请求:', deviceId);
            console.warn('     实际:', settings.deviceId);
        }
        
        // 停止流
        stream.getTracks().forEach(t => t.stop());
        
        return true;
    } catch (err) {
        console.error('   ✗ 获取音频流失败:', err.name, err.message);
        return false;
    }
}

// 3. 运行完整测试
async function runFullTest() {
    console.log('\n=== 开始完整测试 ===\n');
    
    // 步骤 1: 枚举设备
    const devices = await testEnumerateDevices();
    
    if (devices.length === 0) {
        console.log('\n⚠ 没有找到音频设备，请先请求权限');
        console.log('运行: await navigator.mediaDevices.getUserMedia({audio: true})');
        return;
    }
    
    // 步骤 2: 测试每个设备
    console.log('\n=== 测试每个设备 ===');
    for (let i = 0; i < devices.length; i++) {
        const device = devices[i];
        await testSpecificDevice(device.deviceId, device.label || `设备 ${i+1}`);
        
        // 等待一下避免冲突
        await new Promise(resolve => setTimeout(resolve, 500));
    }
    
    console.log('\n=== 测试完成 ===');
    console.log('\n建议:');
    console.log('1. 查看上面的日志，确认每个设备都能正常获取');
    console.log('2. 注意 "设备ID匹配" 的检查结果');
    console.log('3. 如果使用 BlackHole，需要有音频输出到该设备才有声音');
}

// 4. BlackHole 特殊测试
async function testBlackHole() {
    console.log('\n=== BlackHole 设备测试 ===');
    
    const devices = await navigator.mediaDevices.enumerateDevices();
    const blackHole = devices.find(d => 
        d.kind === 'audioinput' && 
        d.label.toLowerCase().includes('blackhole')
    );
    
    if (!blackHole) {
        console.log('⚠ 未找到 BlackHole 设备');
        return;
    }
    
    console.log('找到 BlackHole 设备:', blackHole.label);
    console.log('\n重要提示:');
    console.log('1. BlackHole 是虚拟音频回环设备');
    console.log('2. 它不会录制环境声音');
    console.log('3. 必须有应用程序正在播放音频到 BlackHole');
    console.log('4. 测试方法:');
    console.log('   a. 打开另一个标签页播放音乐');
    console.log('   b. 系统音频输出设置为包含 BlackHole 的 Multi-Output Device');
    console.log('   c. 然后才能从 BlackHole 录制到声音');
    
    await testSpecificDevice(blackHole.deviceId, blackHole.label);
}

// 自动运行测试
console.log('\n可用命令:');
console.log('  runFullTest()     - 运行完整设备测试');
console.log('  testBlackHole()   - 测试 BlackHole 设备');
console.log('\n正在运行完整测试...\n');

runFullTest();
