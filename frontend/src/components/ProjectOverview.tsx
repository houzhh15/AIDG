import React, { useState, useEffect } from 'react';
import {
  Card,
  Descriptions,
  Button,
  Modal,
  Form,
  Input,
  DatePicker,
  message,
  Spin
} from 'antd';
import {
  EditOutlined
} from '@ant-design/icons';
import dayjs from 'dayjs';
import {
  fetchProjectOverview,
  updateProjectMetadata,
  ProjectOverview as ProjectOverviewData,
  UpdateMetadataRequest
} from '../api/projectApi';

const { TextArea } = Input;

interface Props {
  projectId: string;
}

const ProjectOverview: React.FC<Props> = ({ projectId }) => {
  const [overview, setOverview] = useState<ProjectOverviewData | null>(null);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form] = Form.useForm();

  // 加载项目概述
  const loadOverview = async () => {
    setLoading(true);
    try {
      const response = await fetchProjectOverview(projectId);
      if (response.success && response.data) {
        setOverview(response.data);
      } else {
        // 对于失败响应不显示错误提示（可能是权限问题）
        console.warn('加载项目概述失败:', response.message);
      }
    } catch (error: any) {
      // 对403/500等权限/服务器错误不显示提示
      if (error?.response?.status !== 403 && error?.response?.status !== 500) {
        message.error('加载项目概述失败: ' + (error as Error).message);
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (projectId) {
      loadOverview();
    }
  }, [projectId]);

  // 打开编辑弹窗
  const handleEdit = () => {
    if (!overview) return;

    form.setFieldsValue({
      description: overview.basic_info.description,
      owner: overview.basic_info.owner,
      start_date: overview.basic_info.start_date
        ? dayjs(overview.basic_info.start_date)
        : undefined,
      estimated_end_date: overview.basic_info.estimated_end_date
        ? dayjs(overview.basic_info.estimated_end_date)
        : undefined
    });
    setModalVisible(true);
  };

  // 提交编辑
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      const data: UpdateMetadataRequest = {
        description: values.description,
        owner: values.owner,
        start_date: values.start_date
          ? values.start_date.format('YYYY-MM-DD')
          : undefined,
        estimated_end_date: values.estimated_end_date
          ? values.estimated_end_date.format('YYYY-MM-DD')
          : undefined
      };

      setSubmitting(true);
      const response = await updateProjectMetadata(projectId, data);
      if (response.success) {
        message.success('更新成功');
        setModalVisible(false);
        loadOverview();
      } else {
        message.error(response.message || '更新失败');
      }
    } catch (error) {
      message.error('更新失败: ' + (error as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <Card>
        <Spin tip="加载中...">
          <div style={{ height: '300px' }} />
        </Spin>
      </Card>
    );
  }

  if (!overview) {
    return <Card>暂无项目概述数据</Card>;
  }

  const { basic_info } = overview;

  return (
    <>
      {/* 项目基本信息 */}
      <Card
        title="项目基本信息"
        extra={
          <Button type="primary" icon={<EditOutlined />} onClick={handleEdit}>
            编辑元数据
          </Button>
        }
      >
        <Descriptions bordered column={2}>
          <Descriptions.Item label="项目ID">{basic_info.id}</Descriptions.Item>
          <Descriptions.Item label="项目名称">{basic_info.name}</Descriptions.Item>
          <Descriptions.Item label="产品线">
            {basic_info.product_line || '未设置'}
          </Descriptions.Item>
          <Descriptions.Item label="负责人">
            {basic_info.owner || '未设置'}
          </Descriptions.Item>
          <Descriptions.Item label="开始日期">
            {basic_info.start_date
              ? dayjs(basic_info.start_date).format('YYYY-MM-DD')
              : '未设置'}
          </Descriptions.Item>
          <Descriptions.Item label="预计结束日期">
            {basic_info.estimated_end_date
              ? dayjs(basic_info.estimated_end_date).format('YYYY-MM-DD')
              : '未设置'}
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">
            {dayjs(basic_info.created_at).format('YYYY-MM-DD HH:mm:ss')}
          </Descriptions.Item>
          <Descriptions.Item label="更新时间">
            {dayjs(basic_info.updated_at).format('YYYY-MM-DD HH:mm:ss')}
          </Descriptions.Item>
          <Descriptions.Item label="项目描述" span={2}>
            {basic_info.description || '无描述'}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      {/* 编辑元数据弹窗 */}
      <Modal
        title="编辑项目元数据"
        open={modalVisible}
        onCancel={() => {
          setModalVisible(false);
          form.resetFields();
        }}
        onOk={handleSubmit}
        confirmLoading={submitting}
        width={600}
        okText="保存"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item
            label="项目描述"
            name="description"
            rules={[{ max: 500, message: '描述不能超过500字符' }]}
          >
            <TextArea rows={4} placeholder="请输入项目描述" />
          </Form.Item>

          <Form.Item
            label="负责人"
            name="owner"
            rules={[{ max: 100, message: '负责人名称不能超过100字符' }]}
          >
            <Input placeholder="请输入负责人" />
          </Form.Item>

          <Form.Item label="开始日期" name="start_date">
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item label="预计结束日期" name="estimated_end_date">
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
};

export default ProjectOverview;
