#!/bin/bash
# 迁移项目文档从旧格式到新的统一格式
# 旧格式: data/projects/{project}/feature_list.md, architecture_new.md
# 新格式: data/projects/{project}/docs/feature_list/chunks.jsonl, compiled.md

set -e

BASE_PATH="${1:-data/projects}"

migrate_file() {
    local project_dir="$1"
    local old_file="$2"
    local slot_key="$3"
    local project_name=$(basename "$project_dir")
    
    local source_path="${project_dir}/${old_file}"
    local target_dir="${project_dir}/docs/${slot_key}"
    local chunks_file="${target_dir}/chunks.jsonl"
    local compiled_file="${target_dir}/compiled.md"
    local meta_file="${target_dir}/meta.json"
    
    # 检查源文件
    if [ ! -f "$source_path" ]; then
        return 0  # 静默跳过
    fi
    
    # 检查目标是否已迁移
    if [ -f "$chunks_file" ]; then
        echo "⏭️  [$project_name] $slot_key: already migrated"
        return 0
    fi
    
    # 创建目标目录
    mkdir -p "$target_dir"
    
    # 读取内容
    local content=$(cat "$source_path")
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local hash=$(echo -n "$content" | md5 | head -c 16)
    
    # 创建 chunk
    local chunk=$(cat << EOF
{"sequence":1,"timestamp":"$timestamp","op":"replace","content":$(echo -n "$content" | jq -Rs .),"user":"migration","source":"legacy_migration","hash":"$hash","active":true}
EOF
    )
    
    # 写入 chunks.jsonl
    echo "$chunk" > "$chunks_file"
    
    # 写入 compiled.md
    echo "$content" > "$compiled_file"
    
    # 写入 meta.json
    cat > "$meta_file" << EOF
{
  "version": 1,
  "last_sequence": 1,
  "created_at": "$timestamp",
  "updated_at": "$timestamp",
  "doc_type": "$slot_key",
  "hash_window": ["$hash"],
  "chunk_count": 1,
  "deleted_count": 0,
  "etag": "$hash"
}
EOF
    
    # 归档旧文件
    mv "$source_path" "${source_path}.legacy"
    
    echo "✅ [$project_name] $slot_key: migrated"
}

echo "🔄 Starting project documents migration..."
echo ""

# 遍历所有项目
for project_dir in "$BASE_PATH"/*; do
    if [ ! -d "$project_dir" ]; then
        continue
    fi
    
    project_name=$(basename "$project_dir")
    
    # 跳过特殊目录
    case "$project_name" in
        audit_logs|projects|prompts|roles|user_roles|users|others|.svn)
            continue
            ;;
    esac
    
    # 检查是否是项目目录
    if [ ! -f "${project_dir}/tasks.json" ] && [ ! -d "${project_dir}/tasks" ]; then
        continue
    fi
    
    echo "Processing project: $project_name"
    
    # 迁移特性列表
    migrate_file "$project_dir" "feature_list.md" "feature_list"
    migrate_file "$project_dir" "docs/feature_list.md" "feature_list"
    
    # 迁移架构设计
    migrate_file "$project_dir" "architecture_new.md" "architecture_design"
    migrate_file "$project_dir" "docs/architecture_design.md" "architecture_design"
    
    echo ""
done

echo "📊 Migration complete!"
