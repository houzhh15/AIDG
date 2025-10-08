package documents

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ReferenceManager 引用管理器
type ReferenceManager struct {
	index *IndexManager
}

// NewReferenceManager 创建引用管理器
func NewReferenceManager(index *IndexManager) *ReferenceManager {
	return &ReferenceManager{
		index: index,
	}
}

// CreateReference 创建任务文档引用
func (rm *ReferenceManager) CreateReference(req CreateReferenceRequest) (*Reference, error) {
	anchor := ""
	if req.Anchor != nil {
		anchor = strings.TrimSpace(*req.Anchor)
		if anchor != "" {
			if err := rm.validateAnchor(anchor); err != nil {
				return nil, fmt.Errorf("invalid anchor: %w", err)
			}
		}
	}

	context := ""
	if req.Context != nil {
		context = strings.TrimSpace(*req.Context)
	}

	// 验证文档存在
	if _, err := rm.index.GetNode(req.DocumentID); err != nil {
		return nil, fmt.Errorf("document not found: %w", err)
	}

	// 创建引用
	now := time.Now()
	ref := &Reference{
		ID:         "ref_" + uuid.New().String(),
		TaskID:     req.TaskID,
		DocumentID: req.DocumentID,
		Anchor:     anchor,
		Context:    context,
		Status:     RefStatusActive,
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// 添加到索引
	rm.index.mu.Lock()
	rm.index.ReferencesByTask[ref.TaskID] = append(rm.index.ReferencesByTask[ref.TaskID], ref)
	rm.index.ReferencesByDoc[ref.DocumentID] = append(rm.index.ReferencesByDoc[ref.DocumentID], ref)
	rm.index.mu.Unlock()

	// 保存索引
	if err := rm.index.FlushReferences(); err != nil {
		return nil, fmt.Errorf("failed to save references: %w", err)
	}

	return ref, nil
}

// GetReferencesByTask 获取任务的所有引用
func (rm *ReferenceManager) GetReferencesByTask(taskID string) ([]*Reference, error) {
	rm.index.mu.RLock()
	defer rm.index.mu.RUnlock()

	refs, exists := rm.index.ReferencesByTask[taskID]
	if !exists {
		return []*Reference{}, nil
	}

	// 返回副本
	result := make([]*Reference, len(refs))
	copy(result, refs)
	return result, nil
}

// GetReferencesByDoc 获取文档的所有引用
func (rm *ReferenceManager) GetReferencesByDoc(docID string) ([]*Reference, error) {
	rm.index.mu.RLock()
	defer rm.index.mu.RUnlock()

	refs, exists := rm.index.ReferencesByDoc[docID]
	if !exists {
		return []*Reference{}, nil
	}

	// 返回副本
	result := make([]*Reference, len(refs))
	copy(result, refs)
	return result, nil
}

// UpdateReferenceStatus 更新引用状态
func (rm *ReferenceManager) UpdateReferenceStatus(refID string, status ReferenceStatus) error {
	rm.index.mu.Lock()
	defer rm.index.mu.Unlock()

	// 查找引用
	var targetRef *Reference
	for _, refs := range rm.index.ReferencesByTask {
		for _, ref := range refs {
			if ref.ID == refID {
				targetRef = ref
				break
			}
		}
		if targetRef != nil {
			break
		}
	}

	if targetRef == nil {
		return fmt.Errorf("reference not found: %s", refID)
	}

	targetRef.Status = status
	targetRef.UpdatedAt = time.Now()

	if err := rm.index.flushReferencesLocked(); err != nil {
		return fmt.Errorf("failed to save references: %w", err)
	}

	return nil
}

// DeleteReference 删除引用
func (rm *ReferenceManager) DeleteReference(refID string) error {
	rm.index.mu.Lock()
	defer rm.index.mu.Unlock()

	// 查找并删除引用
	var found bool
	for taskID, refs := range rm.index.ReferencesByTask {
		for i, ref := range refs {
			if ref.ID == refID {
				// 从task映射中删除
				rm.index.ReferencesByTask[taskID] = append(refs[:i], refs[i+1:]...)
				if len(rm.index.ReferencesByTask[taskID]) == 0 {
					delete(rm.index.ReferencesByTask, taskID)
				}

				// 从doc映射中删除
				docRefs := rm.index.ReferencesByDoc[ref.DocumentID]
				for j, docRef := range docRefs {
					if docRef.ID == refID {
						rm.index.ReferencesByDoc[ref.DocumentID] = append(docRefs[:j], docRefs[j+1:]...)
						if len(rm.index.ReferencesByDoc[ref.DocumentID]) == 0 {
							delete(rm.index.ReferencesByDoc, ref.DocumentID)
						}
						break
					}
				}

				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("reference not found: %s", refID)
	}

	if err := rm.index.flushReferencesLocked(); err != nil {
		return fmt.Errorf("failed to save references: %w", err)
	}

	return nil
}

// MarkDocumentReferencesOutdated 标记文档的所有引用为过时
func (rm *ReferenceManager) MarkDocumentReferencesOutdated(docID string) error {
	rm.index.mu.Lock()
	defer rm.index.mu.Unlock()

	refs, exists := rm.index.ReferencesByDoc[docID]
	if !exists {
		return nil // 没有引用，直接返回
	}

	// 标记所有引用为过时
	for _, ref := range refs {
		if ref.Status == RefStatusActive {
			ref.Status = RefStatusOutdated
			ref.UpdatedAt = time.Now()
		}
	}

	if err := rm.index.flushReferencesLocked(); err != nil {
		return fmt.Errorf("failed to save references: %w", err)
	}

	return nil
}

// GetActiveReferences 获取活跃的引用
func (rm *ReferenceManager) GetActiveReferences() ([]*Reference, error) {
	rm.index.mu.RLock()
	defer rm.index.mu.RUnlock()

	var activeRefs []*Reference
	for _, refs := range rm.index.ReferencesByTask {
		for _, ref := range refs {
			if ref.Status == RefStatusActive {
				activeRefs = append(activeRefs, ref)
			}
		}
	}

	return activeRefs, nil
}

// GetOutdatedReferences 获取过时的引用
func (rm *ReferenceManager) GetOutdatedReferences() ([]*Reference, error) {
	rm.index.mu.RLock()
	defer rm.index.mu.RUnlock()

	var outdatedRefs []*Reference
	for _, refs := range rm.index.ReferencesByTask {
		for _, ref := range refs {
			if ref.Status == RefStatusOutdated {
				outdatedRefs = append(outdatedRefs, ref)
			}
		}
	}

	return outdatedRefs, nil
}

// validateAnchor 验证锚点格式
func (rm *ReferenceManager) validateAnchor(anchor string) error {
	if anchor == "" {
		return fmt.Errorf("anchor cannot be empty")
	}
	if strings.ContainsAny(anchor, "\r\n\t") {
		return fmt.Errorf("anchor contains invalid whitespace: %s", anchor)
	}
	if utf8.RuneCountInString(anchor) > 120 {
		return fmt.Errorf("anchor is too long (max 120 characters)")
	}
	return nil
}

// FindReference 查找特定引用
func (rm *ReferenceManager) FindReference(refID string) (*Reference, error) {
	rm.index.mu.RLock()
	defer rm.index.mu.RUnlock()

	for _, refs := range rm.index.ReferencesByTask {
		for _, ref := range refs {
			if ref.ID == refID {
				return ref, nil
			}
		}
	}

	return nil, fmt.Errorf("reference not found: %s", refID)
}

// GetReferenceStats 获取引用统计信息
func (rm *ReferenceManager) GetReferenceStats() map[string]interface{} {
	rm.index.mu.RLock()
	defer rm.index.mu.RUnlock()

	stats := map[string]interface{}{
		"total_references": 0,
		"active_count":     0,
		"outdated_count":   0,
		"broken_count":     0,
		"tasks_with_refs":  len(rm.index.ReferencesByTask),
		"docs_with_refs":   len(rm.index.ReferencesByDoc),
	}

	for _, refs := range rm.index.ReferencesByTask {
		for _, ref := range refs {
			stats["total_references"] = stats["total_references"].(int) + 1
			switch ref.Status {
			case RefStatusActive:
				stats["active_count"] = stats["active_count"].(int) + 1
			case RefStatusOutdated:
				stats["outdated_count"] = stats["outdated_count"].(int) + 1
			case RefStatusBroken:
				stats["broken_count"] = stats["broken_count"].(int) + 1
			}
		}
	}

	return stats
}
