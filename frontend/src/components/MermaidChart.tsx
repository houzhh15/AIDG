import React, { useRef, useEffect, useState } from 'react';
import mermaid from 'mermaid';

interface MermaidChartProps {
  chart: string;
}

export const MermaidChart: React.FC<MermaidChartProps> = ({ chart }) => {
  const chartRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // 监听并移除 Mermaid 在 body 中创建的错误提示元素
    const removeErrorElements = () => {
      // 查找并移除所有可能的 Mermaid 错误提示元素
      const errorSelectors = [
        'div[id*="d2h-"]',  // Mermaid 的错误容器 ID
        'div[style*="position: fixed"][style*="z-index: 9999"]',
        'div[style*="position: fixed"][style*="bottom"]'
      ];
      
      errorSelectors.forEach(selector => {
        document.querySelectorAll(selector).forEach(el => {
          const text = el.textContent || '';
          if (text.includes('Syntax error') || text.includes('mermaid')) {
            el.remove();
          }
        });
      });
    };

    // 使用 MutationObserver 监听 DOM 变化
    const observer = new MutationObserver((mutations) => {
      mutations.forEach((mutation) => {
        mutation.addedNodes.forEach((node) => {
          if (node.nodeType === 1) { // Element node
            const element = node as Element;
            const text = element.textContent || '';
            if (text.includes('Syntax error') || text.includes('mermaid')) {
              element.remove();
            }
          }
        });
      });
    });

    // 开始观察 body 的变化
    observer.observe(document.body, {
      childList: true,
      subtree: true
    });

    // 覆盖 Mermaid 的全局错误处理，阻止错误弹窗
    const originalConsoleError = console.error;
    const mermaidErrors: string[] = [];
    
    console.error = (...args: any[]) => {
      const errorMsg = args.join(' ');
      if (errorMsg.includes('mermaid') || errorMsg.includes('Syntax error')) {
        mermaidErrors.push(errorMsg);
        removeErrorElements(); // 立即尝试移除
        return; // 不显示 Mermaid 相关错误
      }
      originalConsoleError.apply(console, args);
    };

    if (chartRef.current && chart) {
      // 清空之前的内容
      chartRef.current.innerHTML = '';
      setError(null);

      try {
        // 初始化 mermaid，完全禁用错误提示
        mermaid.initialize({ 
          startOnLoad: true,
          theme: 'default',
          securityLevel: 'loose',
          fontFamily: 'Arial, sans-serif',
          fontSize: 14,
          flowchart: {
            useMaxWidth: true,
            htmlLabels: true
          },
          themeVariables: {
            primaryColor: '#fa8c16',
            primaryTextColor: '#333',
            primaryBorderColor: '#ffd591',
            lineColor: '#fa8c16',
            secondaryColor: '#fff7e6',
            tertiaryColor: '#fff1b8'
          },
          logLevel: 'fatal'
        });

        // 生成唯一 ID
        const id = `mermaid-chart-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
        
        // 渲染图表
        mermaid.render(id, chart).then((result) => {
          if (chartRef.current) {
            chartRef.current.innerHTML = result.svg;
          }
          removeErrorElements(); // 渲染后再次检查
        }).catch((err) => {
          // 捕获错误但不在控制台显示
          setError('图表语法错误，请检查Mermaid代码');
          removeErrorElements(); // 错误时也移除提示
        });
      } catch (err) {
        setError('图表初始化失败');
      }
    }

    // 清理：停止观察、恢复原始的 console.error、移除残留的错误元素
    return () => {
      observer.disconnect();
      console.error = originalConsoleError;
      removeErrorElements();
    };
  }, [chart]);

  if (error) {
    return (
      <div style={{
        padding: '16px',
        border: '1px solid #ff4d4f',
        borderRadius: '6px',
        backgroundColor: '#fff2f0',
        color: '#ff4d4f',
        textAlign: 'center'
      }}>
        {error}
      </div>
    );
  }

  return (
    <div 
      ref={chartRef}
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        padding: '16px',
        backgroundColor: '#fafafa',
        border: '1px solid #ffd591',
        borderRadius: '6px',
        margin: '16px 0'
      }}
    />
  );
};