// pkg/similarity/task_queue.go
package similarity

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// VectorCalculationQueue 向量计算任务队列（防积压）
// 关键设计：同一文档仅保留最新一次计算任务
type VectorCalculationQueue struct {
	mu           sync.RWMutex
	pendingTasks map[string]*CalculationTask // docID → 最新任务
	worker       chan *CalculationTask
	nlpClient    NLPClientInterface // 抽象接口便于测试
	indexMgr     *VectorIndexManager
	ctx          context.Context
	cancel       context.CancelFunc
}

// CalculationTask 向量计算任务
type CalculationTask struct {
	DocID       string // projectID:taskID:docType
	ProjectID   string
	TaskID      string
	DocType     string
	Sections    []string // 章节内容列表
	SubmittedAt time.Time
	Version     int // 内容版本号（防止旧版本覆盖新版本）
}

// NLPClientInterface NLP客户端接口（便于mock测试）
type NLPClientInterface interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// NewVectorCalculationQueue 创建任务队列
func NewVectorCalculationQueue(
	nlpClient NLPClientInterface,
	indexMgr *VectorIndexManager,
	numWorkers int,
) *VectorCalculationQueue {
	ctx, cancel := context.WithCancel(context.Background())

	q := &VectorCalculationQueue{
		pendingTasks: make(map[string]*CalculationTask),
		worker:       make(chan *CalculationTask, 100), // 缓冲区100
		nlpClient:    nlpClient,
		indexMgr:     indexMgr,
		ctx:          ctx,
		cancel:       cancel,
	}

	// 启动worker池
	q.startWorkers(numWorkers)

	return q
}

// SubmitTask 提交计算任务（覆盖旧任务）
func (q *VectorCalculationQueue) SubmitTask(task *CalculationTask) {
	q.mu.Lock()
	defer q.mu.Unlock()

	docID := fmt.Sprintf("%s:%s:%s", task.ProjectID, task.TaskID, task.DocType)
	task.DocID = docID

	// 检查是否已有待处理任务
	if existingTask, exists := q.pendingTasks[docID]; exists {
		log.Printf("[VectorQueue] 覆盖旧任务: docID=%s, oldVersion=%d, newVersion=%d",
			docID, existingTask.Version, task.Version)
	}

	// 只保留最新版本的任务
	q.pendingTasks[docID] = task

	// 非阻塞发送到worker（如果队列满则跳过，下次定时器会处理）
	select {
	case q.worker <- task:
		delete(q.pendingTasks, docID) // 成功发送后移除
		log.Printf("[VectorQueue] 任务已发送到worker: docID=%s, version=%d", docID, task.Version)
	default:
		log.Printf("[VectorQueue] Worker忙碌，任务将在下次处理: docID=%s", docID)
	}
}

// startWorkers 启动后台worker处理任务
func (q *VectorCalculationQueue) startWorkers(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			for {
				select {
				case task := <-q.worker:
					q.processTask(workerID, task)

				case <-q.ctx.Done():
					log.Printf("[Worker-%d] 收到停止信号，退出", workerID)
					return
				}
			}
		}(i)
	}

	// 启动定时器：每30秒处理一次pending任务（防止任务丢失）
	go q.processPendingTasks()
}

// processTask 处理单个任务
func (q *VectorCalculationQueue) processTask(workerID int, task *CalculationTask) {
	startTime := time.Now()
	log.Printf("[Worker-%d] 开始处理向量计算: docID=%s, version=%d, sections=%d",
		workerID, task.DocID, task.Version, len(task.Sections))

	// 调用NLP服务批量向量化
	vectors, err := q.nlpClient.Embed(q.ctx, task.Sections)
	if err != nil {
		log.Printf("[Worker-%d] 向量计算失败: docID=%s, error=%v", workerID, task.DocID, err)
		// TODO: 实现重试机制（指数退避）
		return
	}

	// 验证返回的向量数量
	if len(vectors) != len(task.Sections) {
		log.Printf("[Worker-%d] 向量数量不匹配: docID=%s, expected=%d, got=%d",
			workerID, task.DocID, len(task.Sections), len(vectors))
		return
	}

	// 构建VectorEntry列表
	entries := make([]*VectorEntry, len(vectors))
	for i, vec := range vectors {
		entries[i] = &VectorEntry{
			ProjectID: task.ProjectID,
			TaskID:    task.TaskID,
			DocType:   task.DocType,
			SectionID: fmt.Sprintf("section_%03d", i), // TODO: 从真实章节ID获取
			Title:     fmt.Sprintf("章节 %d", i+1),
			Vector:    vec,
			UpdatedAt: time.Now().Format(time.RFC3339),
		}
	}

	// 更新向量索引
	q.indexMgr.Update(entries)

	// 异步持久化
	if err := q.indexMgr.Save(); err != nil {
		log.Printf("[Worker-%d] 索引持久化失败: docID=%s, error=%v", workerID, task.DocID, err)
	}

	latency := time.Since(startTime)
	log.Printf("[Worker-%d] 向量计算完成: docID=%s, sections=%d, latency=%s",
		workerID, task.DocID, len(task.Sections), latency)
}

// processPendingTasks 定期处理pending任务（防止任务丢失）
func (q *VectorCalculationQueue) processPendingTasks() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			q.mu.Lock()
			pendingCount := len(q.pendingTasks)
			if pendingCount > 0 {
				log.Printf("[VectorQueue] 处理pending任务: count=%d", pendingCount)
				for docID, task := range q.pendingTasks {
					select {
					case q.worker <- task:
						delete(q.pendingTasks, docID)
						log.Printf("[VectorQueue] Pending任务已发送: docID=%s", docID)
					default:
						log.Printf("[VectorQueue] Worker仍然忙碌，下次再试: docID=%s", docID)
					}
				}
			}
			q.mu.Unlock()

		case <-q.ctx.Done():
			log.Printf("[VectorQueue] 定时器收到停止信号，退出")
			return
		}
	}
}

// Stop 停止任务队列
func (q *VectorCalculationQueue) Stop() {
	log.Printf("[VectorQueue] 停止任务队列...")
	q.cancel()
	close(q.worker)
}

// PendingCount 返回当前pending任务数（用于监控）
func (q *VectorCalculationQueue) PendingCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.pendingTasks)
}
