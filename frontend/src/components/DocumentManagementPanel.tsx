import React, { useState, useEffect, useCallback } from 'react'
import { 
  Tabs, 
  Input, 
  Button, 
  List, 
  Card, 
  Select, 
  Spin, 
  Typography, 
  Badge, 
  Space, 
  Divider,
  AutoComplete,
  Row,
  Col
} from 'antd'
import { 
  SearchOutlined, 
  HistoryOutlined, 
  BranchesOutlined, 
  FileTextOutlined,
  ClockCircleOutlined,
  DiffOutlined
} from '@ant-design/icons'
import { 
  SearchResult, 
  SearchOptions, 
  DiffResult, 
  ImpactResult, 
  SnapshotMeta,
  AnalysisMode,
  documentsAPI 
} from '../api/documents'
import DocumentTreeView from './DocumentTreeView'

const { TabPane } = Tabs
const { Text, Title, Paragraph } = Typography
const { Option } = Select

interface DocumentManagementPanelProps {
  projectId: string
  taskId?: string // 添加 taskId 用于防护机制
  selectedDocumentId?: string
  onDocumentSelect?: (documentId: string) => void
}

type PanelTab = 'tree' | 'search' | 'versions' | 'impact'

const DocumentManagementPanel: React.FC<DocumentManagementPanelProps> = ({
  projectId,
  taskId,
  selectedDocumentId,
  onDocumentSelect
}) => {
  const [activeTab, setActiveTab] = useState<PanelTab>('tree')
  const [searchQuery, setSearchQuery] = useState('')
  const [searchResults, setSearchResults] = useState<SearchResult[]>([])
  const [searchSuggestions, setSuggestions] = useState<string[]>([])
  const [searchLoading, setSearchLoading] = useState(false)
  const [versions, setVersions] = useState<SnapshotMeta[]>([])
  const [diffResult, setDiffResult] = useState<DiffResult | null>(null)
  const [impactResult, setImpactResult] = useState<ImpactResult | null>(null)
  const [selectedVersions, setSelectedVersions] = useState<{ from: number; to: number }>({ from: 1, to: 1 })

  // 搜索功能
  const handleSearch = useCallback(async () => {
    if (!searchQuery.trim()) {
      setSearchResults([])
      return
    }

    setSearchLoading(true)
    try {
      const options: SearchOptions = {
        query: searchQuery,
        max_results: 20,
        context_chars: 150
      }
      
      const response = await documentsAPI.searchDocuments(projectId, options)
      setSearchResults(response.results)
    } catch (error) {
      console.error('Search failed:', error)
      setSearchResults([])
    } finally {
      setSearchLoading(false)
    }
  }, [projectId, searchQuery])

  // 获取搜索建议
  const fetchSuggestions = useCallback(async (query: string) => {
    if (query.length < 2) {
      setSuggestions([])
      return
    }

    try {
      const response = await documentsAPI.getSearchSuggestions(projectId, query, 10)
      setSuggestions(response.suggestions)
    } catch (error) {
      console.error('Failed to fetch suggestions:', error)
      setSuggestions([])
    }
  }, [projectId])

  // 加载版本历史
  const loadVersions = useCallback(async (docId: string) => {
    try {
      const response = await documentsAPI.getVersionHistory(projectId, docId)
      setVersions(response.versions)
      
      // 设置默认选择的版本
      if (response.versions.length >= 2) {
        setSelectedVersions({
          from: response.versions[response.versions.length - 2].version,
          to: response.versions[response.versions.length - 1].version
        })
      }
    } catch (error) {
      console.error('Failed to load versions:', error)
      setVersions([])
    }
  }, [projectId])

  // 执行版本比较
  const compareDiff = useCallback(async (docId: string, from: number, to: number) => {
    try {
      const response = await documentsAPI.compareVersions(projectId, docId, from, to)
      setDiffResult(response.diff)
    } catch (error) {
      console.error('Failed to compare versions:', error)
      setDiffResult(null)
    }
  }, [projectId])

  // 分析影响
  const analyzeImpact = useCallback(async (docId: string, modes?: AnalysisMode[]) => {
    try {
      const response = await documentsAPI.analyzeImpact(projectId, docId, modes)
      setImpactResult(response.impact)
    } catch (error) {
      console.error('Failed to analyze impact:', error)
      setImpactResult(null)
    }
  }, [projectId])

  // 监听选中文档变化
  useEffect(() => {
    if (selectedDocumentId) {
      loadVersions(selectedDocumentId)
      analyzeImpact(selectedDocumentId, ['all'])
    }
  }, [selectedDocumentId, loadVersions, analyzeImpact])

  // 监听搜索查询变化
  useEffect(() => {
    const timer = setTimeout(() => {
      fetchSuggestions(searchQuery)
    }, 300)

    return () => clearTimeout(timer)
  }, [searchQuery, fetchSuggestions])

  // 高亮搜索匹配
  const highlightMatches = (text: string, matches: Array<{ start: number; end: number; text: string }>) => {
    if (!matches || matches.length === 0) {
      return <span>{text}</span>
    }

    const parts = []
    let lastEnd = 0

    matches.forEach((match, index) => {
      // 添加匹配前的文本
      if (match.start > lastEnd) {
        parts.push(<span key={`before-${index}`}>{text.slice(lastEnd, match.start)}</span>)
      }
      
      // 添加高亮的匹配文本
      parts.push(
        <span key={`match-${index}`} style={{ backgroundColor: '#fff2e6', fontWeight: 'bold' }}>
          {match.text}
        </span>
      )
      
      lastEnd = match.end
    })

    // 添加最后的文本
    if (lastEnd < text.length) {
      parts.push(<span key="after">{text.slice(lastEnd)}</span>)
    }

    return <span>{parts}</span>
  }

  const renderSearchTab = () => (
    <div style={{ padding: '16px' }}>
      {/* 搜索输入框 */}
      <AutoComplete
        value={searchQuery}
        onChange={setSearchQuery}
        onSelect={setSearchQuery}
        options={searchSuggestions.map(suggestion => ({ value: suggestion }))}
        style={{ width: '100%', marginBottom: '16px' }}
        placeholder="搜索文档标题和内容..."
      >
        <Input.Search
          enterButton={<SearchOutlined />}
          loading={searchLoading}
          onSearch={handleSearch}
        />
      </AutoComplete>

      {/* 搜索结果 */}
      {searchResults.length > 0 && (
        <Card size="small" title={`搜索结果 (${searchResults.length})`}>
          <List
            itemLayout="vertical"
            dataSource={searchResults}
            renderItem={(result) => (
              <List.Item
                style={{ cursor: 'pointer' }}
                onClick={() => onDocumentSelect?.(result.document_id)}
                actions={[
                  <Space key="score">
                    <Badge count={Math.round(result.score * 100)} />
                    <Text type="secondary">评分</Text>
                  </Space>,
                  <Space key="time">
                    <ClockCircleOutlined />
                    <Text type="secondary">{new Date(result.created_at).toLocaleDateString()}</Text>
                  </Space>
                ]}
              >
                <List.Item.Meta
                  title={
                    <Text strong style={{ color: '#1890ff' }}>
                      {highlightMatches(result.title, result.title_matches)}
                    </Text>
                  }
                  description={
                    result.content_matches.length > 0 && (
                      <div>
                        {result.content_matches.slice(0, 2).map((match: any, index: number) => (
                          <Paragraph key={index} ellipsis={{ rows: 1 }}>
                            <Text type="secondary">...{match.before}</Text>
                            <Text mark>{match.text}</Text>
                            <Text type="secondary">{match.after}...</Text>
                          </Paragraph>
                        ))}
                      </div>
                    )
                  }
                />
              </List.Item>
            )}
          />
        </Card>
      )}
    </div>
  )

  const renderTreeTab = () => (
    <Card 
      size="small"
      style={{ height: '100%' }}
      bodyStyle={{ height: 'calc(100% - 58px)', padding: '12px' }}
    >
      <DocumentTreeView 
        projectId={projectId}
        taskId={taskId}
        selectedDocumentId={selectedDocumentId}
        onDocumentSelect={onDocumentSelect}
      />
    </Card>
  )

  const renderVersionsTab = () => (
    <div style={{ padding: '16px' }}>
      {selectedDocumentId ? (
        <>
          {/* 版本选择 */}
          <Card title={<Space><HistoryOutlined />版本对比</Space>} size="small" style={{ marginBottom: '16px' }}>
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Text strong>对比版本 (从):</Text>
                <Select
                  value={selectedVersions.from}
                  onChange={(value) => setSelectedVersions(prev => ({ ...prev, from: value }))}
                  style={{ width: '100%', marginTop: '8px' }}
                >
                  {versions.map(v => (
                    <Option key={v.version} value={v.version}>
                      v{v.version} - {new Date(v.created_at).toLocaleString()}
                    </Option>
                  ))}
                </Select>
              </Col>
              <Col span={12}>
                <Text strong>对比版本 (到):</Text>
                <Select
                  value={selectedVersions.to}
                  onChange={(value) => setSelectedVersions(prev => ({ ...prev, to: value }))}
                  style={{ width: '100%', marginTop: '8px' }}
                >
                  {versions.map(v => (
                    <Option key={v.version} value={v.version}>
                      v{v.version} - {new Date(v.created_at).toLocaleString()}
                    </Option>
                  ))}
                </Select>
              </Col>
            </Row>
            <Button 
              type="primary"
              icon={<DiffOutlined />}
              onClick={() => compareDiff(selectedDocumentId, selectedVersions.from, selectedVersions.to)}
              style={{ marginTop: '16px' }}
            >
              比较版本差异
            </Button>
          </Card>

          {/* 差异结果 */}
          {diffResult && (
            <Card title="版本差异摘要" size="small">
              <Row gutter={16} style={{ marginBottom: '16px' }}>
                <Col span={8}>
                  <Badge count={`+${diffResult.summary.added}`} style={{ backgroundColor: '#52c41a' }}>
                    <div style={{ padding: '8px 12px', backgroundColor: '#f6ffed', border: '1px solid #b7eb8f', borderRadius: '4px' }}>
                      <Text>新增行</Text>
                    </div>
                  </Badge>
                </Col>
                <Col span={8}>
                  <Badge count={`-${diffResult.summary.deleted}`} style={{ backgroundColor: '#ff4d4f' }}>
                    <div style={{ padding: '8px 12px', backgroundColor: '#fff2f0', border: '1px solid #ffccc7', borderRadius: '4px' }}>
                      <Text>删除行</Text>
                    </div>
                  </Badge>
                </Col>
                <Col span={8}>
                  <Badge count={`~${diffResult.summary.modified}`} style={{ backgroundColor: '#1890ff' }}>
                    <div style={{ padding: '8px 12px', backgroundColor: '#f0f5ff', border: '1px solid #adc6ff', borderRadius: '4px' }}>
                      <Text>修改行</Text>
                    </div>
                  </Badge>
                </Col>
              </Row>

              <div style={{ maxHeight: '400px', overflowY: 'auto', fontFamily: 'monospace', fontSize: '12px' }}>
                {diffResult.lines.map((line, index) => (
                  <div
                    key={index}
                    style={{
                      padding: '2px 8px',
                      backgroundColor: 
                        line.type === 'add' ? '#f6ffed' :
                        line.type === 'delete' ? '#fff2f0' :
                        line.type === 'modify' ? '#f0f5ff' :
                        '#fafafa',
                      color:
                        line.type === 'add' ? '#389e0d' :
                        line.type === 'delete' ? '#cf1322' :
                        line.type === 'modify' ? '#096dd9' :
                        '#595959',
                      borderLeft: `3px solid ${
                        line.type === 'add' ? '#52c41a' :
                        line.type === 'delete' ? '#ff4d4f' :
                        line.type === 'modify' ? '#1890ff' :
                        '#d9d9d9'
                      }`
                    }}
                  >
                    <Text type="secondary" style={{ marginRight: '8px' }}>{line.line_num}:</Text>
                    {line.content}
                  </div>
                ))}
              </div>
            </Card>
          )}
        </>
      ) : (
        <div style={{ textAlign: 'center', padding: '32px' }}>
          <Text type="secondary">请选择一个文档查看版本历史</Text>
        </div>
      )}
    </div>
  )

  const renderImpactTab = () => (
    <div style={{ padding: '16px' }}>
      {selectedDocumentId ? (
        impactResult ? (
          <Card title={<Space><BranchesOutlined />影响分析结果</Space>} size="small">
            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Card size="small" title="父文档" bordered={false} style={{ backgroundColor: '#f0f5ff' }}>
                  <List
                    size="small"
                    dataSource={impactResult.parents.slice(0, 5)}
                    renderItem={id => (
                      <List.Item>
                        <Button type="link" onClick={() => onDocumentSelect?.(id)}>
                          {id}
                        </Button>
                      </List.Item>
                    )}
                  />
                  {impactResult.parents.length > 5 && (
                    <Text type="secondary">...还有 {impactResult.parents.length - 5} 个</Text>
                  )}
                </Card>
              </Col>
              <Col span={12}>
                <Card size="small" title="子文档" bordered={false} style={{ backgroundColor: '#f6ffed' }}>
                  <List
                    size="small"
                    dataSource={impactResult.children.slice(0, 5)}
                    renderItem={id => (
                      <List.Item>
                        <Button type="link" onClick={() => onDocumentSelect?.(id)}>
                          {id}
                        </Button>
                      </List.Item>
                    )}
                  />
                  {impactResult.children.length > 5 && (
                    <Text type="secondary">...还有 {impactResult.children.length - 5} 个</Text>
                  )}
                </Card>
              </Col>
            </Row>
          </Card>
        ) : (
          <div style={{ textAlign: 'center', padding: '32px' }}>
            <Button 
              type="primary"
              icon={<BranchesOutlined />}
              onClick={() => analyzeImpact(selectedDocumentId)}
            >
              分析影响范围
            </Button>
          </div>
        )
      ) : (
        <div style={{ textAlign: 'center', padding: '32px' }}>
          <Text type="secondary">请选择一个文档查看影响分析</Text>
        </div>
      )}
    </div>
  )

  return (
    <Tabs
      activeKey={activeTab}
      onChange={(key) => setActiveTab(key as PanelTab)}
      style={{ height: '100%' }}
      tabBarStyle={{ marginBottom: 0 }}
    >
      <TabPane 
        tab={<Space><FileTextOutlined />文档树</Space>} 
        key="tree"
      >
        {renderTreeTab()}
      </TabPane>
      
      <TabPane 
        tab={<Space><SearchOutlined />搜索</Space>} 
        key="search"
      >
        {renderSearchTab()}
      </TabPane>
      
      <TabPane 
        tab={<Space><HistoryOutlined />版本</Space>} 
        key="versions"
      >
        {renderVersionsTab()}
      </TabPane>
      
      <TabPane 
        tab={<Space><BranchesOutlined />影响</Space>} 
        key="impact"
      >
        {renderImpactTab()}
      </TabPane>
    </Tabs>
  )
}

export default DocumentManagementPanel