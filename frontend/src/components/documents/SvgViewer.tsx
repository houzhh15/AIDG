/**
 * SvgViewer.tsx
 * SVG 专用查看器组件 - 支持图形/源码切换和下载
 */

import React, { useState, useMemo, useCallback } from 'react';
import { Radio, Button, Space, Card, Tooltip, message } from 'antd';
import {
  PictureOutlined,
  CodeOutlined,
  DownloadOutlined,
  CopyOutlined,
} from '@ant-design/icons';
import DOMPurify from 'dompurify';

type ViewMode = 'graphic' | 'source';

interface SvgViewerProps {
  content: string;
  originalFilename?: string;
  title?: string;
  showToolbar?: boolean;
  height?: number | string;
}

const SvgViewer: React.FC<SvgViewerProps> = React.memo(({
  content,
  originalFilename = 'image.svg',
  title,
  showToolbar = true,
  height = 400,
}) => {
  const [viewMode, setViewMode] = useState<ViewMode>('graphic');

  // 使用 DOMPurify 清理 SVG 内容，防止 XSS 攻击
  const sanitizedSvg = useMemo(() => {
    if (!content) return '';
    return DOMPurify.sanitize(content, {
      USE_PROFILES: { svg: true, svgFilters: true },
      ADD_TAGS: ['use'],
      FORBID_TAGS: ['script'],
      FORBID_ATTR: ['onclick', 'onload', 'onerror', 'onmouseover'],
    });
  }, [content]);

  const handleDownload = useCallback(() => {
    try {
      const blob = new Blob([content], { type: 'image/svg+xml' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = originalFilename.endsWith('.svg')
        ? originalFilename
        : `${originalFilename}.svg`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      message.success('SVG 文件下载成功');
    } catch (error) {
      message.error('下载失败');
    }
  }, [content, originalFilename]);

  const handleCopySource = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(content);
      message.success('源码已复制到剪贴板');
    } catch {
      message.error('复制失败');
    }
  }, [content]);

  const handleModeChange = useCallback((e: any) => {
    setViewMode(e.target.value);
  }, []);

  return (
    <Card
      title={title}
      size="small"
      extra={
        showToolbar && (
          <Space>
            <Radio.Group
              value={viewMode}
              onChange={handleModeChange}
              size="small"
              optionType="button"
              buttonStyle="solid"
            >
              <Radio.Button value="graphic">
                <PictureOutlined /> 图形
              </Radio.Button>
              <Radio.Button value="source">
                <CodeOutlined /> 源码
              </Radio.Button>
            </Radio.Group>

            {viewMode === 'source' && (
              <Tooltip title="复制源码">
                <Button
                  size="small"
                  icon={<CopyOutlined />}
                  onClick={handleCopySource}
                />
              </Tooltip>
            )}

            <Tooltip title="下载 SVG">
              <Button
                size="small"
                icon={<DownloadOutlined />}
                onClick={handleDownload}
              />
            </Tooltip>
          </Space>
        )
      }
      bodyStyle={{
        padding: viewMode === 'source' ? 8 : 16,
        height: typeof height === 'number' ? height : undefined,
        overflow: 'auto',
        backgroundColor: viewMode === 'source' ? '#1e1e1e' : '#fff',
      }}
    >
      {viewMode === 'graphic' ? (
        <div
          style={{
            display: 'block',
            minHeight: 200,
            overflow: 'auto',
            width: '100%',
          }}
          dangerouslySetInnerHTML={{ __html: sanitizedSvg }}
        />
      ) : (
        <pre
          style={{
            margin: 0,
            padding: 12,
            fontSize: 12,
            lineHeight: 1.5,
            color: '#d4d4d4',
            backgroundColor: '#1e1e1e',
            overflow: 'auto',
            maxHeight: typeof height === 'number' ? height - 40 : undefined,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
          }}
        >
          {content}
        </pre>
      )}
    </Card>
  );
});

export default SvgViewer;
