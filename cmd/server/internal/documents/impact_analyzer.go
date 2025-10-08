package documents

// 影响分析器，使用BFS算法分析文档节点的影响范围

// AnalysisMode 分析模式
type AnalysisMode string

const (
	ModeParents      AnalysisMode = "parents"
	ModeChildren     AnalysisMode = "children"
	ModeReferences   AnalysisMode = "references"
	ModeDependencies AnalysisMode = "dependencies"
	ModeAll          AnalysisMode = "all"
)

// ImpactResult 影响分析结果
type ImpactResult struct {
	NodeID       string              `json:"node_id"`
	Parents      []string            `json:"parents"`
	Children     []string            `json:"children"`
	References   []string            `json:"references"`
	Dependencies []string            `json:"dependencies"`
	Depth        map[string]int      `json:"depth"`
	Paths        map[string][]string `json:"paths"`
}

// ImpactAnalyzer 影响分析器
type ImpactAnalyzer struct {
	index *IndexManager
}

// NewImpactAnalyzer 创建影响分析器
func NewImpactAnalyzer(index *IndexManager) *ImpactAnalyzer {
	return &ImpactAnalyzer{
		index: index,
	}
}

// Analyze 分析指定节点的影响范围
func (ia *ImpactAnalyzer) Analyze(nodeID string, modes []AnalysisMode) (*ImpactResult, error) {
	ia.index.mu.RLock()
	defer ia.index.mu.RUnlock()

	// 检查节点是否存在
	if _, err := ia.index.GetNode(nodeID); err != nil {
		return nil, err
	}

	result := &ImpactResult{
		NodeID:       nodeID,
		Parents:      []string{},
		Children:     []string{},
		References:   []string{},
		Dependencies: []string{},
		Depth:        make(map[string]int),
		Paths:        make(map[string][]string),
	}

	// 根据模式执行不同的分析
	for _, mode := range modes {
		switch mode {
		case ModeParents:
			ia.analyzeParents(nodeID, result)
		case ModeChildren:
			ia.analyzeChildren(nodeID, result)
		case ModeReferences:
			ia.analyzeReferences(nodeID, result)
		case ModeDependencies:
			ia.analyzeDependencies(nodeID, result)
		case ModeAll:
			ia.analyzeParents(nodeID, result)
			ia.analyzeChildren(nodeID, result)
			ia.analyzeReferences(nodeID, result)
			ia.analyzeDependencies(nodeID, result)
		}
	}

	return result, nil
}

// analyzeParents 分析父级关系（向上遍历）
func (ia *ImpactAnalyzer) analyzeParents(nodeID string, result *ImpactResult) {
	visited := make(map[string]bool)
	queue := []string{nodeID}
	depth := 0

	for len(queue) > 0 {
		levelSize := len(queue)

		for i := 0; i < levelSize; i++ {
			currentID := queue[i]

			if visited[currentID] {
				continue
			}
			visited[currentID] = true

			// 跳过起始节点
			if currentID != nodeID {
				result.Parents = append(result.Parents, currentID)
				result.Depth[currentID] = depth
			}

			// 查找父节点（通过parent_id）
			if node, err := ia.index.GetNode(currentID); err == nil && node.ParentID != nil {
				parentID := *node.ParentID
				if !visited[parentID] {
					queue = append(queue, parentID)
					result.Paths[parentID] = append(result.Paths[currentID], parentID)
				}
			}

			// 查找通过关系连接的父节点
			for _, rel := range ia.index.Relationships {
				if rel.ToID == currentID && rel.Type == RelationParentChild {
					if !visited[rel.FromID] {
						queue = append(queue, rel.FromID)
						result.Paths[rel.FromID] = append(result.Paths[currentID], rel.FromID)
					}
				}
			}
		}

		queue = queue[levelSize:]
		depth++

		// 防止无限循环，限制深度
		if depth > 10 {
			break
		}
	}
}

// analyzeChildren 分析子级关系（向下遍历）
func (ia *ImpactAnalyzer) analyzeChildren(nodeID string, result *ImpactResult) {
	visited := make(map[string]bool)
	queue := []string{nodeID}
	depth := 0

	for len(queue) > 0 {
		levelSize := len(queue)

		for i := 0; i < levelSize; i++ {
			currentID := queue[i]

			if visited[currentID] {
				continue
			}
			visited[currentID] = true

			// 跳过起始节点
			if currentID != nodeID {
				result.Children = append(result.Children, currentID)
				result.Depth[currentID] = depth
			}

			// 查找子节点
			children := ia.index.GetChildren(currentID)
			for _, childID := range children {
				if !visited[childID] {
					queue = append(queue, childID)
					result.Paths[childID] = append(result.Paths[currentID], childID)
				}
			}

			// 查找通过关系连接的子节点
			for _, rel := range ia.index.Relationships {
				if rel.FromID == currentID && rel.Type == RelationParentChild {
					if !visited[rel.ToID] {
						queue = append(queue, rel.ToID)
						result.Paths[rel.ToID] = append(result.Paths[currentID], rel.ToID)
					}
				}
			}
		}

		queue = queue[levelSize:]
		depth++

		// 防止无限循环
		if depth > 10 {
			break
		}
	}
}

// analyzeReferences 分析引用关系
func (ia *ImpactAnalyzer) analyzeReferences(nodeID string, result *ImpactResult) {
	// 查找引用该文档的任务
	if refs, exists := ia.index.ReferencesByDoc[nodeID]; exists {
		for _, ref := range refs {
			if ref.Status == RefStatusActive {
				result.References = append(result.References, ref.TaskID)
			}
		}
	}

	// 查找该文档引用的其他文档（通过任务关联）
	for taskID, refs := range ia.index.ReferencesByTask {
		hasCurrentDoc := false
		otherDocs := []string{}

		for _, ref := range refs {
			if ref.DocumentID == nodeID && ref.Status == RefStatusActive {
				hasCurrentDoc = true
			} else if ref.Status == RefStatusActive {
				otherDocs = append(otherDocs, ref.DocumentID)
			}
		}

		if hasCurrentDoc {
			for _, docID := range otherDocs {
				if !contains(result.References, docID) {
					result.References = append(result.References, docID)
				}
			}
		}

		_ = taskID // 避免未使用变量警告
	}
}

// analyzeDependencies 分析依赖关系
func (ia *ImpactAnalyzer) analyzeDependencies(nodeID string, result *ImpactResult) {
	visited := make(map[string]bool)
	queue := []string{nodeID}

	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]

		if visited[currentID] {
			continue
		}
		visited[currentID] = true

		// 查找依赖关系
		for _, rel := range ia.index.Relationships {
			if rel.FromID == currentID && rel.Type != RelationParentChild {
				if !visited[rel.ToID] {
					result.Dependencies = append(result.Dependencies, rel.ToID)
					queue = append(queue, rel.ToID)
				}
			}
		}
	}
}

// contains 检查字符串数组是否包含指定元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
