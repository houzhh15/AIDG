# Whisper Models Configuration

这个文件包含所有可用的Whisper模型的静态配置信息。

## 文件位置

- 开发环境: `./config/whisper_models.json`
- 生产环境: 可以放在共享卷中，通过环境变量 `CONFIG_PATH` 指定配置目录

## 文件格式

```json
{
  "models": [
    {
      "id": "模型ID（如 ggml-large-v3）",
      "path": "模型文件名（如 ggml-large-v3.bin）",
      "size": 文件大小（字节）,
      "size_mb": "显示用的大小（如 2.88 GB）",
      "description": "模型描述"
    }
  ]
}
```

## 可用模型列表

从 https://huggingface.co/ggerganov/whisper.cpp 获取的模型信息：

- **ggml-tiny**: 74.1 MB - 最小的模型，速度最快但准确度较低
- **ggml-tiny.en**: 74.1 MB - 英文专用tiny模型
- **ggml-base**: 141.1 MB - 基础模型
- **ggml-base.en**: 141.1 MB - 英文专用基础模型
- **ggml-small**: 465.1 MB - 小型模型
- **ggml-small.en**: 465.1 MB - 英文专用小型模型
- **ggml-medium**: 1.43 GB - 中等大小模型
- **ggml-medium.en**: 1.43 GB - 英文专用中等模型
- **ggml-large-v1**: 2.88 GB - 大模型 v1
- **ggml-large-v2**: 2.88 GB - 大模型 v2
- **ggml-large-v3**: 2.88 GB - 大模型 v3（推荐）
- **ggml-large-v3-turbo**: 1.51 GB - 大模型 v3 Turbo版本（速度更快）

## 如何更新

1. 直接编辑此JSON文件
2. 重启服务器以加载新的配置
3. 或者在生产环境中，将文件放在共享卷，通过环境变量 `CONFIG_PATH` 指定目录

## 注意事项

- 模型ID必须与HuggingFace仓库中的文件名（去掉.bin后缀）一致
- size字段应该是准确的字节数
- size_mb字段用于前端显示，可以使用 MB 或 GB 作为单位
- 模型下载后会存储在 `./models/whisper/` 目录（通过docker-compose配置的共享卷）
