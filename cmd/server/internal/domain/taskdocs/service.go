package taskdocs

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// appendChunkInternal执行追加逻辑(内部实现)
// 返回 duplicate=true 表示未写入
func appendChunkInternal(projectID, taskID, docType, content, op, user, source string, expectedVersion *int) (meta DocMeta, newChunk *DocChunk, duplicate bool, err error) {
	start := time.Now()

	meta, err = loadOrInitMeta(projectID, taskID, docType)
	if err != nil {
		return
	}

	if expectedVersion != nil && meta.Version != *expectedVersion {
		err = fmt.Errorf("version_mismatch")
		return
	}

	h := hashDocContent(content)

	// 重复检测: 若命中哈希窗口则直接返回
	if containsHash(meta.HashWindow, h) {
		compiledPath, _ := docCompiledPath(projectID, taskID, docType)
		var compiledSize int64
		if fi, statErr := os.Stat(compiledPath); statErr == nil {
			compiledSize = fi.Size()
		}
		log.Printf("[DOC_APPEND] pid=%s tid=%s doc=%s seq=%d ver=%d op=%s duplicate=true content_size=%d total_chunks=%d compiled_size=%d dur_ms=%d",
			projectID, taskID, docType, meta.LastSequence, meta.Version, op, len(content), meta.ChunkCount, compiledSize, time.Since(start).Milliseconds())
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

	// 写入 chunk
	cp, err := docChunksPath(projectID, taskID, docType)
	if err != nil {
		return
	}

	if err = os.MkdirAll(filepath.Dir(cp), 0755); err != nil {
		return
	}

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
	compiledPath, _ := docCompiledPath(projectID, taskID, docType)
	needSectionSync := false
	if op == "replace_full" {
		_ = os.WriteFile(compiledPath, []byte(content), 0644)
		needSectionSync = true // 全文替换后需要重新解析章节
	} else if strings.HasPrefix(op, "section_") && strings.HasSuffix(op, "_no_parse") {
		// 🔧 特殊操作：章节级操作已经在 SectionService 中处理了同步，不需要再次解析
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

	if err = writeMetaAtomic(projectID, taskID, docType, meta); err != nil {
		return
	}

	// 如果是 replace_full 操作，重新解析章节结构
	if needSectionSync {
		docPath, pathErr := docBaseDir(projectID, taskID, docType)
		if pathErr == nil {
			sm := NewSyncManager(docPath, docType)
			log.Printf("[DOC_APPEND] pid=%s tid=%s doc=%s section_sync_start compiled_size=%d", projectID, taskID, docType, len(compiledBytes))
			if syncErr := sm.SyncFromCompiled(); syncErr != nil {
				log.Printf("[DOC_APPEND] pid=%s tid=%s doc=%s section_sync_error=%v", projectID, taskID, docType, syncErr)
			} else {
				// 重新读取 compiled.md 确认内容
				verifyBytes, _ := os.ReadFile(compiledPath)
				log.Printf("[DOC_APPEND] pid=%s tid=%s doc=%s section_sync=success compiled_size_after=%d", projectID, taskID, docType, len(verifyBytes))
			}
		}
	}

	log.Printf("[DOC_APPEND] pid=%s tid=%s doc=%s seq=%d ver=%d op=%s duplicate=false content_size=%d total_chunks=%d compiled_size=%d dur_ms=%d",
		projectID, taskID, docType, seq, meta.Version, op, len(content), meta.ChunkCount, len(compiledBytes), time.Since(start).Milliseconds())

	return meta, &ck, false, nil
}

// rebuildCompiled 全量重建 compiled 与 meta
func rebuildCompiled(projectID, taskID, docType string) (DocMeta, error) {
	start := time.Now()

	chunks, meta, err := listChunks(projectID, taskID, docType)
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
				// 如果是 replace_full 或 section_full_no_parse，清空之前的内容，从这个 chunk 开始重新构建
				if ck.Op == "replace_full" || ck.Op == "section_full_no_parse" {
					bldr = []string{ck.Content}
				} else {
					// 其他操作类型（add_full 等）都是追加
					bldr = append(bldr, ck.Content)
				}
			}
			lastHashes = pushHashWindow(lastHashes, ck.Hash)
		} else {
			deleted++
		}
	}

	compiled := strings.Join(bldr, "\n")

	cp, _ := docCompiledPath(projectID, taskID, docType)
	_ = os.WriteFile(cp, []byte(compiled), 0644)

	meta.LastSequence = maxSeq
	meta.ChunkCount = len(chunks)
	meta.DeletedCount = deleted
	meta.HashWindow = lastHashes
	meta.UpdatedAt = time.Now()
	meta.ETag = hashDocContent(compiled)

	if err = writeMetaAtomic(projectID, taskID, docType, meta); err != nil {
		return meta, err
	}

	log.Printf("[DOC_REBUILD] pid=%s tid=%s doc=%s total_chunks=%d active_chunks=%d deleted_chunks=%d compiled_size=%d dur_ms=%d",
		projectID, taskID, docType, len(chunks), activeCount, deleted, len(compiled), time.Since(start).Milliseconds())

	return meta, nil
}

// logicalDeleteChunk 软删除 chunk
func logicalDeleteChunk(projectID, taskID, docType string, seq int) (DocMeta, error) {
	start := time.Now()

	cp, err := docChunksPath(projectID, taskID, docType)
	if err != nil {
		return DocMeta{}, err
	}

	data, err := os.ReadFile(cp)
	if err != nil {
		return DocMeta{}, err
	}

	lines := strings.Split(string(data), "\n")
	changed := false

	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}

		var ck DocChunk
		if json.Unmarshal([]byte(ln), &ck) == nil && ck.Sequence == seq && ck.Active {
			ck.Active = false
			enc, _ := json.Marshal(ck)
			lines[i] = string(enc)
			changed = true
			break
		}
	}

	meta, _ := loadOrInitMeta(projectID, taskID, docType)

	if !changed {
		log.Printf("[DOC_DELETE] pid=%s tid=%s doc=%s seq=%d found=false ver=%d dur_ms=%d",
			projectID, taskID, docType, seq, meta.Version, time.Since(start).Milliseconds())
		return meta, nil
	}

	if err := os.WriteFile(cp, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return DocMeta{}, err
	}

	meta, err = rebuildCompiled(projectID, taskID, docType)
	if err != nil {
		return meta, err
	}

	compiledPath, _ := docCompiledPath(projectID, taskID, docType)
	var compiledSize int64
	if fi, statErr := os.Stat(compiledPath); statErr == nil {
		compiledSize = fi.Size()
	}

	log.Printf("[DOC_DELETE] pid=%s tid=%s doc=%s seq=%d found=true ver=%d total_chunks=%d deleted_chunks=%d compiled_size=%d dur_ms=%d",
		projectID, taskID, docType, seq, meta.Version, meta.ChunkCount, meta.DeletedCount, compiledSize, time.Since(start).Milliseconds())

	return meta, nil
}

// Append 封装底层 appendChunkInternal，提供并发互斥
func (s *DocService) Append(projectID, taskID, docType, content, user string, expectedVersion *int, op, source string) (DocMeta, *DocChunk, bool, error) {
	// 只在非 replace_full 模式下拒绝空内容
	// replace_full 允许空内容（用户可能想清空文档）
	if strings.TrimSpace(content) == "" && op != "replace_full" {
		return DocMeta{}, nil, false, fmt.Errorf("empty_content")
	}

	l := s.GetLock(projectID, taskID, docType)
	l.Lock()
	defer l.Unlock()

	return appendChunkInternal(projectID, taskID, docType, content, op, user, source, expectedVersion)
}

// List 返回 chunks + meta
func (s *DocService) List(projectID, taskID, docType string) ([]DocChunk, DocMeta, error) {
	l := s.GetLock(projectID, taskID, docType)
	l.Lock()
	defer l.Unlock()

	return listChunks(projectID, taskID, docType)
}

// Delete 逻辑删除并重建
func (s *DocService) Delete(projectID, taskID, docType string, seq int) (DocMeta, error) {
	l := s.GetLock(projectID, taskID, docType)
	l.Lock()
	defer l.Unlock()

	return logicalDeleteChunk(projectID, taskID, docType, seq)
}

// Rebuild 外部触发重建
func (s *DocService) Rebuild(projectID, taskID, docType string) (DocMeta, error) {
	l := s.GetLock(projectID, taskID, docType)
	l.Lock()
	defer l.Unlock()

	return rebuildCompiled(projectID, taskID, docType)
}

// Toggle 激活/停用某个 chunk
func (s *DocService) Toggle(projectID, taskID, docType string, seq int) (DocMeta, error) {
	l := s.GetLock(projectID, taskID, docType)
	l.Lock()
	defer l.Unlock()

	start := time.Now()

	cp, err := docChunksPath(projectID, taskID, docType)
	if err != nil {
		return DocMeta{}, err
	}

	data, err := os.ReadFile(cp)
	if err != nil {
		return DocMeta{}, err
	}

	lines := strings.Split(string(data), "\n")
	changed := false

	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}

		var ck DocChunk
		if json.Unmarshal([]byte(ln), &ck) == nil && ck.Sequence == seq {
			ck.Active = !ck.Active
			enc, _ := json.Marshal(ck)
			lines[i] = string(enc)
			changed = true
			break
		}
	}

	meta, _ := loadOrInitMeta(projectID, taskID, docType)

	if !changed {
		log.Printf("[DOC_TOGGLE] pid=%s tid=%s doc=%s seq=%d found=false ver=%d dur_ms=%d",
			projectID, taskID, docType, seq, meta.Version, time.Since(start).Milliseconds())
		return meta, nil
	}

	if err := os.WriteFile(cp, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return DocMeta{}, err
	}

	meta, err = rebuildCompiled(projectID, taskID, docType)
	if err != nil {
		return meta, err
	}

	compiledPath, _ := docCompiledPath(projectID, taskID, docType)
	var compiledSize int64
	if fi, statErr := os.Stat(compiledPath); statErr == nil {
		compiledSize = fi.Size()
	}

	log.Printf("[DOC_TOGGLE] pid=%s tid=%s doc=%s seq=%d found=true ver=%d total_chunks=%d deleted_chunks=%d compiled_size=%d dur_ms=%d",
		projectID, taskID, docType, seq, meta.Version, meta.ChunkCount, meta.DeletedCount, compiledSize, time.Since(start).Milliseconds())

	return meta, nil
}

// Squash 合并所有 active chunks 为单一 chunk
func (s *DocService) Squash(projectID, taskID, docType, user string, expectedVersion *int) (DocMeta, error) {
	l := s.GetLock(projectID, taskID, docType)
	l.Lock()
	defer l.Unlock()

	start := time.Now()

	meta, err := loadOrInitMeta(projectID, taskID, docType)
	if err != nil {
		return meta, err
	}

	if expectedVersion != nil && meta.Version != *expectedVersion {
		return meta, fmt.Errorf("version_mismatch")
	}

	// 先 rebuild 确保一致性
	if _, err := rebuildCompiled(projectID, taskID, docType); err != nil {
		return meta, err
	}

	compiledPath, _ := docCompiledPath(projectID, taskID, docType)
	compiledBytes, err := os.ReadFile(compiledPath)
	if err != nil {
		return meta, err
	}

	merged := string(compiledBytes)

	// 归档旧文件
	cp, _ := docChunksPath(projectID, taskID, docType)
	if _, statErr := os.Stat(cp); statErr == nil {
		backup := cp + ".bak-" + time.Now().Format("20060102T150405")
		_ = os.Rename(cp, backup)
	}

	// 创建新的单一 chunk
	seq := meta.LastSequence + 1
	ck := DocChunk{
		Sequence:  seq,
		Timestamp: time.Now(),
		Op:        "replace_full",
		Content:   merged,
		User:      user,
		Source:    "squash",
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

	if err := writeMetaAtomic(projectID, taskID, docType, meta); err != nil {
		return meta, err
	}

	log.Printf("[DOC_SQUASH] pid=%s tid=%s doc=%s seq=%d ver=%d merged_size=%d dur_ms=%d",
		projectID, taskID, docType, seq, meta.Version, len(merged), time.Since(start).Milliseconds())

	return meta, nil
}
