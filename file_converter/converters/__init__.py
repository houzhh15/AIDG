"""
file_converter/converters/__init__.py
转换器模块初始化
"""

from file_converter.converters.base import BaseConverter, ConversionResult

__all__ = ["BaseConverter", "ConversionResult"]
