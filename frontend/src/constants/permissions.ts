/**
 * 权限常量定义
 * 
 * 与后端 constants/permissions.go 保持一致
 */

/**
 * 项目文档权限
 */
export const ScopeProjectDocRead = 'project.doc.read';
export const ScopeProjectDocWrite = 'project.doc.write';
export const ScopeProjectAdmin = 'project.admin';

/**
 * 任务权限
 */
export const ScopeTaskRead = 'task.read';
export const ScopeTaskWrite = 'task.write';
export const ScopeTaskPlanApprove = 'task.plan.approve';

/**
 * 特性权限
 */
export const ScopeFeatureRead = 'feature.read';
export const ScopeFeatureWrite = 'feature.write';

/**
 * 会议权限
 */
export const ScopeMeetingRead = 'meeting.read';
export const ScopeMeetingWrite = 'meeting.write';

/**
 * 用户管理权限
 */
export const ScopeUserManage = 'user.manage';

/**
 * 所有可用的权限范围
 */
export const AllScopes = [
  ScopeProjectDocRead,
  ScopeProjectDocWrite,
  ScopeProjectAdmin,
  ScopeTaskRead,
  ScopeTaskWrite,
  ScopeTaskPlanApprove,
  ScopeFeatureRead,
  ScopeFeatureWrite,
  ScopeMeetingRead,
  ScopeMeetingWrite,
  ScopeUserManage,
] as const;

/**
 * 权限分组 (用于前端选择器)
 */
export interface PermissionGroup {
  title: string;
  scopes: Array<{
    label: string;
    value: string;
    description: string;
  }>;
}

export const PermissionGroups: PermissionGroup[] = [
  {
    title: '项目管理',
    scopes: [
      {
        label: '项目全局管理',
        value: ScopeProjectAdmin,
        description: '创建项目、管理项目的全局配置（系统级权限）',
      },
    ],
  },
  {
    title: '项目文档',
    scopes: [
      {
        label: '读取文档',
        value: ScopeProjectDocRead,
        description: '查看项目文档 (特性列表、架构设计等)',
      },
      {
        label: '编辑文档',
        value: ScopeProjectDocWrite,
        description: '编辑项目文档 (特性列表、架构设计等)',
      },
    ],
  },
  {
    title: '任务管理',
    scopes: [
      {
        label: '查看任务',
        value: ScopeTaskRead,
        description: '查看任务详情、需求文档、设计文档',
      },
      {
        label: '编辑任务',
        value: ScopeTaskWrite,
        description: '编辑任务详情、需求文档、设计文档',
      },
      {
        label: '审批执行计划',
        value: ScopeTaskPlanApprove,
        description: '审批或拒绝任务的执行计划',
      },
    ],
  },
  {
    title: '特性管理',
    scopes: [
      {
        label: '查看特性',
        value: ScopeFeatureRead,
        description: '查看特性列表和详情',
      },
      {
        label: '编辑特性',
        value: ScopeFeatureWrite,
        description: '创建、编辑、删除特性',
      },
    ],
  },
  {
    title: '会议管理',
    scopes: [
      {
        label: '查看会议',
        value: ScopeMeetingRead,
        description: '查看会议详情、转写记录、话题',
      },
      {
        label: '编辑会议',
        value: ScopeMeetingWrite,
        description: '编辑会议信息、话题、参会权限',
      },
    ],
  },
  {
    title: '用户管理',
    scopes: [
      {
        label: '用户管理',
        value: ScopeUserManage,
        description: '管理用户、角色、权限（系统级权限）',
      },
    ],
  },
];

/**
 * 获取权限的显示标签
 */
export function getScopeLabel(scope: string): string {
  for (const group of PermissionGroups) {
    const found = group.scopes.find(s => s.value === scope);
    if (found) return found.label;
  }
  return scope;
}

/**
 * 获取权限的描述
 */
export function getScopeDescription(scope: string): string {
  for (const group of PermissionGroups) {
    const found = group.scopes.find(s => s.value === scope);
    if (found) return found.description;
  }
  return '';
}
