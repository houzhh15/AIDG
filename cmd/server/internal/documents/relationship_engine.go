package documents

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RelationshipEngine 关系管理引擎
type RelationshipEngine struct {
	index *IndexManager
}

// NewRelationshipEngine 创建关系管理引擎
func NewRelationshipEngine(index *IndexManager) *RelationshipEngine {
	return &RelationshipEngine{
		index: index,
	}
}

// AddExplicitRelation 添加显式关系（兄弟关系等）
func (r *RelationshipEngine) AddExplicitRelation(fromID, toID string, relType RelationType) (*Relationship, error) {
	// 验证节点存在
	if _, err := r.index.GetNode(fromID); err != nil {
		return nil, fmt.Errorf("from node not found: %w", err)
	}
	if _, err := r.index.GetNode(toID); err != nil {
		return nil, fmt.Errorf("to node not found: %w", err)
	}

	// 检查循环依赖
	if err := r.DetectCycle(fromID, toID); err != nil {
		return nil, err
	}

	// 创建关系
	now := time.Now()
	rel := &Relationship{
		ID:        "rel_" + uuid.New().String(),
		FromID:    fromID,
		ToID:      toID,
		Type:      relType,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.index.mu.Lock()
	r.index.Relationships[rel.ID] = rel
	r.index.mu.Unlock()

	// 保存索引
	if err := r.index.FlushRelationships(); err != nil {
		return nil, fmt.Errorf("failed to save relationships: %w", err)
	}

	return rel, nil
}

// AddImplicitRelation 添加隐式依赖关系
func (r *RelationshipEngine) AddImplicitRelation(fromID, toID string, depType DependencyType) (*Relationship, error) {
	// 验证节点存在
	if _, err := r.index.GetNode(fromID); err != nil {
		return nil, fmt.Errorf("from node not found: %w", err)
	}
	if _, err := r.index.GetNode(toID); err != nil {
		return nil, fmt.Errorf("to node not found: %w", err)
	}

	// 检查循环依赖
	if err := r.DetectCycle(fromID, toID); err != nil {
		return nil, err
	}

	// 创建依赖关系
	now := time.Now()
	rel := &Relationship{
		ID:             "rel_" + uuid.New().String(),
		FromID:         fromID,
		ToID:           toID,
		Type:           RelationReference,
		DependencyType: &depType,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	r.index.mu.Lock()
	r.index.Relationships[rel.ID] = rel
	r.index.mu.Unlock()

	// 保存索引
	if err := r.index.FlushRelationships(); err != nil {
		return nil, fmt.Errorf("failed to save relationships: %w", err)
	}

	return rel, nil
}

// GetRelated 获取与节点相关的所有关系
func (r *RelationshipEngine) GetRelated(nodeID string) ([]*Relationship, error) {
	r.index.mu.RLock()
	defer r.index.mu.RUnlock()

	var related []*Relationship
	for _, rel := range r.index.Relationships {
		if rel.FromID == nodeID || rel.ToID == nodeID {
			related = append(related, rel)
		}
	}

	return related, nil
}

// GetOutgoing 获取从节点出发的关系
func (r *RelationshipEngine) GetOutgoing(nodeID string) ([]*Relationship, error) {
	r.index.mu.RLock()
	defer r.index.mu.RUnlock()

	var outgoing []*Relationship
	for _, rel := range r.index.Relationships {
		if rel.FromID == nodeID {
			outgoing = append(outgoing, rel)
		}
	}

	return outgoing, nil
}

// GetIncoming 获取指向节点的关系
func (r *RelationshipEngine) GetIncoming(nodeID string) ([]*Relationship, error) {
	r.index.mu.RLock()
	defer r.index.mu.RUnlock()

	var incoming []*Relationship
	for _, rel := range r.index.Relationships {
		if rel.ToID == nodeID {
			incoming = append(incoming, rel)
		}
	}

	return incoming, nil
}

// DeleteRelation 删除关系
func (r *RelationshipEngine) DeleteRelation(relationID string) error {
	r.index.mu.Lock()
	delete(r.index.Relationships, relationID)
	r.index.mu.Unlock()

	return r.index.FlushRelationships()
}

// DetectCycle 检测循环依赖（简化版DFS）
func (r *RelationshipEngine) DetectCycle(fromID, toID string) error {
	// 如果直接依赖自己
	if fromID == toID {
		return ErrCircularDependency
	}

	// 使用DFS检查是否存在从toID到fromID的路径
	visited := make(map[string]bool)
	return r.dfsDetectCycle(toID, fromID, visited)
}

// dfsDetectCycle DFS检测循环依赖
func (r *RelationshipEngine) dfsDetectCycle(current, target string, visited map[string]bool) error {
	if current == target {
		return ErrCircularDependency
	}

	if visited[current] {
		return nil // 已访问过的节点，不是循环
	}

	visited[current] = true

	// 检查所有从current出发的关系
	r.index.mu.RLock()
	for _, rel := range r.index.Relationships {
		if rel.FromID == current {
			if err := r.dfsDetectCycle(rel.ToID, target, visited); err != nil {
				r.index.mu.RUnlock()
				return err
			}
		}
	}
	r.index.mu.RUnlock()

	return nil
}

// GetRelationsByType 按类型获取关系
func (r *RelationshipEngine) GetRelationsByType(relType RelationType) ([]*Relationship, error) {
	r.index.mu.RLock()
	defer r.index.mu.RUnlock()

	var relations []*Relationship
	for _, rel := range r.index.Relationships {
		if rel.Type == relType {
			relations = append(relations, rel)
		}
	}

	return relations, nil
}

// UpdateRelationDescription 更新关系描述
func (r *RelationshipEngine) UpdateRelationDescription(relationID, description string) error {
	r.index.mu.Lock()
	defer r.index.mu.Unlock()

	rel, exists := r.index.Relationships[relationID]
	if !exists {
		return fmt.Errorf("relationship not found: %s", relationID)
	}

	rel.Description = description
	rel.UpdatedAt = time.Now()

	if err := r.index.flushRelationshipsLocked(); err != nil {
		return fmt.Errorf("failed to save relationships: %w", err)
	}

	return nil
}

// GetAllRelations 获取所有关系
func (r *RelationshipEngine) GetAllRelations() []*Relationship {
	r.index.mu.RLock()
	defer r.index.mu.RUnlock()

	relations := make([]*Relationship, 0, len(r.index.Relationships))
	for _, rel := range r.index.Relationships {
		relations = append(relations, rel)
	}
	return relations
}

// RemoveRelation 删除关系
func (r *RelationshipEngine) RemoveRelation(fromID, toID string) error {
	r.index.mu.Lock()
	defer r.index.mu.Unlock()

	// 查找关系
	var relationID string
	for id, rel := range r.index.Relationships {
		if rel.FromID == fromID && rel.ToID == toID {
			relationID = id
			break
		}
	}

	if relationID == "" {
		return ErrRelationNotFound
	}

	delete(r.index.Relationships, relationID)

	if err := r.index.flushRelationshipsLocked(); err != nil {
		return fmt.Errorf("failed to save relationships: %w", err)
	}

	return nil
}
