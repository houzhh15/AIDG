"""
file_converter/ocr/engine.py
OCR 引擎封装模块
提供统一的 OCR 接口，封装 Tesseract 调用
"""

import os
import logging
from typing import Optional, Union
from io import BytesIO

from PIL import Image
import pytesseract

logger = logging.getLogger(__name__)


class OcrEngine:
    """
    Tesseract OCR 引擎封装
    
    使用方式:
        engine = OcrEngine()
        text = engine.recognize(image, lang="chi_sim+eng")
    """
    
    def __init__(self):
        # 从环境变量读取 Tesseract 路径
        tesseract_cmd = os.environ.get("TESSERACT_CMD")
        if tesseract_cmd:
            pytesseract.pytesseract.tesseract_cmd = tesseract_cmd
        
        # 默认语言配置
        self.default_lang = os.environ.get("OCR_LANG", "chi_sim+eng")
        
        # 缓存可用语言列表
        self.available_langs = None
        
        # 验证 Tesseract 可用
        self._verify_tesseract()
    
    def _verify_tesseract(self):
        """验证 Tesseract 是否可用"""
        try:
            version = pytesseract.get_tesseract_version()
            logger.info(f"Tesseract version: {version}")
            # 获取可用语言列表
            self.available_langs = set(pytesseract.get_languages())
            logger.info(f"Available languages: {self.available_langs}")
        except Exception as e:
            logger.warning(f"Tesseract not available: {e}")
    
    def _validate_and_fallback_lang(self, lang: str) -> str:
        """
        验证并降级语言设置
        如果指定的语言不可用，尝试降级到可用的语言
        
        Args:
            lang: 原始语言设置，如 "chi_sim+eng"
        
        Returns:
            验证后的语言设置
        """
        if not self.available_langs:
            logger.warning("Available languages not initialized, using original lang")
            return lang
        
        # 解析多语言组合
        requested_langs = [l.strip() for l in lang.split('+')]
        available = []
        missing = []
        
        for l in requested_langs:
            if l in self.available_langs:
                available.append(l)
            else:
                missing.append(l)
        
        if missing:
            logger.warning(f"Language packs not installed: {missing}")
        
        if not available:
            # 所有语言都不可用，降级到 eng（通常都会安装）
            if 'eng' in self.available_langs:
                logger.warning(f"All requested languages unavailable, fallback to 'eng'")
                return 'eng'
            else:
                # eng 也没有，使用第一个可用语言
                fallback = list(self.available_langs)[0] if self.available_langs else lang
                logger.warning(f"No common language found, fallback to '{fallback}'")
                return fallback
        
        # 返回可用语言组合
        final_lang = '+'.join(available)
        if final_lang != lang:
            logger.info(f"Language adjusted: {lang} -> {final_lang}")
        return final_lang
    
    def recognize(
        self,
        image: Union[Image.Image, bytes, BytesIO],
        lang: Optional[str] = None
    ) -> str:
        """
        识别图片中的文字
        
        Args:
            image: PIL Image 对象, 图片字节数据, 或 BytesIO
            lang: 识别语言, 默认 chi_sim+eng (简体中文+英文)
                  如果指定的语言不可用，自动降级到可用语言
        
        Returns:
            识别出的文本, 识别失败返回空字符串
        """
        try:
            # 统一转换为 PIL Image
            if isinstance(image, bytes):
                image = Image.open(BytesIO(image))
            elif isinstance(image, BytesIO):
                image = Image.open(image)
            
            # 图像预处理 (灰度化提升识别率)
            if image.mode != 'L':
                image = image.convert('L')
            
            # 验证并降级语言设置
            lang = lang or self.default_lang
            validated_lang = self._validate_and_fallback_lang(lang)
            
            # 调用 Tesseract
            text = pytesseract.image_to_string(image, lang=validated_lang)
            
            # 后处理: 清理多余空白
            text = self._clean_text(text)
            
            return text
        
        except Exception as e:
            logger.error(f"OCR recognition failed: {e}")
            return ""
    
    def _clean_text(self, text: str) -> str:
        """清理 OCR 结果中的多余空白和噪声"""
        # 去除多余空行
        lines = [line.strip() for line in text.split('\n')]
        lines = [line for line in lines if line]
        return '\n'.join(lines)
    
    def is_available(self) -> bool:
        """检查 OCR 引擎是否可用"""
        try:
            pytesseract.get_tesseract_version()
            return True
        except Exception:
            return False


# 全局单例
_ocr_engine: Optional[OcrEngine] = None


def get_ocr_engine() -> OcrEngine:
    """获取 OCR 引擎单例"""
    global _ocr_engine
    if _ocr_engine is None:
        _ocr_engine = OcrEngine()
    return _ocr_engine
