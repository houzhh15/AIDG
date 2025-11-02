import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import { Modal, Button } from 'antd';
import { FullscreenOutlined, FullscreenExitOutlined } from '@ant-design/icons';
import '../markdown.css';
import { MermaidChart } from './MermaidChart';

interface Props {
  children: string;
  className?: string;
  allowMermaid?: boolean;
  showFullscreenButton?: boolean;
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

const MarkdownViewer: React.FC<Props> = ({ children, className, allowMermaid = true, showFullscreenButton = false }) => {
  const [isFullscreen, setIsFullscreen] = React.useState(false);

  // 每次渲染时重置ID计数器，确保ID生成的一致性
  const idCountRef = React.useRef(new Map<string, number>());
  
  // 在渲染开始前重置计数器
  React.useMemo(() => {
    idCountRef.current = new Map<string, number>();
  }, [children]);
  
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
  
  const generateHeadingId = (children: any): string => {
    // 先提取纯文本
    const text = extractText(children);
    
    let id = text
      .toLowerCase()
      .replace(/[^\w\u4e00-\u9fa5\s-]/g, '')
      .replace(/\s+/g, '-');
    
    // 处理重复ID，添加序号
    const idCount = idCountRef.current;
    if (idCount.has(id)) {
      const count = idCount.get(id)! + 1;
      idCount.set(id, count);
      id = `${id}-${count}`;
    } else {
      idCount.set(id, 0);
    }
    
    return id;
  };

  return (
    <>
      <div className={className ? `markdown-body ${className}` : 'markdown-body'} style={{ position: 'relative' }}>
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
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          components={{
            // 为标题添加id属性以支持锚点跳转
            h1({ children, ...props }) {
              const id = generateHeadingId(children);
              return <h1 id={id} {...props}>{children}</h1>;
            },
            h2({ children, ...props }) {
              const id = generateHeadingId(children);
              return <h2 id={id} {...props}>{children}</h2>;
            },
            h3({ children, ...props }) {
              const id = generateHeadingId(children);
              return <h3 id={id} {...props}>{children}</h3>;
            },
            h4({ children, ...props }) {
              const id = generateHeadingId(children);
              return <h4 id={id} {...props}>{children}</h4>;
            },
            h5({ children, ...props }) {
              const id = generateHeadingId(children);
              return <h5 id={id} {...props}>{children}</h5>;
            },
            h6({ children, ...props }) {
              const id = generateHeadingId(children);
              return <h6 id={id} {...props}>{children}</h6>;
            },
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
        bodyStyle={{ height: '80vh', overflow: 'auto', padding: '20px' }}
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
