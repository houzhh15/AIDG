"""
file_converter/main.py
FastAPI 文件转换服务入口，支持 OCR
"""

import io
from fastapi import FastAPI, UploadFile, File, Form, HTTPException, BackgroundTasks
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
    description="轻量级文件格式转换服务，支持 PDF/PPTX/DOCX/XLSX/图片 转 Markdown，可选 OCR",
    version="1.1.0"
)

# 已加载的转换器列表
LOADED_CONVERTERS: List[str] = []

# 全局下载状态跟踪
download_status: Dict[str, dict] = {}

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
    
    try:
        from file_converter.converters.image import ImageConverter
        converters.append("image")
    except ImportError:
        logger.warning("ImageConverter not available")
    
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
        "version": "1.1.0",
        "endpoints": [
            "GET /health",
            "POST /convert",
            "POST /convert/pdf",
            "POST /convert/pptx",
            "POST /convert/docx",
            "POST /convert/xlsx",
            "POST /convert/image",
            "GET /ocr/languages",
            "POST /ocr/languages/{lang}/download",
            "GET /ocr/languages/{lang}/download-status"
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
    
    try:
        from file_converter.converters.image import ImageConverter
        ConverterFactory.register(["png", "jpg", "jpeg", "bmp", "tiff", "tif"], ImageConverter())
    except ImportError as e:
        logger.warning(f"ImageConverter not available: {e}")


# 初始化工厂
_init_converter_factory()


async def _convert_file(
    file: UploadFile, 
    converter: BaseConverter, 
    enable_ocr: bool = False, 
    ocr_lang: str = "chi_sim+eng"
) -> dict:
    """通用文件转换逻辑"""
    try:
        # 读取文件内容
        content = await file.read()
        file_stream = io.BytesIO(content)
        filename = file.filename or "unknown"
        
        # 执行转换（传入 OCR 参数）
        result = converter.convert(file_stream, filename, enable_ocr=enable_ocr, ocr_lang=ocr_lang)
        
        return {
            "success": True,
            "content": result.content,
            "original_filename": filename,
            "file_size": len(content),
            "warnings": result.warnings,
            "ocr_used": result.ocr_used,
            "ocr_pages": result.ocr_pages,
            "ocr_lang": ocr_lang if result.ocr_used else None
        }
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e))
    except Exception as e:
        logger.error(f"Conversion failed: {e}")
        raise HTTPException(status_code=500, detail=f"转换失败: {str(e)}")


@app.post("/convert")
async def convert_auto(
    file: UploadFile = File(...),
    enable_ocr: bool = Form(default=False),
    ocr_lang: str = Form(default="chi_sim+eng")
):
    """
    通用转换端点
    自动识别文件类型并调用对应转换器
    
    Args:
        file: 上传的文件
        enable_ocr: 是否启用 OCR（对于纯图片文件自动启用）
        ocr_lang: OCR 语言，如 "chi_sim+eng" 表示简体中文和英文
    """
    filename = file.filename or "unknown"
    converter = ConverterFactory.get_converter(filename)
    
    if not converter:
        supported = ConverterFactory.get_supported_extensions()
        raise HTTPException(
            status_code=400,
            detail=f"不支持的文件格式。支持的格式: {', '.join(supported)}"
        )
    
    logger.info(f"Converting file: {filename}, OCR enabled: {enable_ocr}, lang: {ocr_lang}")
    return await _convert_file(file, converter, enable_ocr=enable_ocr, ocr_lang=ocr_lang)


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


@app.post("/convert/image")
async def convert_image(
    file: UploadFile = File(...),
    ocr_lang: str = Form(default="chi_sim+eng")
):
    """
    图片转换端点
    对图片进行 OCR 识别并转换为 Markdown
    """
    try:
        from file_converter.converters.image import ImageConverter
        converter = ImageConverter()
    except ImportError:
        raise HTTPException(status_code=503, detail="Image 转换器不可用")
    
    logger.info(f"Converting Image: {file.filename}, OCR lang: {ocr_lang}")
    return await _convert_file(file, converter, enable_ocr=True, ocr_lang=ocr_lang)


# ============ OCR 语言包管理 API ============

@app.get("/ocr/languages")
async def list_ocr_languages():
    """
    获取 OCR 语言包列表
    返回所有可用语言包及其安装状态
    """
    try:
        from file_converter.ocr import get_lang_manager
        manager = get_lang_manager()
        
        available = manager.get_available_langs()
        installed = manager.get_installed_langs()
        
        languages = []
        for lang in available:
            languages.append({
                "code": lang,
                "installed": lang in installed
            })
        
        return {
            "languages": languages,
            "installed_count": len(installed),
            "available_count": len(available)
        }
    except Exception as e:
        logger.error(f"Failed to list OCR languages: {e}")
        raise HTTPException(status_code=500, detail=f"获取语言包列表失败: {str(e)}")


def _download_language_pack(lang: str):
    """后台下载语言包任务"""
    try:
        from file_converter.ocr import get_lang_manager
        manager = get_lang_manager()
        
        download_status[lang] = {
            "status": "downloading",
            "progress": 0.0,
            "error": None
        }
        
        def progress_callback(downloaded: int, total: int):
            progress = (downloaded / total * 100) if total > 0 else 0
            download_status[lang]["progress"] = progress
        
        success = manager.download(lang, progress_callback=progress_callback)
        
        if success:
            download_status[lang] = {
                "status": "completed",
                "progress": 100.0,
                "error": None
            }
        else:
            download_status[lang] = {
                "status": "failed",
                "progress": 0.0,
                "error": "下载失败"
            }
    except Exception as e:
        logger.error(f"Failed to download language pack {lang}: {e}")
        download_status[lang] = {
            "status": "failed",
            "progress": 0.0,
            "error": str(e)
        }


@app.post("/ocr/languages/{lang}/download")
async def download_ocr_language(lang: str, background_tasks: BackgroundTasks):
    """
    下载指定的 OCR 语言包
    异步后台下载，返回后可通过 download-status 端点查询进度
    """
    try:
        from file_converter.ocr import get_lang_manager
        manager = get_lang_manager()
        
        available = manager.get_available_langs()
        if lang not in available:
            raise HTTPException(
                status_code=400,
                detail=f"不支持的语言: {lang}。支持的语言: {', '.join(available)}"
            )
        
        # 检查是否已安装
        if manager.is_installed(lang):
            return {
                "success": True,
                "message": f"语言包 {lang} 已安装",
                "status": "completed"
            }
        
        # 检查是否正在下载
        if lang in download_status and download_status[lang]["status"] == "downloading":
            return {
                "success": True,
                "message": f"语言包 {lang} 正在下载中",
                "status": "downloading"
            }
        
        # 启动后台下载
        background_tasks.add_task(_download_language_pack, lang)
        
        return {
            "success": True,
            "message": f"已开始下载语言包 {lang}",
            "status": "downloading"
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to start download for {lang}: {e}")
        raise HTTPException(status_code=500, detail=f"启动下载失败: {str(e)}")


@app.get("/ocr/languages/{lang}/download-status")
async def get_download_status(lang: str):
    """
    获取语言包下载进度
    返回当前下载状态和进度百分比
    """
    try:
        from file_converter.ocr import get_lang_manager
        manager = get_lang_manager()
        
        # 检查是否已安装
        if manager.is_installed(lang):
            return {
                "status": "completed",
                "progress": 100.0,
                "error": None
            }
        
        # 检查下载状态
        if lang in download_status:
            return download_status[lang]
        
        return {
            "status": "not_started",
            "progress": 0.0,
            "error": None
        }
    except Exception as e:
        logger.error(f"Failed to get download status for {lang}: {e}")
        raise HTTPException(status_code=500, detail=f"获取下载状态失败: {str(e)}")
