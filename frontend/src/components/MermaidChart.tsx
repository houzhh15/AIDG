import React, { useRef, useEffect, useState } from 'react';
import mermaid from 'mermaid';

interface MermaidChartProps {
  chart: string;
}

export const MermaidChart: React.FC<MermaidChartProps> = ({ chart }) => {
  const chartRef = useRef<HTMLDivElement>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (chartRef.current && chart) {
      // 清空之前的内容
      chartRef.current.innerHTML = '';
      setError(null);

      try {
        // 初始化 mermaid，配置日志级别以减少错误提示
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
          // 设置日志级别为 fatal，只显示致命错误，抑制语法错误弹窗
          logLevel: 'fatal'
        });

        // 生成唯一 ID
        const id = `mermaid-chart-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
        
        // 渲染图表
        mermaid.render(id, chart).then((result) => {
          if (chartRef.current) {
            chartRef.current.innerHTML = result.svg;
          }
        }).catch((err) => {
          console.error('Mermaid render error:', err);
          setError('图表语法错误，请检查Mermaid代码');
        });
      } catch (err) {
        console.error('Mermaid initialization error:', err);
        setError('图表初始化失败');
      }
    }
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