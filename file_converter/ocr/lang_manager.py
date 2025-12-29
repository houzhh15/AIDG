"""
file_converter/ocr/lang_manager.py
OCR 语言包动态下载管理器
"""

import os
import logging
import requests
from pathlib import Path
from typing import List, Optional, Callable

logger = logging.getLogger(__name__)


class LanguagePackManager:
    """
    Tesseract 语言包管理器
    
    功能：
    - 检查语言包是否已安装
    - 按需下载语言包
    - 支持用户自定义下载目录
    """
    
    # 官方下载源
    TESSDATA_REPO = "https://github.com/tesseract-ocr/tessdata/raw/main"
    
    # 国内镜像（备用）
    TESSDATA_MIRROR = "https://ghproxy.com/https://github.com/tesseract-ocr/tessdata/raw/main"
    
    # 预定义的常用语言包信息
    AVAILABLE_LANGS = {
        "eng": {"name": "English", "size_mb": 4},
        "chi_sim": {"name": "简体中文", "size_mb": 50},
        "chi_tra": {"name": "繁體中文", "size_mb": 50},
        "jpn": {"name": "日本語", "size_mb": 50},
        "kor": {"name": "한국어", "size_mb": 40},
        "fra": {"name": "Français", "size_mb": 4},
        "deu": {"name": "Deutsch", "size_mb": 4},
        "spa": {"name": "Español", "size_mb": 4},
        "rus": {"name": "Русский", "size_mb": 10},
    }
    
    def __init__(self, tessdata_dir: Optional[str] = None):
        """
        初始化语言包管理器
        
        Args:
            tessdata_dir: 语言包存储目录，默认使用环境变量或系统目录
        """
        # 优先使用参数指定的目录
        if tessdata_dir:
            self.tessdata_dir = Path(tessdata_dir)
        # 其次使用环境变量
        elif os.environ.get("TESSDATA_PREFIX"):
            self.tessdata_dir = Path(os.environ["TESSDATA_PREFIX"])
        # macOS Homebrew 安装位置
        elif Path("/opt/homebrew/share/tessdata").exists():
            self.tessdata_dir = Path("/opt/homebrew/share/tessdata")
        elif Path("/usr/local/share/tessdata").exists():
            self.tessdata_dir = Path("/usr/local/share/tessdata")
        # Linux 系统位置
        elif Path("/usr/share/tessdata").exists():
            self.tessdata_dir = Path("/usr/share/tessdata")
        # Docker/生产环境
        else:
            self.tessdata_dir = Path("/app/tessdata")
        
        # 尝试创建目录（如果有权限）
        try:
            self.tessdata_dir.mkdir(parents=True, exist_ok=True)
        except (OSError, PermissionError):
            # 如果没有写权限，尝试使用当前用户目录
            home_tessdata = Path.home() / ".tessdata"
            home_tessdata.mkdir(parents=True, exist_ok=True)
            self.tessdata_dir = home_tessdata
        
        # 设置 Tesseract 环境变量
        os.environ["TESSDATA_PREFIX"] = str(self.tessdata_dir)
        
        # 是否使用国内镜像
        self.use_mirror = os.environ.get("USE_TESSDATA_MIRROR", "false").lower() == "true"
    
    def get_installed_langs(self) -> List[str]:
        """获取已安装的语言包列表"""
        installed = []
        for file in self.tessdata_dir.glob("*.traineddata"):
            lang = file.stem
            installed.append(lang)
        return installed
    
    def is_installed(self, lang: str) -> bool:
        """检查语言包是否已安装"""
        return (self.tessdata_dir / f"{lang}.traineddata").exists()
    
    def download(
        self, 
        lang: str, 
        progress_callback: Optional[Callable[[int, int], None]] = None
    ) -> bool:
        """
        下载指定语言包
        
        Args:
            lang: 语言代码（如 chi_sim, eng）
            progress_callback: 下载进度回调 (downloaded_bytes, total_bytes)
        
        Returns:
            下载是否成功
        """
        if self.is_installed(lang):
            logger.info(f"Language pack '{lang}' already installed")
            return True
        
        # 选择下载源
        base_url = self.TESSDATA_MIRROR if self.use_mirror else self.TESSDATA_REPO
        url = f"{base_url}/{lang}.traineddata"
        
        target_path = self.tessdata_dir / f"{lang}.traineddata"
        temp_path = self.tessdata_dir / f"{lang}.traineddata.tmp"
        
        try:
            logger.info(f"Downloading language pack: {lang} from {url}")
            
            response = requests.get(url, stream=True, timeout=300)
            response.raise_for_status()
            
            total_size = int(response.headers.get('content-length', 0))
            downloaded = 0
            
            with open(temp_path, 'wb') as f:
                for chunk in response.iter_content(chunk_size=8192):
                    f.write(chunk)
                    downloaded += len(chunk)
                    if progress_callback:
                        progress_callback(downloaded, total_size)
            
            # 下载完成，重命名
            temp_path.rename(target_path)
            logger.info(f"Language pack '{lang}' downloaded successfully")
            return True
        
        except Exception as e:
            logger.error(f"Failed to download language pack '{lang}': {e}")
            if temp_path.exists():
                temp_path.unlink()
            return False
    
    def ensure_langs(self, langs: List[str]) -> List[str]:
        """
        确保指定的语言包都已安装，返回缺失的语言列表
        
        Args:
            langs: 需要的语言代码列表
        
        Returns:
            下载失败的语言列表（空列表表示全部成功）
        """
        failed = []
        for lang in langs:
            if not self.download(lang):
                failed.append(lang)
        return failed
    
    def get_available_langs(self) -> dict:
        """获取可下载的语言包信息"""
        result = {}
        for lang, info in self.AVAILABLE_LANGS.items():
            result[lang] = {
                **info,
                "installed": self.is_installed(lang)
            }
        return result


# 全局单例
_lang_manager: Optional[LanguagePackManager] = None


def get_lang_manager() -> LanguagePackManager:
    """获取语言包管理器单例"""
    global _lang_manager
    if _lang_manager is None:
        _lang_manager = LanguagePackManager()
    return _lang_manager
