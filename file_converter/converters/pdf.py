"""
file_converter/converters/pdf.py
PDF 文件转换器 - 使用 pdfplumber 提取文本和表格，支持扫描件 OCR
"""

import io
import logging
from typing import BinaryIO, List, Tuple, Optional

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
    - 支持扫描件 OCR 识别
    """
    
    supported_extensions = ["pdf"]
    
    # 字号阈值用于判断标题层级
    HEADING_THRESHOLDS = [
        (24, 1),  # 字号 >= 24 为 H1
        (18, 2),  # 字号 >= 18 为 H2
        (14, 3),  # 字号 >= 14 为 H3
    ]
    
    # 文本字符数阈值，低于此值认为是扫描页
    MIN_TEXT_THRESHOLD = 50
    
    def convert(
        self, 
        file: BinaryIO, 
        filename: str,
        enable_ocr: bool = False,
        ocr_lang: Optional[str] = None
    ) -> ConversionResult:
        """将 PDF 转换为 Markdown，支持扫描件 OCR"""
        warnings: List[str] = []
        content_parts: List[str] = []
        ocr_pages = 0
        
        try:
            # 使用 pdfplumber 打开 PDF
            with pdfplumber.open(file) as pdf:
                total_pages = len(pdf.pages)
                logger.info(f"Processing PDF: {filename}, {total_pages} pages, enable_ocr={enable_ocr}")
                
                for page_num, page in enumerate(pdf.pages, 1):
                    page_content = self._process_page(page, page_num, warnings)
                    
                    # 检查是否需要 OCR
                    if enable_ocr and len(page_content.strip()) < self.MIN_TEXT_THRESHOLD:
                        # 疑似扫描页，使用 OCR
                        ocr_text = self._ocr_page(page, page_num, warnings, ocr_lang)
                        if ocr_text:
                            page_content = ocr_text
                            ocr_pages += 1
                    
                    if page_content.strip():
                        content_parts.append(page_content)
                
                # 检查是否有图片（仅在非 OCR 模式下提示）
                if not enable_ocr:
                    for page in pdf.pages:
                        if page.images:
                            warnings.append(f"PDF 包含图片，如需提取图片中的文字请启用 OCR")
                            break
        
        except Exception as e:
            logger.error(f"PDF conversion failed: {filename}, error: {e}")
            raise ValueError(f"PDF 转换失败: {str(e)}")
        
        content = "\n\n".join(content_parts)
        
        if not content.strip():
            warnings.append("PDF 未提取到任何文本内容")
        
        if ocr_pages > 0:
            warnings.append(f"使用 OCR 识别了 {ocr_pages} 页")
        
        return ConversionResult(
            content=content, 
            warnings=warnings,
            ocr_used=ocr_pages > 0,
            ocr_pages=ocr_pages
        )
    
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
    
    def _ocr_page(
        self, 
        page, 
        page_num: int, 
        warnings: List[str],
        ocr_lang: Optional[str] = None
    ) -> str:
        """对 PDF 页面进行 OCR"""
        try:
            from file_converter.ocr.engine import get_ocr_engine
            
            # 将页面转为图片 (200 DPI)
            pil_image = page.to_image(resolution=200).original
            
            # OCR 识别
            ocr_engine = get_ocr_engine()
            if not ocr_engine.is_available():
                warnings.append(f"第 {page_num} 页：OCR 引擎不可用")
                return ""
            
            text = ocr_engine.recognize(pil_image, lang=ocr_lang)
            
            logger.info(f"OCR page {page_num}: extracted {len(text)} chars")
            return text
        
        except Exception as e:
            warnings.append(f"第 {page_num} 页 OCR 失败: {str(e)}")
            return ""
