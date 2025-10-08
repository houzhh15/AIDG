import React, { useState, useEffect, useRef, useCallback } from 'react'
import documentsAPI, { Relationship, DocumentTreeDTO, DocMetaEntry } from '../api/documents'
import { getCurrentTask } from '../api/currentTask'

interface RelationshipVisualizerProps {
  projectId?: string
  nodeId?: string
  className?: string
}

interface VisNode {
  id: string
  title: string
  type: string
  level: number
  x: number
  y: number
}

interface VisEdge {
  from: string
  to: string
  type: string
  label?: string
}

const NODE_COLORS = {
  feature_list: '#3B82F6',   // blue
  architecture: '#10B981',   // green  
  tech_design: '#F59E0B',    // amber
  background: '#8B5CF6'      // purple
}

const NODE_LABELS = {
  feature_list: '特性',
  architecture: '架构', 
  tech_design: '技术',
  background: '背景'
}

const EDGE_COLORS = {
  parent_child: '#6B7280',   // gray
  sibling: '#3B82F6',        // blue
  reference: '#10B981'       // green
}

const RelationshipVisualizer: React.FC<RelationshipVisualizerProps> = ({ 
  projectId: propProjectId, 
  nodeId, 
  className = '' 
}) => {
  const [relationships, setRelationships] = useState<Relationship[]>([])
  const [nodes, setNodes] = useState<VisNode[]>([])
  const [edges, setEdges] = useState<VisEdge[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [currentTask, setCurrentTask] = useState<any>(null)
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const svgRef = useRef<SVGSVGElement>(null)

  // 获取当前任务信息
  useEffect(() => {
    const fetchCurrentTask = async () => {
      try {
        const task = await getCurrentTask()
        setCurrentTask(task)
      } catch (err) {
        console.error('Failed to get current task:', err)
      }
    }

    if (!propProjectId) {
      fetchCurrentTask()
    }
  }, [propProjectId])

  const projectId = propProjectId || currentTask?.project_id

  const normalizeTree = useCallback((tree: DocumentTreeDTO | DocumentTreeDTO[] | undefined): DocumentTreeDTO[] => {
    if (!tree) {
      return []
    }
    const base = Array.isArray(tree) ? tree : [tree]
    if (base.length === 1 && base[0]?.node?.id === 'virtual_root') {
      return base[0].children ?? []
    }
    return base
  }, [])

  const extractNodesFromTree = useCallback((tree: DocumentTreeDTO, level = 0, baseIndex = 0): { nodes: VisNode[]; size: number } => {
    const collected: VisNode[] = []
    const node = tree.node

    const nodeX = level * 200 + 100
    const nodeY = baseIndex * 80 + 60

    collected.push({
      id: node.id,
      title: node.title,
      type: node.type,
      level: node.level,
      x: nodeX,
      y: nodeY
    })

    let offset = 1
    if (tree.children && tree.children.length > 0) {
      tree.children.forEach(child => {
        const result = extractNodesFromTree(child, level + 1, baseIndex + offset)
        collected.push(...result.nodes)
        offset += result.size
      })
    }

    return { nodes: collected, size: offset }
  }, [])

  // 获取文档树和关系数据
  useEffect(() => {
    const fetchData = async () => {
      if (!projectId) return

      setLoading(true)
      setError(null)
      
      try {
        const [treeResult, relResult] = await Promise.all([
          documentsAPI.getTree(projectId, undefined, 3),
          documentsAPI.getRelationships(projectId, nodeId)
        ])

        setRelationships(relResult.relationships || [])

        const roots = normalizeTree(treeResult.tree)
        let aggregatedNodes: VisNode[] = []
        let offset = 0
        roots.forEach(root => {
          const { nodes: rootNodes, size } = extractNodesFromTree(root, 0, offset)
          aggregatedNodes = aggregatedNodes.concat(rootNodes)
          offset += size
        })
        setNodes(aggregatedNodes)
        
        const generatedEdges = generateEdges(relResult.relationships || [])
        setEdges(generatedEdges)
        
      } catch (err) {
        console.error('Failed to fetch data:', err)
        setError('获取数据失败')
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [projectId, nodeId, extractNodesFromTree, normalizeTree])

  // 生成边数据
  const generateEdges = (relationships: Relationship[]): VisEdge[] => {
    return relationships.map(rel => ({
      from: rel.from_id,
      to: rel.to_id,
      type: rel.type,
      label: rel.description
    }))
  }

  // 处理节点点击
  const handleNodeClick = (nodeId: string) => {
    setSelectedNode(selectedNode === nodeId ? null : nodeId)
  }

  // SVG渲染
  const renderGraph = () => {
    if (nodes.length === 0) return null

    const width = Math.max(...nodes.map(n => n.x)) + 150
    const height = Math.max(...nodes.map(n => n.y)) + 100

    return (
      <svg ref={svgRef} width={width} height={height} className="border border-gray-200 rounded">
        {/* 渲染边 */}
        <g>
          {edges.map((edge, index) => {
            const fromNode = nodes.find(n => n.id === edge.from)
            const toNode = nodes.find(n => n.id === edge.to)
            
            if (!fromNode || !toNode) return null

            return (
              <g key={index}>
                <line
                  x1={fromNode.x}
                  y1={fromNode.y}
                  x2={toNode.x}
                  y2={toNode.y}
                  stroke={EDGE_COLORS[edge.type as keyof typeof EDGE_COLORS] || '#6B7280'}
                  strokeWidth="2"
                  markerEnd="url(#arrowhead)"
                />
                {edge.label && (
                  <text
                    x={(fromNode.x + toNode.x) / 2}
                    y={(fromNode.y + toNode.y) / 2 - 5}
                    textAnchor="middle"
                    className="text-xs fill-gray-600"
                  >
                    {edge.label}
                  </text>
                )}
              </g>
            )
          })}
        </g>

        {/* 渲染节点 */}
        <g>
          {nodes.map((node) => (
            <g key={node.id}>
              <circle
                cx={node.x}
                cy={node.y}
                r="24"
                fill={NODE_COLORS[node.type as keyof typeof NODE_COLORS] || '#6B7280'}
                stroke={selectedNode === node.id ? '#EF4444' : '#FFFFFF'}
                strokeWidth={selectedNode === node.id ? '3' : '2'}
                className="cursor-pointer hover:opacity-80"
                onClick={() => handleNodeClick(node.id)}
              />
              <text
                x={node.x}
                y={node.y - 30}
                textAnchor="middle"
                className="text-sm font-medium fill-gray-900 pointer-events-none"
              >
                {node.title.length > 10 ? `${node.title.slice(0, 10)}...` : node.title}
              </text>
              <text
                x={node.x}
                y={node.y + 5}
                textAnchor="middle"
                className="text-xs fill-white font-medium pointer-events-none"
              >
                {NODE_LABELS[node.type as keyof typeof NODE_LABELS] || node.type}
              </text>
            </g>
          ))}
        </g>

        {/* 箭头标记 */}
        <defs>
          <marker
            id="arrowhead"
            markerWidth="10"
            markerHeight="7" 
            refX="9"
            refY="3.5"
            orient="auto"
          >
            <polygon
              points="0 0, 10 3.5, 0 7"
              fill="#6B7280"
            />
          </marker>
        </defs>
      </svg>
    )
  }

  if (loading) {
    return (
      <div className={`p-4 ${className}`}>
        <div className="flex items-center justify-center h-64">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
          <span className="ml-2 text-sm text-gray-600">加载关系图...</span>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className={`p-4 ${className}`}>
        <div className="text-sm text-red-600 text-center h-64 flex items-center justify-center">
          {error}
        </div>
      </div>
    )
  }

  return (
    <div className={`bg-white rounded-lg shadow-sm border ${className}`}>
      <div className="p-4 border-b border-gray-200">
        <h3 className="text-lg font-medium text-gray-900">关系可视化</h3>
        <p className="text-sm text-gray-500 mt-1">
          {nodes.length} 个节点，{edges.length} 个关系
        </p>
      </div>

      <div className="p-4">
        {/* 图例 */}
        <div className="mb-4 flex flex-wrap gap-4 text-xs">
          <div className="flex items-center space-x-1">
            <span className="font-medium">节点类型:</span>
            {Object.entries(NODE_LABELS).map(([type, label]) => (
              <div key={type} className="flex items-center space-x-1">
                <div 
                  className="w-3 h-3 rounded-full"
                  style={{ backgroundColor: NODE_COLORS[type as keyof typeof NODE_COLORS] }}
                ></div>
                <span>{label}</span>
              </div>
            ))}
          </div>
        </div>

        {/* 关系图 */}
        <div className="overflow-auto">
          {nodes.length === 0 ? (
            <div className="text-center text-gray-500 py-8">
              暂无文档节点
            </div>
          ) : (
            renderGraph()
          )}
        </div>

        {/* 选中节点信息 */}
        {selectedNode && (
          <div className="mt-4 p-3 bg-blue-50 rounded border-l-4 border-blue-400">
            <h4 className="font-medium text-blue-900">选中节点详情</h4>
            {(() => {
              const node = nodes.find(n => n.id === selectedNode)
              if (!node) return null
              
              const relatedEdges = edges.filter(e => e.from === selectedNode || e.to === selectedNode)
              
              return (
                <div className="mt-2 text-sm text-blue-800">
                  <div><strong>标题:</strong> {node.title}</div>
                  <div><strong>类型:</strong> {NODE_LABELS[node.type as keyof typeof NODE_LABELS]}</div>
                  <div><strong>层级:</strong> {node.level}</div>
                  <div><strong>关联:</strong> {relatedEdges.length} 个关系</div>
                </div>
              )
            })()}
          </div>
        )}
      </div>
    </div>
  )
}

export default RelationshipVisualizer