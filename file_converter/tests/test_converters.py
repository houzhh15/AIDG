"""
file_converter 转换器单元测试
测试所有支持的文件格式转换
"""
import io
import pytest
from unittest.mock import MagicMock, patch
from file_converter.converters.base import BaseConverter, ConversionResult
from file_converter.converters.pdf import PdfConverter
from file_converter.converters.pptx import PptxConverter
from file_converter.converters.docx import DocxConverter
from file_converter.converters.xlsx import XlsxConverter


class TestConversionResult:
    """测试 ConversionResult 数据类"""
    
    def test_create_with_content_only(self):
        result = ConversionResult(content="Hello World")
        assert result.content == "Hello World"
        assert result.warnings == []
    
    def test_create_with_warnings(self):
        result = ConversionResult(
            content="Hello",
            warnings=["Warning 1", "Warning 2"]
        )
        assert result.content == "Hello"
        assert len(result.warnings) == 2


class TestPdfConverter:
    """测试 PDF 转换器"""
    
    def test_supported_extensions(self):
        converter = PdfConverter()
        assert 'pdf' in converter.supported_extensions
    
    @patch('file_converter.converters.pdf.pdfplumber')
    def test_convert_pdf_basic(self, mock_pdfplumber):
        """测试基本 PDF 转换"""
        # 模拟 PDF 内容
        mock_page = MagicMock()
        mock_page.extract_text.return_value = "This is test content from PDF."
        mock_page.extract_tables.return_value = []
        mock_page.images = []
        mock_page.chars = [{'size': 12}]
        
        mock_pdf = MagicMock()
        mock_pdf.pages = [mock_page]
        mock_pdf.__enter__ = MagicMock(return_value=mock_pdf)
        mock_pdf.__exit__ = MagicMock(return_value=False)
        mock_pdfplumber.open.return_value = mock_pdf
        
        converter = PdfConverter()
        file_bytes = io.BytesIO(b"fake pdf content")
        result = converter.convert(file_bytes, "test.pdf")
        
        assert isinstance(result, ConversionResult)
        assert "This is test content from PDF." in result.content
    
    @patch('file_converter.converters.pdf.pdfplumber')
    def test_convert_pdf_with_tables(self, mock_pdfplumber):
        """测试包含表格的 PDF 转换"""
        mock_page = MagicMock()
        mock_page.extract_text.return_value = "Content with table:"
        mock_page.extract_tables.return_value = [
            [["Header1", "Header2"], ["Value1", "Value2"]]
        ]
        mock_page.images = []
        mock_page.chars = [{'size': 12}]
        
        mock_pdf = MagicMock()
        mock_pdf.pages = [mock_page]
        mock_pdf.__enter__ = MagicMock(return_value=mock_pdf)
        mock_pdf.__exit__ = MagicMock(return_value=False)
        mock_pdfplumber.open.return_value = mock_pdf
        
        converter = PdfConverter()
        file_bytes = io.BytesIO(b"fake pdf content")
        result = converter.convert(file_bytes, "test.pdf")
        
        assert "Header1" in result.content
        assert "|" in result.content  # Markdown table syntax


class TestPptxConverter:
    """测试 PPTX 转换器"""
    
    def test_supported_extensions(self):
        converter = PptxConverter()
        assert 'pptx' in converter.supported_extensions
        assert 'ppt' in converter.supported_extensions
    
    @patch('file_converter.converters.pptx.Presentation')
    def test_convert_pptx_basic(self, mock_presentation_class):
        """测试基本 PPTX 转换"""
        # 模拟幻灯片内容
        mock_shape = MagicMock()
        mock_shape.has_text_frame = True
        mock_shape.text_frame.text = "Slide content"
        mock_shape.has_table = False
        mock_shape.shape_type = 1  # Not picture
        
        mock_title_shape = MagicMock()
        mock_title_shape.has_text_frame = True
        mock_title_shape.text_frame.text = "Slide Title"
        
        mock_slide = MagicMock()
        mock_slide.shapes.title = mock_title_shape
        mock_slide.shapes.__iter__ = lambda self: iter([mock_shape])
        mock_slide.has_notes_slide = False
        
        mock_prs = MagicMock()
        mock_prs.slides = [mock_slide]
        mock_presentation_class.return_value = mock_prs
        
        converter = PptxConverter()
        file_bytes = io.BytesIO(b"fake pptx content")
        result = converter.convert(file_bytes, "test.pptx")
        
        assert isinstance(result, ConversionResult)
        assert "Slide Title" in result.content or "Slide 1" in result.content


class TestDocxConverter:
    """测试 DOCX 转换器"""
    
    def test_supported_extensions(self):
        converter = DocxConverter()
        assert 'docx' in converter.supported_extensions
        assert 'doc' in converter.supported_extensions
    
    @patch('file_converter.converters.docx.Document')
    def test_convert_docx_basic(self, mock_document_class):
        """测试基本 DOCX 转换"""
        # 模拟段落内容
        mock_para = MagicMock()
        mock_para.style.name = "Normal"
        mock_para.text = "This is document content."
        mock_para.runs = []
        
        mock_doc = MagicMock()
        mock_doc.paragraphs = [mock_para]
        mock_doc.tables = []
        mock_document_class.return_value = mock_doc
        
        converter = DocxConverter()
        file_bytes = io.BytesIO(b"fake docx content")
        result = converter.convert(file_bytes, "test.docx")
        
        assert isinstance(result, ConversionResult)
        assert "This is document content." in result.content
    
    @patch('file_converter.converters.docx.Document')
    def test_convert_docx_with_heading(self, mock_document_class):
        """测试包含标题的 DOCX 转换"""
        mock_para = MagicMock()
        mock_para.style.name = "Heading 1"
        mock_para.text = "Chapter 1"
        mock_para.runs = []
        
        mock_doc = MagicMock()
        mock_doc.paragraphs = [mock_para]
        mock_doc.tables = []
        mock_document_class.return_value = mock_doc
        
        converter = DocxConverter()
        file_bytes = io.BytesIO(b"fake docx content")
        result = converter.convert(file_bytes, "test.docx")
        
        assert "# Chapter 1" in result.content


class TestXlsxConverter:
    """测试 XLSX 转换器"""
    
    def test_supported_extensions(self):
        converter = XlsxConverter()
        assert 'xlsx' in converter.supported_extensions
        assert 'xls' in converter.supported_extensions
    
    @patch('file_converter.converters.xlsx.load_workbook')
    def test_convert_xlsx_basic(self, mock_load_workbook):
        """测试基本 XLSX 转换"""
        # 模拟工作簿
        mock_row1 = [MagicMock(value="Header1"), MagicMock(value="Header2")]
        mock_row2 = [MagicMock(value="Value1"), MagicMock(value="Value2")]
        
        mock_sheet = MagicMock()
        mock_sheet.title = "Sheet1"
        mock_sheet.iter_rows.return_value = [mock_row1, mock_row2]
        mock_sheet.max_row = 2
        mock_sheet.max_column = 2
        
        mock_wb = MagicMock()
        mock_wb.sheetnames = ["Sheet1"]
        mock_wb.__getitem__ = lambda self, key: mock_sheet
        mock_load_workbook.return_value = mock_wb
        
        converter = XlsxConverter()
        file_bytes = io.BytesIO(b"fake xlsx content")
        result = converter.convert(file_bytes, "test.xlsx")
        
        assert isinstance(result, ConversionResult)
        assert "Sheet1" in result.content
        assert "Header1" in result.content
        assert "|" in result.content  # Markdown table syntax


class TestConverterFactory:
    """测试转换器工厂"""
    
    def test_get_converter_pdf(self):
        from file_converter.main import ConverterFactory
        converter = ConverterFactory.get_converter("test.pdf")
        assert isinstance(converter, PdfConverter)
    
    def test_get_converter_pptx(self):
        from file_converter.main import ConverterFactory
        converter = ConverterFactory.get_converter("test.pptx")
        assert isinstance(converter, PptxConverter)
    
    def test_get_converter_docx(self):
        from file_converter.main import ConverterFactory
        converter = ConverterFactory.get_converter("test.docx")
        assert isinstance(converter, DocxConverter)
    
    def test_get_converter_xlsx(self):
        from file_converter.main import ConverterFactory
        converter = ConverterFactory.get_converter("test.xlsx")
        assert isinstance(converter, XlsxConverter)
    
    def test_get_converter_unsupported(self):
        from file_converter.main import ConverterFactory
        converter = ConverterFactory.get_converter("test.txt")
        assert converter is None


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
