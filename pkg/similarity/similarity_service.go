// Package similarity provides similarity service for semantic recommendations
package similarity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"
)

// SectionServiceInterface defines interface for section operations
type SectionServiceInterface interface {
	GetSections(projectID, taskID, docType string) (*SectionsResponse, error)
	GetSection(projectID, taskID, docType, sectionID string, includeChildren bool) (*SectionResponse, error)
}

// SectionsResponse represents response from GetSections
type SectionsResponse struct {
	Sections []SectionMeta `json:"sections"`
}

// SectionMeta represents section metadata
type SectionMeta struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// SectionResponse represents response from GetSection
type SectionResponse struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// NLPClient encapsulates HTTP client for Python NLP service
type NLPClient struct {
	baseURL       string
	httpClient    *http.Client
	lastHealthy   time.Time
	healthCheckMu sync.RWMutex
	isHealthy     bool
	lastCheckTime time.Time
	checkInterval time.Duration // 健康检查间隔（默认30秒）
}

// EmbedRequest represents request to NLP embed endpoint
type EmbedRequest struct {
	Texts []string `json:"texts"`
	Model string   `json:"model,omitempty"`
}

// EmbedResponse represents response from NLP embed endpoint
type EmbedResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	Model      string      `json:"model"`
	Dim        int         `json:"dim"`
}

// NewNLPClient creates a new NLP client
func NewNLPClient(baseURL string, timeout time.Duration) *NLPClient {
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	client := &NLPClient{
		baseURL:       baseURL,
		checkInterval: 30 * time.Second, // 30秒检查一次健康状态
		isHealthy:     false,            // 初始假设不健康,等待首次检查
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}

	// 启动时立即同步检查一次健康状态,避免首次调用时尝试连接不可用的服务
	log.Printf("[INFO] NLPClient initializing, checking health for %s", baseURL)
	client.checkHealth()
	log.Printf("[INFO] NLPClient initialized, isHealthy=%v", client.isHealthy)

	// 之后定期异步检查
	go func() {
		ticker := time.NewTicker(client.checkInterval)
		defer ticker.Stop()
		for range ticker.C {
			client.checkHealth()
		}
	}()

	return client
}

// checkHealth 检查 NLP 服务健康状态
func (c *NLPClient) checkHealth() {
	c.healthCheckMu.Lock()
	defer c.healthCheckMu.Unlock()

	// 避免频繁检查(初始化时跳过此检查)
	if !c.lastCheckTime.IsZero() && time.Since(c.lastCheckTime) < c.checkInterval {
		return
	}

	c.lastCheckTime = time.Now()

	// 尝试发送一个简单的健康检查请求
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		c.isHealthy = false
		log.Printf("[WARN] NLP service health check failed (request creation): %v", err)
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.isHealthy = false
		log.Printf("[WARN] NLP service is NOT healthy: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		c.isHealthy = true
		c.lastHealthy = time.Now()
		//log.Printf("[INFO] NLP service is healthy")
	} else {
		c.isHealthy = false
		//log.Printf("[WARN] NLP service health check returned status %d", resp.StatusCode)
	}
}

// IsHealthy 返回服务是否健康（带自动检查）
func (c *NLPClient) IsHealthy() bool {
	c.healthCheckMu.RLock()
	needCheck := time.Since(c.lastCheckTime) >= c.checkInterval
	healthy := c.isHealthy
	c.healthCheckMu.RUnlock()

	// 如果需要重新检查，启动异步检查
	if needCheck {
		go c.checkHealth()
	}

	return healthy
}

// Embed sends texts to NLP service for vectorization
func (c *NLPClient) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	// 检查服务健康状态
	if !c.IsHealthy() {
		return nil, fmt.Errorf("NLP service is currently unavailable")
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("texts cannot be empty")
	}

	if len(texts) > 100 {
		return nil, fmt.Errorf("batch size exceeds 100")
	}

	reqBody := EmbedRequest{
		Texts: texts,
		Model: "text2vec-base-chinese",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/nlp/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// 连接失败,立即标记为不健康
		c.healthCheckMu.Lock()
		c.isHealthy = false
		c.lastCheckTime = time.Now()
		c.healthCheckMu.Unlock()
		log.Printf("[WARN] NLP service connection failed, marked as unhealthy: %v", err)
		return nil, fmt.Errorf("failed to call NLP service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("NLP service returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var embedResp EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return embedResp.Embeddings, nil
}

// SimilarityService provides similarity search and recommendation
type SimilarityService struct {
	indexMgr   *VectorIndexManager
	nlpClient  *NLPClient
	sectionSvc SectionServiceInterface
}

// NewSimilarityService creates a new similarity service
func NewSimilarityService(
	indexMgr *VectorIndexManager,
	nlpClient *NLPClient,
	sectionSvc SectionServiceInterface,
) *SimilarityService {
	return &SimilarityService{
		indexMgr:   indexMgr,
		nlpClient:  nlpClient,
		sectionSvc: sectionSvc,
	}
}

// GetRecommendations gets recommendations based on saved document content
// Queries all sections with content >= 50 chars, aggregates results by similarity
func (s *SimilarityService) GetRecommendations(
	ctx context.Context,
	projectID, taskID, docType string,
	topK int,
) ([]RecommendationResult, error) {
	// 1. Get current task document sections
	sections, err := s.sectionSvc.GetSections(projectID, taskID, docType)
	if err != nil {
		return nil, fmt.Errorf("failed to get sections: %w", err)
	}

	if len(sections.Sections) == 0 {
		return []RecommendationResult{}, nil
	}

	// 2. Collect all valid sections (content >= 50 chars) for querying
	type SectionQuery struct {
		SectionID string
		Title     string
		Content   string
	}

	var validSections []SectionQuery
	for _, secMeta := range sections.Sections {
		sec, err := s.sectionSvc.GetSection(projectID, taskID, docType, secMeta.ID, false)
		if err != nil {
			continue
		}

		contentLen := len([]rune(sec.Content))
		if contentLen >= 50 { // Filter: content must be >= 50 chars
			validSections = append(validSections, SectionQuery{
				SectionID: secMeta.ID,
				Title:     secMeta.Title,
				Content:   sec.Content,
			})
		}
	}

	if len(validSections) == 0 {
		return []RecommendationResult{}, nil
	}

	// 提前检查NLP服务健康状态,避免每个section都尝试并打印错误
	nlpHealthy := s.nlpClient.IsHealthy()
	if !nlpHealthy {
		log.Printf("[INFO] GetRecommendations: NLP service unavailable, will use cached vectors only")
	}

	// 3. Query each section and aggregate results
	// Use map to track best similarity for each recommendation
	type AggregatedResult struct {
		Result          RecommendationResult
		SourceSectionID string // Which section of current task matched this
		SourceTitle     string
	}

	resultMap := make(map[string]*AggregatedResult) // key: taskID:docType:sectionID

	// 统计NLP调用情况
	var cachedCount, nlpSuccessCount, nlpFailCount int

	for _, secQuery := range validSections {
		// Try to get cached vector first
		cachedVector := s.indexMgr.GetVector(taskID, docType, secQuery.SectionID)

		var queryVector []float64
		if cachedVector != nil {
			// Use cached vector (fast path - no NLP API call)
			queryVector = cachedVector
			cachedCount++
		} else if nlpHealthy {
			// Fallback: Vectorize section content (slow path - calls NLP service)
			queryVectors, err := s.nlpClient.Embed(ctx, []string{secQuery.Content})
			if err != nil {
				nlpFailCount++
				continue
			}
			queryVector = queryVectors[0]
			nlpSuccessCount++

			// CRITICAL FIX: Cache the vector result for future use
			entry := &VectorEntry{
				ProjectID: projectID,
				TaskID:    taskID,
				DocType:   docType,
				SectionID: secQuery.SectionID,
				Title:     secQuery.Title,
				Vector:    queryVector,
				UpdatedAt: time.Now().Format(time.RFC3339),
			}
			s.indexMgr.Update([]*VectorEntry{entry})

			// Persist to disk
			if err := s.indexMgr.Save(); err != nil {
				log.Printf("[WARN] Failed to save vector index: %v", err)
			}
		} else {
			// NLP service unavailable and no cache, skip this section
			nlpFailCount++
			continue
		}

		// Query similar vectors (get more candidates for filtering)
		results := s.indexMgr.Query(queryVector, topK*3, 0.6)

		// Aggregate results
		for _, result := range results {
			// Skip current task
			if result.TaskID == taskID {
				continue
			}

			key := fmt.Sprintf("%s:%s:%s", result.TaskID, result.DocType, result.SectionID)

			// Keep the highest similarity for each recommendation
			if existing, ok := resultMap[key]; !ok || result.Similarity > existing.Result.Similarity {
				resultMap[key] = &AggregatedResult{
					Result:          result,
					SourceSectionID: secQuery.SectionID,
					SourceTitle:     secQuery.Title,
				}
			}
		}
	}

	// 打印统计摘要(一次性)
	log.Printf("[INFO] GetRecommendations: processed %d sections (cached: %d, NLP success: %d, NLP fail/skip: %d)",
		len(validSections), cachedCount, nlpSuccessCount, nlpFailCount)

	// 4. Convert map to slice and sort by similarity
	var aggregatedResults []*AggregatedResult
	for _, aggResult := range resultMap {
		aggregatedResults = append(aggregatedResults, aggResult)
	}

	sort.Slice(aggregatedResults, func(i, j int) bool {
		return aggregatedResults[i].Result.Similarity > aggregatedResults[j].Result.Similarity
	})

	// 5. Take top-K results and fill snippets
	if len(aggregatedResults) > topK {
		aggregatedResults = aggregatedResults[:topK]
	}

	var finalResults []RecommendationResult
	for _, aggResult := range aggregatedResults {
		result := aggResult.Result

		// Get section content for snippet
		section, _ := s.sectionSvc.GetSection(projectID, result.TaskID, result.DocType, result.SectionID, false)
		if section != nil {
			result.Snippet = truncateText(section.Content, 50)
		}

		// 填充源章节信息(用于前端对比展示)
		result.SourceSectionID = aggResult.SourceSectionID
		result.SourceTitle = aggResult.SourceTitle

		// Append source info to title (which section of current task matched)
		result.Title = fmt.Sprintf("%s (匹配: %s)", result.Title, aggResult.SourceTitle)

		finalResults = append(finalResults, result)
	}

	return finalResults, nil
}

// GetRecommendationsByQuery gets recommendations based on query text
func (s *SimilarityService) GetRecommendationsByQuery(
	ctx context.Context,
	projectID string,
	queryText string,
	docType string,
	topK int,
	threshold float64,
	excludeTaskID string,
) ([]RecommendationResult, error) {
	// 1. Validate input
	if queryText == "" {
		return []RecommendationResult{}, nil
	}

	// Limit query text length
	runes := []rune(queryText)
	if len(runes) > 1000 {
		queryText = string(runes[:1000])
	}

	// 2. Vectorize query text
	queryVectors, err := s.nlpClient.Embed(ctx, []string{queryText})
	if err != nil {
		return nil, fmt.Errorf("failed to embed query text: %w", err)
	}

	// 3. Query similar vectors
	results := s.indexMgr.Query(queryVectors[0], topK*2, threshold)

	// 4. Filter results
	var filteredResults []RecommendationResult
	for _, result := range results {
		// Exclude specified task
		if excludeTaskID != "" && result.TaskID == excludeTaskID {
			continue
		}

		// Filter by doc type if specified
		if docType != "" && result.DocType != docType {
			continue
		}

		// Fill snippet
		section, _ := s.sectionSvc.GetSection(projectID, result.TaskID, result.DocType, result.SectionID, false)
		if section != nil {
			result.Snippet = truncateText(section.Content, 50)
		}

		filteredResults = append(filteredResults, result)

		if len(filteredResults) >= topK {
			break
		}
	}

	return filteredResults, nil
}

// VectorizeDocument asynchronously vectorizes document sections
func (s *SimilarityService) VectorizeDocument(
	ctx context.Context,
	projectID, taskID, docType string,
) error {
	go func() {
		// 1. Get all sections
		sections, err := s.sectionSvc.GetSections(projectID, taskID, docType)
		if err != nil {
			log.Printf("[ERROR] VectorizeDocument: failed to get sections: %v", err)
			return
		}

		var texts []string
		var sectionMetas []*VectorEntry

		// 2. Collect section contents
		for _, secMeta := range sections.Sections {
			sec, err := s.sectionSvc.GetSection(projectID, taskID, docType, secMeta.ID, false)
			if err != nil {
				log.Printf("[WARN] VectorizeDocument: skip section %s: %v", secMeta.ID, err)
				continue
			}

			// Skip empty or too short sections (< 10 characters)
			// This prevents meaningless vectorization and improves recommendation quality
			contentRunes := []rune(sec.Content)
			if len(contentRunes) < 10 {
				log.Printf("[INFO] VectorizeDocument: skip section %s (content too short: %d chars)", secMeta.ID, len(contentRunes))
				continue
			}

			texts = append(texts, sec.Content)
			sectionMetas = append(sectionMetas, &VectorEntry{
				ProjectID: projectID,
				TaskID:    taskID,
				DocType:   docType,
				SectionID: secMeta.ID,
				Title:     sec.Title,
			})
		}

		if len(texts) == 0 {
			log.Printf("[WARN] VectorizeDocument: no sections to vectorize for task %s", taskID)
			return
		}

		// 3. Batch vectorize (max 100 sections)
		if len(texts) > 100 {
			log.Printf("[WARN] VectorizeDocument: truncating to 100 sections (found %d)", len(texts))
			texts = texts[:100]
			sectionMetas = sectionMetas[:100]
		}

		vectors, err := s.nlpClient.Embed(ctx, texts)
		if err != nil {
			log.Printf("[ERROR] VectorizeDocument: embedding failed: %v", err)
			return
		}

		// 4. Update index
		now := time.Now().Format(time.RFC3339)
		for i, vec := range vectors {
			sectionMetas[i].Vector = vec
			sectionMetas[i].UpdatedAt = now
		}

		s.indexMgr.Update(sectionMetas)

		// 5. Async persist
		if err := s.indexMgr.Save(); err != nil {
			log.Printf("[ERROR] VectorizeDocument: failed to save index: %v", err)
			return
		}

		log.Printf("[INFO] VectorizeDocument: successfully vectorized %d sections for task %s", len(sectionMetas), taskID)
	}()

	return nil
}

// truncateText truncates text to max length
func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}
