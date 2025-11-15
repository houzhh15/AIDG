import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
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
  
  // 如果 children 改变了，重置计数器
  if (prevChildrenRef.current !== children) {
    prevChildrenRef.current = children;
    idCountRef.current = new Map<string, number>();
  }
  
  // 从React节点中提取纯文本（处理加粗、斜体等格式）
  const extractText = (node: any): string => {
    if (typeof node === 'string') {
      return node;
    }
    if (Array.isArray(node)) {
      return node.map(extractText).join('');
    }
    if (node && node.props && node.props.children) {
      return extractText(node.props.children);
    }
    return '';
  };
  
  // 创建带编辑按钮的标题组件
  const createHeadingWithEditButton = (level: 1 | 2 | 3 | 4 | 5 | 6) => {
    return ({ children, ...props }: any) => {
      const id = generateHeadingId(children);
      const text = extractText(children);
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
  
  const generateHeadingId = (children: any): string => {
    // 先提取纯文本
    const text = extractText(children);
    
    let id = text
      .toLowerCase()
      .replace(/[^\w\u4e00-\u9fa5\s-]/g, '')
      .replace(/\s+/g, '-');
    
    // 处理重复ID，添加序号
    // 注意：第一次出现的ID不加后缀，从第二次开始才加 -1, -2, ...
    const idCount = idCountRef.current;
    const currentCount = idCount.get(id);
    
    if (currentCount !== undefined) {
      // 不是第一次出现
      const count = currentCount + 1;
      idCount.set(id, count);
      return `${id}-${count}`;
    } else {
      // 第一次出现，不添加后缀
      idCount.set(id, 0);
      return id;
    }
  };

  // 使用 useMemo 缓存 ReactMarkdown 渲染结果，只有当 children 改变时才重新渲染
  // 这样可以确保 ID 计数器不会在相同内容下被重复计数
  const markdownContent = React.useMemo(() => {
    return (
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
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
  }, [children, allowMermaid]); // 依赖 children 和 allowMermaid

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

export default MarkdownViewer;
