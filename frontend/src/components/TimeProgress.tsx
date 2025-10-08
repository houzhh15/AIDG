import React, { useState, useEffect } from 'react';
import {
  Card,
  Tree,
  Button,
  Input,
  message,
  Spin,
  Space,
  Typography,
  Row,
  Col,
  Empty
} from 'antd';
import {
  CalendarOutlined,
  SaveOutlined,
  ReloadOutlined,
  EditOutlined
} from '@ant-design/icons';
import type { DataNode } from 'antd/es/tree';
import dayjs from 'dayjs';
import isoWeek from 'dayjs/plugin/isoWeek';
import MarkdownViewer from './MarkdownViewer';
import {
  fetchYearProgress,
  fetchWeekProgress,
  updateWeekProgress,
  YearProgress,
  WeekProgress
} from '../api/progressApi';
import { fetchProjectOverview } from '../api/projectApi';

dayjs.extend(isoWeek);

const { TextArea } = Input;
const { Title, Text } = Typography;

interface Props {
  projectId: string;
}

interface NodeData {
  type: 'year' | 'quarter' | 'month' | 'week';
  year: number;
  quarter?: number;
  month?: number;
  week?: string; // 完整的周编号 (YYYY-WW)
  weekInt?: number; // 周数字
  summary?: string;
  content?: string;
}

const TimeProgress: React.FC<Props> = ({ projectId }) => {
  const [loading, setLoading] = useState(false);
  const [yearProgress, setYearProgress] = useState<YearProgress | null>(null);
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [selectedNode, setSelectedNode] = useState<NodeData | null>(null);
  const [weekProgress, setWeekProgress] = useState<WeekProgress | null>(null);
  const [editing, setEditing] = useState(false);
  const [editContent, setEditContent] = useState('');
  const [saving, setSaving] = useState(false);
  const [projectStartDate, setProjectStartDate] = useState<string | null>(null);

  // 加载项目概览，获取开始日期
  const loadProjectOverview = async (): Promise<string | null> => {
    try {
      const response = await fetchProjectOverview(projectId);
      if (response.success && response.data) {
        // 如果没有 start_date，使用 created_at 作为替代
        const startDate = response.data.basic_info.start_date || response.data.basic_info.created_at;
        setProjectStartDate(startDate || null);
        return startDate || null;
      }
    } catch (error) {
      console.warn('获取项目开始日期失败:', error);
    }
    return null;
  };

  // 加载年度进展树
  const loadYearProgress = async (year: number, explicitStartDate?: string | null) => {
    setLoading(true);
    try {
      const response = await fetchYearProgress(projectId, year);
      if (response.success && response.data) {
        setYearProgress(response.data);
        buildTreeData(response.data, explicitStartDate);
      } else {
        message.error(response.message || '加载进展树失败');
      }
    } catch (error: any) {
      // 对403权限错误不显示提示，让无权限页面处理
      if (error?.response?.status !== 403) {
        message.error('加载进展树失败: ' + (error as Error).message);
      }
    } finally {
      setLoading(false);
    }
  };

  // 获取某个月份中的所有周（只显示从项目开始到当前日期的周）
  const getWeeksInMonth = (year: number, month: number, existingWeeks: any[], explicitStartDate?: string | null): any[] => {
    const weeks = [];
    const firstDayOfMonth = dayjs(`${year}-${String(month).padStart(2, '0')}-01`);
    const lastDayOfMonth = firstDayOfMonth.endOf('month');
    
    // 确定开始日期：显式传入的日期 > 项目开始日期 > 3个月前
    let startDate: dayjs.Dayjs;
    const effectiveStartDate = explicitStartDate !== undefined ? explicitStartDate : projectStartDate;
    if (effectiveStartDate) {
      startDate = dayjs(effectiveStartDate);
    } else {
      startDate = dayjs().subtract(3, 'month');
    }
    
    // 当前日期
    const currentDate = dayjs();
    
    // 从该月第一天开始，以周为单位遍历
    let currentDay = firstDayOfMonth;
    const addedWeeks = new Set();
    
    while (currentDay.isBefore(lastDayOfMonth) || currentDay.isSame(lastDayOfMonth, 'day')) {
      const weekStart = currentDay.startOf('isoWeek');
      const weekEnd = currentDay.endOf('isoWeek');
      const weekNumber = currentDay.isoWeek();
      const weekKey = `${year}-${String(weekNumber).padStart(2, '0')}`;
      
      // 关键修改：只添加周一在当前月份内的周
      // 这样可以确保跨月的周只出现在包含周一的那个月份中
      if (!addedWeeks.has(weekKey) && 
          weekStart.month() + 1 === month && 
          weekStart.year() === year) {
        
        // 检查这一周是否在有效日期范围内
        if ((weekEnd.isAfter(startDate) || weekEnd.isSame(startDate, 'day')) && 
            (weekStart.isBefore(currentDate) || weekStart.isSame(currentDate, 'day'))) {
          addedWeeks.add(weekKey);
          
          // 生成周的日期范围显示名称 (格式: "M/D~M/D")
          const startFormatted = weekStart.format('M/D');
          const endFormatted = weekEnd.format('M/D');
          const displayName = `${startFormatted}~${endFormatted}`;
          
          // 查找是否有现有的周数据
          const existingWeek = existingWeeks.find(w => w.week_number === weekKey);
          weeks.push({
            week_number: weekKey,
            week_number_int: weekNumber,
            display_name: displayName,
            summary: existingWeek?.summary || ''
          });
        }
      }
      
      currentDay = currentDay.add(1, 'week');
    }
    
    return weeks;
  };

  // 构建树形结构数据（只显示包含有效周的季度和月份）
  const buildTreeData = (data: YearProgress, explicitStartDate?: string | null) => {
    const quarterNodes: DataNode[] = [];
    
    for (const quarter of data.quarters) {
      const monthNodes: DataNode[] = [];
      
      for (const month of quarter.months) {
        // 动态生成月份中的所有周（只包含在有效范围内的周）
        const allWeeks = getWeeksInMonth(data.year, month.month_number, month.weeks || [], explicitStartDate);
        
        // 只添加包含周的月份
        if (allWeeks.length > 0) {
          monthNodes.push({
            title: `${month.month_number}月`,
            key: `month-${data.year}-${quarter.quarter_number}-${month.month_number}`,
            children: allWeeks.map((week) => ({
              title: `${week.display_name}`,
              key: `week-${data.year}-${quarter.quarter_number}-${month.month_number}-${week.week_number}`,
              isLeaf: true
            }))
          });
        }
      }
      
      // 只添加包含月份的季度
      if (monthNodes.length > 0) {
        quarterNodes.push({
          title: `Q${quarter.quarter_number}`,
          key: `quarter-${data.year}-${quarter.quarter_number}`,
          children: monthNodes
        });
      }
    }
    
    // 只有当有季度节点时才显示年节点
    const nodes: DataNode[] = quarterNodes.length > 0 ? [
      {
        title: `${data.year}年`,
        key: `year-${data.year}`,
        icon: <CalendarOutlined />,
        children: quarterNodes
      }
    ] : [];
    
    setTreeData(nodes);
  };

  // 计算ISO 8601周编号 (格式: YYYY-WW)
  const getWeekNumber = (year: number, week: number): string => {
    return `${year}-${String(week).padStart(2, '0')}`;
  };

  // 加载周进展详情
  const loadWeekProgress = async (year: number, week: number) => {
    try {
      const weekNumber = getWeekNumber(year, week);
      const response = await fetchWeekProgress(projectId, weekNumber);
      if (response.success && response.data) {
        setWeekProgress(response.data);
        setEditContent(response.data.week?.summary || '');
      } else {
        message.error(response.message || '加载周进展失败');
      }
    } catch (error) {
      message.error('加载周进展失败: ' + (error as Error).message);
    }
  };

  // 树节点选择
  const handleSelect = async (selectedKeys: React.Key[], info: any) => {
    if (selectedKeys.length === 0) return;

    const key = selectedKeys[0] as string;
    const parts = key.split('-');
    const type = parts[0] as 'year' | 'quarter' | 'month' | 'week';

    const nodeData: NodeData = {
      type,
      year: parseInt(parts[1])
    };

    if (type === 'quarter') {
      nodeData.quarter = parseInt(parts[2]);
      // 季度节点：加载该季度第一个月的第一周数据以获取季度信息
      const firstWeekOfQuarter = (nodeData.quarter - 1) * 13 + 1;
      await loadWeekProgress(nodeData.year, firstWeekOfQuarter);
    } else if (type === 'month') {
      nodeData.quarter = parseInt(parts[2]);
      nodeData.month = parseInt(parts[3]);
      // 月节点：加载该月的第一周数据以获取月和季度信息
      // 需要从该月的第一天计算出周数
      const firstDayOfMonth = dayjs(`${nodeData.year}-${String(nodeData.month).padStart(2, '0')}-01`);
      const weekInt = firstDayOfMonth.isoWeek();
      nodeData.weekInt = weekInt;
      await loadWeekProgress(nodeData.year, weekInt);
    } else if (type === 'week') {
      nodeData.quarter = parseInt(parts[2]);
      nodeData.month = parseInt(parts[3]);
      // 周编号格式是 YYYY-WW，需要组合 parts[4] 和 parts[5]
      nodeData.week = `${parts[4]}-${parts[5]}`; // 例如 "2025-38"
      nodeData.weekInt = parseInt(parts[5]); // 周数字
      // 加载周进展详情
      await loadWeekProgress(nodeData.year, nodeData.weekInt);
    }

    setSelectedNode(nodeData);
    setEditing(false);
  };

  // 保存编辑
  const handleSave = async () => {
    if (!selectedNode) {
      message.warning('请选择一个节点进行编辑');
      return;
    }

    // 支持季度、月和周的编辑
    if (selectedNode.type !== 'week' && selectedNode.type !== 'month' && selectedNode.type !== 'quarter') {
      message.warning('仅支持编辑季度、月和周节点');
      return;
    }

    setSaving(true);
    try {
      // 根据节点类型构造不同的保存参数
      let weekNumberForApi: string;
      let updateData: any = {};

      if (selectedNode.type === 'week' && selectedNode.week) {
        // 周节点：使用week字段作为API参数
        weekNumberForApi = selectedNode.week;
        updateData.week_summary = editContent;
      } else if (selectedNode.type === 'month' && selectedNode.weekInt) {
        // 月节点：需要使用该月的任意一周作为定位（使用第一周）
        weekNumberForApi = getWeekNumber(selectedNode.year, selectedNode.weekInt);
        updateData.month_summary = editContent;
      } else if (selectedNode.type === 'quarter') {
        // 季度节点：使用该季度第一周作为定位
        const firstWeekOfQuarter = (selectedNode.quarter! - 1) * 13 + 1;
        weekNumberForApi = getWeekNumber(selectedNode.year, firstWeekOfQuarter);
        updateData.quarter_summary = editContent;
      } else {
        message.error('节点数据不完整，无法保存');
        return;
      }

      const response = await updateWeekProgress(
        projectId,
        weekNumberForApi,
        updateData
      );

      if (response.success) {
        message.success('保存成功');
        setEditing(false);
        // 重新加载数据
        if (selectedNode.weekInt) {
          await loadWeekProgress(selectedNode.year, selectedNode.weekInt);
        }
        await loadYearProgress(selectedNode.year, projectStartDate);
      } else {
        message.error(response.message || '保存失败');
      }
    } catch (error) {
      message.error('保存失败: ' + (error as Error).message);
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    if (projectId) {
      // 先加载项目概览获取开始日期，然后传递给加载年度进展
      loadProjectOverview().then((startDate) => {
        const currentYear = dayjs().year();
        loadYearProgress(currentYear, startDate);
      });
    }
  }, [projectId]);

  return (
    <Card
      title={
        <Space>
          <CalendarOutlined />
          <span>时间维度进展</span>
        </Space>
      }
      extra={
        <Button
          icon={<ReloadOutlined />}
          onClick={() => {
            const currentYear = dayjs().year();
            loadYearProgress(currentYear, projectStartDate);
          }}
        >
          刷新
        </Button>
      }
    >
      <Spin spinning={loading}>
        <Row gutter={16} style={{ minHeight: '600px' }}>
          {/* 左栏：树形导航 */}
          <Col span={6} style={{ borderRight: '1px solid #f0f0f0' }}>
            <Title level={5}>时间导航</Title>
            {treeData.length > 0 ? (
              <Tree
                treeData={treeData}
                onSelect={handleSelect}
                defaultExpandAll
                showIcon
              />
            ) : (
              <Empty description="暂无进展数据" />
            )}
          </Col>

          {/* 中栏：季度总结 */}
          <Col span={9} style={{ borderRight: '1px solid #f0f0f0', padding: '0 16px' }}>
            <Space style={{ marginBottom: '16px' }}>
              <Title level={5} style={{ margin: 0 }}>季度总结</Title>
              {selectedNode && selectedNode.quarter && selectedNode.type === 'quarter' && !editing && (
                <Button
                  type="primary"
                  icon={<EditOutlined />}
                  onClick={() => {
                    setEditing(true);
                    setEditContent(weekProgress?.quarter?.summary || '');
                  }}
                  size="small"
                >
                  编辑季度总结
                </Button>
              )}
              {editing && selectedNode?.type === 'quarter' && (
                <Button
                  type="primary"
                  icon={<SaveOutlined />}
                  onClick={handleSave}
                  loading={saving}
                  size="small"
                >
                  保存
                </Button>
              )}
            </Space>
            {selectedNode && selectedNode.quarter ? (
              selectedNode.type === 'quarter' && editing ? (
                <TextArea
                  value={editContent}
                  onChange={(e) => setEditContent(e.target.value)}
                  placeholder="请输入季度总结内容（支持 Markdown）"
                  rows={20}
                  style={{ fontFamily: 'monospace' }}
                />
              ) : (
                <div
                  style={{
                    padding: '12px',
                    backgroundColor: '#fafafa',
                    borderRadius: '4px',
                    maxHeight: '500px',
                    overflow: 'auto'
                  }}
                >
                  <div style={{ marginBottom: '12px', paddingBottom: '8px', borderBottom: '1px solid #d9d9d9' }}>
                    <Text strong style={{ fontSize: '16px' }}>
                      {`${selectedNode.year}年 Q${selectedNode.quarter}`}
                    </Text>
                  </div>
                  {weekProgress?.quarter?.summary ? (
                    <MarkdownViewer>{weekProgress.quarter.summary}</MarkdownViewer>
                  ) : (
                    <Text type="secondary">暂无季度总结</Text>
                  )}
                </div>
              )
            ) : (
              <Empty description="请选择时间节点查看季度总结" />
            )}
          </Col>

          {/* 右栏：月/周总结 */}
          <Col span={9} style={{ padding: '0 16px' }}>
            <Space style={{ marginBottom: '16px' }}>
              <Title level={5} style={{ margin: 0 }}>
                {selectedNode?.type === 'week' ? '周总结' : 
                 selectedNode?.type === 'month' ? '月总结' : '内容详情'}
              </Title>
              {selectedNode && !editing && (selectedNode.type === 'week' || selectedNode.type === 'month') && (
                <Button
                  type="primary"
                  icon={<EditOutlined />}
                  onClick={() => {
                    setEditing(true);
                    // 根据节点类型初始化编辑内容
                    if (selectedNode.type === 'week' && weekProgress) {
                      setEditContent(weekProgress.week?.summary || '');
                    } else if (selectedNode.type === 'month' && weekProgress) {
                      setEditContent(weekProgress.month?.summary || '');
                    } else if (selectedNode.type === 'quarter' && weekProgress) {
                      setEditContent(weekProgress.quarter?.summary || '');
                    } else {
                      setEditContent('');
                    }
                  }}
                  size="small"
                >
                  {selectedNode.type === 'week' ? '编辑周总结' : '编辑月总结'}
                </Button>
              )}
              {editing && (
                <Button
                  type="primary"
                  icon={<SaveOutlined />}
                  onClick={handleSave}
                  loading={saving}
                  size="small"
                >
                  保存
                </Button>
              )}
            </Space>

            {selectedNode ? (
              (selectedNode.type === 'week' || selectedNode.type === 'month') ? (
                editing ? (
                  <TextArea
                    value={editContent}
                    onChange={(e) => setEditContent(e.target.value)}
                    placeholder={`请输入${selectedNode.type === 'week' ? '周' : '月'}进展内容（支持 Markdown）`}
                    rows={20}
                    style={{ fontFamily: 'monospace' }}
                  />
                ) : (
                  <div
                    style={{
                      padding: '12px',
                      backgroundColor: '#fafafa',
                      borderRadius: '4px',
                      maxHeight: '500px',
                      overflow: 'auto'
                    }}
                  >
                    <div style={{ marginBottom: '12px', paddingBottom: '8px', borderBottom: '1px solid #d9d9d9' }}>
                      <Text strong style={{ fontSize: '16px' }}>
                        {selectedNode.type === 'week' && selectedNode.weekInt ? (
                          (() => {
                            const weekStart = dayjs().year(selectedNode.year).isoWeek(selectedNode.weekInt).startOf('isoWeek');
                            const weekEnd = weekStart.endOf('isoWeek');
                            return `${weekStart.format('M/D')}~${weekEnd.format('M/D')}`;
                          })()
                        ) : selectedNode.type === 'month' ? (
                          `${selectedNode.year}年${selectedNode.month}月`
                        ) : ''}
                      </Text>
                    </div>
                    {selectedNode.type === 'week' && weekProgress?.week?.summary ? (
                      <MarkdownViewer>{weekProgress.week.summary}</MarkdownViewer>
                    ) : selectedNode.type === 'month' && weekProgress?.month?.summary ? (
                      <MarkdownViewer>{weekProgress.month.summary}</MarkdownViewer>
                    ) : (
                      <Text type="secondary">暂无{selectedNode.type === 'week' ? '周' : '月'}总结内容</Text>
                    )}
                  </div>
                )
              ) : (
                <Empty 
                  description={
                    <span>
                      当前选中：<strong>{
                        selectedNode.type === 'year' ? '年' :
                        selectedNode.type === 'quarter' ? '季度' : ''
                      }</strong> 节点
                      <br />
                      请选择<strong>月</strong>或<strong>周</strong>节点查看详情
                    </span>
                  } 
                />
              )
            ) : (
              <Empty description="请先在左侧选择一个时间节点" />
            )}
          </Col>
        </Row>
      </Spin>
    </Card>
  );
};

export default TimeProgress;
