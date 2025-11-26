package taskdocs

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== WithPath 方法：支持自定义路径的文档操作 ==========

// AppendWithPath 使用自定义路径追加文档内容
func (s *DocService) AppendWithPath(basePath, content, op, user, source string, expectedVersion *int) (DocMeta, *DocChunk, bool, error) {
	// 使用路径作为锁键
	lockKey := basePath
	l := s.getLockByKey(lockKey)
	l.Lock()
	defer l.Unlock()

	return appendChunkInternalWithPath(basePath, content, op, user, source, expectedVersion)
}

// ListWithPath 使用自定义路径列出 chunks
func (s *DocService) ListWithPath(basePath string) ([]DocChunk, DocMeta, error) {
	lockKey := basePath
	l := s.getLockByKey(lockKey)
	l.Lock()
	defer l.Unlock()

	return listChunksWithPath(basePath)
}

// RebuildWithPath 使用自定义路径重建 compiled.md
func (s *DocService) RebuildWithPath(basePath string) (DocMeta, error) {
	lockKey := basePath
	l := s.getLockByKey(lockKey)
	l.Lock()
	defer l.Unlock()

	return rebuildCompiledWithPath(basePath)
}

// SquashWithPath 使用自定义路径压缩 chunks
func (s *DocService) SquashWithPath(basePath, user, source string, expectedVersion *int) (DocMeta, error) {
	lockKey := basePath
	l := s.getLockByKey(lockKey)
	l.Lock()
	defer l.Unlock()

	return squashWithPath(basePath, user, source, expectedVersion)
}

// getLockByKey 根据自定义键获取锁
func (s *DocService) getLockByKey(key string) *sync.Mutex {
	s.mu.Lock()
	l := s.locks[key]
	if l == nil {
		l = &sync.Mutex{}
		s.locks[key] = l
	}
	s.mu.Unlock()
	return l
}

// ========== 内部辅助函数 ==========

// docBaseDirPath 返回文档基础目录路径
func docBaseDirPath(basePath string) string {
	return basePath
}

// docMetaPathFromBase 返回 meta.json 路径
func docMetaPathFromBase(basePath string) string {
	return filepath.Join(basePath, "meta.json")
}

// docChunksPathFromBase 返回 chunks.ndjson 路径
func docChunksPathFromBase(basePath string) string {
	return filepath.Join(basePath, "chunks.ndjson")
}

// docCompiledPathFromBase 返回 compiled.md 路径
func docCompiledPathFromBase(basePath string) string {
	return filepath.Join(basePath, "compiled.md")
}

// loadOrInitMetaWithPath 从自定义路径加载或初始化 meta
func loadOrInitMetaWithPath(basePath string) (DocMeta, error) {
	mp := docMetaPathFromBase(basePath)
	data, err := os.ReadFile(mp)
	if err != nil {
		if os.IsNotExist(err) {
			// 从路径提取 docType
			docType := filepath.Base(basePath)
			return initDocMeta(docType), nil
		}
		return DocMeta{}, fmt.Errorf("read meta: %w", err)
	}

	var meta DocMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		docType := filepath.Base(basePath)
		return initDocMeta(docType), nil
	}

	return meta, nil
}

// writeMetaAtomicWithPath 原子写入 meta.json
func writeMetaAtomicWithPath(basePath string, meta DocMeta) error {
	mp := docMetaPathFromBase(basePath)
	if err := os.MkdirAll(filepath.Dir(mp), 0755); err != nil {
		return fmt.Errorf("create meta dir: %w", err)
	}

	tmp := mp + ".tmp"
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}

	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("write tmp meta: %w", err)
	}

	if err := os.Rename(tmp, mp); err != nil {
		return fmt.Errorf("rename meta: %w", err)
	}

	return nil
}

// appendChunkInternalWithPath 使用自定义路径追加 chunk
func appendChunkInternalWithPath(basePath, content, op, user, source string, expectedVersion *int) (meta DocMeta, newChunk *DocChunk, duplicate bool, err error) {
	start := time.Now()

	meta, err = loadOrInitMetaWithPath(basePath)
	if err != nil {
		return
	}

	if expectedVersion != nil && meta.Version != *expectedVersion {
		err = fmt.Errorf("version_mismatch")
		return
	}

	h := hashDocContent(content)

	// 重复检测
	if containsHash(meta.HashWindow, h) {
		compiledPath := docCompiledPathFromBase(basePath)
		var compiledSize int64
		if fi, statErr := os.Stat(compiledPath); statErr == nil {
			compiledSize = fi.Size()
		}
		log.Printf("[DOC_APPEND_PATH] path=%s seq=%d ver=%d op=%s duplicate=true content_size=%d compiled_size=%d dur_ms=%d",
			basePath, meta.LastSequence, meta.Version, op, len(content), compiledSize, time.Since(start).Milliseconds())
		return meta, nil, true, nil
	}

	if op == "" {
		op = "add_full"
	}
	if source == "" {
		source = "api"
	}

	seq := meta.LastSequence + 1
	ck := DocChunk{
		Sequence:  seq,
		Timestamp: time.Now(),
		Op:        op,
		Content:   content,
		User:      user,
		Source:    source,
		Hash:      h,
		Active:    true,
	}

	// 确保目录存在
	if err = os.MkdirAll(basePath, 0755); err != nil {
		return
	}

	// 写入 chunk
	cp := docChunksPathFromBase(basePath)
	f, err := os.OpenFile(cp, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}

	enc, _ := json.Marshal(ck)
	if _, err = f.Write(append(enc, '\n')); err != nil {
		f.Close()
		return
	}
	f.Close()

	// 更新 compiled
	compiledPath := docCompiledPathFromBase(basePath)
	needSectionSync := false
	if op == "replace_full" {
		_ = os.WriteFile(compiledPath, []byte(content), 0644)
		needSectionSync = true
	} else if strings.HasPrefix(op, "section_") && strings.HasSuffix(op, "_no_parse") {
		_ = os.WriteFile(compiledPath, []byte(content), 0644)
		needSectionSync = false
	} else {
		// append 模式
		if _, statErr := os.Stat(compiledPath); statErr == nil {
			rf, _ := os.OpenFile(compiledPath, os.O_WRONLY|os.O_APPEND, 0644)
			if rf != nil {
				_, _ = rf.Write([]byte("\n" + content))
				rf.Close()
			}
		} else {
			_ = os.WriteFile(compiledPath, []byte(content), 0644)
		}
	}

	compiledBytes, _ := os.ReadFile(compiledPath)

	// 更新 meta
	meta.Version++
	meta.LastSequence = seq
	meta.HashWindow = pushHashWindow(meta.HashWindow, h)
	meta.ChunkCount++
	meta.UpdatedAt = time.Now()
	meta.ETag = hashDocContent(string(compiledBytes))

	if err = writeMetaAtomicWithPath(basePath, meta); err != nil {
		return
	}

	// 如果需要同步章节
	if needSectionSync {
		docType := filepath.Base(basePath)
		sm := NewSyncManager(basePath, docType)
		log.Printf("[DOC_APPEND_PATH] path=%s section_sync_start compiled_size=%d", basePath, len(compiledBytes))
		if syncErr := sm.SyncFromCompiled(); syncErr != nil {
			log.Printf("[DOC_APPEND_PATH] path=%s section_sync_error=%v", basePath, syncErr)
		}
	}

	log.Printf("[DOC_APPEND_PATH] path=%s seq=%d ver=%d op=%s duplicate=false content_size=%d compiled_size=%d dur_ms=%d",
		basePath, seq, meta.Version, op, len(content), len(compiledBytes), time.Since(start).Milliseconds())

	return meta, &ck, false, nil
}

// listChunksWithPath 使用自定义路径列出 chunks
func listChunksWithPath(basePath string) ([]DocChunk, DocMeta, error) {
	cp := docChunksPathFromBase(basePath)
	meta, _ := loadOrInitMetaWithPath(basePath)

	data, err := os.ReadFile(cp)
	if err != nil {
		if os.IsNotExist(err) {
			return []DocChunk{}, meta, nil
		}
		return nil, meta, fmt.Errorf("read chunks: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	res := []DocChunk{}

	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		var ck DocChunk
		if json.Unmarshal([]byte(ln), &ck) == nil {
			res = append(res, ck)
		}
	}

	return res, meta, nil
}

// rebuildCompiledWithPath 使用自定义路径重建 compiled.md
func rebuildCompiledWithPath(basePath string) (DocMeta, error) {
	start := time.Now()

	chunks, meta, err := listChunksWithPath(basePath)
	if err != nil {
		return meta, err
	}

	var bldr []string
	maxSeq := 0
	activeCount := 0
	deleted := 0
	var lastHashes []string

	for _, ck := range chunks {
		if ck.Sequence > maxSeq {
			maxSeq = ck.Sequence
		}

		if ck.Active {
			activeCount++
			if ck.Content != "" {
				if ck.Op == "replace_full" || ck.Op == "section_full_no_parse" {
					bldr = []string{ck.Content}
				} else {
					bldr = append(bldr, ck.Content)
				}
			}
			lastHashes = pushHashWindow(lastHashes, ck.Hash)
		} else {
			deleted++
		}
	}

	compiled := strings.Join(bldr, "\n")
	cp := docCompiledPathFromBase(basePath)
	_ = os.WriteFile(cp, []byte(compiled), 0644)

	meta.LastSequence = maxSeq
	meta.ChunkCount = len(chunks)
	meta.DeletedCount = deleted
	meta.HashWindow = lastHashes
	meta.UpdatedAt = time.Now()
	meta.ETag = hashDocContent(compiled)

	if err = writeMetaAtomicWithPath(basePath, meta); err != nil {
		return meta, err
	}

	log.Printf("[DOC_REBUILD_PATH] path=%s total_chunks=%d active_chunks=%d deleted_chunks=%d compiled_size=%d dur_ms=%d",
		basePath, len(chunks), activeCount, deleted, len(compiled), time.Since(start).Milliseconds())

	return meta, nil
}

// squashWithPath 使用自定义路径压缩 chunks
func squashWithPath(basePath, user, source string, expectedVersion *int) (DocMeta, error) {
	start := time.Now()

	meta, err := loadOrInitMetaWithPath(basePath)
	if err != nil {
		return meta, err
	}

	if expectedVersion != nil && meta.Version != *expectedVersion {
		return meta, fmt.Errorf("version_mismatch")
	}

	// 先 rebuild 确保一致性
	if _, err := rebuildCompiledWithPath(basePath); err != nil {
		return meta, err
	}

	compiledPath := docCompiledPathFromBase(basePath)
	compiledBytes, err := os.ReadFile(compiledPath)
	if err != nil {
		return meta, err
	}

	merged := string(compiledBytes)

	// 归档旧文件
	cp := docChunksPathFromBase(basePath)
	if _, statErr := os.Stat(cp); statErr == nil {
		backup := cp + ".bak-" + time.Now().Format("20060102T150405")
		_ = os.Rename(cp, backup)
	}

	// 创建新的单一 chunk
	seq := meta.LastSequence + 1
	if source == "" {
		source = "squash"
	}
	ck := DocChunk{
		Sequence:  seq,
		Timestamp: time.Now(),
		Op:        "replace_full",
		Content:   merged,
		User:      user,
		Source:    source,
		Hash:      hashDocContent(merged),
		Active:    true,
	}

	enc, _ := json.Marshal(ck)
	if err := os.WriteFile(cp, append(enc, '\n'), 0644); err != nil {
		return meta, err
	}

	if err := os.WriteFile(compiledPath, []byte(merged), 0644); err != nil {
		return meta, err
	}

	meta.Version++
	meta.LastSequence = seq
	meta.ChunkCount = 1
	meta.DeletedCount = 0
	meta.HashWindow = pushHashWindow([]string{}, ck.Hash)
	meta.UpdatedAt = time.Now()
	meta.ETag = hashDocContent(merged)

	if err := writeMetaAtomicWithPath(basePath, meta); err != nil {
		return meta, err
	}

	log.Printf("[DOC_SQUASH_PATH] path=%s seq=%d ver=%d merged_size=%d dur_ms=%d",
		basePath, seq, meta.Version, len(merged), time.Since(start).Milliseconds())

	return meta, nil
}

// ExportWithPath 导出文档内容
func ExportWithPath(basePath string) (content string, meta DocMeta, err error) {
	compiledPath := docCompiledPathFromBase(basePath)
	meta, err = loadOrInitMetaWithPath(basePath)
	if err != nil {
		return "", meta, err
	}

	data, err := os.ReadFile(compiledPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", meta, nil
		}
		return "", meta, err
	}

	return string(data), meta, nil
}
