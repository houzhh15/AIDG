package taskdocs

import (
	"encoding/json"
	"errors"
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
	if expectedVersion == nil || *expectedVersion < 0 {
		return DocMeta{}, fmt.Errorf("expected version is required")
	}
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
		return DocMeta{}, fmt.Errorf("decode meta: %w", err)
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
	basePath, err = filepath.Abs(basePath)
	if err != nil {
		return
	}
	unlock := lockAppendPath(basePath)
	defer unlock()
	if err = recoverPendingWithPath(basePath); err != nil {
		return
	}

	meta, err = loadOrInitMetaWithPath(basePath)
	if err != nil {
		return
	}

	if expectedVersion != nil && meta.Version != *expectedVersion {
		err = fmt.Errorf("version_mismatch")
		return
	}

	h := hashDocContent(content)

	if op == "" {
		op = "add_full"
	}

	// 重复检测。replace_full 只有在当前 compiled.md 已经等于目标内容时才是 no-op；
	// 不能仅凭历史 HashWindow 判断，否则会阻止恢复到早先出现过的内容。
	compiledPath := docCompiledPathFromBase(basePath)
	if containsHash(meta.HashWindow, h) {
		compiledBytes, readErr := os.ReadFile(compiledPath)
		if readErr != nil {
			err = fmt.Errorf("read compiled for dedup: %w", readErr)
			return
		}
		if op != "replace_full" || hashDocContent(string(compiledBytes)) == h {
			compiledSize := int64(len(compiledBytes))
			log.Printf("[DOC_APPEND_PATH] seq=%d ver=%d op=%s duplicate=true content_size=%d compiled_size=%d dur_ms=%d",
				meta.LastSequence, meta.Version, op, len(content), compiledSize, time.Since(start).Milliseconds())
			return meta, nil, true, nil
		}
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

	cp := docChunksPathFromBase(basePath)
	chunksBefore, oldChunks, err := stateOf(cp)
	if err != nil {
		return meta, nil, false, err
	}
	compiledBefore, oldCompiled, err := stateOf(compiledPath)
	if err != nil {
		return meta, nil, false, err
	}
	metaPath := docMetaPathFromBase(basePath)
	metaBefore, _, err := stateOf(metaPath)
	if err != nil {
		return meta, nil, false, err
	}
	enc, marshalErr := json.Marshal(ck)
	if marshalErr != nil {
		return meta, nil, false, marshalErr
	}
	targetChunks := append(append([]byte(nil), oldChunks...), append(enc, '\n')...)
	needSectionSync := op == "replace_full"
	targetCompiled := []byte(content)
	if !operationReplacesCompiled(op) {
		if len(oldCompiled) > 0 {
			targetCompiled = append(append(append([]byte(nil), oldCompiled...), '\n'), []byte(content)...)
		}
	}
	meta.Version++
	meta.LastSequence = seq
	meta.HashWindow = pushHashWindow(meta.HashWindow, h)
	meta.ChunkCount++
	meta.UpdatedAt = time.Now()
	meta.ETag = hashDocContent(string(targetCompiled))
	intent := pendingCommit{Protocol: 1, OperationID: fmt.Sprintf("%d-%s", time.Now().UnixNano(), h[:12]), BeforeVersion: meta.Version - 1, TargetVersion: meta.Version, BeforeChunks: chunksBefore, BeforeCompiled: compiledBefore, BeforeMeta: metaBefore, TargetChunks: targetChunks, TargetCompiled: targetCompiled, TargetMeta: meta, NeedSectionSync: needSectionSync, CreatedAt: time.Now()}
	if err = persistPending(basePath, intent); err != nil {
		return
	}
	if err = recoverPendingWithPath(basePath); err != nil {
		return
	}
	compiledBytes := targetCompiled

	log.Printf("[DOC_APPEND_PATH] seq=%d ver=%d op=%s duplicate=false content_size=%d compiled_size=%d dur_ms=%d",
		seq, meta.Version, op, len(content), len(compiledBytes), time.Since(start).Milliseconds())

	return meta, &ck, false, nil
}

// listChunksWithPath 使用自定义路径列出 chunks
func listChunksWithPath(basePath string) ([]DocChunk, DocMeta, error) {
	basePath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, DocMeta{}, err
	}
	unlock := lockAppendPath(basePath)
	defer unlock()
	return listChunksUnlockedWithPath(basePath)
}
func listChunksUnlockedWithPath(basePath string) ([]DocChunk, DocMeta, error) {
	if err := recoverPendingWithPath(basePath); err != nil {
		return nil, DocMeta{}, err
	}
	cp := docChunksPathFromBase(basePath)
	meta, err := loadOrInitMetaWithPath(basePath)
	if err != nil {
		return nil, DocMeta{}, err
	}

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
			if operationReplacesCompiled(ck.Op) {
				bldr = []string{ck.Content}
			} else if ck.Content != "" {
				bldr = append(bldr, ck.Content)
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

	log.Printf("[DOC_REBUILD_PATH] total_chunks=%d active_chunks=%d deleted_chunks=%d compiled_size=%d dur_ms=%d",
		len(chunks), activeCount, deleted, len(compiled), time.Since(start).Milliseconds())

	return meta, nil
}

// squashWithPath 使用自定义路径压缩 chunks
func squashWithPath(basePath, user, source string, expectedVersion *int) (DocMeta, error) {
	start := time.Now()
	basePath, err := filepath.Abs(basePath)
	if err != nil {
		return DocMeta{}, err
	}
	unlock := lockAppendPath(basePath)
	defer unlock()
	if err = recoverPendingWithPath(basePath); err != nil {
		return DocMeta{}, err
	}

	meta, err := loadOrInitMetaWithPath(basePath)
	if err != nil {
		return meta, err
	}
	if expectedVersion != nil && meta.Version != *expectedVersion {
		return meta, fmt.Errorf("version_mismatch")
	}

	cp := docChunksPathFromBase(basePath)
	chunksBefore, oldChunks, err := stateOf(cp)
	if err != nil {
		return meta, err
	}
	merged, chunks, err := compileTargetChunks(oldChunks)
	if err != nil {
		return meta, fmt.Errorf("replay chunks before squash: %w", err)
	}
	if len(chunks) != meta.ChunkCount || hashDocContent(merged) != meta.ETag {
		return meta, errors.New("document state inconsistent before squash")
	}
	compiledPath := docCompiledPathFromBase(basePath)
	compiledBefore, compiledBytes, err := stateOf(compiledPath)
	if err != nil {
		return meta, err
	}
	if string(compiledBytes) != merged {
		return meta, errors.New("compiled content inconsistent before squash")
	}
	metaPath := docMetaPathFromBase(basePath)
	metaBefore, _, err := stateOf(metaPath)
	if err != nil {
		return meta, err
	}

	seq := meta.LastSequence + 1
	if source == "" {
		source = "squash"
	}
	ck := DocChunk{Sequence: seq, Timestamp: time.Now(), Op: "replace_full", Content: merged, User: user, Source: source, Hash: hashDocContent(merged), Active: true}
	enc, err := json.Marshal(ck)
	if err != nil {
		return meta, err
	}
	targetChunks := append(enc, '\n')
	meta.Version++
	meta.LastSequence = seq
	meta.ChunkCount = 1
	meta.DeletedCount = 0
	meta.HashWindow = []string{ck.Hash}
	meta.UpdatedAt = time.Now()
	meta.ETag = ck.Hash
	intent := pendingCommit{Protocol: 1, OperationID: fmt.Sprintf("%d-squash", time.Now().UnixNano()), BeforeVersion: meta.Version - 1, TargetVersion: meta.Version, BeforeChunks: chunksBefore, BeforeCompiled: compiledBefore, BeforeMeta: metaBefore, TargetChunks: targetChunks, TargetCompiled: []byte(merged), TargetMeta: meta, NeedSectionSync: false, CreatedAt: time.Now()}
	if err = persistPending(basePath, intent); err != nil {
		return meta, err
	}
	if err = recoverPendingWithPath(basePath); err != nil {
		return meta, err
	}

	log.Printf("[DOC_SQUASH_PATH] seq=%d ver=%d merged_size=%d dur_ms=%d", seq, meta.Version, len(merged), time.Since(start).Milliseconds())
	return meta, nil
}

// ExportWithPath 导出文档内容
func ExportWithPath(basePath string) (content string, meta DocMeta, err error) {
	basePath, err = filepath.Abs(basePath)
	if err != nil {
		return
	}
	unlock := lockAppendPath(basePath)
	defer unlock()
	if err = recoverPendingWithPath(basePath); err != nil {
		return
	}
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

// appendPathLocks coordinates the canonical Append implementation across instances.
// Other operations still require the outer docslot lock; this is not a transaction.
var appendPathLocks = struct {
	sync.Mutex
	entries map[string]*appendPathLock
}{entries: make(map[string]*appendPathLock)}

type appendPathLock struct {
	mu   sync.Mutex
	refs int
}

func lockAppendPath(path string) func() {
	appendPathLocks.Lock()
	entry := appendPathLocks.entries[path]
	if entry == nil {
		entry = &appendPathLock{}
		appendPathLocks.entries[path] = entry
	}
	entry.refs++
	appendPathLocks.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		appendPathLocks.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(appendPathLocks.entries, path)
		}
		appendPathLocks.Unlock()
	}
}
