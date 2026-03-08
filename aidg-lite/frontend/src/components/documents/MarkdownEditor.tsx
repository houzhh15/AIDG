import React, { useState, useEffect, useRef, useLayoutEffect } from 'react';
import { 
  Card, 
  Button, 
  Typography, 
  Space, 
  Tabs, 
  Input, 
  Tooltip, 
  Modal,
  message,
  Dropdown,
  Menu,
  Divider,
  Tree
} from 'antd';
import type { DataNode } from 'antd/es/tree';
import { 
  EditOutlined, 
  EyeOutlined, 
  SaveOutlined, 
  UndoOutlined,
  RedoOutlined,
  BoldOutlined,
  ItalicOutlined,
  UnorderedListOutlined,
  OrderedListOutlined,
  LinkOutlined,
  PictureOutlined,
  CodeOutlined,
  TableOutlined,
  FullscreenOutlined,
  CompressOutlined,
  MenuOutlined
} from '@ant-design/icons';
import MarkdownViewer from '../MarkdownViewer';

const { Title, Text } = Typography;
const { TabPane } = Tabs;
const { TextArea } = Input;

interface MarkdownEditorProps {
  value: string;
  onChange: (value: string) => void;
  onSave?: (value: string) => void;
  loading?: boolean;
  height?: number;
  placeholder?: string;
  readOnly?: boolean;
  showToolbar?: boolean;
  showPreview?: boolean;
  autoSave?: boolean;
  autoSaveInterval?: number;
}

// 标题结构
interface HeadingItem {
  id: string;
  text: string;
  level: number;
  children?: HeadingItem[];
}

// Markdown工具栏按钮配置
interface ToolbarButton {
  key: string;
  icon: React.ReactNode;
  title: string;
  action: (editor: any) => void;
  shortcut?: string;
}

const MarkdownEditor: React.FC<MarkdownEditorProps> = ({
  value,
  onChange,
  onSave,
  loading = false,
  height = 400,
  placeholder = '请输入Markdown内容...',
  readOnly = false,
  showToolbar = true,
  showPreview = true,
  autoSave = false,
  autoSaveInterval = 5000
}) => {
  const [currentValue, setCurrentValue] = useState(value);
  const [activeTab, setActiveTab] = useState<string>(showPreview ? 'preview' : 'edit');
  const [fullscreen, setFullscreen] = useState<boolean>(false);
  const [history, setHistory] = useState<string[]>([value]);
  const [historyIndex, setHistoryIndex] = useState(0);
  const [isModified, setIsModified] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [showToc, setShowToc] = useState(false); // 显示目录 - 默认隐藏
  const [headings, setHeadings] = useState<HeadingItem[]>([]); // 标题列表
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([]); // 展开的节点
  const textAreaRef = useRef<any>(null);
  const autoSaveTimerRef = useRef<NodeJS.Timeout | null>(null);
  const saveInProgressRef = useRef(false);
  const previewContainerRef = useRef<HTMLDivElement>(null);

  // 将扁平的标题列表转换为树形结构
  const buildHeadingTree = (headings: HeadingItem[]): DataNode[] => {
    const root: DataNode[] = [];
    const stack: { node: DataNode; level: number }[] = [];

    headings.forEach(heading => {
      const node: DataNode = {
        key: heading.id,
        title: (
          <div 
            style={{ 
              fontSize: 12,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              cursor: 'pointer'
            }}
            onClick={(e) => {
              e.stopPropagation();
              
              const container = previewContainerRef.current;
              let element = document.getElementById(heading.id);
              
              // 如果通过ID找不到，尝试通过文本内容查找（处理ID重复加序号的情况）
              if (!element && container) {
                const allHeadings = container.querySelectorAll<HTMLHeadingElement>('h1, h2, h3, h4, h5, h6');
                element = Array.from(allHeadings).find(h => 
                  h.textContent?.trim() === heading.text
                ) || null;
              }
              
              if (!element) {
                return;
              }
              
              if (container && container.contains(element)) {
                // 计算元素相对于容器的位置
                const containerRect = container.getBoundingClientRect();
                const elementRect = element.getBoundingClientRect();
                const scrollTop = container.scrollTop + (elementRect.top - containerRect.top) - 16; // 16px padding
                
                // 平滑滚动到目标位置
                container.scrollTo({
                  top: scrollTop,
                  behavior: 'smooth'
                });
              } else {
                // 全屏模式
                element.scrollIntoView({ behavior: 'smooth', block: 'start' });
              }
            }}
          >
            {heading.text}
          </div>
        )
        // 不预先初始化children，让Tree组件自动判断是否有子节点
      };

      // 找到父节点
      while (stack.length > 0 && stack[stack.length - 1].level >= heading.level) {
        stack.pop();
      }

      if (stack.length === 0) {
        // 顶层节点
        root.push(node);
      } else {
        // 作为子节点添加
        const parent = stack[stack.length - 1].node;
        if (!parent.children) {
          parent.children = [];
        }
        parent.children.push(node);
      }

      stack.push({ node, level: heading.level });
    });

    return root;
  };

  useEffect(() => {
    // 只有当值真的发生变化且不是在保存过程中时才更新
    if (value !== currentValue && !saveInProgressRef.current) {
      setCurrentValue(value);
      setHistory([value]);
      setHistoryIndex(0);
      setIsModified(false);

      if (showPreview) {
        setActiveTab('preview');
      }
    }
  }, [value, currentValue, showPreview]);

  useLayoutEffect(() => {
    if (!showPreview) {
      return;
    }
    if (activeTab !== 'preview') {
      return;
    }

    const container = previewContainerRef.current;
    if (!container) {
      return;
    }

    const headingElements = container.querySelectorAll<HTMLHeadingElement>('h1, h2, h3, h4, h5, h6');
    if (headingElements.length === 0) {
      if (headings.length !== 0) {
        setHeadings([]);
        setExpandedKeys([]);
      }
      return;
    }

    const newHeadings: HeadingItem[] = [];

    headingElements.forEach((el) => {
      const level = Number.parseInt(el.tagName.substring(1), 10) || 1;
      const text = (el.textContent || '').trim();
      if (!text) {
        return;
      }

      // 直接使用元素上已有的ID（由MarkdownViewer生成）
      const id = el.id;
      if (!id) {
        return;
      }

      newHeadings.push({ id, text, level });
    });

    const newExpandedKeys = newHeadings.map((h) => h.id);

    const isSameHeadings =
      headings.length === newHeadings.length &&
      headings.every((item, index) => {
        const target = newHeadings[index];
        return item.id === target.id && item.text === target.text && item.level === target.level;
      });

    if (!isSameHeadings) {
      setHeadings(newHeadings);
      setExpandedKeys((prev) => {
        if (prev.length === 0) {
          return newExpandedKeys;
        }

        const filtered = prev.filter((key) => newExpandedKeys.includes(String(key)));
        return filtered.length > 0 ? filtered : newExpandedKeys;
      });
      return;
    }

    setExpandedKeys((prev) => {
      if (prev.length === 0) {
        return prev;
      }
      const filtered = prev.filter((key) => newExpandedKeys.includes(String(key)));
      return filtered.length === prev.length ? prev : filtered;
    });
  }, [showPreview, activeTab, currentValue]); // 移除 headings 和 expandedKeys 避免无限循环

  useEffect(() => {
    if (autoSave && isModified && onSave) {
      if (autoSaveTimerRef.current) {
        clearTimeout(autoSaveTimerRef.current);
      }
      autoSaveTimerRef.current = setTimeout(() => {
        onSave(currentValue);
        setIsModified(false);
        message.success('自动保存成功');
      }, autoSaveInterval);
    }

    return () => {
      if (autoSaveTimerRef.current) {
        clearTimeout(autoSaveTimerRef.current);
      }
    };
  }, [currentValue, autoSave, autoSaveInterval, isModified, onSave]);

  const handleChange = (newValue: string) => {
    setCurrentValue(newValue);
    onChange(newValue);
    setIsModified(true);

    // 添加到历史记录
    if (newValue !== history[historyIndex]) {
      const newHistory = history.slice(0, historyIndex + 1);
      newHistory.push(newValue);
      setHistory(newHistory);
      setHistoryIndex(newHistory.length - 1);
    }
  };

  const handleSave = async () => {
    // 防抖：如果正在保存中，直接返回
    if (!onSave || saveInProgressRef.current || isSaving) {
      console.warn('MarkdownEditor: 保存操作已在进行中或onSave未定义');
      return;
    }

    console.log('MarkdownEditor: 开始保存文档');
    
    try {
      // 设置保存状态
      saveInProgressRef.current = true;
      setIsSaving(true);
      
      // 调用父组件的保存函数
      await onSave(currentValue);
      
      console.log('MarkdownEditor: 保存成功');
      
      // 保存成功后标记为未修改
      setIsModified(false);
      
    } catch (error) {
      console.error('MarkdownEditor: 保存失败:', error);
      // 保存失败时不重置 isModified，让用户知道还有未保存的更改
    } finally {
      // 无论成功失败都要清除保存状态
      saveInProgressRef.current = false;
      setIsSaving(false);
    }
  };

  const handleUndo = () => {
    if (historyIndex > 0) {
      const newIndex = historyIndex - 1;
      setHistoryIndex(newIndex);
      const newValue = history[newIndex];
      setCurrentValue(newValue);
      onChange(newValue);
    }
  };

  const handleRedo = () => {
    if (historyIndex < history.length - 1) {
      const newIndex = historyIndex + 1;
      setHistoryIndex(newIndex);
      const newValue = history[newIndex];
      setCurrentValue(newValue);
      onChange(newValue);
    }
  };

  const insertText = (before: string, after: string = '', placeholder: string = '') => {
    const textarea = textAreaRef.current?.resizableTextArea?.textArea;
    if (!textarea) return;

    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const selectedText = currentValue.substring(start, end);
    const textToInsert = selectedText || placeholder;
    const newText = before + textToInsert + after;
    
    const newValue = currentValue.substring(0, start) + newText + currentValue.substring(end);
    handleChange(newValue);

    // 设置光标位置
    setTimeout(() => {
      textarea.focus();
      if (selectedText) {
        textarea.setSelectionRange(start + before.length, start + before.length + selectedText.length);
      } else {
        textarea.setSelectionRange(start + before.length, start + before.length + placeholder.length);
      }
    }, 0);
  };

  // 工具栏按钮配置
  const toolbarButtons: ToolbarButton[] = [
    {
      key: 'bold',
      icon: <BoldOutlined />,
      title: '粗体',
      action: () => insertText('**', '**', '粗体文本'),
      shortcut: 'Ctrl+B'
    },
    {
      key: 'italic',
      icon: <ItalicOutlined />,
      title: '斜体',
      action: () => insertText('*', '*', '斜体文本'),
      shortcut: 'Ctrl+I'
    },
    {
      key: 'code',
      icon: <CodeOutlined />,
      title: '代码',
      action: () => insertText('`', '`', '代码'),
      shortcut: 'Ctrl+`'
    },
    {
      key: 'link',
      icon: <LinkOutlined />,
      title: '链接',
      action: () => insertText('[', '](url)', '链接文本'),
      shortcut: 'Ctrl+K'
    },
    {
      key: 'image',
      icon: <PictureOutlined />,
      title: '图片',
      action: () => insertText('![', '](image-url)', '图片描述'),
      shortcut: 'Ctrl+Shift+I'
    },
    {
      key: 'unordered-list',
      icon: <UnorderedListOutlined />,
      title: '无序列表',
      action: () => insertText('- ', '', '列表项'),
      shortcut: 'Ctrl+Shift+8'
    },
    {
      key: 'ordered-list',
      icon: <OrderedListOutlined />,
      title: '有序列表',
      action: () => insertText('1. ', '', '列表项'),
      shortcut: 'Ctrl+Shift+7'
    },
    {
      key: 'table',
      icon: <TableOutlined />,
      title: '表格',
      action: () => insertText('\n| 标题1 | 标题2 |\n| --- | --- |\n| 内容1 | 内容2 |\n'),
      shortcut: 'Ctrl+Shift+T'
    }
  ];

  const headingMenu = (
    <Menu onClick={({ key }) => {
      const level = parseInt(key as string);
      const prefix = '#'.repeat(level) + ' ';
      insertText(prefix, '', `标题${level}`);
    }}>
      <Menu.Item key="1">H1 - 一级标题</Menu.Item>
      <Menu.Item key="2">H2 - 二级标题</Menu.Item>
      <Menu.Item key="3">H3 - 三级标题</Menu.Item>
      <Menu.Item key="4">H4 - 四级标题</Menu.Item>
      <Menu.Item key="5">H5 - 五级标题</Menu.Item>
      <Menu.Item key="6">H6 - 六级标题</Menu.Item>
    </Menu>
  );

  const renderToolbar = () => (
    <div style={{ 
      borderBottom: '1px solid #f0f0f0', 
      padding: '8px 12px',
      display: 'flex',
      alignItems: 'center',
      gap: 4
    }}>
      <Space size={4}>
        {/* 撤销重做 */}
        <Tooltip title={`撤销 (Ctrl+Z)`}>
          <Button 
            size="small" 
            icon={<UndoOutlined />} 
            onClick={handleUndo}
            disabled={historyIndex <= 0}
          />
        </Tooltip>
        <Tooltip title={`重做 (Ctrl+Y)`}>
          <Button 
            size="small" 
            icon={<RedoOutlined />} 
            onClick={handleRedo}
            disabled={historyIndex >= history.length - 1}
          />
        </Tooltip>

        <Divider type="vertical" />

        {/* 标题 */}
        <Dropdown overlay={headingMenu} placement="bottomLeft">
          <Button size="small">
            标题 ▼
          </Button>
        </Dropdown>

        <Divider type="vertical" />

        {/* 格式化工具 */}
        {toolbarButtons.map(button => (
          <Tooltip key={button.key} title={`${button.title} (${button.shortcut})`}>
            <Button 
              size="small" 
              icon={button.icon} 
              onClick={button.action}
            />
          </Tooltip>
        ))}

        <Divider type="vertical" />

        {/* 保存和全屏 */}
        <Tooltip title="保存 (Ctrl+S)">
          <Button 
            size="small" 
            type={isModified ? 'primary' : 'default'}
            icon={<SaveOutlined />} 
            onClick={handleSave}
            disabled={!onSave || loading || isSaving}
            loading={loading || isSaving}
          />
        </Tooltip>

        <Tooltip title={fullscreen ? '退出全屏 (Esc)' : '全屏编辑 (F11)'}>
          <Button 
            size="small" 
            icon={fullscreen ? <CompressOutlined /> : <FullscreenOutlined />} 
            onClick={() => setFullscreen(!fullscreen)}
          />
        </Tooltip>
      </Space>

      {isModified && (
        <Text type="secondary" style={{ marginLeft: 'auto', fontSize: 12 }}>
          未保存的更改
        </Text>
      )}
    </div>
  );

  const renderPreview = () => {
    // 计算预览区域高度
    let previewHeight;
    if (fullscreen) {
      // 全屏模式：100vh - Modal标题(55px) - 工具栏(~41px) - Tabs标签栏(47px) - 余量(5px)
      previewHeight = 'calc(100vh - 148px)';
    } else {
      // 普通模式：减去Tabs标签栏高度(47px)
      previewHeight = height - 47;
    }
    
    return (
    <div style={{ display: 'flex', height: previewHeight }}>
      {/* 左侧目录导航 */}
      {showToc && headings.length > 0 && (
        <div
          style={{
            width: 220,
            borderRight: '1px solid #f0f0f0',
            overflowY: 'auto',
            padding: '16px 12px',
            flexShrink: 0
          }}
        >
          <div style={{ 
            fontSize: 12, 
            fontWeight: 600, 
            marginBottom: 12,
            color: '#000',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between'
          }}>
            <span>目录</span>
            <Button 
              type="text" 
              size="small" 
              icon={<MenuOutlined />}
              onClick={() => setShowToc(false)}
              style={{ marginRight: -8 }}
            />
          </div>
          <Tree
            showLine
            showIcon={false}
            defaultExpandAll
            expandedKeys={expandedKeys}
            onExpand={(keys) => setExpandedKeys(keys)}
            treeData={buildHeadingTree(headings)}
            style={{
              fontSize: 12,
              background: 'transparent'
            }}
          />
        </div>
      )}

      {/* 预览内容区域 */}
      <div
        ref={previewContainerRef}
        style={{
          flex: 1,
          padding: 16,
          overflow: 'auto',
          lineHeight: 1.6,
          position: 'relative'
        }}
      >
        <MarkdownViewer>{currentValue}</MarkdownViewer>
      </div>
    </div>
    );
  };

  const renderEditor = () => {
    // 计算编辑器高度
    let editorHeight;
    if (fullscreen) {
      // 全屏模式：100vh - Modal标题(55px) - 工具栏(~41px) - Tabs标签栏(47px) - 余量(5px)
      editorHeight = 'calc(100vh - 148px)';
    } else {
      // 普通模式：减去Tabs标签栏高度(47px)
      editorHeight = height - 47;
    }
    
    return (
    <div style={{ position: 'relative' }}>
      <TextArea
        ref={textAreaRef}
        value={currentValue}
        onChange={(e) => handleChange(e.target.value)}
        placeholder={placeholder}
        readOnly={readOnly}
        style={{ 
          height: editorHeight,
          resize: 'none',
          fontFamily: 'Consolas, Monaco, "Courier New", monospace'
        }}
        onKeyDown={(e) => {
          // 快捷键处理
          if (e.ctrlKey || e.metaKey) {
            switch (e.key) {
              case 's':
                e.preventDefault();
                handleSave();
                break;
              case 'z':
                e.preventDefault();
                handleUndo();
                break;
              case 'y':
                e.preventDefault();
                handleRedo();
                break;
              case 'b':
                e.preventDefault();
                insertText('**', '**', '粗体文本');
                break;
              case 'i':
                e.preventDefault();
                insertText('*', '*', '斜体文本');
                break;
              case '`':
                e.preventDefault();
                insertText('`', '`', '代码');
                break;
              case 'k':
                e.preventDefault();
                insertText('[', '](url)', '链接文本');
                break;
            }
          }
          
          if (e.key === 'F11') {
            e.preventDefault();
            setFullscreen(!fullscreen);
          }
        }}
      />
    </div>
    );
  };

  const editorContent = (
    <Card 
      loading={loading}
      style={{ 
        width: fullscreen ? '100vw' : '100%',
        height: fullscreen ? '100vh' : 'auto'
      }}
      bodyStyle={{ padding: 0 }}
    >
      {showToolbar && !readOnly && renderToolbar()}
      
      {showPreview && !readOnly ? (
        <Tabs 
          activeKey={activeTab} 
          onChange={setActiveTab}
          size="small"
          tabBarStyle={{ margin: 0, paddingLeft: 12 }}
          tabBarExtraContent={
            activeTab === 'preview' && !showToc && headings.length > 0 ? (
              <Button
                type="primary"
                size="small"
                icon={<MenuOutlined />}
                onClick={() => setShowToc(true)}
                style={{ marginRight: fullscreen ? 24 : 12 }}
              >
                显示目录
              </Button>
            ) : null
          }
        >
          <TabPane tab={<span><EditOutlined />编辑</span>} key="edit">
            {renderEditor()}
          </TabPane>
          <TabPane tab={<span><EyeOutlined />预览</span>} key="preview">
            {renderPreview()}
          </TabPane>
        </Tabs>
      ) : (
        readOnly ? renderPreview() : renderEditor()
      )}
    </Card>
  );

  if (fullscreen) {
    return (
      <Modal
        title="Markdown编辑器"
        open={fullscreen}
        onCancel={() => setFullscreen(false)}
        footer={null}
        width="100vw"
        style={{ 
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          maxWidth: 'none', 
          margin: 0, 
          padding: 0,
          paddingBottom: 0
        }}
        bodyStyle={{ 
          padding: 0, 
          height: 'calc(100vh - 55px)', 
          overflow: 'hidden'
        }}
        styles={{ 
          body: { padding: 0 },
          content: { padding: 0 },
          header: { padding: '16px 24px' },
          wrapper: { position: 'fixed', top: 0, left: 0, right: 0, bottom: 0 }
        }}
        modalRender={(modal) => (
          <div style={{ position: 'fixed', top: 0, left: 0, width: '100vw', height: '100vh' }}>
            {modal}
          </div>
        )}
      >
        {editorContent}
      </Modal>
    );
  }

  return editorContent;
};

export default MarkdownEditor;