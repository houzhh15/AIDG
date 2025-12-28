"""
file_converter/converters/base.py
转换器抽象基类定义
"""

from abc import ABC, abstractmethod
from dataclasses import dataclass, field
from typing import List, BinaryIO


@dataclass
class ConversionResult:
    """转换结果数据类"""
    content: str                          # 转换后的 Markdown 内容
    warnings: List[str] = field(default_factory=list)  # 转换警告信息


class BaseConverter(ABC):
    """
    文件转换器抽象基类
    
    所有格式转换器必须继承此类并实现 convert 方法
    """
    
    # 支持的文件扩展名列表
    supported_extensions: List[str] = []
    
    @abstractmethod
    def convert(self, file: BinaryIO, filename: str) -> ConversionResult:
        """
        将文件转换为 Markdown 格式
        
        Args:
            file: 文件二进制流
            filename: 原始文件名（用于日志和警告信息）
            
        Returns:
            ConversionResult: 包含转换后内容和警告信息的结果对象
            
        Raises:
            ValueError: 文件格式不支持或文件损坏
            IOError: 文件读取失败
        """
        pass
    
    def get_extension(self, filename: str) -> str:
        """获取文件扩展名（小写）"""
        if '.' in filename:
            return filename.rsplit('.', 1)[-1].lower()
        return ''
    
    def is_supported(self, filename: str) -> bool:
        """检查文件是否受支持"""
        ext = self.get_extension(filename)
        return ext in self.supported_extensions
