import React, { useState, useEffect, useCallback } from 'react'
import { Tree, Button, Modal, Form, Input, Select, Dropdown, Space, message, Spin, Col, Row } from 'antd'
import { 
  PlusOutlined, 
  FolderOutlined, 
  FileTextOutlined, 
  EditOutlined, 
  DeleteOutlined,
  MoreOutlined,
  DragOutlined
} from '@ant-design/icons'
import DocumentContentViewer from './DocumentContentViewer'
import type { DataNode } from 'antd/es/tree'
import { 
  documentsAPI, 
  DocumentTreeDTO, 
  DocMetaEntry, 
  DocumentType,
  CreateNodeRequest 
} from '../api/documents'

interface DocumentTreeViewProps {
  projectId: string
  taskId?: string // 添加 taskId 用于防护机制
  onDocumentSelect?: (documentId: string) => void
  selectedDocumentId?: string
}

interface TreeNodeData extends DataNode {
  node: DocMetaEntry
  children?: TreeNodeData[]
}

const DocumentTreeView: React.FC<DocumentTreeViewProps> = ({
  projectId,
  taskId,
  onDocumentSelect,
  selectedDocumentId
}) => {
  const [treeData, setTreeData] = useState<TreeNodeData[]>([])
  const [loading, setLoading] = useState(false)
  const [createModalVisible, setCreateModalVisible] = useState(false)
  const [editModalVisible, setEditModalVisible] = useState(false)
  const [parentNode, setParentNode] = useState<DocMetaEntry | null>(null)
  const [editingNode, setEditingNode] = useState<DocMetaEntry | null>(null)
  const [expandedKeys, setExpandedKeys] = useState<string[]>([])
  const [selectedKeys, setSelectedKeys] = useState<string[]>([])
  const [selectedNode, setSelectedNode] = useState<DocMetaEntry | null>(null)
  const [showContentViewer, setShowContentViewer] = useState(false)

  const [createForm] = Form.useForm()
  const [editForm] = Form.useForm()

  // 加载文档树
  const loadDocumentTree = useCallback(async () => {
    setLoading(true)
    try {
      const response = await documentsAPI.getTree(projectId)
      console.log('[DEBUG] GetTree response:', response)
      const tree = Array.isArray(response.tree) ? response.tree : (response.tree ? [response.tree] : [])
      console.log('[DEBUG] Processed tree:', tree)
      const treeNodes = convertToTreeData(tree)
      console.log('[DEBUG] Converted tree nodes:', treeNodes)
      setTreeData(treeNodes)
      
      // 自动展开第一层
      const rootKeys = treeNodes.map(node => node.key as string)
      setExpandedKeys(rootKeys)
    } catch (error) {
      console.error('Failed to load document tree:', error)
      message.error('加载文档树失败')
      setTreeData([])
    } finally {
      setLoading(false)
    }
  }, [projectId])

  // 转换为Tree组件需要的数据格式
  const convertToTreeData = (nodes: DocumentTreeDTO[]): TreeNodeData[] => {
    return nodes
      .filter(item => item.node.id !== 'virtual_root') // 过滤虚拟根节点
      .map(item => ({
        title: renderTreeNode(item.node),
        key: item.node.id,
        icon: getDocumentTypeIcon(item.node.type),
        node: item.node,
        children: item.children ? convertToTreeData(item.children) : undefined
      }))
  }

  // 渲染树节点
  const renderTreeNode = (node: DocMetaEntry) => (
    <div 
      style={{ 
        display: 'flex', 
        justifyContent: 'space-between', 
        alignItems: 'center',
        width: '100%',
        minWidth: 0
      }}
    >
      <span 
        style={{ 
          flex: 1, 
          overflow: 'hidden', 
          textOverflow: 'ellipsis', 
          whiteSpace: 'nowrap' 
        }}
      >
        {node.title}
      </span>
      <Dropdown
        menu={{
          items: [
            {
              key: 'create-child',
              label: '创建子文档',
              icon: <PlusOutlined />
            },
            {
              key: 'edit',
              label: '编辑',
              icon: <EditOutlined />
            },
            {
              key: 'delete',
              label: '删除',
              icon: <DeleteOutlined />,
              danger: true
            }
          ],
          onClick: (info) => handleNodeMenuClick(info.key, node)
        }}
        trigger={['hover']}
        placement="bottomRight"
      >
        <Button 
          type="text" 
          size="small" 
          icon={<MoreOutlined />}
          onClick={(e) => e.stopPropagation()}
          style={{ opacity: 0.6 }}
        />
      </Dropdown>
    </div>
  )

  // 获取文档类型图标
  const getDocumentTypeIcon = (type: DocumentType) => {
    switch (type) {
      case 'feature_list':
        return <FileTextOutlined style={{ color: '#1890ff' }} />
      case 'architecture':
        return <FolderOutlined style={{ color: '#52c41a' }} />
      case 'tech_design':
        return <FileTextOutlined style={{ color: '#722ed1' }} />
      case 'background':
        return <FileTextOutlined style={{ color: '#fa8c16' }} />
      default:
        return <FileTextOutlined />
    }
  }

  // 节点菜单点击处理
  const handleNodeMenuClick = (key: string, node: DocMetaEntry) => {
    switch (key) {
      case 'create-child':
        setParentNode(node)
        setCreateModalVisible(true)
        createForm.resetFields()
        break
      case 'edit':
        setEditingNode(node)
        setEditModalVisible(true)
        editForm.setFieldsValue({
          title: node.title,
          type: node.type
        })
        break
      case 'delete':
        handleDeleteNode(node)
        break
    }
  }

  // 创建文档
  const handleCreateDocument = async (values: any) => {
    try {
      // 如果parentNode是虚拟根节点或null，则创建根文档
      const isRootDocument = !parentNode || parentNode.id === 'virtual_root'
      
      const request: CreateNodeRequest = {
        parent_id: isRootDocument ? undefined : parentNode.id,
        title: values.title,
        type: values.type,
        content: values.content || `# ${values.title}\n\n请在此编写文档内容...`
      }
      
      console.log('[DEBUG] Frontend request:', JSON.stringify(request, null, 2))
      console.log('[DEBUG] parentNode:', parentNode)
      console.log('[DEBUG] isRootDocument:', isRootDocument)
      
      await documentsAPI.createNode(projectId, request)
      message.success('文档创建成功')
      setCreateModalVisible(false)
      loadDocumentTree()
    } catch (error) {
      console.error('Failed to create document:', error)
      message.error('创建文档失败')
    }
  }

  // 编辑文档元信息
  const handleEditDocument = async (values: any) => {
    if (!editingNode) return
    
    try {
      // 这里需要更新文档的标题和类型
      // 注意：当前API可能不支持更新元信息，这是一个待实现的功能
      message.info('编辑功能待完善')
      setEditModalVisible(false)
    } catch (error) {
      console.error('Failed to edit document:', error)
      message.error('编辑文档失败')
    }
  }

  // 删除文档
  const handleDeleteNode = (node: DocMetaEntry) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除文档"${node.title}"吗？此操作不可恢复。`,
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await documentsAPI.deleteNode(projectId, node.id)
          message.success('文档删除成功')
          loadDocumentTree()
        } catch (error) {
          console.error('Failed to delete document:', error)
          message.error('删除文档失败')
        }
      }
    })
  }

  // 创建根文档
  const handleCreateRootDocument = () => {
    setParentNode(null)
    setCreateModalVisible(true)
    createForm.resetFields()
  }

  // 根据节点ID查找节点数据
  const findNodeById = (nodeId: string, nodes: TreeNodeData[]): DocMetaEntry | null => {
    for (const node of nodes) {
      if (node.node.id === nodeId) {
        return node.node
      }
      if (node.children) {
        const found = findNodeById(nodeId, node.children)
        if (found) return found
      }
    }
    return null
  }

  // 树节点选择
  const handleTreeSelect = (selectedKeys: React.Key[]) => {
    const nodeId = selectedKeys[0] as string
    setSelectedKeys(selectedKeys as string[])
    
    if (nodeId) {
      const nodeData = findNodeById(nodeId, treeData)
      if (nodeData) {
        setSelectedNode(nodeData)
        setShowContentViewer(true)
        
        if (onDocumentSelect) {
          onDocumentSelect(nodeId)
        }
      }
    } else {
      setSelectedNode(null)
      setShowContentViewer(false)
    }
  }

  useEffect(() => {
    if (projectId) {
      loadDocumentTree()
    }
  }, [projectId, loadDocumentTree])

  useEffect(() => {
    if (selectedDocumentId && selectedKeys[0] !== selectedDocumentId) {
      setSelectedKeys([selectedDocumentId])
    }
  }, [selectedDocumentId])

  return (
    <div>
      <Row gutter={16}>
        {/* 左侧：文档树 */}
        <Col span={showContentViewer ? 8 : 24}>
          {/* 工具栏 */}
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
            <h4 style={{ margin: 0 }}>文档树</h4>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              size="small"
              onClick={handleCreateRootDocument}
            >
              创建根文档
            </Button>
          </div>

          {/* 文档树 */}
          <Spin spinning={loading}>
            <Tree
              treeData={treeData}
              expandedKeys={expandedKeys}
              selectedKeys={selectedKeys}
              onExpand={(keys) => setExpandedKeys(keys as string[])}
              onSelect={handleTreeSelect}
              showIcon
              style={{ 
                background: '#fafafa', 
                padding: 8, 
                borderRadius: 4, 
                minHeight: showContentViewer ? 500 : 200 
              }}
            />
          </Spin>
        </Col>

        {/* 右侧：文档内容查看器 */}
        {showContentViewer && selectedNode && (
          <Col span={16}>
            <DocumentContentViewer
              projectId={projectId}
              nodeId={selectedNode.id}
              nodeName={selectedNode.title}
              taskId={taskId}
              onClose={() => {
                setShowContentViewer(false)
                setSelectedNode(null)
                setSelectedKeys([])
              }}
            />
          </Col>
        )}
      </Row>

      {/* 创建文档对话框 */}
      <Modal
        title={parentNode ? `在"${parentNode.title}"下创建子文档` : '创建根文档'}
        open={createModalVisible}
        onCancel={() => setCreateModalVisible(false)}
        footer={null}
      >
        <Form
          form={createForm}
          layout="vertical"
          onFinish={handleCreateDocument}
        >
          <Form.Item
            name="title"
            label="文档标题"
            rules={[{ required: true, message: '请输入文档标题' }]}
          >
            <Input placeholder="输入文档标题" />
          </Form.Item>
          
          <Form.Item
            name="type"
            label="文档类型"
            rules={[{ required: true, message: '请选择文档类型' }]}
            initialValue="tech_design"
          >
            <Select>
              <Select.Option value="feature_list">特性列表</Select.Option>
              <Select.Option value="architecture">架构设计</Select.Option>
              <Select.Option value="tech_design">技术方案</Select.Option>
              <Select.Option value="background">背景资料</Select.Option>
            </Select>
          </Form.Item>
          
          <Form.Item
            name="content"
            label="初始内容"
          >
            <Input.TextArea 
              rows={4} 
              placeholder="可选：输入文档初始内容，支持Markdown格式"
            />
          </Form.Item>
          
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setCreateModalVisible(false)}>取消</Button>
              <Button type="primary" htmlType="submit">创建</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 编辑文档对话框 */}
      <Modal
        title={`编辑文档: ${editingNode?.title}`}
        open={editModalVisible}
        onCancel={() => setEditModalVisible(false)}
        footer={null}
      >
        <Form
          form={editForm}
          layout="vertical"
          onFinish={handleEditDocument}
        >
          <Form.Item
            name="title"
            label="文档标题"
            rules={[{ required: true, message: '请输入文档标题' }]}
          >
            <Input placeholder="输入文档标题" />
          </Form.Item>
          
          <Form.Item
            name="type"
            label="文档类型"
            rules={[{ required: true, message: '请选择文档类型' }]}
          >
            <Select>
              <Select.Option value="feature_list">特性列表</Select.Option>
              <Select.Option value="architecture">架构设计</Select.Option>
              <Select.Option value="tech_design">技术方案</Select.Option>
              <Select.Option value="background">背景资料</Select.Option>
            </Select>
          </Form.Item>
          
          <Form.Item style={{ marginBottom: 0, textAlign: 'right' }}>
            <Space>
              <Button onClick={() => setEditModalVisible(false)}>取消</Button>
              <Button type="primary" htmlType="submit">保存</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default DocumentTreeView