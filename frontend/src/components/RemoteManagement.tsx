import React, { useState, useEffect, useCallback } from 'react';
import { Table, Button, Modal, Form, Input, Space, Tag, message, Popconfirm, Tooltip } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, ApiOutlined, CheckCircleOutlined, CloseCircleOutlined, LoadingOutlined } from '@ant-design/icons';
import type { RemoteSafe, CreateRemoteRequest, UpdateRemoteRequest, TestResult } from '../api/remotes';
import { listRemotes, createRemote, updateRemote, deleteRemote, testRemote, testRemoteURL } from '../api/remotes';

export const RemoteManagement: React.FC = () => {
  const [remotes, setRemotes] = useState<RemoteSafe[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<RemoteSafe | null>(null);
  const [testResults, setTestResults] = useState<Record<string, TestResult | 'loading'>>({});
  const [form] = Form.useForm();

  const fetchRemotes = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listRemotes();
      setRemotes(data || []);
    } catch (e: any) {
      message.error('加载远端列表失败: ' + (e?.response?.data?.error || e.message));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchRemotes(); }, [fetchRemotes]);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ url: 'http://', secret: '' });
    setModalOpen(true);
  };

  const openEdit = (record: RemoteSafe) => {
    setEditing(record);
    form.setFieldsValue({ name: record.name, url: record.url, secret: '' });
    setModalOpen(true);
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      if (editing) {
        const req: UpdateRemoteRequest = {};
        if (values.name) req.name = values.name;
        if (values.url) req.url = values.url;
        if (values.secret) req.secret = values.secret;
        await updateRemote(editing.id, req);
        message.success('更新成功');
      } else {
        const req: CreateRemoteRequest = { name: values.name, url: values.url };
        if (values.secret) req.secret = values.secret;
        await createRemote(req);
        message.success('添加成功');
      }
      setModalOpen(false);
      fetchRemotes();
    } catch (e: any) {
      if (e?.errorFields) return; // form validation
      message.error(e?.response?.data?.error || e.message);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteRemote(id);
      message.success('已删除');
      fetchRemotes();
    } catch (e: any) {
      message.error(e?.response?.data?.error || e.message);
    }
  };

  const handleTest = async (record: RemoteSafe) => {
    setTestResults(prev => ({ ...prev, [record.id]: 'loading' }));
    try {
      const result = await testRemote(record.id);
      setTestResults(prev => ({ ...prev, [record.id]: result }));
      if (result.reachable) {
        message.success(`${record.name} 连接成功 (${result.latency})`);
      } else {
        message.warning(`${record.name} 不可达: ${result.error || result.status}`);
      }
    } catch (e: any) {
      setTestResults(prev => ({
        ...prev,
        [record.id]: { reachable: false, status: 'error', latency: '', error: e.message },
      }));
      message.error(`测试失败: ${e.message}`);
    }
  };

  const handleTestAll = async () => {
    for (const r of remotes) {
      handleTest(r);
    }
  };

  const renderStatus = (record: RemoteSafe) => {
    const result = testResults[record.id];
    if (!result) return <Tag>未测试</Tag>;
    if (result === 'loading') return <Tag icon={<LoadingOutlined spin />} color="processing">测试中</Tag>;
    if (result.reachable) {
      return (
        <Tooltip title={`${result.service || 'AIDG'} ${result.version || ''} - ${result.latency}`}>
          <Tag icon={<CheckCircleOutlined />} color="success">在线</Tag>
        </Tooltip>
      );
    }
    return (
      <Tooltip title={result.error || result.status}>
        <Tag icon={<CloseCircleOutlined />} color="error">离线</Tag>
      </Tooltip>
    );
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name', width: 180 },
    { title: '地址', dataIndex: 'url', key: 'url', ellipsis: true },
    {
      title: '状态', key: 'status', width: 100,
      render: (_: any, record: RemoteSafe) => renderStatus(record),
    },
    {
      title: '操作', key: 'actions', width: 200,
      render: (_: any, record: RemoteSafe) => (
        <Space size="small">
          <Button size="small" icon={<ApiOutlined />} onClick={() => handleTest(record)}>测试</Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>编辑</Button>
          <Popconfirm title="确定删除此远端？" onConfirm={() => handleDelete(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 16 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h3 style={{ margin: 0 }}>远端 AIDG 系统管理</h3>
        <Space>
          <Button onClick={handleTestAll} disabled={remotes.length === 0}>全部测试</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>添加远端</Button>
        </Space>
      </div>

      <Table
        dataSource={remotes}
        columns={columns}
        rowKey="id"
        loading={loading}
        size="small"
        pagination={false}
        locale={{ emptyText: '暂无远端系统配置，点击"添加远端"开始' }}
      />

      <Modal
        title={editing ? '编辑远端系统' : '添加远端系统'}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        okText={editing ? '保存' : '添加'}
        cancelText="取消"
        destroyOnClose
      >
        <Form form={form} layout="vertical" autoComplete="off">
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入远端名称' }]}>
            <Input placeholder="例如: 会议室B的AIDG" />
          </Form.Item>
          <Form.Item
            name="url"
            label="服务器地址"
            rules={[
              { required: true, message: '请输入服务器地址' },
              { pattern: /^https?:\/\//, message: '地址需以 http:// 或 https:// 开头' },
            ]}
          >
            <Input placeholder="http://192.168.1.100:8000" />
          </Form.Item>
          <Form.Item name="secret" label="共享密钥" extra="留空则使用系统默认密钥 (SYNC_SHARED_SECRET)">
            <Input.Password placeholder="可选" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default RemoteManagement;
