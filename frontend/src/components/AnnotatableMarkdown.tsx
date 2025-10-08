import React, { useState, useEffect, useRef } from 'react';
import { Button, ColorPicker, Space, message, Input, Typography } from 'antd';
import { HighlightOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons';
import MarkdownViewer from './MarkdownViewer';
import { authedApi } from '../api/auth';

const { TextArea } = Input;

interface Annotation {
  id: string;
  startIndex: number;
  endIndex: number;
  text: string;
  color: string;
  note?: string;
  createdAt: string;
}

interface Props {
  content: string;
  taskId: string;
  editable?: boolean;
}

export const AnnotatableMarkdown: React.FC<Props> = ({ content, taskId, editable = true }) => {
  const [annotations, setAnnotations] = useState<Annotation[]>([]);
  const [selectedText, setSelectedText] = useState<{text: string, range: {start: number, end: number}} | null>(null);
  const [selectedColor, setSelectedColor] = useState('#ffeb3b');
  const [saving, setSaving] = useState(false);
  const [noteInput, setNoteInput] = useState('');
  const [showNoteInput, setShowNoteInput] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // 预设颜色
  const presetColors = [
    '#ffeb3b', // 黄色
    '#ff9800', // 橙色
    '#f44336', // 红色
    '#e91e63', // 粉色
    '#9c27b0', // 紫色
    '#3f51b5', // 深蓝
    '#2196f3', // 蓝色
    '#00bcd4', // 青色
    '#4caf50', // 绿色
    '#8bc34a', // 浅绿
  ];

  // 加载标注数据
  useEffect(() => {
    loadAnnotations();
  }, [taskId]);

  const loadAnnotations = async () => {
    if (!taskId) return;
    try {
      const response = await authedApi.get(`/tasks/${taskId}/polish-annotations`);
      setAnnotations(response.data.annotations || []);
    } catch (error) {
      console.error('Failed to load annotations:', error);
    }
  };

  const saveAnnotations = async (newAnnotations: Annotation[]) => {
    if (!taskId) return;
    setSaving(true);
    try {
      await authedApi.put(`/tasks/${taskId}/polish-annotations`, { 
        annotations: newAnnotations 
      });
      
      setAnnotations(newAnnotations);
      message.success('标注已保存');
    } catch (error) {
      message.error('保存失败');
    } finally {
      setSaving(false);
    }
  };

  // 处理文本选择
  const handleTextSelection = () => {
    if (!editable) return;
    
    // 使用setTimeout确保selection已经完成
    setTimeout(() => {
      const selection = window.getSelection();
      if (!selection || selection.rangeCount === 0) {
        setSelectedText(null);
        return;
      }
      
      const range = selection.getRangeAt(0);
      const selectedTextContent = range.toString().trim();
      
      // 过滤掉很短的选择（可能是意外点击）
      if (selectedTextContent.length < 3) {
        setSelectedText(null);
        return;
      }
      
      // 检查选择是否在我们的容器内
      const containerElement = containerRef.current;
      if (!containerElement || !containerElement.contains(range.commonAncestorContainer)) {
        setSelectedText(null);
        return;
      }
      
      // 计算在原始内容中的位置
      const textContent = containerElement.textContent || '';
      const startIndex = textContent.indexOf(selectedTextContent);
      
      if (startIndex >= 0) {
        setSelectedText({
          text: selectedTextContent,
          range: { start: startIndex, end: startIndex + selectedTextContent.length }
        });
      }
    }, 10);
  };

  // 添加标注
  const addAnnotation = () => {
    if (!selectedText) return;
    
    const newAnnotation: Annotation = {
      id: `annotation-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      startIndex: selectedText.range.start,
      endIndex: selectedText.range.end,
      text: selectedText.text,
      color: selectedColor,
      note: noteInput,
      createdAt: new Date().toISOString(),
    };
    
    const newAnnotations = [...annotations, newAnnotation];
    saveAnnotations(newAnnotations);
    
    // 清理状态
    setSelectedText(null);
    setNoteInput('');
    setShowNoteInput(false);
    window.getSelection()?.removeAllRanges();
  };

  // 删除标注
  const removeAnnotation = (annotationId: string) => {
    const newAnnotations = annotations.filter(a => a.id !== annotationId);
    saveAnnotations(newAnnotations);
  };

  // 渲染带标注的内容（简化版本）
  const renderAnnotatedContent = () => {
    if (annotations.length === 0) {
      return <MarkdownViewer>{content}</MarkdownViewer>;
    }

    // 简化版本：先渲染markdown，然后显示标注信息
    // 真正的实现需要更复杂的文本处理来直接在rendered HTML中高亮
    return (
      <div style={{ position: 'relative' }}>
        <MarkdownViewer>{content}</MarkdownViewer>
        {/* 未来可以在这里添加overlay标注层 */}
      </div>
    );
  };

  // 渲染浮动标注工具栏
  const renderFloatingToolbar = () => {
    if (!editable || !selectedText) return null;

    return (
      <div style={{
        position: 'fixed',
        top: '50%',
        right: 20,
        transform: 'translateY(-50%)',
        width: 300,
        backgroundColor: 'white',
        border: '1px solid #d9d9d9',
        borderRadius: 12,
        boxShadow: '0 8px 24px rgba(0, 0, 0, 0.12)',
        zIndex: 1000,
        padding: 16,
        maxHeight: '80vh',
        overflow: 'hidden',
        animation: 'slideInRight 0.3s ease-out'
      }}>
        <Space direction="vertical" size="small" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
            <Typography.Text strong style={{ color: '#1890ff', fontSize: '14px' }}>
              ✨ 添加重点标注
            </Typography.Text>
            <Button 
              type="text" 
              size="small"
              onClick={() => {
                setSelectedText(null);
                setNoteInput('');
                window.getSelection()?.removeAllRanges();
              }}
              style={{ 
                width: 24, 
                height: 24, 
                display: 'flex', 
                alignItems: 'center', 
                justifyContent: 'center',
                color: '#999'
              }}
            >
              ✕
            </Button>
          </div>
          
          <div>
            <Typography.Text strong>已选择文本:</Typography.Text>
            <div style={{ 
              background: '#f5f5f5', 
              padding: 8, 
              borderRadius: 4, 
              marginTop: 4,
              fontSize: '12px',
              maxHeight: 60,
              overflow: 'auto',
              border: '1px solid #e8e8e8'
            }}>
              "{selectedText.text}"
            </div>
          </div>
          
          <div>
            <Typography.Text strong style={{ fontSize: '13px' }}>标注颜色:</Typography.Text>
            <div style={{ 
              marginTop: 8, 
              display: 'grid', 
              gridTemplateColumns: 'repeat(5, 1fr)', 
              gap: 6 
            }}>
              {presetColors.map(color => (
                <div
                  key={color}
                  onClick={() => setSelectedColor(color)}
                  style={{
                    width: 32,
                    height: 32,
                    backgroundColor: color,
                    borderRadius: 6,
                    cursor: 'pointer',
                    border: selectedColor === color ? '3px solid #1890ff' : '2px solid #e8e8e8',
                    transition: 'all 0.15s ease',
                    boxShadow: selectedColor === color ? '0 0 0 1px rgba(24, 144, 255, 0.2)' : 'none',
                    transform: selectedColor === color ? 'scale(1.1)' : 'scale(1)',
                  }}
                  title={color}
                />
              ))}
            </div>
            <div style={{ marginTop: 8, display: 'flex', alignItems: 'center', gap: 8 }}>
              <Typography.Text style={{ fontSize: '12px', color: '#666' }}>自定义:</Typography.Text>
              <ColorPicker 
                value={selectedColor} 
                onChange={(color) => setSelectedColor(color.toHexString())}
                showText
                size="small"
              />
            </div>
          </div>

          <div>
            <Typography.Text>备注 (可选):</Typography.Text>
            <TextArea
              value={noteInput}
              onChange={(e) => setNoteInput(e.target.value)}
              placeholder="为这段重要内容添加备注..."
              rows={2}
              style={{ marginTop: 4 }}
            />
          </div>

          <Space style={{ width: '100%' }}>
            <Button 
              onClick={() => {
                setSelectedText(null);
                setNoteInput('');
                window.getSelection()?.removeAllRanges();
              }}
            >
              取消
            </Button>
            <Button 
              type="primary" 
              icon={<HighlightOutlined />}
              onClick={addAnnotation}
              loading={saving}
              style={{ flex: 1 }}
            >
              {saving ? '保存中...' : '添加标注'}
            </Button>
          </Space>
        </Space>
      </div>
    );
  };

  return (
    <div style={{ position: 'relative' }}>
      <style>{`
        @keyframes slideInRight {
          from {
            opacity: 0;
            transform: translateY(-50%) translateX(20px);
          }
          to {
            opacity: 1;
            transform: translateY(-50%) translateX(0);
          }
        }
      `}</style>
      
      {/* 浮动标注工具栏 */}
      {renderFloatingToolbar()}
      
      {/* 点击遮罩层取消选择 */}
      {editable && selectedText && (
        <div
          style={{
            position: 'fixed',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            zIndex: 999,
            backgroundColor: 'transparent'
          }}
          onClick={() => {
            setSelectedText(null);
            setNoteInput('');
            window.getSelection()?.removeAllRanges();
          }}
        />
      )}
      
      {/* 主要内容区域 */}
      <div 
        ref={containerRef}
        onMouseUp={handleTextSelection}
        style={{ 
          userSelect: editable ? 'text' : 'none',
          cursor: editable ? 'text' : 'default',
          position: 'relative',
          zIndex: 1
        }}
      >
        {renderAnnotatedContent()}
      </div>

      {/* 标注列表 */}
      {annotations.length > 0 && (
        <div style={{ 
          marginTop: 24, 
          borderTop: '1px solid #f0f0f0', 
          paddingTop: 16 
        }}>
          <Typography.Title level={5}>
            重点标注 ({annotations.length})
          </Typography.Title>
          
          <div style={{ maxHeight: 300, overflow: 'auto' }}>
            {annotations.map(annotation => (
              <div 
                key={annotation.id} 
                style={{ 
                  margin: '8px 0', 
                  padding: '12px',
                  backgroundColor: annotation.color + '15',
                  borderLeft: `4px solid ${annotation.color}`,
                  borderRadius: 6,
                  position: 'relative'
                }}
              >
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                  <div style={{ flex: 1, paddingRight: 8 }}>
                    <div style={{ 
                      fontWeight: 500, 
                      marginBottom: 6,
                      lineHeight: 1.4,
                      backgroundColor: annotation.color + '25',
                      padding: '4px 8px',
                      borderRadius: 4,
                      display: 'inline-block'
                    }}>
                      "{annotation.text}"
                    </div>
                    
                    {annotation.note && (
                      <div style={{ 
                        fontSize: '13px', 
                        color: '#666', 
                        marginTop: 6,
                        fontStyle: 'italic'
                      }}>
                        💡 {annotation.note}
                      </div>
                    )}
                    
                    <div style={{ 
                      fontSize: '11px', 
                      color: '#999', 
                      marginTop: 8 
                    }}>
                      📅 {new Date(annotation.createdAt).toLocaleString('zh-CN')}
                    </div>
                  </div>
                  
                  {editable && (
                    <Button 
                      size="small" 
                      type="text" 
                      danger 
                      icon={<DeleteOutlined />}
                      onClick={() => removeAnnotation(annotation.id)}
                      style={{ flexShrink: 0 }}
                    />
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
      
      {editable && (
        <div style={{ 
          fontSize: '12px', 
          color: '#999', 
          marginTop: 12,
          padding: 8,
          background: '#fafafa',
          borderRadius: 4,
          textAlign: 'center'
        }}>
          💡 提示：选中文本后会自动弹出标注工具
        </div>
      )}
    </div>
  );
};
