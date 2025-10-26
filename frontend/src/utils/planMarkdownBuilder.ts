/**
 * planMarkdownBuilder.ts
 * 执行计划 Markdown 构建工具
 * 提供步骤重排、Markdown 生成、Frontmatter 合并功能
 */

export type StepStatus = 'pending' | 'in-progress' | 'succeeded' | 'failed' | 'cancelled';
export type StepPriority = 'high' | 'medium' | 'low';

export interface ExecutionPlanStep {
  id: string;
  description: string;
  status: StepStatus;
  priority?: StepPriority;
  dependencies?: string[];
}

export interface ExecutionPlanDependency {
  source: string;
  target: string;
}

export interface ExecutionPlanFrontmatter {
  plan_id: string;
  task_id: string;
  status: string;
  created_at: string;
  updated_at: string;
  dependencies: ExecutionPlanDependency[];
}

/**
 * 状态符号映射表
 * pending -> ' ', in-progress -> '>', succeeded -> 'x', failed -> '!', cancelled -> '~'
 */
const statusSymbolMap: Record<StepStatus, string> = {
  pending: ' ',
  'in-progress': '>',
  succeeded: 'x',
  failed: '!',
  cancelled: '~',
};

/**
 * 构建完整的 Markdown 格式执行计划
 * @param frontmatter - Frontmatter 对象
 * @param steps - 步骤数组
 * @returns Markdown 字符串
 */
export function buildMarkdown(
  frontmatter: ExecutionPlanFrontmatter,
  steps: ExecutionPlanStep[]
): string {
  const now = new Date().toISOString();
  const updatedFrontmatter = {
    ...frontmatter,
    updated_at: now,
  };

  // 生成 YAML Frontmatter
  const frontmatterLines = [
    '---',
    `plan_id: "${updatedFrontmatter.plan_id}"`,
    `task_id: "${updatedFrontmatter.task_id}"`,
    `status: "${updatedFrontmatter.status}"`,
    `created_at: "${updatedFrontmatter.created_at}"`,
    `updated_at: "${updatedFrontmatter.updated_at}"`,
  ];

  // 生成 dependencies 数组
  if (updatedFrontmatter.dependencies && updatedFrontmatter.dependencies.length > 0) {
    frontmatterLines.push('dependencies:');
    updatedFrontmatter.dependencies.forEach((dep) => {
      frontmatterLines.push(`  - { source: "${dep.source}", target: "${dep.target}" }`);
    });
  } else {
    frontmatterLines.push('dependencies: []');
  }

  frontmatterLines.push('---');

  // 生成步骤列表
  const stepLines = steps.map((step) => {
    const symbol = statusSymbolMap[step.status] || ' ';
    let line = `- [${symbol}] ${step.id}: ${step.description}`;
    
    if (step.priority) {
      line += ` priority:${step.priority}`;
    }
    
    return line;
  });

  return [...frontmatterLines, ...stepLines].join('\n');
}

/**
 * 重新编号步骤并更新依赖引用
 * @param steps - 原始步骤数组
 * @param insertIndex - 可选的插入位置（在该位置之前插入会导致后续编号变化）
 * @returns 重新编号后的步骤数组
 */
export function renumberSteps(
  steps: ExecutionPlanStep[],
  insertIndex?: number
): ExecutionPlanStep[] {
  // 创建旧ID到新ID的映射
  const idMapping: Record<string, string> = {};
  
  const renumberedSteps = steps.map((step, index) => {
    const newId = `step-${String(index + 1).padStart(2, '0')}`;
    idMapping[step.id] = newId;
    
    return {
      ...step,
      id: newId,
    };
  });

  // 更新依赖引用
  return renumberedSteps.map((step) => ({
    ...step,
    dependencies: step.dependencies?.map((dep) => idMapping[dep] || dep),
  }));
}

/**
 * 合并 Frontmatter，保留关键字段
 * @param originalContent - 原始 Markdown 内容
 * @param updates - 要更新的字段
 * @returns 更新后的 Frontmatter 对象
 */
export function mergeFrontmatter(
  originalContent: string,
  updates: Partial<ExecutionPlanFrontmatter>
): ExecutionPlanFrontmatter {
  const frontmatterMatch = originalContent.match(/^---\n([\s\S]*?)\n---/);
  
  if (!frontmatterMatch) {
    throw new Error('Invalid execution plan format: missing frontmatter');
  }

  const frontmatterText = frontmatterMatch[1];
  const lines = frontmatterText.split('\n');
  
  const parsed: Partial<ExecutionPlanFrontmatter> = {
    dependencies: [],
  };

  let inDependencies = false;
  
  for (const line of lines) {
    const trimmed = line.trim();
    
    if (trimmed.startsWith('dependencies:')) {
      inDependencies = true;
      const inline = trimmed.replace('dependencies:', '').trim();
      if (inline === '[]') {
        parsed.dependencies = [];
        inDependencies = false;
      }
      continue;
    }
    
    if (inDependencies) {
      if (trimmed.startsWith('-')) {
        // 解析依赖关系 { source: "step-01", target: "step-02" }
        const match = trimmed.match(/source:\s*["']([^"']+)["'],\s*target:\s*["']([^"']+)["']/);
        if (match) {
          parsed.dependencies!.push({
            source: match[1],
            target: match[2],
          });
        }
      } else if (trimmed && !trimmed.startsWith('-')) {
        inDependencies = false;
      }
    }
    
    if (!inDependencies) {
      const colonIndex = trimmed.indexOf(':');
      if (colonIndex > 0) {
        const key = trimmed.substring(0, colonIndex).trim();
        let value = trimmed.substring(colonIndex + 1).trim();
        
        // 移除引号
        value = value.replace(/^["']|["']$/g, '');
        
        if (key === 'plan_id') parsed.plan_id = value;
        else if (key === 'task_id') parsed.task_id = value;
        else if (key === 'status') parsed.status = value;
        else if (key === 'created_at') parsed.created_at = value;
        else if (key === 'updated_at') parsed.updated_at = value;
      }
    }
  }

  // 合并更新
  const merged: ExecutionPlanFrontmatter = {
    plan_id: parsed.plan_id || '',
    task_id: parsed.task_id || '',
    status: updates.status || parsed.status || 'Draft',
    created_at: parsed.created_at || new Date().toISOString(),
    updated_at: new Date().toISOString(),
    dependencies: updates.dependencies || parsed.dependencies || [],
  };

  return merged;
}

/**
 * 从 Markdown 内容中提取步骤列表
 * @param content - Markdown 内容
 * @returns 步骤数组
 */
export function parseStepsFromMarkdown(content: string): ExecutionPlanStep[] {
  const lines = content.split('\n');
  const steps: ExecutionPlanStep[] = [];
  let inBody = false;

  for (const line of lines) {
    if (line.trim() === '---') {
      inBody = !inBody;
      continue;
    }

    if (!inBody) continue;

    // 匹配步骤行格式: - [symbol] step-XX: description priority:xxx
    const match = line.match(/^-\s*\[([^\]]*)\]\s*([^:]+):\s*(.+)$/);
    if (match) {
      const symbol = match[1].trim();
      const id = match[2].trim();
      let description = match[3].trim();
      let priority: StepPriority | undefined;

      // 提取 priority
      const priorityMatch = description.match(/\s+priority:(high|medium|low)\s*$/);
      if (priorityMatch) {
        priority = priorityMatch[1] as StepPriority;
        description = description.replace(/\s+priority:(high|medium|low)\s*$/, '').trim();
      }

      // 反向映射符号到状态
      const status = Object.entries(statusSymbolMap).find(
        ([_, sym]) => sym === symbol
      )?.[0] as StepStatus || 'pending';

      steps.push({
        id,
        description,
        status,
        priority,
      });
    }
  }

  return steps;
}
