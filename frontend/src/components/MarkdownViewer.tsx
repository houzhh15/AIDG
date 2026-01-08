import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Modal, Button } from 'antd';
import { FullscreenOutlined, FullscreenExitOutlined, EditOutlined, CopyOutlined, PlusOutlined } from '@ant-design/icons';
import '../markdown.css';
import { MermaidChart } from './MermaidChart';

interface Props {
  children: string;
  className?: string;
  allowMermaid?: boolean;
  showFullscreenButton?: boolean;
  onEditSection?: (sectionTitle: string) => void; // 编辑章节的回调
  onCopySectionName?: (sectionTitle: string) => void; // 复制章节名的回调
  onAddToMCP?: (sectionTitle: string) => void; // 添加到MCP资源的回调
}

const tableBaseStyle: React.CSSProperties = {
  borderCollapse: 'collapse',
  width: '100%',
  marginBottom: '16px',
  border: '1px solid #d0d7de'
};

const syntaxHighlighterCustomStyle: React.CSSProperties = {
  background: '#36383B',
  borderRadius: 8,
  padding: 12,
  margin: '16px 0',
  overflow: 'auto',
  boxShadow: 'inset 0 0 0 1px rgba(0,0,0,0.35)'
};

const syntaxHighlighterCodeProps = {
  style: {
    background: 'transparent',
    fontFamily: '"Fira Code", "Source Code Pro", Consolas, Monaco, "Courier New", monospace'
  }
};

const MarkdownViewer: React.FC<Props> = ({ 
  children, 
  className, 
  allowMermaid = true, 
  showFullscreenButton = false, 
  onEditSection,
  onCopySectionName,
  onAddToMCP
}) => {
  const [isFullscreen, setIsFullscreen] = React.useState(false);

  // 使用 ref 来存储上一次的 children 和 ID 计数器
  const prevChildrenRef = React.useRef<string>();
  const idCountRef = React.useRef(new Map<string, number>());
  
  // 使用 useEffect 来检测 children 的真实变化并重置计数器
  React.useEffect(() => {
    if (prevChildrenRef.current !== children) {
      console.log('[MarkdownViewer] Content changed, resetting ID counter');
      prevChildrenRef.current = children;
      idCountRef.current = new Map<string, number>();
    }
  }, [children]);
  
  // 从React节点中提取纯文本（处理加粗、斜体等格式）
  // 使用 useCallback 确保函数引用稳定
  const extractText = React.useCallback((node: React.ReactNode, visited = new WeakSet()): string => {
    // 处理基本类型
    if (typeof node === 'string') {
      return node;
    }
    if (typeof node === 'number') {
      return String(node);
    }
    if (!node || typeof node === 'boolean') {
      return '';
    }
    
    // 处理数组
    if (Array.isArray(node)) {
      return node.map(n => extractText(n, visited)).join('');
    }
    
    // 处理对象 - 检查循环引用
    if (typeof node === 'object') {
      // 检查是否已经访问过这个对象
      if (visited.has(node)) {
        console.warn('[MarkdownViewer] Circular reference detected in extractText');
        return '';
      }
      
      // 标记为已访问
      visited.add(node);
      
      // 处理 React 元素
      if ('props' in node && node.props && 'children' in node.props) {
        return extractText(node.props.children, visited);
      }
    }
    
    return '';
  }, []);
  
  // 创建带编辑按钮的标题组件
  const createHeadingWithEditButton = (level: 1 | 2 | 3 | 4 | 5 | 6) => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return ({ children, ...props }: any) => {
      const text = extractText(children);
      
      // 生成ID（简化版，避免递归调用generateHeadingId）
      const id = text
        .toLowerCase()
        .replace(/[^\w\u4e00-\u9fa5\s-]/g, '')
        .replace(/\s+/g, '-');
      
      // 处理重复ID
      const idCount = idCountRef.current;
      const currentCount = idCount.get(id);
      let finalId: string;
      if (currentCount !== undefined) {
        const count = currentCount + 1;
        idCount.set(id, count);
        finalId = `${id}-${count}`;
      } else {
        idCount.set(id, 0);
        finalId = id;
      }
      
      const HeadingTag = `h${level}` as keyof JSX.IntrinsicElements;
      
      // 检查是否有可用操作
      const hasActions = onEditSection || onCopySectionName || onAddToMCP;
      
      return (
        <HeadingTag 
          id={id} 
          {...props} 
          style={{ 
            position: 'relative',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            ...props.style 
          }}
          className={hasActions ? "markdown-heading-with-edit" : ""}
        >
          {hasActions && (
            <span 
              className="heading-interactive-indicator"
              style={{
                fontSize: '0.7em',
                color: '#1890ff',
                opacity: 0.4,
                transition: 'opacity 0.2s',
                marginRight: '4px',
              }}
            >
              ✦
            </span>
          )}
          {children}
          {hasActions && (
            <div 
              className="heading-action-buttons"
              style={{
                opacity: 0,
                transition: 'opacity 0.2s',
                display: 'flex',
                gap: '4px',
                marginLeft: '8px'
              }}
            >
              {onEditSection && (
                <Button
                  type="text"
                  size="small"
                  icon={<EditOutlined />}
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    onEditSection(text);
                  }}
                  style={{
                    padding: '2px 6px',
                    height: 'auto',
                    fontSize: '12px',
                    color: '#1890ff',
                  }}
                  title="编辑章节"
                />
              )}
              {onCopySectionName && (
                <Button
                  type="text"
                  size="small"
                  icon={<CopyOutlined />}
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    onCopySectionName(text);
                  }}
                  style={{
                    padding: '2px 6px',
                    height: 'auto',
                    fontSize: '12px',
                    color: '#52c41a',
                  }}
                  title="复制章节名"
                />
              )}
              {onAddToMCP && (
                <Button
                  type="text"
                  size="small"
                  icon={<PlusOutlined />}
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    onAddToMCP(text);
                  }}
                  style={{
                    padding: '2px 6px',
                    height: 'auto',
                    fontSize: '12px',
                    color: '#722ed1',
                  }}
                  title="添加到MCP资源"
                />
              )}
            </div>
          )}
        </HeadingTag>
      );
    };
  };
  
  // 移除了 generateHeadingId 函数，因为逻辑已经合并到 createHeadingWithEditButton 中


  // 使用 useMemo 缓存 ReactMarkdown 渲染结果，只有当 children 改变时才重新渲染
  // 这样可以确保 ID 计数器不会在相同内容下被重复计数
  const markdownContent = React.useMemo(() => {
    return (
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
        components={{
          // 为标题添加id属性以支持锚点跳转，并添加编辑按钮
          h1: createHeadingWithEditButton(1),
          h2: createHeadingWithEditButton(2),
          h3: createHeadingWithEditButton(3),
          h4: createHeadingWithEditButton(4),
          h5: createHeadingWithEditButton(5),
          h6: createHeadingWithEditButton(6),
          code({ className: codeClassName, children: codeChildren, ...props }) {
            const match = /language-(\w+)/.exec(codeClassName || '');
            const content = String(codeChildren).replace(/\n$/, '');

            if (match && match[1] === 'mermaid' && allowMermaid) {
              return <MermaidChart chart={content} />;
            }

            if (match) {
              return (
                <SyntaxHighlighter
                  style={vscDarkPlus}
                  language={match[1]}
                  PreTag="div"
                  customStyle={syntaxHighlighterCustomStyle}
                  codeTagProps={syntaxHighlighterCodeProps}
                >
                  {content}
                </SyntaxHighlighter>
              );
            }

            return (
              <code className={codeClassName} {...props}>
                {codeChildren}
              </code>
            );
          },
          table({ children: tableChildren, ...props }) {
            return (
              <table style={tableBaseStyle} {...props}>
                {tableChildren}
              </table>
            );
          },
          th({ children: thChildren, ...props }) {
            return (
              <th
                style={{
                  border: '1px solid #d0d7de',
                  padding: '8px 12px',
                  backgroundColor: '#f6f8fa',
                  fontWeight: 600,
                  textAlign: 'left'
                }}
                {...props}
              >
                {thChildren}
              </th>
            );
          },
          td({ children: tdChildren, ...props }) {
            return (
              <td
                style={{
                  border: '1px solid #d0d7de',
                  padding: '8px 12px'
                }}
                {...props}
              >
                {tdChildren}
              </td>
            );
          }
        }}
      >
        {children || ''}
      </ReactMarkdown>
    );
  }, [children, allowMermaid, onEditSection, onCopySectionName, onAddToMCP]); // 添加所有依赖

  // 检查是否有可交互的标题
  const hasInteractiveHeadings = !!(onEditSection || onCopySectionName || onAddToMCP);
  
  // 构建 className
  const containerClassName = [
    'markdown-body',
    hasInteractiveHeadings ? 'has-interactive-headings' : '',
    className || ''
  ].filter(Boolean).join(' ');

  return (
    <>
      <div className={containerClassName} style={{ position: 'relative' }}>
        {showFullscreenButton && (
          <Button
            type="text"
            icon={<FullscreenOutlined />}
            onClick={() => setIsFullscreen(true)}
            style={{
              position: 'absolute',
              top: 8,
              right: 8,
              zIndex: 10,
              color: '#666'
            }}
            title="全屏显示"
          />
        )}
        {markdownContent}
      </div>

      <Modal
        title={
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span>全屏查看</span>
            <Button
              type="text"
              icon={<FullscreenExitOutlined />}
              onClick={() => setIsFullscreen(false)}
              style={{ color: '#666' }}
            />
          </div>
        }
        open={isFullscreen}
        onCancel={() => setIsFullscreen(false)}
        footer={null}
        width="90vw"
        style={{ top: 20 }}
        styles={{
          body: { height: '80vh', overflow: 'auto', padding: '20px' }
        }}
        centered
      >
        <MarkdownViewer
          children={children}
          allowMermaid={allowMermaid}
          showFullscreenButton={false}
        />
      </Modal>
    </>
  );
};

// 使用 React.memo 优化组件，防止不必要的重新渲染
// 只有当 children 内容真正改变时才重新渲染
export default React.memo(MarkdownViewer, (prevProps, nextProps) => {
  // 比较所有 props，children 是最重要的
  return (
    prevProps.children === nextProps.children &&
    prevProps.className === nextProps.className &&
    prevProps.allowMermaid === nextProps.allowMermaid &&
    prevProps.showFullscreenButton === nextProps.showFullscreenButton &&
    prevProps.onEditSection === nextProps.onEditSection &&
    prevProps.onCopySectionName === nextProps.onCopySectionName &&
    prevProps.onAddToMCP === nextProps.onAddToMCP
  );
});
