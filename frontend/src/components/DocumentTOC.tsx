import React, { useMemo, useState, useEffect, useRef } from 'react';
import { Dropdown, Modal, Form, Input, message } from 'antd';
import type { MenuProps } from 'antd';
import { CopyOutlined, PlusOutlined, EditOutlined } from '@ant-design/icons';
import { addCustomResource } from '../api/resourceApi';
import { loadAuth } from '../api/auth';

interface TOCItem {
  id: string;
  text: string;
  level: number;
}

interface Props {
  content: string;
  minLevel?: number; // minimum heading level to include
  maxLevel?: number; // maximum heading level to include
  projectId?: string;
  taskId?: string;
  docType?: 'requirements' | 'design' | 'test';
  onEditSection?: (sectionId: string) => void; // 编辑章节的回调
}

// Generate slug consistent with MarkdownViewer (without duplicate handling yet)
function baseSlug(text: string) {
  return text
    .toLowerCase()
    .replace(/[^\w\u4e00-\u9fa5\s-]/g, '')
    .replace(/\s+/g, '-');
}

// 全局版本计数器，不会因组件重新挂载而重置
let globalVersionCounter = 0;

export const DocumentTOC: React.FC<Props> = ({ content, minLevel = 1, maxLevel = 4, projectId, taskId, docType, onEditSection }) => {
  const [modalVisible, setModalVisible] = useState(false);
  const [contextMenuItem, setContextMenuItem] = useState<TOCItem | null>(null);
  const [form] = Form.useForm();
  const [saving, setSaving] = useState(false);
  const [contentVersion, setContentVersion] = useState(() => ++globalVersionCounter);
  const contentVersionRef = useRef(contentVersion);
  const pendingTimersRef = useRef<Set<NodeJS.Timeout>>(new Set());
  const lastClickTimeRef = useRef<Record<string, number>>({});

  // 组件卸载时的清理
  useEffect(() => {
    return () => {
      pendingTimersRef.current.forEach(timer => clearTimeout(timer));
      pendingTimersRef.current.clear();
    };
  }, [docType]);

  const items: TOCItem[] = useMemo(() => {
    // 先移除代码块内容,避免识别代码块中的标题
    let processedContent = content;
    
    // 移除代码块 (```)
    processedContent = processedContent.replace(/```[\s\S]*?```/g, '');
    
    // 移除行内代码 (`)
    processedContent = processedContent.replace(/`[^`\n]+`/g, '');
    
    const lines = processedContent.split(/\n/);
    const result: TOCItem[] = [];
    const slugCount: Record<string, number> = {};
    const headingRegex = /^(#{1,6})\s+(.+?)\s*$/;
    
    for (const line of lines) {
      const m = headingRegex.exec(line);
      if (!m) continue;
      const level = m[1].length;
      if (level < minLevel || level > maxLevel) continue;
      const rawText = m[2].trim();
      const base = baseSlug(rawText);
      let id = base;
      if (slugCount[base] !== undefined) {
        slugCount[base] += 1;
        id = `${base}-${slugCount[base]}`;
      } else {
        slugCount[base] = 0;
      }
      result.push({ id, text: rawText, level });
    }
    return result;
  }, [content, minLevel, maxLevel]);

  // 每次content或docType变化时递增版本号并清理旧的定时器
  useEffect(() => {
    // 清理所有pending的定时器
    pendingTimersRef.current.forEach(timer => clearTimeout(timer));
    pendingTimersRef.current.clear();
    
    const newVersion = ++globalVersionCounter;
    setContentVersion(newVersion);
    contentVersionRef.current = newVersion;
    
    // 组件卸载时清理所有定时器
    return () => {
      pendingTimersRef.current.forEach(timer => clearTimeout(timer));
      pendingTimersRef.current.clear();
    };
  }, [content, docType]);

  const handleClick = (id: string, e?: React.MouseEvent) => {
    // 阻止浏览器的默认双击行为（选中文本）
    if (e) {
      e.preventDefault();
    }
    
    // 防抖：如果在500ms内重复点击同一项，忽略后续点击
    const now = Date.now();
    const lastClickTime = lastClickTimeRef.current[id] || 0;
    const timeDiff = now - lastClickTime;
    
    if (timeDiff < 500) {
      return;
    }
    lastClickTimeRef.current[id] = now;
    
    const clickVersion = contentVersionRef.current;
    
    const scrollToElement = (retryCount = 0) => {
      if (clickVersion !== contentVersionRef.current) {
        return false;
      }
      
      const el = document.getElementById(id);
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'start' });
        return true;
      }
      
      // 使用指数退避策略，最多重试20次
      if (retryCount < 20) {
        const delay = Math.min(100 + retryCount * 50, 500);
        const timer = setTimeout(() => {
          pendingTimersRef.current.delete(timer);
          scrollToElement(retryCount + 1);
        }, delay);
        pendingTimersRef.current.add(timer);
        return false;
      }
      
      return false;
    };
    
    // 首次尝试前等待更长时间，确保tab切换和渲染完成
    const initialTimer = setTimeout(() => {
      pendingTimersRef.current.delete(initialTimer);
      scrollToElement();
    }, 250);
    pendingTimersRef.current.add(initialTimer);
  };

  // 从 content 中提取章节及其子章节的内容
  const getSectionContent = (item: TOCItem): string => {
    const lines = content.split('\n');
    const headingRegex = /^(#{1,6})\s+(.+?)\s*$/;
    
    let startIndex = -1;
    let endIndex = lines.length;
    let currentLevel = 0;

    // 找到当前章节的起始位置
    for (let i = 0; i < lines.length; i++) {
      const m = headingRegex.exec(lines[i]);
      if (m && m[2].trim() === item.text) {
        startIndex = i;
        currentLevel = m[1].length;
        break;
      }
    }

    if (startIndex === -1) return '';

    // 找到下一个同级或更高级标题的位置
    for (let i = startIndex + 1; i < lines.length; i++) {
      const m = headingRegex.exec(lines[i]);
      if (m && m[1].length <= currentLevel) {
        endIndex = i;
        break;
      }
    }

    // 提取内容
    return lines.slice(startIndex, endIndex).join('\n');
  };

  // 复制章节名
  const handleCopySectionName = (item: TOCItem) => {
    if (!taskId || !docType) {
      message.error('缺少任务或文档类型信息');
      return;
    }

    const docTypeMap = {
      requirements: '需求文档',
      design: '设计文档',
      test: '测试文档'
    };

    const copyText = `${taskId}::${docTypeMap[docType]}::${item.text}`;
    
    navigator.clipboard.writeText(copyText).then(() => {
      message.success(`已复制: ${copyText}`);
    }).catch(err => {
      console.error('复制失败:', err);
      message.error('复制失败');
    });
  };

  // 添加到MCP资源
  const handleAddToMCPResource = (item: TOCItem) => {
    setContextMenuItem(item);
    form.setFieldsValue({
      name: `${item.text} - ${taskId}`,
      description: `来自任务 ${taskId} 的章节内容`,
    });
    setModalVisible(true);
  };

  // 提交MCP资源
  const handleSubmitMCPResource = async () => {
    if (!contextMenuItem) return;

    try {
      const values = await form.validateFields();
      const auth = loadAuth();
      if (!auth) {
        message.error('请先登录');
        return;
      }

      setSaving(true);

      // 获取章节及其子章节的内容
      const sectionContent = getSectionContent(contextMenuItem);

      await addCustomResource(auth.username, {
        name: values.name,
        description: values.description,
        content: sectionContent,
        visibility: 'private',
        projectId: projectId,
        taskId: taskId,
      });

      message.success('已添加到MCP资源');
      setModalVisible(false);
      form.resetFields();
    } catch (error: any) {
      console.error('添加MCP资源失败:', error);
      message.error('添加失败: ' + (error.message || '未知错误'));
    } finally {
      setSaving(false);
    }
  };

  // 编辑章节
  const handleEditSection = (item: TOCItem) => {
    if (!onEditSection) {
      message.warning('编辑功能不可用');
      return;
    }
    
    // 将标题文本传递给父组件，让父组件根据标题查找对应的章节ID
    // 这里使用标题文本作为标识，父组件需要在章节列表中查找匹配的章节
    onEditSection(item.text);
  };

  // 右键菜单
  const getContextMenu = (item: TOCItem): MenuProps => ({
    items: [
      {
        key: 'edit-section',
        icon: <EditOutlined />,
        label: '章节编辑',
        onClick: () => handleEditSection(item),
        disabled: !projectId || !taskId || !docType || !onEditSection,
      },
      {
        key: 'copy-name',
        icon: <CopyOutlined />,
        label: '复制章节名',
        onClick: () => handleCopySectionName(item),
      },
      {
        key: 'add-to-mcp',
        icon: <PlusOutlined />,
        label: '添加到MCP资源',
        onClick: () => handleAddToMCPResource(item),
      },
    ],
  });

  if (!content || items.length === 0) {
    return <div style={{ fontSize: 12, color: '#999' }}>无可用目录</div>;
  }

  return (
    <>
      <nav style={{ fontSize: 12, lineHeight: 1.4 }}>
        <div style={{ fontWeight: 600, marginBottom: 8 }}>目录</div>
        <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
          {items.map(item => (
            <Dropdown
              key={item.id}
              menu={getContextMenu(item)}
              trigger={['contextMenu']}
              disabled={!projectId || !taskId || !docType}
            >
              <li
                style={{
                  margin: '4px 0',
                  paddingLeft: (item.level - 1) * 12,
                  cursor: 'pointer',
                  userSelect: 'none', // 防止双击选中文本
                }}
                onClick={(e) => handleClick(item.id, e)}
              >
                <span
                  style={{
                    display: 'inline-block',
                    maxWidth: '100%',
                    whiteSpace: 'nowrap',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis'
                  }}
                  title={item.text}
                >
                  {item.text}
                </span>
              </li>
            </Dropdown>
          ))}
        </ul>
      </nav>

      {/* 添加到MCP资源的模态框 */}
      <Modal
        title="添加到MCP资源"
        open={modalVisible}
        onOk={handleSubmitMCPResource}
        onCancel={() => {
          setModalVisible(false);
          form.resetFields();
        }}
        confirmLoading={saving}
        okText="添加"
        cancelText="取消"
      >
        <Form
          form={form}
          layout="vertical"
        >
          <Form.Item
            name="name"
            label="资源名称"
            rules={[{ required: true, message: '请输入资源名称' }]}
          >
            <Input placeholder="请输入资源名称" />
          </Form.Item>
          <Form.Item
            name="description"
            label="资源描述"
          >
            <Input.TextArea
              rows={3}
              placeholder="请输入资源描述（可选）"
            />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

export default DocumentTOC;
