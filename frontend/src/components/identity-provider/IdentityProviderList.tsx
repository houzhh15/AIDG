/**
 * 身份源列表组件
 */
import React, { useState, useEffect, useMemo } from 'react';
import {
  Table,
  Button,
  Input,
  Select,
  Space,
  Tag,
  Badge,
  Popconfirm,
  message,
  Card,
  Tooltip,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ApiOutlined,
  SyncOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import {
  getIdentityProviders,
  deleteIdentityProvider,
  testConnection,
} from '../../api/identityProviders';
import { IdentityProvider, IdPType, IdPStatus } from '../../types/identityProvider';
import { usePermission } from '../../hooks/usePermission';
import IdentityProviderForm from './IdentityProviderForm';
import IdentityProviderSync from './IdentityProviderSync';

type SubView = 'list' | 'form' | 'sync';

const IdentityProviderList: React.FC = () => {
  const [idps, setIdps] = useState<IdentityProvider[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchText, setSearchText] = useState('');
  const [filterType, setFilterType] = useState<IdPType | 'all'>('all');
  const [filterStatus, setFilterStatus] = useState<IdPStatus | 'all'>('all');
  const [testingId, setTestingId] = useState<string | null>(null);

  // 子视图状态
  const [subView, setSubView] = useState<SubView>('list');
  const [selectedIdpId, setSelectedIdpId] = useState<string | undefined>();

  const { hasPermission } = usePermission();
  const hasReadPermission = hasPermission('idp.read');
  const hasWritePermission = hasPermission('idp.write');

  // 加载数据
  const loadData = async () => {
    setLoading(true);
    try {
      const res = await getIdentityProviders();
      if (res.success && res.data) {
        setIdps(res.data);
      } else {
        message.error(res.error || '加载身份源列表失败');
      }
    } catch (err: any) {
      console.error('[IdP] Failed to load identity providers:', err);
      message.error('加载身份源列表失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (hasReadPermission) {
      loadData();
    }
  }, [hasReadPermission]);

  // 过滤数据
  const filteredIdps = useMemo(() => {
    return idps.filter((idp) => {
      // 名称搜索
      if (searchText && !idp.name.toLowerCase().includes(searchText.toLowerCase())) {
        return false;
      }
      // 类型筛选
      if (filterType !== 'all' && idp.type !== filterType) {
        return false;
      }
      // 状态筛选
      if (filterStatus !== 'all' && idp.status !== filterStatus) {
        return false;
      }
      return true;
    });
  }, [idps, searchText, filterType, filterStatus]);

  // 删除身份源
  const handleDelete = async (id: string) => {
    try {
      const res = await deleteIdentityProvider(id);
      if (res.success) {
        message.success('删除成功');
        loadData();
      } else {
        message.error(res.error || '删除失败');
      }
    } catch (err: any) {
      console.error('[IdP] Failed to delete identity provider:', err);
      message.error(err.response?.data?.error || '删除失败');
    }
  };

  // 测试连接
  const handleTestConnection = async (idp: IdentityProvider) => {
    setTestingId(idp.id);
    try {
      const res = await testConnection({
        type: idp.type,
        config: idp.config,
      });
      if (res.success && res.data) {
        if (res.data.success) {
          message.success('连接测试成功');
        } else {
          message.error(`连接测试失败: ${res.data.message}`);
        }
      } else {
        message.error(res.error || '连接测试失败');
      }
    } catch (err: any) {
      console.error('[IdP] Test connection error:', err);
      message.error(err.response?.data?.error || '连接测试失败');
    } finally {
      setTestingId(null);
    }
  };

  // 表格列定义
  const columns: ColumnsType<IdentityProvider> = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      width: 200,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (type: IdPType) => (
        <Tag color={type === 'OIDC' ? 'blue' : 'purple'}>{type}</Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: IdPStatus) => (
        <Badge
          status={status === 'Enabled' ? 'success' : 'default'}
          text={status === 'Enabled' ? '启用' : '禁用'}
        />
      ),
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      key: 'priority',
      width: 80,
      sorter: (a, b) => a.priority - b.priority,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (time: string) => new Date(time).toLocaleString('zh-CN'),
      sorter: (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
    },
    {
      title: '操作',
      key: 'actions',
      width: 280,
      render: (_, record) => (
        <Space size="small">
          <Tooltip title="编辑">
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => {
                setSelectedIdpId(record.id);
                setSubView('form');
              }}
              disabled={!hasWritePermission}
            >
              编辑
            </Button>
          </Tooltip>
          <Tooltip title="测试连接">
            <Button
              type="link"
              size="small"
              icon={<ApiOutlined />}
              loading={testingId === record.id}
              onClick={() => handleTestConnection(record)}
              disabled={!hasWritePermission}
            >
              测试
            </Button>
          </Tooltip>
          {record.type === 'LDAP' && (
            <Tooltip title="同步管理">
              <Button
                type="link"
                size="small"
                icon={<SyncOutlined />}
                onClick={() => {
                  setSelectedIdpId(record.id);
                  setSubView('sync');
                }}
              >
                同步
              </Button>
            </Tooltip>
          )}
          <Popconfirm
            title="确认删除"
            description="删除后不可恢复，确定要删除这个身份源吗？"
            onConfirm={() => handleDelete(record.id)}
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
          >
            <Button
              type="link"
              size="small"
              danger
              icon={<DeleteOutlined />}
              disabled={!hasWritePermission}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  // 根据子视图渲染不同内容
  if (subView === 'form') {
    return (
      <IdentityProviderForm
        idpId={selectedIdpId}
        onSuccess={() => {
          setSubView('list');
          setSelectedIdpId(undefined);
          loadData();
        }}
        onCancel={() => {
          setSubView('list');
          setSelectedIdpId(undefined);
        }}
      />
    );
  }

  if (subView === 'sync' && selectedIdpId) {
    return (
      <IdentityProviderSync
        idpId={selectedIdpId}
        onBack={() => {
          setSubView('list');
          setSelectedIdpId(undefined);
        }}
      />
    );
  }

  // 列表视图
  return (
    <Card title="身份源管理">
      {/* 工具栏 */}
      <Space style={{ marginBottom: 16 }} wrap>
        <Input
          placeholder="搜索名称"
          prefix={<SearchOutlined />}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          style={{ width: 200 }}
          allowClear
        />
        <Select
          placeholder="类型"
          value={filterType}
          onChange={setFilterType}
          style={{ width: 120 }}
          options={[
            { value: 'all', label: '全部类型' },
            { value: 'OIDC', label: 'OIDC' },
            { value: 'LDAP', label: 'LDAP' },
          ]}
        />
        <Select
          placeholder="状态"
          value={filterStatus}
          onChange={setFilterStatus}
          style={{ width: 120 }}
          options={[
            { value: 'all', label: '全部状态' },
            { value: 'Enabled', label: '启用' },
            { value: 'Disabled', label: '禁用' },
          ]}
        />
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => {
            setSelectedIdpId(undefined);
            setSubView('form');
          }}
          disabled={!hasWritePermission}
        >
          新建身份源
        </Button>
      </Space>

      {/* 表格 */}
      <Table
        columns={columns}
        dataSource={filteredIdps}
        rowKey="id"
        loading={loading}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => `共 ${total} 条`,
        }}
      />
    </Card>
  );
};

export default IdentityProviderList;
