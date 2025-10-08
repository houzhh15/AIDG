import React, { useState, useEffect, useRef, useCallback } from 'react';
import { Card, Button, Select, Typography, Space, Tooltip, Modal, message } from 'antd';
import { FullscreenOutlined, ReloadOutlined, SaveOutlined, ZoomInOutlined, ZoomOutOutlined } from '@ant-design/icons';

const { Text } = Typography;
const { Option } = Select;

// 节点类型定义
interface GraphNode {
  id: string;
  label: string;
  type: 'architecture' | 'tech_design' | 'requirements' | 'task' | 'meeting';
  x?: number;
  y?: number;
  style?: any;
}

// 边类型定义
interface GraphEdge {
  id: string;
  source: string;
  target: string;
  type: 'inherits' | 'implements' | 'references' | 'depends_on' | 'related_to';
  label?: string;
  style?: any;
}

interface RelationshipGraphProps {
  projectId: string;
  nodes: GraphNode[];
  edges: GraphEdge[];
  loading?: boolean;
  onNodeSelect?: (nodeId: string) => void;
  onEdgeSelect?: (edgeId: string) => void;
  onLayoutChange?: (nodes: GraphNode[], edges: GraphEdge[]) => void;
}

// 模拟的简单图形渲染引擎
class SimpleGraph {
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private nodes: GraphNode[] = [];
  private edges: GraphEdge[] = [];
  private scale = 1;
  private offsetX = 0;
  private offsetY = 0;
  private isDragging = false;
  private dragStart = { x: 0, y: 0 };
  private selectedNode: string | null = null;
  private mouseDownHandler: (event: MouseEvent) => void;
  private mouseMoveHandler: (event: MouseEvent) => void;
  private mouseUpHandler: () => void;
  private wheelHandler: (event: WheelEvent) => void;
  
  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas;
    this.ctx = canvas.getContext('2d')!;
    this.mouseDownHandler = this.handleMouseDown.bind(this);
    this.mouseMoveHandler = this.handleMouseMove.bind(this);
    this.mouseUpHandler = this.handleMouseUp.bind(this);
    this.wheelHandler = this.handleWheel.bind(this);
    this.setupEventListeners();
  }

  private setupEventListeners() {
    this.canvas.addEventListener('mousedown', this.mouseDownHandler);
    this.canvas.addEventListener('mousemove', this.mouseMoveHandler);
    this.canvas.addEventListener('mouseup', this.mouseUpHandler);
    this.canvas.addEventListener('wheel', this.wheelHandler);
  }

  dispose() {
    this.canvas.removeEventListener('mousedown', this.mouseDownHandler);
    this.canvas.removeEventListener('mousemove', this.mouseMoveHandler);
    this.canvas.removeEventListener('mouseup', this.mouseUpHandler);
    this.canvas.removeEventListener('wheel', this.wheelHandler);
  }

  private handleMouseDown(event: MouseEvent) {
    const rect = this.canvas.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    
    const clickedNode = this.getNodeAt(x, y);
    if (clickedNode) {
      this.selectedNode = clickedNode.id;
      this.render();
    } else {
      this.isDragging = true;
      this.dragStart = { x: event.clientX, y: event.clientY };
    }
  }

  private handleMouseMove(event: MouseEvent) {
    if (this.isDragging) {
      const dx = event.clientX - this.dragStart.x;
      const dy = event.clientY - this.dragStart.y;
      this.offsetX += dx;
      this.offsetY += dy;
      this.dragStart = { x: event.clientX, y: event.clientY };
      this.render();
    }
  }

  private handleMouseUp() {
    this.isDragging = false;
  }

  private handleWheel(event: WheelEvent) {
    event.preventDefault();
    const scaleFactor = event.deltaY < 0 ? 1.1 : 0.9;
    this.scale *= scaleFactor;
    this.scale = Math.max(0.1, Math.min(3, this.scale));
    this.render();
  }

  private getNodeAt(x: number, y: number): GraphNode | null {
    for (const node of this.nodes) {
      const nodeX = (node.x! + this.offsetX) * this.scale;
      const nodeY = (node.y! + this.offsetY) * this.scale;
      const radius = 30 * this.scale;
      
      const distance = Math.sqrt((x - nodeX) ** 2 + (y - nodeY) ** 2);
      if (distance <= radius) {
        return node;
      }
    }
    return null;
  }

  setData(nodes: GraphNode[], edges: GraphEdge[]) {
    this.nodes = nodes.map((node, index) => ({
      ...node,
      x: node.x || 100 + (index % 3) * 200,
      y: node.y || 100 + Math.floor(index / 3) * 150
    }));
    this.edges = edges;
    this.render();
  }

  render() {
    const { width, height } = this.canvas;
    this.ctx.clearRect(0, 0, width, height);
    
    // 绘制连线
    this.edges.forEach(edge => {
      const sourceNode = this.nodes.find(n => n.id === edge.source);
      const targetNode = this.nodes.find(n => n.id === edge.target);
      
      if (sourceNode && targetNode) {
        this.drawEdge(sourceNode, targetNode, edge);
      }
    });
    
    // 绘制节点
    this.nodes.forEach(node => {
      this.drawNode(node);
    });
  }

  private drawNode(node: GraphNode) {
    const x = (node.x! + this.offsetX) * this.scale;
    const y = (node.y! + this.offsetY) * this.scale;
    const radius = 30 * this.scale;
    
    // 节点颜色配置
    const nodeColors = {
      architecture: '#1890ff',
      tech_design: '#52c41a',
      requirements: '#fa8c16',
      task: '#eb2f96',
      meeting: '#722ed1'
    };
    
    // 绘制节点圆形
    this.ctx.beginPath();
    this.ctx.arc(x, y, radius, 0, 2 * Math.PI);
    this.ctx.fillStyle = nodeColors[node.type] || '#666';
    this.ctx.fill();
    
    if (this.selectedNode === node.id) {
      this.ctx.strokeStyle = '#ff4d4f';
      this.ctx.lineWidth = 3;
      this.ctx.stroke();
    }
    
    // 绘制节点标签
    this.ctx.fillStyle = '#fff';
    this.ctx.font = `${12 * this.scale}px Arial`;
    this.ctx.textAlign = 'center';
    this.ctx.textBaseline = 'middle';
    this.ctx.fillText(node.label.substring(0, 8), x, y);
  }

  private drawEdge(sourceNode: GraphNode, targetNode: GraphNode, edge: GraphEdge) {
    const sourceX = (sourceNode.x! + this.offsetX) * this.scale;
    const sourceY = (sourceNode.y! + this.offsetY) * this.scale;
    const targetX = (targetNode.x! + this.offsetX) * this.scale;
    const targetY = (targetNode.y! + this.offsetY) * this.scale;
    
    // 边类型颜色配置
    const edgeColors = {
      inherits: '#1890ff',
      implements: '#52c41a',
      references: '#fa8c16',
      depends_on: '#ff4d4f',
      related_to: '#722ed1'
    };
    
    this.ctx.beginPath();
    this.ctx.moveTo(sourceX, sourceY);
    this.ctx.lineTo(targetX, targetY);
    this.ctx.strokeStyle = edgeColors[edge.type] || '#666';
    this.ctx.lineWidth = 2;
    this.ctx.stroke();
    
    // 绘制箭头
    this.drawArrow(sourceX, sourceY, targetX, targetY);
  }

  private drawArrow(fromX: number, fromY: number, toX: number, toY: number) {
    const headlen = 10 * this.scale;
    const angle = Math.atan2(toY - fromY, toX - fromX);
    
    this.ctx.beginPath();
    this.ctx.moveTo(toX, toY);
    this.ctx.lineTo(toX - headlen * Math.cos(angle - Math.PI / 6), toY - headlen * Math.sin(angle - Math.PI / 6));
    this.ctx.moveTo(toX, toY);
    this.ctx.lineTo(toX - headlen * Math.cos(angle + Math.PI / 6), toY - headlen * Math.sin(angle + Math.PI / 6));
    this.ctx.stroke();
  }

  zoomIn() {
    this.scale *= 1.2;
    this.scale = Math.min(3, this.scale);
    this.render();
  }

  zoomOut() {
    this.scale *= 0.8;
    this.scale = Math.max(0.1, this.scale);
    this.render();
  }

  reset() {
    this.scale = 1;
    this.offsetX = 0;
    this.offsetY = 0;
    this.selectedNode = null;
    this.render();
  }
}

const RelationshipGraph: React.FC<RelationshipGraphProps> = ({
  projectId,
  nodes,
  edges,
  loading = false,
  onNodeSelect,
  onEdgeSelect,
  onLayoutChange
}) => {
  const graphRef = useRef<SimpleGraph | null>(null);
  const [fullscreen, setFullscreen] = useState<boolean>(false);
  const [layoutType, setLayoutType] = useState<string>('force');
  const latestDataRef = useRef<{ nodes: GraphNode[]; edges: GraphEdge[] }>({ nodes, edges });

  useEffect(() => {
    latestDataRef.current = { nodes, edges };
    graphRef.current?.setData(nodes, edges);
  }, [nodes, edges]);

  const setCanvasRef = useCallback((node: HTMLCanvasElement | null) => {
    if (node) {
      graphRef.current = new SimpleGraph(node);
      graphRef.current.setData(latestDataRef.current.nodes, latestDataRef.current.edges);
    } else {
      graphRef.current?.dispose();
      graphRef.current = null;
    }
  }, []);

  const handleZoomIn = () => {
    graphRef.current?.zoomIn();
  };

  const handleZoomOut = () => {
    graphRef.current?.zoomOut();
  };

  const handleReset = () => {
    graphRef.current?.reset();
  };

  const handleFullscreen = () => {
    setFullscreen(true);
  };

  const handleSaveLayout = () => {
    // 保存当前布局
    onLayoutChange?.(nodes, edges);
    message.success('布局已保存');
  };

  const handleLayoutChange = (newLayoutType: string) => {
    setLayoutType(newLayoutType);
    // 这里可以实现不同的布局算法
    message.info(`切换到${newLayoutType}布局`);
  };

  const graphContent = (
    <div style={{ position: 'relative', width: '100%', height: fullscreen ? '100vh' : '500px' }}>
      <div style={{ 
        position: 'absolute', 
        top: 8, 
        left: 8, 
        zIndex: 10,
        display: 'flex',
        gap: 8,
        flexWrap: 'wrap'
      }}>
        <Select
          size="small"
          value={layoutType}
          onChange={handleLayoutChange}
          style={{ width: 120 }}
        >
          <Option value="force">力导向图</Option>
          <Option value="circular">环形布局</Option>
          <Option value="hierarchical">层次布局</Option>
          <Option value="grid">网格布局</Option>
        </Select>
        
        <Space>
          <Button size="small" icon={<ZoomInOutlined />} onClick={handleZoomIn} />
          <Button size="small" icon={<ZoomOutOutlined />} onClick={handleZoomOut} />
          <Button size="small" icon={<ReloadOutlined />} onClick={handleReset} />
          <Button size="small" icon={<SaveOutlined />} onClick={handleSaveLayout} />
          {!fullscreen && (
            <Button size="small" icon={<FullscreenOutlined />} onClick={handleFullscreen} />
          )}
        </Space>
      </div>

      <canvas
        ref={setCanvasRef}
        width={fullscreen ? window.innerWidth : 800}
        height={fullscreen ? window.innerHeight : 500}
        style={{ 
          width: '100%', 
          height: '100%',
          border: '1px solid #d9d9d9',
          borderRadius: 6,
          cursor: 'grab'
        }}
      />

      {/* 图例 */}
      <div style={{
        position: 'absolute',
        bottom: 8,
        right: 8,
        background: 'rgba(255, 255, 255, 0.9)',
        padding: 8,
        borderRadius: 4,
        fontSize: 12
      }}>
        <div><span style={{color: '#1890ff'}}>●</span> 架构文档</div>
        <div><span style={{color: '#52c41a'}}>●</span> 技术方案</div>
        <div><span style={{color: '#fa8c16'}}>●</span> 需求文档</div>
        <div><span style={{color: '#eb2f96'}}>●</span> 任务</div>
        <div><span style={{color: '#722ed1'}}>●</span> 会议</div>
      </div>
    </div>
  );

  if (fullscreen) {
    return (
      <Modal
        title="关系图全屏视图"
        open={fullscreen}
        onCancel={() => setFullscreen(false)}
        footer={null}
        width="100vw"
        style={{ top: 0, maxWidth: 'none' }}
        bodyStyle={{ padding: 0, height: '100vh' }}
      >
        {graphContent}
      </Modal>
    );
  }

  return (
    <Card
      title={
        <Space>
          <Text strong style={{ fontSize: 14 }}>文档关系图</Text>
          <Tooltip title="显示文档之间的引用和依赖关系">
            <Button type="text" size="small" />
          </Tooltip>
        </Space>
      }
      loading={loading}
      bodyStyle={{ padding: 0 }}
    >
      {graphContent}
    </Card>
  );
};

export default RelationshipGraph;