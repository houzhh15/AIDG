/**
 * 权限选择器组件
 * 
 * 功能:
 * - 按分组展示所有可选权限
 * - 支持全选/取消全选
 * - 支持组级别的全选/取消
 * - Checkbox 展示权限标签和描述
 * 
 * 使用:
 * ```tsx
 * <PermissionSelector
 *   value={selectedScopes}
 *   onChange={setSelectedScopes}
 * />
 * ```
 */

import React, { useMemo } from 'react';
import { Card, Checkbox, Space, Tooltip } from 'antd';
import { InfoCircleOutlined } from '@ant-design/icons';
import { PermissionGroups, getScopeDescription } from '../../constants/permissions';
import type { CheckboxChangeEvent } from 'antd/es/checkbox';

interface PermissionSelectorProps {
  /** 已选择的权限 scopes */
  value?: string[];
  
  /** 权限变更回调 */
  onChange?: (scopes: string[]) => void;
  
  /** 是否禁用 */
  disabled?: boolean;
  
  /** 卡片样式 */
  cardStyle?: React.CSSProperties;
}

/**
 * 权限选择器组件
 */
export const PermissionSelector: React.FC<PermissionSelectorProps> = ({
  value = [],
  onChange,
  disabled = false,
  cardStyle,
}) => {
  // 所有可选权限 scopes
  const allScopes = useMemo(() => {
    return PermissionGroups.flatMap(group => group.scopes.map(item => item.value));
  }, []);

  // 检查是否全选
  const isAllChecked = useMemo(() => {
    return allScopes.length > 0 && allScopes.every(scope => value.includes(scope));
  }, [allScopes, value]);

  // 检查是否部分选中
  const isIndeterminate = useMemo(() => {
    const checkedCount = allScopes.filter(scope => value.includes(scope)).length;
    return checkedCount > 0 && checkedCount < allScopes.length;
  }, [allScopes, value]);

  // 全选/取消全选
  const handleCheckAll = (e: CheckboxChangeEvent) => {
    if (e.target.checked) {
      onChange?.(allScopes);
    } else {
      onChange?.([]);
    }
  };

  // 组级别全选/取消
  const handleCheckGroup = (groupScopes: string[], checked: boolean) => {
    if (checked) {
      // 添加组内所有权限
      const newScopes = [...new Set([...value, ...groupScopes])];
      onChange?.(newScopes);
    } else {
      // 移除组内所有权限
      const newScopes = value.filter(scope => !groupScopes.includes(scope));
      onChange?.(newScopes);
    }
  };

  // 单个权限勾选
  const handleCheckScope = (scope: string, checked: boolean) => {
    if (checked) {
      onChange?.([...value, scope]);
    } else {
      onChange?.(value.filter(s => s !== scope));
    }
  };

  // 检查组是否全选
  const isGroupChecked = (groupScopes: string[]) => {
    return groupScopes.length > 0 && groupScopes.every(scope => value.includes(scope));
  };

  // 检查组是否部分选中
  const isGroupIndeterminate = (groupScopes: string[]) => {
    const checkedCount = groupScopes.filter(scope => value.includes(scope)).length;
    return checkedCount > 0 && checkedCount < groupScopes.length;
  };

  return (
    <div className="permission-selector">
      {/* 全选 */}
      <div style={{ marginBottom: 16 }}>
        <Checkbox
          checked={isAllChecked}
          indeterminate={isIndeterminate}
          onChange={handleCheckAll}
          disabled={disabled}
        >
          <strong>全选 ({value.length}/{allScopes.length})</strong>
        </Checkbox>
      </div>

      {/* 权限分组 */}
      <Space direction="vertical" style={{ width: '100%' }} size="middle">
        {PermissionGroups.map((group, groupIndex) => {
          const groupScopes = group.scopes.map(item => item.value);
          const groupChecked = isGroupChecked(groupScopes);
          const groupIndeterminate = isGroupIndeterminate(groupScopes);

          return (
            <Card
              key={groupIndex}
              title={
                <Checkbox
                  checked={groupChecked}
                  indeterminate={groupIndeterminate}
                  onChange={(e) => handleCheckGroup(groupScopes, e.target.checked)}
                  disabled={disabled}
                >
                  <strong>{group.title}</strong>
                </Checkbox>
              }
              size="small"
              style={cardStyle}
            >
              <Space direction="vertical" style={{ width: '100%' }}>
                {group.scopes.map(item => {
                  const description = getScopeDescription(item.value);
                  
                  return (
                    <Checkbox
                      key={item.value}
                      checked={value.includes(item.value)}
                      onChange={(e) => handleCheckScope(item.value, e.target.checked)}
                      disabled={disabled}
                    >
                      <span>{item.label}</span>
                      {description && (
                        <Tooltip title={description}>
                          <InfoCircleOutlined style={{ marginLeft: 4, color: '#999' }} />
                        </Tooltip>
                      )}
                    </Checkbox>
                  );
                })}
              </Space>
            </Card>
          );
        })}
      </Space>
    </div>
  );
};

export default PermissionSelector;
