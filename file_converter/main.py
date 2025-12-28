"""
file_converter/main.py
FastAPI 文件转换服务入口
"""

import io
from fastapi import FastAPI, UploadFile, File, HTTPException
from fastapi.responses import JSONResponse
from typing import List, Dict, Optional, Type
import logging

from file_converter.converters.base import BaseConverter, ConversionResult

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = FastAPI(
    title="AIDG File Converter",
    description="轻量级文件格式转换服务，支持 PDF/PPTX/DOCX/XLSX 转 Markdown",
    version="1.0.0"
)

# 已加载的转换器列表
LOADED_CONVERTERS: List[str] = []

def _load_converters() -> List[str]:
    """加载所有可用的转换器"""
    converters = []
    try:
        from file_converter.converters.pdf import PdfConverter
        converters.append("pdf")
    except ImportError:
        logger.warning("PdfConverter not available")
    
    try:
        from file_converter.converters.pptx import PptxConverter
        converters.append("pptx")
    except ImportError:
        logger.warning("PptxConverter not available")
    
    try:
        from file_converter.converters.docx import DocxConverter
        converters.append("docx")
    except ImportError:
        logger.warning("DocxConverter not available")
    
    try:
        from file_converter.converters.xlsx import XlsxConverter
        converters.append("xlsx")
    except ImportError:
        logger.warning("XlsxConverter not available")
    
    return converters


@app.on_event("startup")
async def startup_event():
    """应用启动时加载转换器"""
    global LOADED_CONVERTERS
    LOADED_CONVERTERS = _load_converters()
    logger.info(f"File converter service started. Loaded converters: {LOADED_CONVERTERS}")


@app.get("/health")
async def health_check():
    """
    健康检查端点
    返回服务状态和已加载的转换器列表
    """
    return {
        "status": "healthy",
        "converters": LOADED_CONVERTERS
    }


@app.get("/")
async def root():
    """根路径，返回服务信息"""
    return {
        "service": "AIDG File Converter",
        "version": "1.0.0",
        "endpoints": [
            "GET /health",
            "POST /convert",
            "POST /convert/pdf",
            "POST /convert/pptx",
            "POST /convert/docx",
            "POST /convert/xlsx"
        ]
    }


class ConverterFactory:
    """
    转换器工厂类
    根据文件扩展名自动选择对应的转换器
    """
    
    _converters: Dict[str, BaseConverter] = {}
    
    @classmethod
    def register(cls, extensions: List[str], converter: BaseConverter):
        """注册转换器"""
        for ext in extensions:
            cls._converters[ext.lower()] = converter
    
    @classmethod
    def get_converter(cls, filename: str) -> Optional[BaseConverter]:
        """根据文件名获取对应的转换器"""
        if '.' not in filename:
            return None
        ext = filename.rsplit('.', 1)[-1].lower()
        return cls._converters.get(ext)
    
    @classmethod
    def get_supported_extensions(cls) -> List[str]:
        """获取所有支持的扩展名"""
        return list(cls._converters.keys())


def _init_converter_factory():
    """初始化转换器工厂"""
    try:
        from file_converter.converters.pdf import PdfConverter
        ConverterFactory.register(["pdf"], PdfConverter())
    except ImportError as e:
        logger.warning(f"PdfConverter not available: {e}")
    
    try:
        from file_converter.converters.pptx import PptxConverter
        ConverterFactory.register(["pptx", "ppt"], PptxConverter())
    except ImportError as e:
        logger.warning(f"PptxConverter not available: {e}")
    
    try:
        from file_converter.converters.docx import DocxConverter
        ConverterFactory.register(["docx", "doc"], DocxConverter())
    except ImportError as e:
        logger.warning(f"DocxConverter not available: {e}")
    
    try:
        from file_converter.converters.xlsx import XlsxConverter
        ConverterFactory.register(["xlsx", "xls"], XlsxConverter())
    except ImportError as e:
        logger.warning(f"XlsxConverter not available: {e}")


# 初始化工厂
_init_converter_factory()


async def _convert_file(file: UploadFile, converter: BaseConverter) -> dict:
    """通用文件转换逻辑"""
    try:
        # 读取文件内容
        content = await file.read()
        file_stream = io.BytesIO(content)
        filename = file.filename or "unknown"
        
        # 执行转换
        result = converter.convert(file_stream, filename)
        
        return {
            "success": True,
            "content": result.content,
            "original_filename": filename,
            "file_size": len(content),
            "warnings": result.warnings
        }
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Conversion failed: {e}")
        raise HTTPException(status_code=500, detail=f"转换失败: {str(e)}")


@app.post("/convert")
async def convert_auto(file: UploadFile = File(...)):
    """
    通用转换端点
    自动识别文件类型并调用对应转换器
    """
    filename = file.filename or "unknown"
    converter = ConverterFactory.get_converter(filename)
    
    if not converter:
        supported = ConverterFactory.get_supported_extensions()
        raise HTTPException(
            status_code=400,
            detail=f"不支持的文件格式。支持的格式: {', '.join(supported)}"
        )
    
    logger.info(f"Converting file: {filename}")
    return await _convert_file(file, converter)


@app.post("/convert/pdf")
async def convert_pdf(file: UploadFile = File(...)):
    """PDF 转换端点"""
    try:
        from file_converter.converters.pdf import PdfConverter
        converter = PdfConverter()
    except ImportError:
        raise HTTPException(status_code=503, detail="PDF 转换器不可用")
    
    logger.info(f"Converting PDF: {file.filename}")
    return await _convert_file(file, converter)


@app.post("/convert/pptx")
async def convert_pptx(file: UploadFile = File(...)):
    """PPTX 转换端点"""
    try:
        from file_converter.converters.pptx import PptxConverter
        converter = PptxConverter()
    except ImportError:
        raise HTTPException(status_code=503, detail="PPTX 转换器不可用")
    
    logger.info(f"Converting PPTX: {file.filename}")
    return await _convert_file(file, converter)


@app.post("/convert/docx")
async def convert_docx(file: UploadFile = File(...)):
    """DOCX 转换端点"""
    try:
        from file_converter.converters.docx import DocxConverter
        converter = DocxConverter()
    except ImportError:
        raise HTTPException(status_code=503, detail="DOCX 转换器不可用")
    
    logger.info(f"Converting DOCX: {file.filename}")
    return await _convert_file(file, converter)


@app.post("/convert/xlsx")
async def convert_xlsx(file: UploadFile = File(...)):
    """XLSX 转换端点"""
    try:
        from file_converter.converters.xlsx import XlsxConverter
        converter = XlsxConverter()
    except ImportError:
        raise HTTPException(status_code=503, detail="XLSX 转换器不可用")
    
    logger.info(f"Converting XLSX: {file.filename}")
    return await _convert_file(file, converter)
