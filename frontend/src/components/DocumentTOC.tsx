import React, { useMemo } from 'react';

interface TOCItem {
  id: string;
  text: string;
  level: number;
}

interface Props {
  content: string;
  minLevel?: number; // minimum heading level to include
  maxLevel?: number; // maximum heading level to include
}

// Generate slug consistent with MarkdownViewer (without duplicate handling yet)
function baseSlug(text: string) {
  return text
    .toLowerCase()
    .replace(/[^\w\u4e00-\u9fa5\s-]/g, '')
    .replace(/\s+/g, '-');
}

export const DocumentTOC: React.FC<Props> = ({ content, minLevel = 1, maxLevel = 4 }) => {
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

  const handleClick = (id: string) => {
    const el = document.getElementById(id);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };

  if (!content || items.length === 0) {
    return <div style={{ fontSize: 12, color: '#999' }}>无可用目录</div>;
  }

  return (
    <nav style={{ fontSize: 12, lineHeight: 1.4 }}>
      <div style={{ fontWeight: 600, marginBottom: 8 }}>目录</div>
      <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
        {items.map(item => (
          <li
            key={item.id}
            style={{
              margin: '4px 0',
              paddingLeft: (item.level - 1) * 12,
              cursor: 'pointer'
            }}
            onClick={() => handleClick(item.id)}
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
        ))}
      </ul>
    </nav>
  );
};

export default DocumentTOC;
