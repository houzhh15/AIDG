import React, { useState, useEffect } from 'react'
import documentsAPI, { Reference, ReferenceStatus } from '../api/documents'
import { getCurrentTask } from '../api/currentTask'

interface ReferencesPanelProps {
  projectId?: string
  taskId?: string
  docId?: string
  className?: string
}

const StatusBadge: React.FC<{ status: ReferenceStatus }> = ({ status }) => {
  const colors = {
    active: 'bg-green-100 text-green-800',
    outdated: 'bg-yellow-100 text-yellow-800', 
    broken: 'bg-red-100 text-red-800'
  }
  
  const labels = {
    active: '有效',
    outdated: '过期',
    broken: '失效'
  }
  
  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium ${colors[status]}`}>
      {labels[status]}
    </span>
  )
}

const ReferencesPanel: React.FC<ReferencesPanelProps> = ({ 
  projectId: propProjectId, 
  taskId: propTaskId, 
  docId, 
  className = '' 
}) => {
  const [references, setReferences] = useState<Reference[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [currentTask, setCurrentTask] = useState<any>(null)

  // 获取当前任务信息（如果未提供projectId/taskId）
  useEffect(() => {
    const fetchCurrentTask = async () => {
      try {
        const task = await getCurrentTask()
        setCurrentTask(task)
      } catch (err) {
        console.error('Failed to get current task:', err)
      }
    }

    if (!propProjectId || !propTaskId) {
      fetchCurrentTask()
    }
  }, [propProjectId, propTaskId])

  const projectId = propProjectId || currentTask?.project_id
  const taskId = propTaskId || currentTask?.task_id

  // 获取引用数据
  useEffect(() => {
    const fetchReferences = async () => {
      if (!projectId) return

      setLoading(true)
      setError(null)
      
      try {
        let result
        if (docId) {
          // 获取文档的引用
          result = await documentsAPI.getDocumentReferences(projectId, docId)
        } else if (taskId) {
          // 获取任务的引用
          result = await documentsAPI.getTaskReferences(projectId, taskId)
        } else {
          setReferences([])
          return
        }
        
        setReferences(result.references || [])
      } catch (err) {
        console.error('Failed to fetch references:', err)
        setError('获取引用失败')
      } finally {
        setLoading(false)
      }
    }

    fetchReferences()
  }, [projectId, taskId, docId])

  // 更新引用状态
  const handleStatusUpdate = async (referenceId: string, status: ReferenceStatus) => {
    if (!projectId) return

    try {
      await documentsAPI.updateReferenceStatus(projectId, referenceId, status)
      
      // 更新本地状态
      setReferences(refs => refs.map(ref => 
        ref.id === referenceId ? { ...ref, status } : ref
      ))
    } catch (err) {
      console.error('Failed to update reference status:', err)
      setError('更新状态失败')
    }
  }

  // 跳转到文档锚点
  const handleJumpToAnchor = (documentId: string, anchor?: string | null) => {
    if (!anchor || !anchor.trim()) {
      console.warn('缺少锚点信息，无法跳转', { documentId })
      return
    }
    // TODO: 实现跳转逻辑，可能需要与文档编辑器集成
    console.log('Jump to:', documentId, anchor)
    
    // 简单的滚动到锚点实现
    const element = document.getElementById(anchor)
    if (element) {
      element.scrollIntoView({ behavior: 'smooth' })
    }
  }

  if (loading) {
    return (
      <div className={`p-4 ${className}`}>
        <div className="flex items-center justify-center">
          <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500"></div>
          <span className="ml-2 text-sm text-gray-600">加载中...</span>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className={`p-4 ${className}`}>
        <div className="text-sm text-red-600">{error}</div>
      </div>
    )
  }

  return (
    <div className={`bg-white rounded-lg shadow-sm border ${className}`}>
      <div className="p-4 border-b border-gray-200">
        <h3 className="text-lg font-medium text-gray-900">
          {docId ? '文档引用' : '任务引用'}
        </h3>
        <p className="text-sm text-gray-500 mt-1">
          {references.length} 个引用
        </p>
      </div>

      <div className="divide-y divide-gray-200">
        {references.length === 0 ? (
          <div className="p-4 text-center text-gray-500 text-sm">
            暂无引用
          </div>
        ) : (
          references.map((ref) => (
            <div key={ref.id} className="p-4 hover:bg-gray-50">
              <div className="flex items-start justify-between">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center space-x-2 mb-2">
                    <StatusBadge status={ref.status} />
                    <span className="text-xs text-gray-500">
                      版本 {ref.version}
                    </span>
                    <span className="text-xs text-gray-500">
                      {new Date(ref.created_at).toLocaleDateString()}
                    </span>
                  </div>
                  
                  <div className="text-sm font-medium text-gray-900 mb-1">
                    锚点: {ref.anchor && ref.anchor.trim() ? ref.anchor : '未设置'}
                  </div>
                  
                  {ref.context && ref.context.trim() ? (
                    <div className="text-sm text-gray-600 bg-gray-50 p-2 rounded border-l-4 border-blue-200">
                      {ref.context}
                    </div>
                  ) : (
                    <div className="text-xs text-gray-400 mt-1">未填写引用上下文</div>
                  )}
                  
                  <div className="text-xs text-gray-500 mt-2">
                    {docId ? `任务: ${ref.task_id}` : `文档: ${ref.document_id}`}
                  </div>
                </div>

                <div className="flex flex-col space-y-1 ml-4">
                  <button
                    onClick={() => handleJumpToAnchor(ref.document_id, ref.anchor)}
                    className={`text-xs px-2 py-1 rounded ${ref.anchor && ref.anchor.trim() ? 'bg-blue-100 text-blue-700 hover:bg-blue-200' : 'bg-gray-100 text-gray-400 cursor-not-allowed'}`}
                    title={ref.anchor && ref.anchor.trim() ? '跳转到引用位置' : '当前引用未设置锚点'}
                    disabled={!ref.anchor || !ref.anchor.trim()}
                  >
                    跳转
                  </button>
                  
                  {ref.status !== 'active' && (
                    <button
                      onClick={() => handleStatusUpdate(ref.id, 'active')}
                      className="text-xs bg-green-100 text-green-700 px-2 py-1 rounded hover:bg-green-200"
                      title="标记为有效"
                    >
                      激活
                    </button>
                  )}
                  
                  {ref.status === 'active' && (
                    <button
                      onClick={() => handleStatusUpdate(ref.id, 'outdated')}
                      className="text-xs bg-yellow-100 text-yellow-700 px-2 py-1 rounded hover:bg-yellow-200"
                      title="标记为过期"
                    >
                      过期
                    </button>
                  )}
                </div>
              </div>
            </div>
          ))
        )}
      </div>

      {references.length > 0 && (
        <div className="p-4 border-t border-gray-200 bg-gray-50">
          <div className="flex justify-between items-center text-xs text-gray-500">
            <span>共 {references.length} 个引用</span>
            <span>
              有效: {references.filter(r => r.status === 'active').length} | 
              过期: {references.filter(r => r.status === 'outdated').length} | 
              失效: {references.filter(r => r.status === 'broken').length}
            </span>
          </div>
        </div>
      )}
    </div>
  )
}

export default ReferencesPanel