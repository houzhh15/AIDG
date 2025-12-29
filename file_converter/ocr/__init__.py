"""
file_converter/ocr/__init__.py
OCR 模块初始化文件
提供 Tesseract OCR 封装和语言包管理功能
"""

from file_converter.ocr.engine import OcrEngine, get_ocr_engine
from file_converter.ocr.lang_manager import LanguagePackManager, get_lang_manager

__all__ = [
    "OcrEngine",
    "get_ocr_engine",
    "LanguagePackManager",
    "get_lang_manager",
]
