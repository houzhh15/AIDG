"""
file_converter API 端点集成测试
测试 FastAPI 应用的 HTTP 接口
"""
import io
import pytest
import pytest_asyncio
from unittest.mock import patch, MagicMock
import httpx
from file_converter.main import app
from file_converter.converters.base import ConversionResult

# 配置 pytest-asyncio
pytestmark = pytest.mark.asyncio(loop_scope="function")


@pytest_asyncio.fixture
async def client():
    """创建异步测试客户端"""
    transport = httpx.ASGITransport(app=app)
    async with httpx.AsyncClient(transport=transport, base_url="http://test") as c:
        yield c


class TestHealthEndpoint:
    """测试健康检查端点"""
    
    async def test_health_check(self, client):
        """测试健康检查返回 200"""
        response = await client.get("/health")
        assert response.status_code == 200
        data = response.json()
        assert data["status"] == "healthy"
        assert "converters" in data


class TestConvertEndpoint:
    """测试通用转换端点"""
    
    @patch('file_converter.main.ConverterFactory.get_converter')
    async def test_convert_pdf_success(self, mock_get_converter, client):
        """测试成功转换 PDF"""
        mock_converter = MagicMock()
        mock_converter.convert.return_value = ConversionResult(
            content="# Converted PDF Content\n\nThis is the extracted text.",
            warnings=[]
        )
        mock_get_converter.return_value = mock_converter
        
        # 创建模拟 PDF 文件
        file_content = b"%PDF-1.4 fake content"
        files = {"file": ("test.pdf", io.BytesIO(file_content), "application/pdf")}
        
        response = await client.post("/convert", files=files)
        
        assert response.status_code == 200
        data = response.json()
        assert data["success"] is True
        assert "Converted PDF Content" in data["content"]
        assert data["original_filename"] == "test.pdf"
    
    @patch('file_converter.main.ConverterFactory.get_converter')
    async def test_convert_pptx_success(self, mock_get_converter, client):
        """测试成功转换 PPTX"""
        mock_converter = MagicMock()
        mock_converter.convert.return_value = ConversionResult(
            content="## Slide 1: Introduction\n\nSlide content here",
            warnings=[]
        )
        mock_get_converter.return_value = mock_converter
        
        file_content = b"PK fake pptx content"
        files = {"file": ("presentation.pptx", io.BytesIO(file_content), "application/vnd.openxmlformats-officedocument.presentationml.presentation")}
        
        response = await client.post("/convert", files=files)
        
        assert response.status_code == 200
        data = response.json()
        assert data["success"] is True
        assert "Slide 1" in data["content"]
    
    async def test_convert_unsupported_format(self, client):
        """测试不支持的文件格式"""
        file_content = b"plain text content"
        files = {"file": ("test.txt", io.BytesIO(file_content), "text/plain")}
        
        response = await client.post("/convert", files=files)
        
        assert response.status_code == 400
        data = response.json()
        assert "不支持" in data["detail"] or "unsupported" in data["detail"].lower()
    
    @patch('file_converter.main.ConverterFactory.get_converter')
    async def test_convert_with_warnings(self, mock_get_converter, client):
        """测试转换时产生警告"""
        mock_converter = MagicMock()
        mock_converter.convert.return_value = ConversionResult(
            content="Partial content",
            warnings=["Some images could not be extracted"]
        )
        mock_get_converter.return_value = mock_converter
        
        file_content = b"fake pdf content"
        files = {"file": ("test.pdf", io.BytesIO(file_content), "application/pdf")}
        
        response = await client.post("/convert", files=files)
        
        assert response.status_code == 200
        data = response.json()
        assert data["success"] is True
        assert len(data["warnings"]) > 0
    
    @patch('file_converter.main.ConverterFactory.get_converter')
    async def test_convert_error(self, mock_get_converter, client):
        """测试转换过程中出错"""
        mock_converter = MagicMock()
        mock_converter.convert.side_effect = Exception("Conversion failed")
        mock_get_converter.return_value = mock_converter
        
        file_content = b"corrupted content"
        files = {"file": ("test.pdf", io.BytesIO(file_content), "application/pdf")}
        
        response = await client.post("/convert", files=files)
        
        assert response.status_code == 500


class TestSpecificConvertEndpoints:
    """测试特定格式转换端点"""
    
    @patch('file_converter.converters.pdf.PdfConverter.convert')
    async def test_convert_pdf_endpoint(self, mock_convert, client):
        """测试 /convert/pdf 端点"""
        mock_convert.return_value = ConversionResult(
            content="PDF content",
            warnings=[]
        )
        
        file_content = b"fake pdf"
        files = {"file": ("test.pdf", io.BytesIO(file_content), "application/pdf")}
        
        response = await client.post("/convert/pdf", files=files)
        
        assert response.status_code == 200
        assert response.json()["success"] is True
    
    @patch('file_converter.converters.pptx.PptxConverter.convert')
    async def test_convert_pptx_endpoint(self, mock_convert, client):
        """测试 /convert/pptx 端点"""
        mock_convert.return_value = ConversionResult(
            content="PPTX content",
            warnings=[]
        )
        
        file_content = b"fake pptx"
        files = {"file": ("test.pptx", io.BytesIO(file_content), "application/vnd.openxmlformats-officedocument.presentationml.presentation")}
        
        response = await client.post("/convert/pptx", files=files)
        
        assert response.status_code == 200
        assert response.json()["success"] is True
    
    @patch('file_converter.converters.docx.DocxConverter.convert')
    async def test_convert_docx_endpoint(self, mock_convert, client):
        """测试 /convert/docx 端点"""
        mock_convert.return_value = ConversionResult(
            content="DOCX content",
            warnings=[]
        )
        
        file_content = b"fake docx"
        files = {"file": ("test.docx", io.BytesIO(file_content), "application/vnd.openxmlformats-officedocument.wordprocessingml.document")}
        
        response = await client.post("/convert/docx", files=files)
        
        assert response.status_code == 200
        assert response.json()["success"] is True
    
    @patch('file_converter.converters.xlsx.XlsxConverter.convert')
    async def test_convert_xlsx_endpoint(self, mock_convert, client):
        """测试 /convert/xlsx 端点"""
        mock_convert.return_value = ConversionResult(
            content="XLSX content",
            warnings=[]
        )
        
        file_content = b"fake xlsx"
        files = {"file": ("test.xlsx", io.BytesIO(file_content), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")}
        
        response = await client.post("/convert/xlsx", files=files)
        
        assert response.status_code == 200
        assert response.json()["success"] is True


class TestFileSizeValidation:
    """测试文件大小校验"""
    
    async def test_empty_file(self, client):
        """测试空文件"""
        files = {"file": ("empty.pdf", io.BytesIO(b""), "application/pdf")}
        
        response = await client.post("/convert", files=files)
        
        # 应该返回错误或成功处理空文件
        assert response.status_code in [200, 400]


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
