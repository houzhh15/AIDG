/**
 * FileUploadArea.tsx
 * 文件上传区域组件 - 支持拖拽上传、文件格式转换和 OCR 识别
 */

import React, { useState, useEffect, useCallback } from 'react';
import { Upload, message, Progress, Typography, Space, Alert, Checkbox, Select, Tooltip, Button } from 'antd';
import { InboxOutlined, FileOutlined, CheckCircleOutlined, DownloadOutlined, CheckOutlined, LoadingOutlined } from '@ant-design/icons';
import type { UploadProps } from 'antd';
import { documentsAPI, ImportMeta, ImportFileResponse } from '../../api/documents';

const { Dragger } = Upload;
const { Text } = Typography;
const { Option } = Select;

// 支持的文件类型
const ACCEPTED_TYPES = '.pdf,.ppt,.pptx,.doc,.docx,.xls,.xlsx,.svg,.png,.jpg,.jpeg,.bmp,.tiff,.tif';
const MAX_FILE_SIZE = 20 * 1024 * 1024; // 20MB

// OCR 语言名称映射
const OCR_LANG_NAMES: Record<string, string> = {
  'eng': 'English',
  'chi_sim': '简体中文',
  'chi_tra': '繁体中文',
  'jpn': '日本語',
  'kor': '한국어',
  'fra': 'Français',
  'deu': 'Deutsch',
  'spa': 'Español',
  'rus': 'Русский',
};

interface OcrLanguage {
  code: string;
  installed: boolean;
}

interface FileUploadAreaProps {
  projectId: string;
  onImportComplete: (content: string, importMeta: ImportMeta) => void;
  onError?: (error: string) => void;
  enableOcr?: boolean;
  selectedOcrLang?: string;
  onEnableOcrChange?: (enabled: boolean) => void;
  onOcrLangChange?: (lang: string) => void;
}

type UploadStatus = 'idle' | 'uploading' | 'converting' | 'success' | 'error';

const FileUploadArea: React.FC<FileUploadAreaProps> = ({
  projectId,
  onImportComplete,
  onError,
  enableOcr = false,
  selectedOcrLang = 'chi_sim+eng',
  onEnableOcrChange,
  onOcrLangChange,
}) => {
  const [status, setStatus] = useState<UploadStatus>('idle');
  const [progress, setProgress] = useState<number>(0);
  const [fileName, setFileName] = useState<string>('');
  const [warnings, setWarnings] = useState<string[]>([]);
  
  // OCR 相关状态
  const [ocrLanguages, setOcrLanguages] = useState<OcrLanguage[]>([]);
  const [downloadingLangs, setDownloadingLangs] = useState<Set<string>>(new Set());
  const [downloadProgress, setDownloadProgress] = useState<Map<string, number>>(new Map());

  // 获取 OCR 语言包列表
  const fetchOcrLanguages = useCallback(async () => {
    try {
      const response = await fetch('/api/ocr/languages');
      if (response.ok) {
        const data = await response.json();
        console.log('OCR languages fetched:', data);
        setOcrLanguages(data.languages || []);
      } else {
        console.error('Failed to fetch OCR languages, status:', response.status);
      }
    } catch (error) {
      console.error('Failed to fetch OCR languages:', error);
    }
  }, []);

  // 当启用 OCR 时获取语言包列表
  useEffect(() => {
    if (enableOcr) {
      fetchOcrLanguages();
    }
  }, [enableOcr, fetchOcrLanguages]);

  // 下载语言包
  const handleDownloadLang = async (langCode: string, e: React.MouseEvent) => {
    e.stopPropagation(); // 阻止事件冒泡到 Select
    
    if (downloadingLangs.has(langCode)) return;
    
    setDownloadingLangs(prev => new Set(prev).add(langCode));
    setDownloadProgress(prev => new Map(prev).set(langCode, 0));
    
    try {
      // 发起下载请求
      const response = await fetch(`/api/ocr/languages/${langCode}/download`, {
        method: 'POST',
      });
      
      if (!response.ok) {
        throw new Error('下载启动失败');
      }
      
      // 轮询下载进度
      const pollProgress = async () => {
        try {
          const statusResponse = await fetch(`/api/ocr/languages/${langCode}/download-status`);
          if (statusResponse.ok) {
            const statusData = await statusResponse.json();
            
            if (statusData.status === 'completed') {
              setDownloadingLangs(prev => {
                const newSet = new Set(prev);
                newSet.delete(langCode);
                return newSet;
              });
              setDownloadProgress(prev => {
                const newMap = new Map(prev);
                newMap.delete(langCode);
                return newMap;
              });
              // 刷新语言包列表
              fetchOcrLanguages();
              message.success(`语言包 ${OCR_LANG_NAMES[langCode] || langCode} 下载完成`);
              return;
            } else if (statusData.status === 'failed') {
              throw new Error(statusData.error || '下载失败');
            } else {
              setDownloadProgress(prev => new Map(prev).set(langCode, statusData.progress || 0));
              // 继续轮询
              setTimeout(pollProgress, 1000);
            }
          }
        } catch (error) {
          console.error('Download progress check failed:', error);
          setDownloadingLangs(prev => {
            const newSet = new Set(prev);
            newSet.delete(langCode);
            return newSet;
          });
          message.error(`下载 ${OCR_LANG_NAMES[langCode] || langCode} 失败`);
        }
      };
      
      // 开始轮询
      setTimeout(pollProgress, 1000);
    } catch (error) {
      console.error('Failed to download language pack:', error);
      setDownloadingLangs(prev => {
        const newSet = new Set(prev);
        newSet.delete(langCode);
        return newSet;
      });
      message.error('下载语言包失败');
    }
  };

  const handleUpload = async (file: File): Promise<boolean> => {
    // 验证文件大小
    if (file.size > MAX_FILE_SIZE) {
      const errorMsg = `文件大小超过限制（最大 20MB）`;
      message.error(errorMsg);
      onError?.(errorMsg);
      return false;
    }

    setFileName(file.name);
    setStatus('uploading');
    setProgress(0);
    setWarnings([]);

    try {
      // 模拟上传进度
      const progressInterval = setInterval(() => {
        setProgress((prev) => {
          if (prev >= 30) {
            clearInterval(progressInterval);
            return 30;
          }
          return prev + 10;
        });
      }, 100);

      setStatus('converting');
      setProgress(50);

      // 调用 API 上传并转换（传入 OCR 参数）
      const response: ImportFileResponse = await documentsAPI.importFile(projectId, file, {
        enableOcr: enableOcr,
        ocrLang: selectedOcrLang,
      });

      clearInterval(progressInterval);
      setProgress(100);
      setStatus('success');

      if (response.warnings && response.warnings.length > 0) {
        setWarnings(response.warnings);
      }

      // 构建导入元数据
      const importMeta: ImportMeta = {
        source_type: 'file_import',
        original_filename: response.original_filename,
        file_size: response.file_size,
        content_type: response.content_type,
      };

      // 回调通知父组件
      onImportComplete(response.content, importMeta);
      message.success('文件导入成功');
    } catch (error: any) {
      setStatus('error');
      const errorMsg = error?.response?.data?.error || error?.message || '文件导入失败';
      message.error(errorMsg);
      onError?.(errorMsg);
    }

    return false; // 阻止 antd 默认上传行为
  };

  const uploadProps: UploadProps = {
    name: 'file',
    multiple: false,
    maxCount: 1,
    accept: ACCEPTED_TYPES,
    showUploadList: false,
    beforeUpload: handleUpload,
  };

  const renderContent = () => {
    switch (status) {
      case 'uploading':
        return (
          <Space direction="vertical" align="center">
            <Progress type="circle" percent={progress} size={60} />
            <Text>正在上传 {fileName}...</Text>
          </Space>
        );
      case 'converting':
        return (
          <Space direction="vertical" align="center">
            <Progress type="circle" percent={progress} size={60} status="active" />
            <Text>正在转换文件格式...</Text>
          </Space>
        );
      case 'success':
        return (
          <Space direction="vertical" align="center">
            <CheckCircleOutlined style={{ fontSize: 48, color: '#52c41a' }} />
            <Text strong>{fileName}</Text>
            <Text type="secondary">导入成功，可以预览内容</Text>
            {warnings.length > 0 && (
              <Alert
                type="warning"
                message="转换警告"
                description={warnings.join('; ')}
                showIcon
                style={{ marginTop: 8, textAlign: 'left' }}
              />
            )}
          </Space>
        );
      case 'error':
        return (
          <Space direction="vertical" align="center">
            <FileOutlined style={{ fontSize: 48, color: '#ff4d4f' }} />
            <Text type="danger">导入失败，请重试</Text>
          </Space>
        );
      default:
        return (
          <>
            <p className="ant-upload-drag-icon">
              <InboxOutlined />
            </p>
            <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
            <p className="ant-upload-hint">
              支持 PDF、PPT、DOC、EXCEL、SVG、图片格式，最大 20MB
            </p>
          </>
        );
    }
  };

  // 渲染 OCR 语言选项
  const renderOcrLangOption = (lang: OcrLanguage) => {
    const isDownloading = downloadingLangs.has(lang.code);
    const progress = downloadProgress.get(lang.code) || 0;
    const langName = OCR_LANG_NAMES[lang.code] || lang.code;

    return (
      <Option 
        key={lang.code} 
        value={lang.code} 
        disabled={!lang.installed && !isDownloading}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <span>{langName} ({lang.code})</span>
          {lang.installed ? (
            <CheckOutlined style={{ color: '#52c41a' }} />
          ) : isDownloading ? (
            <Space size="small">
              <LoadingOutlined />
              <span style={{ fontSize: 12 }}>{Math.round(progress)}%</span>
            </Space>
          ) : (
            <Tooltip title="点击下载语言包">
              <Button 
                type="link" 
                size="small" 
                icon={<DownloadOutlined />}
                onClick={(e) => handleDownloadLang(lang.code, e)}
              />
            </Tooltip>
          )}
        </div>
      </Option>
    );
  };

  return (
    <div>
      {/* OCR 设置区域 */}
      <div style={{ marginBottom: 16 }}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Checkbox 
            checked={enableOcr} 
            onChange={(e) => onEnableOcrChange?.(e.target.checked)}
          >
            启用 OCR 识别（用于扫描件或图片中的文字提取）
          </Checkbox>
          
          {enableOcr && (
            <Space direction="vertical" style={{ width: '100%' }}>
              <Space>
                <Text>识别语言：</Text>
                <Select
                  style={{ width: 360 }}
                  value={selectedOcrLang}
                  onChange={(value) => onOcrLangChange?.(value)}
                  placeholder="选择 OCR 识别语言"
                  dropdownStyle={{ minWidth: 380 }}
                  loading={ocrLanguages.length === 0}
                  notFoundContent={ocrLanguages.length === 0 ? "加载中..." : "无可用语言"}
                >
                  {/* 常用多语言组合 - 推荐用于混合文档 */}
                  <Option key="combo-chi-eng" value="chi_sim+eng">
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%' }}>
                      <span>简体中文 + English (推荐)</span>
                      <span style={{ color: '#1890ff', fontSize: 12 }}>多语言</span>
                    </div>
                  </Option>
                  <Option key="combo-tra-eng" value="chi_tra+eng">
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%' }}>
                      <span>繁体中文 + English</span>
                      <span style={{ color: '#1890ff', fontSize: 12 }}>多语言</span>
                    </div>
                  </Option>
                  
                  {/* 分隔线 */}
                  {ocrLanguages.length > 0 && (
                    <Option key="divider" disabled style={{ borderTop: '1px solid #f0f0f0', marginTop: 4, paddingTop: 4 }}>
                      <Text type="secondary" style={{ fontSize: 12 }}>单语言选项（仅识别该语言）</Text>
                    </Option>
                  )}
                  
                  {/* 动态语言列表 - 来自 API */}
                  {ocrLanguages.map(renderOcrLangOption)}
                </Select>
              </Space>
              
              {/* 下载状态提示 */}
              {downloadingLangs.size > 0 && (
                <div style={{ fontSize: 12, color: '#1890ff' }}>
                  📥 正在下载语言包：{Array.from(downloadingLangs).map(code => 
                    `${OCR_LANG_NAMES[code] || code} (${Math.round(downloadProgress.get(code) || 0)}%)`
                  ).join(', ')}
                </div>
              )}
              
              <Text type="secondary" style={{ fontSize: 12 }}>
                已下载 ✅ 可选 | 未下载 ⬇️ 需先点击下载按钮
              </Text>
            </Space>
          )}
        </Space>
      </div>
      
      {/* 上传区域 */}
      <Dragger
        {...uploadProps}
        style={{
          padding: '20px',
          minHeight: 200,
          background: status === 'success' ? '#f6ffed' : undefined,
        }}
      >
        {renderContent()}
      </Dragger>
    </div>
  );
};

export default FileUploadArea;
