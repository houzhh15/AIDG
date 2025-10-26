import React, { useEffect, useState } from 'react';
import { Layout, List, Button, Typography, Space, Modal, Form, Input, Popconfirm, message, Tag, Tooltip, Dropdown, type MenuProps } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, CopyOutlined, HistoryOutlined, MenuFoldOutlined, MenuUnfoldOutlined } from '@ant-design/icons';
import { ProjectSummary, listProjects, createProject, deleteProject, patchProject } from '../api/projects';

interface Props {
  current?: string;
  onSelect: (id: string)=>void;
}

export const ProjectSidebar: React.FC<Props> = ({ current, onSelect }) => {
  const [projects, setProjects] = useState<ProjectSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<ProjectSummary | null>(null);
  const [form] = Form.useForm<{ name: string; product_line?: string; from_task_id?: string }>();
  const [collapsed, setCollapsed] = useState(true);

  async function refresh(){
    setLoading(true);
    try {
      const list = await listProjects();
      setProjects(list);
      if (list.length > 0 && !list.some(p => p.id === current)) {
        onSelect(list[0].id);
      }
    } catch(e:any){ 
      // 对 403 权限错误不显示提示，让页面显示空状态
      if (e?.response?.status !== 403) {
        message.error(e.message);
      }
      // 权限不足时设置空项目列表
      setProjects([]);
    } finally { setLoading(false); }
  }
  useEffect(()=>{ refresh(); },[]);

  function openCreate(){
    setEditing(null); form.resetFields(); setModalOpen(true);
  }
  function openEdit(p: ProjectSummary){
    setEditing(p); form.setFieldsValue({ name: p.name, product_line: p.product_line }); setModalOpen(true);
  }

  async function handleSubmit(){
    try { const values = await form.validateFields();
      if(editing){
        await patchProject(editing.id, { name: values.name, product_line: values.product_line });
        message.success('项目已更新');
      } else {
        await createProject({ name: values.name, product_line: values.product_line, from_task_id: values.from_task_id });
        message.success('项目已创建');
      }
      setModalOpen(false); refresh();
    } catch(e:any){ /* validation or API error already shown */ }
  }

  async function handleDelete(id: string){
    try { await deleteProject(id); message.success('已删除'); if(current===id) onSelect(''); refresh(); } catch(e:any){ message.error(e.message); }
  }

  async function handleCopyProjectId(id: string) {
    try {
      await navigator.clipboard.writeText(id);
      message.success('项目ID已复制到剪贴板');
    } catch (error) {
      message.error('复制失败');
    }
  }

  const toggleCollapsed = () => setCollapsed((prev) => !prev);

  const getContextMenuItems = (project: ProjectSummary): MenuProps['items'] => [
    {
      key: 'copy-id',
      icon: <CopyOutlined />,
      label: '拷贝项目ID',
      onClick: () => handleCopyProjectId(project.id),
    },
    {
      key: 'edit',
      icon: <EditOutlined />,
      label: '编辑项目',
      onClick: () => openEdit(project),
    },
    {
      key: 'delete',
      icon: <DeleteOutlined />,
      label: '删除项目',
      danger: true,
      onClick: () => handleDelete(project.id),
    },
    {
      type: 'divider',
    },
    {
      key: 'create',
      icon: <PlusOutlined />,
      label: '新建项目',
      onClick: openCreate,
    },
  ];

  return (
    <Layout.Sider
      width={150}
      collapsedWidth={80}
      collapsed={collapsed}
      collapsible
      trigger={null}
      style={{
        background: '#fff',
        borderRight: '1px solid #eee',
        height: '100%',
        display: 'flex',
        flexDirection: 'column'
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: collapsed ? 'center' : 'space-between',
          padding: collapsed ? '8px 4px' : '8px 12px',
          borderBottom: '1px solid #eee',
          flexShrink: 0
        }}
      >
        {!collapsed && (
          <Typography.Text strong style={{ fontSize: 13 }}>
            项目列表
          </Typography.Text>
        )}
        <Tooltip  placement="right">
          <Button
            type="text"
            size="small"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={toggleCollapsed}
          />
        </Tooltip>
      </div>

      <div
        className="scroll-region"
        style={{
          flex: 1,
          overflow: 'auto'
        }}
      >
        <List
          size="small"
          dataSource={projects}
          loading={loading}
          renderItem={p => (
            <Dropdown
              menu={{ items: getContextMenuItems(p) }}
              trigger={['contextMenu']}
            >
              <List.Item
                onClick={()=>onSelect(p.id)}
                style={{
                  cursor: 'pointer',
                  background: p.id===current ? '#f0f5ff' : undefined,
                  padding: collapsed ? '8px 4px' : '8px 8px'
                }}
              >
              <div style={{ display:'flex', width:'100%', alignItems:'center' }}>
                <div style={{ flex:1, minWidth:0 }}>
                  <Typography.Text
                    strong
                    style={{
                      display: 'block',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      textAlign: 'center',
                      maxWidth: collapsed ? 68 : 140,
                      fontSize: collapsed ? 12 : 13
                    }}
                  >
                    {p.name || p.id}
                  </Typography.Text>
                  <div
                    style={{
                      height: 16,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center'
                    }}
                  >
                    <Typography.Text
                      type="secondary"
                      style={{
                        fontSize: 10,
                        textAlign: 'center',
                        visibility: collapsed ? 'hidden' : 'visible'
                      }}
                    >
                      {p.product_line || '\u00A0'}
                    </Typography.Text>
                  </div>
                  <div
                    style={{
                      height: 16,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center'
                    }}
                  >
                    <Typography.Text
                      type="secondary"
                      style={{
                        fontSize: 10,
                        textAlign: 'center',
                        visibility: collapsed ? 'hidden' : 'visible'
                      }}
                    >
                      项目
                    </Typography.Text>
                  </div>
                </div>
                {!collapsed && (
                  <div style={{ display:'flex', flexDirection:'column', gap:4 }} onClick={e=>e.stopPropagation()}>
                    <Button size="small" icon={<EditOutlined />} onClick={()=>openEdit(p)} />
                    <Popconfirm title="确认删除?" onConfirm={()=>handleDelete(p.id)}>
                      <Button size="small" danger icon={<DeleteOutlined />} />
                    </Popconfirm>
                  </div>
                )}
              </div>
            </List.Item>
            </Dropdown>
          )}
        />
      </div>

      {!collapsed && (
        <div
          style={{
            borderTop: '1px solid #eee',
            background: '#fff',
            padding: 8,
            flexShrink: 0
          }}
        >
          <Space style={{ width: '100%' }}>
            <Button
              icon={<PlusOutlined />}
              type="dashed"
              onClick={openCreate}
              block
            >
              新建项目
            </Button>
          </Space>
        </div>
      )}
      <Modal
        title={editing ? '编辑项目' : '创建项目'}
        open={modalOpen}
        onCancel={()=>setModalOpen(false)}
        onOk={handleSubmit}
        okText={editing ? '保存' : '创建'}
        destroyOnClose
      >
        <Form layout="vertical" form={form} preserve={false}>
          <Form.Item name="name" label="名称" rules={[{ required:true, message:'请输入项目名称'}]}>
            <Input placeholder="项目名称" />
          </Form.Item>
          <Form.Item name="product_line" label="产品线">
            <Input placeholder="所属产品线" />
          </Form.Item>
          {!editing && (
            <Form.Item name="from_task_id" label="从任务拷贝 (可选)" tooltip="输入已有任务ID, 初始拷贝其特性/架构/技术设计">
              <Input placeholder="任务 ID" />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </Layout.Sider>
  );
};
