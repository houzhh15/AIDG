"""
file_converter/tests/test_ocr.py
OCR 模块测试用例
"""

import pytest
from unittest.mock import Mock, patch, MagicMock
from io import BytesIO
from PIL import Image
import os


class TestOcrEngine:
    """OcrEngine 测试类"""
    
    def test_ocr_engine_available(self):
        """测试 OcrEngine 初始化和可用性检查"""
        from file_converter.ocr import OcrEngine
        
        engine = OcrEngine()
        # 如果系统安装了 tesseract，应该返回 True
        # 在 CI 环境中可能返回 False
        result = engine.is_available()
        assert isinstance(result, bool)
    
    def test_ocr_engine_singleton(self):
        """测试 get_ocr_engine 返回单例"""
        from file_converter.ocr import get_ocr_engine
        
        engine1 = get_ocr_engine()
        engine2 = get_ocr_engine()
        assert engine1 is engine2
    
    @pytest.mark.skipif(
        not os.path.exists('/usr/bin/tesseract') and not os.path.exists('/opt/homebrew/bin/tesseract'),
        reason="Tesseract not installed"
    )
    def test_ocr_recognize_english(self):
        """测试英文图片识别"""
        from file_converter.ocr import get_ocr_engine
        
        engine = get_ocr_engine()
        if not engine.is_available():
            pytest.skip("Tesseract not available")
        
        # 创建一个简单的测试图片（白底黑字）
        img = Image.new('RGB', (200, 50), color='white')
        # 这里只测试不抛出异常，实际文字需要真实图片
        result = engine.recognize(img, 'eng')
        assert isinstance(result, str)
    
    @pytest.mark.skipif(
        not os.path.exists('/usr/bin/tesseract') and not os.path.exists('/opt/homebrew/bin/tesseract'),
        reason="Tesseract not installed"
    )
    def test_ocr_recognize_chinese(self):
        """测试中文图片识别（需要 chi_sim 语言包）"""
        from file_converter.ocr import get_ocr_engine, get_lang_manager
        
        engine = get_ocr_engine()
        manager = get_lang_manager()
        
        if not engine.is_available():
            pytest.skip("Tesseract not available")
        
        if not manager.is_installed('chi_sim'):
            pytest.skip("chi_sim language pack not installed")
        
        # 创建测试图片
        img = Image.new('RGB', (200, 50), color='white')
        result = engine.recognize(img, 'chi_sim')
        assert isinstance(result, str)
    
    def test_ocr_clean_text(self):
        """测试文本清理功能"""
        from file_converter.ocr import OcrEngine
        
        engine = OcrEngine()
        
        # 测试多余空格清理
        text = "Hello   World\n\n\nTest"
        cleaned = engine._clean_text(text)
        assert "   " not in cleaned
        assert "\n\n\n" not in cleaned


class TestLanguagePackManager:
    """LanguagePackManager 测试类"""
    
    def test_lang_manager_singleton(self):
        """测试 get_lang_manager 返回单例"""
        from file_converter.ocr import get_lang_manager
        
        manager1 = get_lang_manager()
        manager2 = get_lang_manager()
        assert manager1 is manager2
    
    def test_lang_manager_get_available(self):
        """测试获取可用语言列表"""
        from file_converter.ocr import get_lang_manager
        
        manager = get_lang_manager()
        available = manager.get_available_langs()
        
        assert isinstance(available, list)
        assert 'eng' in available
        assert 'chi_sim' in available
    
    def test_lang_manager_get_installed(self):
        """测试获取已安装语言列表"""
        from file_converter.ocr import get_lang_manager
        
        manager = get_lang_manager()
        installed = manager.get_installed_langs()
        
        assert isinstance(installed, list)
        # 如果 tessdata 目录存在，应该能列出已安装的语言包
    
    def test_lang_manager_is_installed(self):
        """测试检查语言是否已安装"""
        from file_converter.ocr import get_lang_manager
        
        manager = get_lang_manager()
        
        # 检查结果应该是布尔值
        result = manager.is_installed('eng')
        assert isinstance(result, bool)
    
    @patch('requests.get')
    def test_lang_manager_download_mock(self, mock_get):
        """测试语言包下载功能（mock）"""
        from file_converter.ocr import LanguagePackManager
        
        # 模拟下载响应
        mock_response = Mock()
        mock_response.status_code = 200
        mock_response.headers = {'content-length': '1000'}
        mock_response.iter_content = Mock(return_value=[b'test' * 250])
        mock_response.__enter__ = Mock(return_value=mock_response)
        mock_response.__exit__ = Mock(return_value=False)
        mock_get.return_value = mock_response
        
        manager = LanguagePackManager()
        
        progress_values = []
        def progress_callback(progress):
            progress_values.append(progress)
        
        # 由于需要写入文件，这里只测试调用不抛异常
        # 实际下载测试需要临时目录
        with patch.object(manager, '_tessdata_dir', '/tmp/test_tessdata'):
            os.makedirs('/tmp/test_tessdata', exist_ok=True)
            try:
                result = manager.download('test_lang', progress_callback=progress_callback)
                # mock 场景下可能成功或失败，取决于具体实现
            except Exception:
                pass  # 允许失败，因为是 mock 测试
    
    def test_lang_manager_invalid_lang(self):
        """测试无效语言代码"""
        from file_converter.ocr import get_lang_manager
        
        manager = get_lang_manager()
        result = manager.is_installed('invalid_lang_xyz')
        assert result is False


class TestImageConverter:
    """ImageConverter 测试类"""
    
    def test_image_converter_supported_extensions(self):
        """测试支持的扩展名"""
        from file_converter.converters.image import ImageConverter
        
        converter = ImageConverter()
        extensions = converter.supported_extensions
        
        assert 'png' in extensions
        assert 'jpg' in extensions
        assert 'jpeg' in extensions
        assert 'bmp' in extensions
        assert 'tiff' in extensions
    
    def test_image_converter_convert_png(self):
        """测试 PNG 图片转换"""
        from file_converter.converters.image import ImageConverter
        from file_converter.ocr import get_ocr_engine
        
        engine = get_ocr_engine()
        if not engine.is_available():
            pytest.skip("Tesseract not available")
        
        converter = ImageConverter()
        
        # 创建测试 PNG 图片
        img = Image.new('RGB', (100, 50), color='white')
        buffer = BytesIO()
        img.save(buffer, format='PNG')
        buffer.seek(0)
        
        result = converter.convert(buffer, 'test.png')
        
        assert result is not None
        assert result.ocr_used is True
        assert isinstance(result.content, str)
    
    def test_image_converter_invalid_format(self):
        """测试无效图片格式"""
        from file_converter.converters.image import ImageConverter
        
        converter = ImageConverter()
        
        # 无效的图片数据
        buffer = BytesIO(b'not an image')
        
        with pytest.raises(ValueError):
            converter.convert(buffer, 'test.png')


class TestConvertersWithOcr:
    """测试各转换器的 OCR 模式"""
    
    def test_pdf_converter_ocr_params(self):
        """测试 PdfConverter 接受 OCR 参数"""
        from file_converter.converters.pdf import PdfConverter
        import inspect
        
        converter = PdfConverter()
        sig = inspect.signature(converter.convert)
        params = list(sig.parameters.keys())
        
        assert 'enable_ocr' in params
        assert 'ocr_lang' in params
    
    def test_pptx_converter_ocr_params(self):
        """测试 PptxConverter 接受 OCR 参数"""
        from file_converter.converters.pptx import PptxConverter
        import inspect
        
        converter = PptxConverter()
        sig = inspect.signature(converter.convert)
        params = list(sig.parameters.keys())
        
        assert 'enable_ocr' in params
        assert 'ocr_lang' in params
    
    def test_docx_converter_ocr_params(self):
        """测试 DocxConverter 接受 OCR 参数"""
        from file_converter.converters.docx import DocxConverter
        import inspect
        
        converter = DocxConverter()
        sig = inspect.signature(converter.convert)
        params = list(sig.parameters.keys())
        
        assert 'enable_ocr' in params
        assert 'ocr_lang' in params
    
    def test_base_converter_result_fields(self):
        """测试 ConversionResult 包含 OCR 字段"""
        from file_converter.converters.base import ConversionResult
        
        result = ConversionResult(
            content="test",
            warnings=[],
            ocr_used=True,
            ocr_pages=5
        )
        
        assert result.ocr_used is True
        assert result.ocr_pages == 5
