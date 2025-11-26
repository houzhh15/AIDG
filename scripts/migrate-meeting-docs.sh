#!/bin/bash
# 会议文档迁移脚本
# 将旧的 .md 文件迁移到新的 chunks.jsonl 格式

MEETINGS_DIR="/Users/tshinjeii/Documents/code/python/AIDG/data/meetings"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 迁移函数
migrate_doc() {
    local meeting_dir="$1"
    local source_file="$2"
    local slot_key="$3"
    
    local meeting_name=$(basename "$meeting_dir")
    local source_path="$meeting_dir/$source_file"
    local target_dir="$meeting_dir/docs/$slot_key"
    
    # 检查源文件是否存在
    if [ ! -f "$source_path" ]; then
        return 1
    fi
    
    # 检查目标是否已存在
    if [ -f "$target_dir/chunks.jsonl" ]; then
        echo -e "${YELLOW}[SKIP]${NC} $meeting_name/$slot_key - 已存在 chunks.jsonl"
        return 0
    fi
    
    # 创建目标目录
    mkdir -p "$target_dir"
    mkdir -p "$target_dir/sections"
    
    # 读取源文件内容
    local content=$(cat "$source_path")
    
    if [ -z "$content" ]; then
        echo -e "${YELLOW}[SKIP]${NC} $meeting_name/$slot_key - 源文件为空"
        return 0
    fi
    
    # 生成时间戳
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local chunk_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
    
    # 创建 chunks.jsonl (JSON Lines 格式)
    # 需要转义内容中的特殊字符
    local escaped_content=$(echo "$content" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()))')
    
    cat > "$target_dir/chunks.jsonl" << EOF
{"id":"$chunk_id","type":"append","content":$escaped_content,"timestamp":"$timestamp","author":"migration"}
EOF

    # 创建 meta.json
    cat > "$target_dir/meta.json" << EOF
{
  "version": 1,
  "lastModified": "$timestamp",
  "chunkCount": 1
}
EOF

    # 创建 compiled.md (与源文件相同)
    cp "$source_path" "$target_dir/compiled.md"
    
    echo -e "${GREEN}[OK]${NC} $meeting_name/$slot_key <- $source_file"
    return 0
}

# 统计
total_meetings=0
migrated_count=0
skipped_count=0

echo "=========================================="
echo "会议文档迁移工具"
echo "=========================================="
echo ""

# 遍历所有会议目录
for meeting_dir in "$MEETINGS_DIR"/*/; do
    # 跳过非目录
    [ -d "$meeting_dir" ] || continue
    
    # 跳过 .svn 和隐藏目录
    meeting_name=$(basename "$meeting_dir")
    [[ "$meeting_name" == .* ]] && continue
    
    ((total_meetings++))
    
    # 迁移 polish_all.md -> docs/polish/
    if migrate_doc "$meeting_dir" "polish_all.md" "polish"; then
        ((migrated_count++))
    fi
    
    # 迁移 meeting_summary.md -> docs/summary/
    if migrate_doc "$meeting_dir" "meeting_summary.md" "summary"; then
        ((migrated_count++))
    fi
    
    # 迁移 topic.md -> docs/topic/
    if migrate_doc "$meeting_dir" "topic.md" "topic"; then
        ((migrated_count++))
    fi
    
    # 迁移 feature_list.md -> docs/feature_list/
    if migrate_doc "$meeting_dir" "feature_list.md" "feature_list"; then
        ((migrated_count++))
    fi
done

echo ""
echo "=========================================="
echo "迁移完成"
echo "  总会议数: $total_meetings"
echo "  迁移文档数: $migrated_count"
echo "=========================================="
