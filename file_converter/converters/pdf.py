"""
file_converter/converters/pdf.py
PDF 文件转换器 - 使用 pdfplumber 提取文本和表格
"""

import io
import logging
from typing import BinaryIO, List, Tuple

import pdfplumber

from file_converter.converters.base import BaseConverter, ConversionResult

logger = logging.getLogger(__name__)


class PdfConverter(BaseConverter):
    """
    PDF 转 Markdown 转换器
    
    功能：
    - 提取 PDF 文本内容
    - 识别并转换表格为 Markdown 表格语法
    - 基于字号推断标题层级
    - 图片位置标注为 [图片略]
    """
    
    supported_extensions = ["pdf"]
    
    # 字号阈值用于判断标题层级
    HEADING_THRESHOLDS = [
        (24, 1),  # 字号 >= 24 为 H1
        (18, 2),  # 字号 >= 18 为 H2
        (14, 3),  # 字号 >= 14 为 H3
    ]
    
    def convert(self, file: BinaryIO, filename: str) -> ConversionResult:
        """将 PDF 转换为 Markdown"""
        warnings: List[str] = []
        content_parts: List[str] = []
        
        try:
            # 使用 pdfplumber 打开 PDF
            with pdfplumber.open(file) as pdf:
                total_pages = len(pdf.pages)
                logger.info(f"Processing PDF: {filename}, {total_pages} pages")
                
                for page_num, page in enumerate(pdf.pages, 1):
                    page_content = self._process_page(page, page_num, warnings)
                    if page_content.strip():
                        content_parts.append(page_content)
                
                # 检查是否有图片
                for page in pdf.pages:
                    if page.images:
                        warnings.append(f"PDF 包含 {len(page.images)} 张图片，已跳过")
                        break
        
        except Exception as e:
            logger.error(f"PDF conversion failed: {filename}, error: {e}")
            raise ValueError(f"PDF 转换失败: {str(e)}")
        
        content = "\n\n".join(content_parts)
        
        if not content.strip():
            warnings.append("PDF 未提取到任何文本内容")
        
        return ConversionResult(content=content, warnings=warnings)
    
    def _process_page(self, page, page_num: int, warnings: List[str]) -> str:
        """处理单页 PDF"""
        parts: List[str] = []
        
        # 提取表格
        tables = page.extract_tables()
        table_bboxes = []
        
        for table in tables:
            if table:
                table_md = self._table_to_markdown(table)
                parts.append(table_md)
                # 记录表格区域，后续避免重复提取
        
        # 提取文本
        text = page.extract_text()
        if text:
            # 简单处理：按行分割，尝试识别标题
            lines = text.split('\n')
            processed_lines = []
            
            for line in lines:
                line = line.strip()
                if not line:
                    continue
                
                # 检查是否可能是标题（短行且不以标点结尾）
                if len(line) < 50 and not line.endswith(('。', '，', '；', '：', '.', ',')):
                    # 尝试获取字符信息判断字号
                    heading_level = self._detect_heading_level(page, line)
                    if heading_level:
                        line = '#' * heading_level + ' ' + line
                
                processed_lines.append(line)
            
            if processed_lines:
                parts.append('\n'.join(processed_lines))
        
        # 标记图片位置
        if page.images:
            parts.append("\n[图片略]\n")
        
        return '\n\n'.join(parts)
    
    def _detect_heading_level(self, page, text: str) -> int:
        """
        尝试检测文本的标题层级
        基于字号大小判断
        """
        try:
            chars = page.chars
            for char in chars:
                if text[:3] in char.get('text', ''):
                    size = char.get('size', 12)
                    for threshold, level in self.HEADING_THRESHOLDS:
                        if size >= threshold:
                            return level
        except Exception:
            pass
        return 0  # 非标题
    
    def _table_to_markdown(self, table: List[List[str]]) -> str:
        """将表格转换为 Markdown 格式"""
        if not table or not table[0]:
            return ""
        
        lines: List[str] = []
        
        # 处理表头
        header = table[0]
        header_cells = [self._escape_cell(cell) for cell in header]
        lines.append("| " + " | ".join(header_cells) + " |")
        
        # 分隔行
        lines.append("| " + " | ".join(["---"] * len(header)) + " |")
        
        # 数据行
        for row in table[1:]:
            if row:
                cells = [self._escape_cell(cell) for cell in row]
                # 确保列数一致
                while len(cells) < len(header):
                    cells.append("")
                lines.append("| " + " | ".join(cells[:len(header)]) + " |")
        
        return "\n".join(lines)
    
    def _escape_cell(self, cell) -> str:
        """转义单元格内容"""
        if cell is None:
            return ""
        text = str(cell).strip()
        # 替换管道符和换行符
        text = text.replace("|", "\\|")
        text = text.replace("\n", " ")
        return text
