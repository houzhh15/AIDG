import React, { useEffect, useRef, useState } from 'react';
import { Button, Space, Typography } from 'antd';
import { Terminal } from 'xterm';
import 'xterm/css/xterm.css';
import { authedApi } from '../api/auth';

interface Props {
  taskId: string;
  height?: number; // 可选：父组件传入专用高度，精确填满
}

interface WsMessage {
  type: string;
  data?: string;
  code?: number;
  cols?: number;
  rows?: number;
  id?: string;
}

// 协议草案：
// 客户端发送: {type:'data', data:'user input'} 普通输入
// 客户端发送: {type:'resize', cols, rows}
// 后端推送: {type:'data', data:'chunk'}
//           {type:'exit', code:0}
//           {type:'ready'} 表示进程已启动
// create API: POST /api/v1/tasks/:taskId/codebuddy-terminal => { id, wsUrl }

export const CodeBuddyTerminal: React.FC<Props> = ({ taskId, height }) => {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const termRef = useRef<Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const [connecting, setConnecting] = useState(false);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [status, setStatus] = useState<'idle'|'connecting'|'running'|'finished'|'error'>('idle');
  const [loadingPrompt, setLoadingPrompt] = useState<string | null>(null);

  // 终端初始化 - 固定尺寸
  useEffect(()=>{
    if(!containerRef.current || termRef.current) return;
    
    const term = new Terminal({
      fontSize: 14,
      fontFamily: 'Monaco, Menlo, "Courier New", monospace',
      convertEol: true,
      cursorBlink: true,
      scrollback: 500,
      cols: 70,  // 固定列数
      rows: 22   // 增加到30行
    });
    
    term.open(containerRef.current);
    termRef.current = term;
    
    return () => {
      term.dispose();
      termRef.current = null;
    };
  },[]);

  const connect = async () => {
    if(connecting || status === 'running') return;
    
    setConnecting(true);
    setStatus('connecting');
    
    try {
      // 创建终端会话 - 使用正确的后端API
      const res = await authedApi.post(`/tasks/${taskId}/codebuddy-terminal`);
      setSessionId(res.data.id);
      
      // WebSocket 连接 - 智能选择协议和端口
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const host = window.location.host; // 自动获取当前host和port
      const wsUrl = `${protocol}//${host}${res.data.wsUrl}`;
      console.log('尝试连接 WebSocket:', wsUrl);
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;
      
      ws.onopen = () => {
        console.log('WebSocket 连接成功');
        setStatus('running');
        setConnecting(false);
        // 发送固定尺寸
        ws.send(JSON.stringify({ 
          type: 'resize', 
          cols: 70, 
          rows: 22 
        }));
      };
      
      ws.onmessage = (event) => {
        if(!termRef.current) return;
        try {
          const msg = JSON.parse(event.data);
          if(msg.type === 'data') {
            termRef.current.write(msg.data);
          } else if(msg.type === 'exit') {
            setStatus('finished');
          }
        } catch(e) {
          console.error('解析消息失败:', e);
        }
      };
      
      ws.onclose = (event) => {
        console.log('WebSocket 连接关闭:', event.code, event.reason);
        setStatus('idle');
        setConnecting(false);
      };
      
      ws.onerror = (error) => {
        console.error('WebSocket 错误:', error);
        setStatus('error');
        setConnecting(false);
      };
      
      // 用户输入处理
      termRef.current?.onData((data) => {
        if(ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ type: 'data', data }));
        }
      });
      
    } catch(error) {
      console.error('创建终端会话失败:', error);
      setStatus('error');
      setConnecting(false);
      // 在终端显示错误信息
      if(termRef.current) {
        termRef.current.writeln('\x1b[31m连接失败: 请确保后端服务正在运行\x1b[0m');
        termRef.current.writeln('检查端口 8000 是否可访问');
      }
    }
  };

  // 注入 Prompt 到终端
  const injectPrompt = async (promptName: string) => {
    if (!taskId || status !== 'running' || !wsRef.current) {
      return;
    }

    setLoadingPrompt(promptName);
    try {
      // 根据 promptName 映射到对应的文件名
      const fileMap: Record<string, string> = {
        'POLISH': 'meeting_polish.txt',
        'TOPIC': 'topic.txt',
        'FEATURE_LIST': 'feature_list.txt', 
        'ARCHITECTURE': 'architecture_new.txt'
      };

      const fileName = fileMap[promptName];
      if (!fileName) {
        throw new Error(`未知的 prompt 类型: ${promptName}`);
      }

      // 获取 prompt 内容
      const response = await authedApi.get(`/tasks/${taskId}/files/${fileName}`, {
        responseType: 'text'
      });
      
      const promptContent = response.data;
      
      // 发送到终端
      if (wsRef.current.readyState === WebSocket.OPEN) {
        // 先发送一个换行，确保在新行开始
        wsRef.current.send(JSON.stringify({ type: 'data', data: '\r' }));
        // 发送 prompt 内容
        wsRef.current.send(JSON.stringify({ type: 'data', data: promptContent }));
        // 最后发送回车执行
        wsRef.current.send(JSON.stringify({ type: 'data', data: '\r' }));
      }
    } catch (error) {
      console.error('注入 Prompt 失败:', error);
      if (termRef.current) {
        termRef.current.writeln(`\x1b[31m加载 ${promptName} 失败\x1b[0m`);
      }
    } finally {
      setLoadingPrompt(null);
    }
  };

  return (
    <div style={{ 
      width: '100%',
      maxWidth: '1000px', // 增加宽度容纳右侧按钮
      margin: '0 auto',
      border: '1px solid #d9d9d9',
      borderRadius: '6px',
      backgroundColor: '#fff',
      boxShadow: '0 2px 8px rgba(0,0,0,0.1)'
    }}>
      {/* 卡片标题栏 */}
      <div style={{ 
        padding: '12px 16px', 
        borderBottom: '1px solid #d9d9d9',
        backgroundColor: '#fafafa',
        borderRadius: '6px 6px 0 0',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between'
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ fontWeight: 500, color: '#262626' }}>CodeBuddy 终端</span>
          <Button 
            type="primary" 
            size="small"
            onClick={connect} 
            loading={connecting}
            disabled={status === 'running' || status === 'connecting'}
          >
            {status === 'idle' ? '启动' : '重启'}
          </Button>
        </div>
        <span style={{ fontSize: 12, color: '#8c8c8c' }}>
          {status === 'idle' && '待启动'}
          {status === 'connecting' && '🔄 连接中...'}
          {status === 'running' && '✅ 运行中'}
          {status === 'finished' && '⭕ 已结束'}
          {status === 'error' && '❌ 连接失败'}
        </span>
      </div>
      
      {/* 主要内容区域 - 终端 + 按钮 */}
      <div style={{ 
        display: 'flex',
        padding: '16px',
        gap: '16px',
        height: '550px' // 增加高度
      }}>
        {/* 左侧：终端区域 */}
        <div style={{ 
          background: '#000',
          // 80列 x 30行，增加行数
          width: '700px', // 80 * 8.4
          height: '540px', // 30 * 18
          borderRadius: '4px',
          overflow: 'hidden'
        }}>
          <div 
            ref={containerRef} 
            style={{ 
              width: '100%', 
              height: '100%'
            }} 
          />
        </div>

        {/* 右侧：Prompt 按钮区域 */}
        <div style={{ 
          width: '200px',
          display: 'flex',
          flexDirection: 'column',
          gap: '12px',
          paddingTop: '20px'
        }}>
          <div style={{ 
            marginBottom: '8px',
            fontSize: '14px',
            fontWeight: 500,
            color: '#262626'
          }}>
            快速 Prompt
          </div>
          
          {(['POLISH', 'TOPIC', 'FEATURE_LIST', 'ARCHITECTURE'] as const).map((promptName) => (
            <Button
              key={promptName}
              size="small"
              type="default"
              loading={loadingPrompt === promptName}
              disabled={status !== 'running'}
              onClick={() => injectPrompt(promptName)}
              style={{
                textAlign: 'left',
                height: '32px'
              }}
            >
              {promptName.replace('_', ' ')}
            </Button>
          ))}
          
          <div style={{ 
            marginTop: '16px',
            fontSize: '12px',
            color: '#8c8c8c',
            lineHeight: 1.4
          }}>
            点击按钮可将预设 Prompt 注入到终端中执行
          </div>
        </div>
      </div>
    </div>
  );
};
