"""
file_converter/converters/image.py
图片文件转换器 - 使用 OCR 识别图片中的文字
"""

import logging
from typing import BinaryIO, List, Optional

from PIL import Image

from file_converter.converters.base import BaseConverter, ConversionResult
from file_converter.ocr.engine import get_ocr_engine

logger = logging.getLogger(__name__)


class ImageConverter(BaseConverter):
    """
    图片转 Markdown 转换器
    
    功能：
    - 支持 PNG, JPG, JPEG, BMP, TIFF 格式
    - 使用 OCR 识别图片中的文字
    - 返回识别的纯文本内容
    """
    
    supported_extensions = ["png", "jpg", "jpeg", "bmp", "tiff", "tif"]
    
    def convert(
        self,
        file: BinaryIO,
        filename: str,
        enable_ocr: bool = True,  # 图片格式必须 OCR
        ocr_lang: Optional[str] = None
    ) -> ConversionResult:
        """将图片转换为文本"""
        warnings: List[str] = []
        
        try:
            # 打开图片
            image = Image.open(file)
            logger.info(f"Processing image: {filename}, size: {image.size}, mode: {image.mode}")
            
            # 检查 OCR 引擎
            ocr_engine = get_ocr_engine()
            if not ocr_engine.is_available():
                warnings.append("OCR 引擎不可用，无法识别图片内容")
                return ConversionResult(
                    content="[图片内容无法识别]",
                    warnings=warnings,
                    ocr_used=False
                )
            
            # 执行 OCR
            text = ocr_engine.recognize(image, lang=ocr_lang)
            
            if not text.strip():
                warnings.append("图片中未识别到文字内容")
                content = "[图片中未识别到文字]"
            else:
                content = text
            
            return ConversionResult(
                content=content, 
                warnings=warnings,
                ocr_used=True,
                ocr_pages=1
            )
        
        except Exception as e:
            logger.error(f"Image conversion failed: {filename}, error: {e}")
            raise ValueError(f"图片处理失败: {str(e)}")
