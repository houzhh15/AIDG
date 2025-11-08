/**
 * MCP Prompts Management Component
 * Prompts 管理界面 - 支持三层架构（global/project/personal）
 */

import React, { useState, useEffect } from 'react';
import {
  Table,
  Button,
  Modal,
  Select,
  Space,
  Tag,
  message,
  Row,
  Col,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  EyeOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { Prompt, PromptArgument } from '../types/prompt';
import PromptsEditorModal from './PromptsEditorModal';
import { authedApi } from '../api/auth';

const { Option } = Select;
const { confirm } = Modal;

interface PromptsManagementProps {
  scope: 'global' | 'project' | 'personal';
  projectId?: string;
  username: string;
  className?: string;
}

const PromptsManagement: React.FC<PromptsManagementProps> = ({
  scope,
  projectId,
  username,
  className = '',
}) => {
  const [prompts, setPrompts] = useState<Prompt[]>([]);
  const [loading, setLoading] = useState(false);

  // 编辑器 Modal 状态
  const [modalState, setModalState] = useState<{
    visible: boolean;
    mode: 'create' | 'edit';
    prompt?: Prompt;
  }>({
    visible: false,
    mode: 'create',
    prompt: undefined,
  });

  // 过滤器状态
  const [filters, setFilters] = useState<{
    visibility?: 'public' | 'private';
  }>({});

  // 加载 Prompts 列表
  const loadPrompts = async () => {
    try {
      setLoading(true);

      // 构造 API URL（authedApi 已配置 baseURL='/api/v1'，使用相对路径）
      let url = '/prompts';
      if (scope === 'project' && projectId) {
        url = `/projects/${projectId}/prompts`;
      }

      // 添加查询参数
      const params: any = { scope };
      if (filters.visibility) {
        params.visibility = filters.visibility;
      }

      const response = await authedApi.get(url, { params });

      if (response.data.success) {
        setPrompts(response.data.data || []);
      } else {
        throw new Error(response.data.error || '加载失败');
      }
    } catch (err) {
      message.error(`加载 Prompts 失败: ${err instanceof Error ? err.message : '未知错误'}`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPrompts();
  }, [scope, projectId, filters]);

  // 打开创建对话框
  const handleCreate = () => {
    setModalState({
      visible: true,
      mode: 'create',
      prompt: undefined,
    });
  };

  // 打开编辑对话框 - 需要先获取完整数据（包含content）
  const handleEdit = async (prompt: Prompt) => {
    try {
      setLoading(true);
      let url = `/prompts/${prompt.prompt_id}`;
      if (scope === 'project' && projectId) {
        url = `/projects/${projectId}/prompts/${prompt.prompt_id}`;
      }
      
      const response = await authedApi.get(url);
      if (response.data.success) {
        setModalState({
          visible: true,
          mode: 'edit',
          prompt: response.data.data, // 使用完整的数据（包含content）
        });
      } else {
        throw new Error(response.data.error || '获取失败');
      }
    } catch (err) {
      message.error(`获取 Prompt 详情失败: ${err instanceof Error ? err.message : '未知错误'}`);
    } finally {
      setLoading(false);
    }
  };

  // 查看 Prompt 详情
  const handleView = async (prompt: Prompt) => {
    try {
      setLoading(true);
      let url = `/prompts/${prompt.prompt_id}`;
      if (scope === 'project' && projectId) {
        url = `/projects/${projectId}/prompts/${prompt.prompt_id}`;
      }
      
      const response = await authedApi.get(url);
      if (response.data.success) {
        const fullPrompt = response.data.data;
        Modal.info({
          title: fullPrompt.name,
          content: (
            <div>
              {fullPrompt.description && (
                <p style={{ color: '#666', marginBottom: 16 }}>
                  {fullPrompt.description}
                </p>
              )}
              {fullPrompt.arguments && fullPrompt.arguments.length > 0 && (
                <div style={{ marginBottom: 16 }}>
                  <strong>参数：</strong>
                  <ul>
                    {fullPrompt.arguments.map((arg: PromptArgument, idx: number) => (
                      <li key={idx}>
                        <code>{arg.name}</code>
                        {arg.required && <span style={{ color: 'red' }}> *</span>}
                        {arg.description && `: ${arg.description}`}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              <div>
                <strong>内容：</strong>
                <pre style={{ 
                  background: '#f5f5f5', 
                  padding: 12, 
                  borderRadius: 4,
                  maxHeight: 400,
                  overflow: 'auto',
                  whiteSpace: 'pre-wrap',
                  wordWrap: 'break-word'
                }}>
                  {fullPrompt.content}
                </pre>
              </div>
            </div>
          ),
          width: 800,
        });
      } else {
        throw new Error(response.data.error || '获取失败');
      }
    } catch (err) {
      message.error(`获取 Prompt 详情失败: ${err instanceof Error ? err.message : '未知错误'}`);
    } finally {
      setLoading(false);
    }
  };

  // 删除 Prompt
  const handleDelete = (promptId: string) => {
    confirm({
      title: '确认删除',
      content: '删除后将无法恢复，确定要删除这个 Prompt 吗？',
      okText: '删除',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          let url = `/prompts/${promptId}`;
          if (scope === 'project' && projectId) {
            url = `/projects/${projectId}/prompts/${promptId}`;
          }

          const response = await authedApi.delete(url);

          if (response.data.success) {
            message.success('删除成功');
            loadPrompts();
          } else {
            throw new Error(response.data.error || '删除失败');
          }
        } catch (err) {
          message.error(`删除失败: ${err instanceof Error ? err.message : '未知错误'}`);
        }
      },
    });
  };

  // Modal 保存回调
  const handleModalSave = () => {
    setModalState({ visible: false, mode: 'create', prompt: undefined });
    loadPrompts();
  };

  // 表格列定义
  const columns: ColumnsType<Prompt> = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      width: 200,
      ellipsis: true,
    },
    {
      title: 'Version',
      dataIndex: 'version',
      key: 'version',
      width: 100,
      render: (version: number) => `v${version}`,
    },
    {
      title: 'Description',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: 'Visibility',
      dataIndex: 'visibility',
      key: 'visibility',
      width: 100,
      render: (visibility: string) => (
        <Tag color={visibility === 'public' ? 'green' : 'orange'}>
          {visibility === 'public' ? 'Public' : 'Private'}
        </Tag>
      ),
    },
    {
      title: 'Owner',
      dataIndex: 'owner',
      key: 'owner',
      width: 120,
    },
    {
      title: 'Created At',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (date: string) => new Date(date).toLocaleString('zh-CN'),
    },
    {
      title: 'Actions',
      key: 'actions',
      width: 150,
      fixed: 'right',
      render: (_: any, record: Prompt) => {
        const isOwner = record.owner === username;
        const isAdmin = username === 'admin'; // 简化判断

        return (
          <Space size="small">
            <Button
              icon={<EyeOutlined />}
              size="small"
              onClick={() => handleView(record)}
            />
            <Button
              icon={<EditOutlined />}
              size="small"
              disabled={!isOwner && !isAdmin}
              onClick={() => handleEdit(record)}
            />
            <Button
              icon={<DeleteOutlined />}
              size="small"
              danger
              disabled={!isOwner && !isAdmin}
              onClick={() => handleDelete(record.prompt_id)}
            />
          </Space>
        );
      },
    },
  ];

  return (
    <div className={className}>
      {/* 工具栏 */}
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col flex="auto">
          <Space>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={handleCreate}
            >
              添加 Prompt
            </Button>
            <Button icon={<ReloadOutlined />} onClick={loadPrompts}>
              刷新
            </Button>
          </Space>
        </Col>
      </Row>

      {/* 表格 */}
      <Table
        columns={columns}
        dataSource={prompts}
        rowKey="prompt_id"
        loading={loading}
        pagination={{
          pageSize: 20,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 个 Prompts`,
        }}
        scroll={{ x: 1200 }}
      />

      {/* 编辑器 Modal */}
      <PromptsEditorModal
        visible={modalState.visible}
        mode={modalState.mode}
        initialPrompt={modalState.prompt}
        scope={scope}
        projectId={projectId}
        onClose={() => setModalState({ ...modalState, visible: false })}
        onSuccess={handleModalSave}
      />
    </div>
  );
};

export default PromptsManagement;
