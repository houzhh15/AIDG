import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Empty, message, Spin, Typography, Space, Tag, Divider } from 'antd';
import { documentsAPI, Reference } from '../api/documents';
import { DocumentTreeDTO } from '../api/documents';
import { DocumentTreeNode } from '../types/documents';
import { EnhancedTreeView, MarkdownEditor } from './documents';

interface TaskLinkedDocumentsProps {
  projectId: string;
  taskId: string;
}

interface LoadedDocumentMeta {
  id: string;
  title: string;
  version: number;
}

const convertTreeDto = (dto: DocumentTreeDTO): DocumentTreeNode => ({
  ...dto.node,
  children: dto.children ? dto.children.map(convertTreeDto) : undefined
});

const normalizeTree = (tree?: DocumentTreeDTO | DocumentTreeDTO[]): DocumentTreeDTO[] => {
  if (!tree) {
    return [];
  }
  const base = Array.isArray(tree) ? tree : [tree];
  if (base.length === 1 && base[0]?.node?.id === 'virtual_root') {
    return base[0].children ?? [];
  }
  return base;
};

const filterTreeByDocuments = (
  nodes: DocumentTreeNode[],
  allowed: Set<string>
): DocumentTreeNode[] => {
  const filterNode = (node: DocumentTreeNode): DocumentTreeNode | null => {
    const childMatches = node.children
      ? node.children.map(filterNode).filter((child): child is DocumentTreeNode => Boolean(child))
      : undefined;

    if (allowed.has(node.id) || (childMatches && childMatches.length > 0)) {
      return {
        ...node,
        children: childMatches
      };
    }

    return null;
  };

  return nodes
    .map(filterNode)
    .filter((node): node is DocumentTreeNode => Boolean(node));
};

const collectExpandedKeys = (nodes: DocumentTreeNode[]): string[] => {
  const keys: string[] = [];
  const traverse = (nodeList: DocumentTreeNode[]) => {
    nodeList.forEach((node) => {
      keys.push(node.id);
      if (node.children && node.children.length > 0) {
        traverse(node.children);
      }
    });
  };
  traverse(nodes);
  return keys;
};

const TaskLinkedDocuments: React.FC<TaskLinkedDocumentsProps> = ({ projectId, taskId }) => {
  const [loading, setLoading] = useState<boolean>(false);
  const [treeData, setTreeData] = useState<DocumentTreeNode[]>([]);
  const [references, setReferences] = useState<Reference[]>([]);
  const [selectedKeys, setSelectedKeys] = useState<string[]>([]);
  const [expandedKeys, setExpandedKeys] = useState<string[]>([]);
  const [documentContent, setDocumentContent] = useState<string>('');
  const [documentLoading, setDocumentLoading] = useState<boolean>(false);
  const [documentMeta, setDocumentMeta] = useState<LoadedDocumentMeta | null>(null);

  const loadData = useCallback(async () => {
    if (!projectId || !taskId) {
      return;
    }

    setLoading(true);
    try {
      const [referencesRes, treeRes] = await Promise.all([
        documentsAPI.getTaskReferences(projectId, taskId),
        documentsAPI.getTree(projectId, undefined, 10)
      ]);

      const refs = referencesRes.references || [];
      setReferences(refs);

      const allowedDocIds = new Set(refs.map((ref) => ref.document_id));
      if (allowedDocIds.size === 0) {
        setTreeData([]);
        setExpandedKeys([]);
        setSelectedKeys([]);
        setDocumentMeta(null);
        setDocumentContent('');
        return;
      }

      const normalized = normalizeTree(treeRes.tree).map(convertTreeDto);
      const filteredTree = filterTreeByDocuments(normalized, allowedDocIds);

      setTreeData(filteredTree);
      setExpandedKeys(collectExpandedKeys(filteredTree));

      const firstDocId = refs[0]?.document_id;
      if (firstDocId) {
        setSelectedKeys([firstDocId]);
      }
    } catch (error) {
      console.error('加载任务关联文档失败:', error);
      message.error('加载关联文档失败，请稍后重试');
      setTreeData([]);
      setExpandedKeys([]);
      setSelectedKeys([]);
    } finally {
      setLoading(false);
    }
  }, [projectId, taskId]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const loadDocumentContent = useCallback(
    async (docId: string) => {
      setDocumentLoading(true);
      try {
        const response = await documentsAPI.getContent(projectId, docId);
        setDocumentMeta({
          id: docId,
          title: response.meta?.title ?? docId,
          version: response.meta?.version ?? 1
        });
        setDocumentContent(response.content || '');
      } catch (error) {
        console.error('加载文档内容失败:', error);
        message.error('加载文档内容失败');
      } finally {
        setDocumentLoading(false);
      }
    },
    [projectId]
  );

  useEffect(() => {
    if (selectedKeys.length === 0) {
      setDocumentMeta(null);
      setDocumentContent('');
      return;
    }
    const docId = selectedKeys[0];
    loadDocumentContent(docId);
  }, [selectedKeys, loadDocumentContent]);

  const handleTreeSelect = (keys: string[]) => {
    if (!keys.length) {
      return;
    }
    const docId = keys[0];
    if (docId === selectedKeys[0]) {
      return;
    }
    setSelectedKeys([docId]);
  };

  const handleSaveDocument = useCallback(
    async (content: string) => {
      if (!documentMeta) {
        return;
      }
      try {
        const result = await documentsAPI.updateContent(projectId, documentMeta.id, {
          content,
          version: documentMeta.version
        });
        setDocumentContent(content);
        setDocumentMeta((prev) =>
          prev
            ? {
                ...prev,
                version: result.version ?? documentMeta.version + 1
              }
            : prev
        );
        message.success('文档保存成功');
      } catch (error: any) {
        console.error('保存文档失败:', error);
        if (error?.response?.data?.code === 'VERSION_MISMATCH') {
          message.error('保存失败：检测到版本冲突，请刷新后重试');
        } else {
          message.error('保存文档失败，请稍后重试');
        }
      }
    },
    [documentMeta, projectId]
  );

  const handleUnlinkTask = useCallback(async (documentId: string) => {
    try {
      // 找到该文档与当前任务的所有引用
      const refsToDelete = references.filter(
        (ref) => ref.document_id === documentId && ref.task_id === taskId
      );

      if (refsToDelete.length === 0) {
        message.warning('该文档未与当前任务关联');
        return;
      }

      // 删除所有相关的引用
      for (const ref of refsToDelete) {
        await documentsAPI.deleteReference(projectId, ref.id);
      }

      // 重新加载数据
      await loadData();
      message.success('解除关联成功');
    } catch (error) {
      console.error('解除关联失败:', error);
      message.error('解除关联失败，请稍后重试');
    }
  }, [projectId, taskId, references, loadData]);

  const associatedReferences = useMemo(() => {
    if (!documentMeta) {
      return [] as Reference[];
    }
    return references.filter((ref) => ref.document_id === documentMeta.id);
  }, [references, documentMeta]);

  if (!projectId || !taskId) {
    return <Empty description="请选择任务" />;
  }

  if (loading) {
    return (
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%' }}>
        <Spin tip="加载关联文档中..." />
      </div>
    );
  }

  if (!treeData.length) {
    return (
      <Empty
        style={{ padding: '32px 0' }}
        description={
          <span>
            当前任务暂无关联文档
          </span>
        }
      />
    );
  }

  return (
    <div style={{ display: 'flex', gap: 16, height: '100%' }}>
      <div
        style={{
          width: 320,
          minWidth: 280,
          border: '1px solid #e5e7eb',
          borderRadius: 10,
          backgroundColor: '#fff',
          padding: 12,
          display: 'flex',
          flexDirection: 'column',
          boxShadow: '0 1px 2px rgba(15,23,42,0.08)'
        }}
      >
        <Typography.Title level={5} style={{ margin: '0 0 12px' }}>
          关联文档结构
        </Typography.Title>
        <EnhancedTreeView
          treeData={treeData}
          selectedKeys={selectedKeys}
          expandedKeys={expandedKeys}
          onSelect={handleTreeSelect}
          onExpand={setExpandedKeys}
          loading={false}
          searchable={false}
          draggable={false}
          showContextMenu={true}
          showToolbar={false}
          onUnlinkTask={handleUnlinkTask}
        />
      </div>

      <div
        style={{
          flex: 1,
          minWidth: 0,
          border: '1px solid #e5e7eb',
          borderRadius: 10,
          backgroundColor: '#fff',
          padding: 16,
          display: 'flex',
          flexDirection: 'column',
          boxShadow: '0 1px 2px rgba(15,23,42,0.06)'
        }}
      >
        {documentMeta ? (
          <>
            <Space align="baseline" size={12} style={{ marginBottom: 12 }}>
              <Typography.Title level={4} style={{ margin: 0 }}>
                {documentMeta.title}
              </Typography.Title>
              <Tag color="blue">v{documentMeta.version}</Tag>
            </Space>
            <div style={{ flex: 1, minHeight: 0 }}>
              <Spin spinning={documentLoading}>
                <MarkdownEditor
                  value={documentContent}
                  onChange={setDocumentContent}
                  onSave={handleSaveDocument}
                  loading={documentLoading}
                  height={520}
                  showPreview
                  showToolbar
                />
              </Spin>
            </div>
            <Divider style={{ margin: '16px 0' }} />
            <Typography.Title level={5} style={{ marginBottom: 8 }}>
              任务引用位置
            </Typography.Title>
            {associatedReferences.length ? (
              <Space direction="vertical" size={8} style={{ fontSize: 12 }}>
                {associatedReferences.map((ref) => (
                  <div
                    key={ref.id}
                    style={{
                      background: '#f5f7fa',
                      borderRadius: 8,
                      padding: '8px 12px',
                      border: '1px solid #e2e8f0'
                    }}
                  >
                    <Space direction="vertical" size={4} style={{ width: '100%' }}>
                      <div>
                        <Typography.Text strong>锚点：</Typography.Text>
                        <Typography.Text>{ref.anchor?.trim() || '未设置'}</Typography.Text>
                      </div>
                      {ref.context?.trim() && (
                        <Typography.Paragraph style={{ margin: 0 }}>
                          {ref.context}
                        </Typography.Paragraph>
                      )}
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        引用状态：{ref.status === 'active' ? '有效' : ref.status === 'outdated' ? '可能过期' : '失效'}
                      </Typography.Text>
                    </Space>
                  </div>
                ))}
              </Space>
            ) : (
              <Typography.Text type="secondary">该文档未被当前任务引用</Typography.Text>
            )}
          </>
        ) : (
          <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Empty description="请选择文档" />
          </div>
        )}
      </div>
    </div>
  );
};

export default TaskLinkedDocuments;
