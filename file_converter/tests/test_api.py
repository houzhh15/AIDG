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


class TestConvertWithOcr:
    """测试 OCR 相关转换端点"""
    
    @patch('file_converter.main.ConverterFactory.get_converter')
    async def test_convert_pdf_with_ocr(self, mock_get_converter, client):
        """测试 PDF 启用 OCR 转换"""
        mock_converter = MagicMock()
        mock_converter.convert.return_value = ConversionResult(
            content="# OCR Extracted Content\n\nText from scanned PDF.",
            warnings=[],
            ocr_used=True,
            ocr_pages=3
        )
        mock_get_converter.return_value = mock_converter
        
        file_content = b"%PDF-1.4 scanned content"
        files = {"file": ("scanned.pdf", io.BytesIO(file_content), "application/pdf")}
        data = {"enable_ocr": "true", "ocr_lang": "chi_sim+eng"}
        
        response = await client.post("/convert", files=files, data=data)
        
        assert response.status_code == 200
        result = response.json()
        assert result["success"] is True
        assert result["ocr_used"] is True
        assert result["ocr_pages"] == 3
        assert result["ocr_lang"] == "chi_sim+eng"
        
        # 验证转换器被正确调用
        mock_converter.convert.assert_called_once()
        call_kwargs = mock_converter.convert.call_args.kwargs
        assert call_kwargs.get("enable_ocr") is True
        assert call_kwargs.get("ocr_lang") == "chi_sim+eng"
    
    @patch('file_converter.converters.image.ImageConverter.convert')
    async def test_convert_image_png(self, mock_convert, client):
        """测试 PNG 图片转换"""
        mock_convert.return_value = ConversionResult(
            content="Text extracted from image",
            warnings=[],
            ocr_used=True,
            ocr_pages=1
        )
        
        # 创建简单的 PNG 文件头
        from PIL import Image
        img = Image.new('RGB', (100, 50), color='white')
        buffer = io.BytesIO()
        img.save(buffer, format='PNG')
        buffer.seek(0)
        
        files = {"file": ("test.png", buffer, "image/png")}
        data = {"ocr_lang": "eng"}
        
        response = await client.post("/convert/image", files=files, data=data)
        
        assert response.status_code == 200
        result = response.json()
        assert result["success"] is True
        assert result["ocr_used"] is True
    
    @patch('file_converter.main.ConverterFactory.get_converter')
    async def test_convert_without_ocr(self, mock_get_converter, client):
        """测试不启用 OCR 的转换"""
        mock_converter = MagicMock()
        mock_converter.convert.return_value = ConversionResult(
            content="# Normal PDF Content",
            warnings=[],
            ocr_used=False,
            ocr_pages=0
        )
        mock_get_converter.return_value = mock_converter
        
        file_content = b"%PDF-1.4 normal content"
        files = {"file": ("normal.pdf", io.BytesIO(file_content), "application/pdf")}
        
        response = await client.post("/convert", files=files)
        
        assert response.status_code == 200
        result = response.json()
        assert result["success"] is True
        assert result["ocr_used"] is False
        assert result["ocr_lang"] is None


class TestOcrLanguagesApi:
    """测试 OCR 语言包管理 API"""
    
    @patch('file_converter.ocr.get_lang_manager')
    async def test_ocr_languages_list(self, mock_get_manager, client):
        """测试语言包列表接口"""
        mock_manager = MagicMock()
        mock_manager.get_available_langs.return_value = ['eng', 'chi_sim', 'chi_tra', 'jpn']
        mock_manager.get_installed_langs.return_value = ['eng', 'chi_sim']
        mock_get_manager.return_value = mock_manager
        
        response = await client.get("/ocr/languages")
        
        assert response.status_code == 200
        result = response.json()
        assert "languages" in result
        assert result["installed_count"] == 2
        assert result["available_count"] == 4
        
        # 验证语言状态
        langs = {lang["code"]: lang["installed"] for lang in result["languages"]}
        assert langs["eng"] is True
        assert langs["chi_sim"] is True
        assert langs["chi_tra"] is False
        assert langs["jpn"] is False
    
    @patch('file_converter.ocr.get_lang_manager')
    async def test_ocr_languages_download(self, mock_get_manager, client):
        """测试语言包下载接口"""
        mock_manager = MagicMock()
        mock_manager.get_available_langs.return_value = ['eng', 'chi_sim', 'jpn']
        mock_manager.is_installed.return_value = False
        mock_get_manager.return_value = mock_manager
        
        response = await client.post("/ocr/languages/jpn/download")
        
        assert response.status_code == 200
        result = response.json()
        assert result["success"] is True
        assert result["status"] == "downloading"
    
    @patch('file_converter.ocr.get_lang_manager')
    async def test_ocr_languages_download_already_installed(self, mock_get_manager, client):
        """测试下载已安装的语言包"""
        mock_manager = MagicMock()
        mock_manager.get_available_langs.return_value = ['eng', 'chi_sim']
        mock_manager.is_installed.return_value = True
        mock_get_manager.return_value = mock_manager
        
        response = await client.post("/ocr/languages/eng/download")
        
        assert response.status_code == 200
        result = response.json()
        assert result["success"] is True
        assert result["status"] == "completed"
        assert "已安装" in result["message"]
    
    @patch('file_converter.ocr.get_lang_manager')
    async def test_ocr_languages_download_invalid_lang(self, mock_get_manager, client):
        """测试下载无效语言包"""
        mock_manager = MagicMock()
        mock_manager.get_available_langs.return_value = ['eng', 'chi_sim']
        mock_get_manager.return_value = mock_manager
        
        response = await client.post("/ocr/languages/invalid_lang/download")
        
        assert response.status_code == 400
        result = response.json()
        assert "不支持" in result["detail"]
    
    @patch('file_converter.ocr.get_lang_manager')
    async def test_ocr_download_status(self, mock_get_manager, client):
        """测试下载状态查询接口"""
        mock_manager = MagicMock()
        mock_manager.is_installed.return_value = False
        mock_get_manager.return_value = mock_manager
        
        response = await client.get("/ocr/languages/jpn/download-status")
        
        assert response.status_code == 200
        result = response.json()
        assert "status" in result
        assert "progress" in result
    
    @patch('file_converter.ocr.get_lang_manager')
    async def test_ocr_download_status_installed(self, mock_get_manager, client):
        """测试已安装语言的下载状态"""
        mock_manager = MagicMock()
        mock_manager.is_installed.return_value = True
        mock_get_manager.return_value = mock_manager
        
        response = await client.get("/ocr/languages/eng/download-status")
        
        assert response.status_code == 200
        result = response.json()
        assert result["status"] == "completed"
        assert result["progress"] == 100.0


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
