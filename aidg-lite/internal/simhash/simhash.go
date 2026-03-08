package simhash

import (
	"strings"

	"github.com/go-dedup/simhash"
)

// SIMHASH_THRESHOLD 定义相似度阈值：汉明距离<=28视为相似

const SIMHASH_THRESHOLD = 28

// TaskFeatureSet 实现 simhash.FeatureSet 接口，用于任务名称的特征提取
type TaskFeatureSet struct {
	text string
}

// GetFeatures 提取文本特征
// 使用字符级bigram特征，适合中文短文本语义捕捉
func (t TaskFeatureSet) GetFeatures() []simhash.Feature {
	text := strings.TrimSpace(t.text)
	if text == "" {
		return []simhash.Feature{}
	}

	features := make([]simhash.Feature, 0)
	runes := []rune(text)

	// 使用字符级bigram特征（滑动窗口大小=2）
	// 这种方式对中文短文本效果更好，能更好地捕捉语义相似性
	for i := 0; i < len(runes)-1; i++ {
		// 跳过标点符号
		r1, r2 := runes[i], runes[i+1]
		if isPunctuation(r1) || isPunctuation(r2) {
			continue
		}
		bigram := string([]rune{r1, r2})
		features = append(features, simhash.NewFeature([]byte(bigram)))
	}

	// 如果文本很短（<4个字符），添加单字符特征增强区分度
	if len(runes) < 4 {
		for _, r := range runes {
			if !isPunctuation(r) {
				features = append(features, simhash.NewFeature([]byte(string(r))))
			}
		}
	}

	return features
}

// isPunctuation 判断是否为标点符号
func isPunctuation(r rune) bool {
	return r == ' ' || r == ',' || r == '.' || r == '!' || r == '?' ||
		r == '：' || r == '、' || r == '。' || r == '，' || r == '；' ||
		r == '！' || r == '？' || r == '-' || r == '_' || r == '/' ||
		r == '（' || r == '）' || r == '(' || r == ')' || r == '\t' || r == '\n'
}

// CalculateSimHash 计算文本的 SimHash 指纹
// 参数:
//   - text: 任务名称或搜索关键词
//
// 返回:
//   - uint64: 64位SimHash指纹值
func CalculateSimHash(text string) uint64 {
	sh := simhash.NewSimhash()
	featureSet := TaskFeatureSet{text: text}
	return sh.GetSimhash(featureSet)
}

// HammingDistance 计算两个 SimHash 指纹的汉明距离
// 汉明距离表示两个64位数字中不同位的数量
// 参数:
//   - hash1: 第一个SimHash指纹
//   - hash2: 第二个SimHash指纹
//
// 返回:
//   - int: 汉明距离（0-64）
func HammingDistance(hash1, hash2 uint64) int {
	// XOR 操作：相同位为0，不同位为1
	x := hash1 ^ hash2
	count := 0

	// 计算1的个数（Brian Kernighan算法）
	for x != 0 {
		count++
		x &= x - 1 // 清除最右边的1
	}

	return count
}

// IsSimilar 判断两个文本是否相似
// 参数:
//   - text1: 第一个文本
//   - text2: 第二个文本
//
// 返回:
//   - bool: 是否相似（汉明距离 <= SIMHASH_THRESHOLD）
func IsSimilar(text1, text2 string) bool {
	hash1 := CalculateSimHash(text1)
	hash2 := CalculateSimHash(text2)
	distance := HammingDistance(hash1, hash2)
	return distance <= SIMHASH_THRESHOLD
}

// SearchSimilarTasks 在任务列表中搜索与查询词相似的任务
// 参数:
//   - query: 搜索关键词
//   - taskNames: 任务名称列表
//   - taskHashes: 任务SimHash指纹列表（可选，为nil时自动计算）
//
// 返回:
//   - []int: 相似任务的索引列表（按相似度排序，汉明距离从小到大）
func SearchSimilarTasks(query string, taskNames []string, taskHashes []uint64) []int {
	queryHash := CalculateSimHash(query)

	// 如果未提供预计算的hash，则实时计算
	if taskHashes == nil {
		taskHashes = make([]uint64, len(taskNames))
		for i, name := range taskNames {
			taskHashes[i] = CalculateSimHash(name)
		}
	}

	// 存储匹配结果：索引和距离
	type match struct {
		index    int
		distance int
	}
	matches := []match{}

	// 计算每个任务的相似度
	for i, hash := range taskHashes {
		distance := HammingDistance(queryHash, hash)
		if distance <= SIMHASH_THRESHOLD {
			matches = append(matches, match{index: i, distance: distance})
		}
	}

	// 按距离排序（距离小的更相似，排在前面）
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].distance > matches[j].distance {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	// 提取索引
	result := make([]int, len(matches))
	for i, m := range matches {
		result[i] = m.index
	}

	return result
}
