import React, { useState, useEffect, useMemo } from 'react';
import { User, UserSource, getUsers, createUser, updateUserScopes, deleteUser, disableUser, enableUser, AVAILABLE_SCOPES } from '../api/users';
import UserProjectRolesPanel from './UserProjectRolesPanel';

interface UserManagementProps {
  className?: string;
}

const UserManagement: React.FC<UserManagementProps> = ({ className = '' }) => {
  const [users, setUsers] = useState<User[]>([]);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string>('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [searchText, setSearchText] = useState('');
  
  // 新增筛选状态
  const [filterSource, setFilterSource] = useState<UserSource | 'all'>('all');
  const [filterStatus, setFilterStatus] = useState<'all' | 'active' | 'disabled'>('all');

  // 加载用户列表
  const loadUsers = async () => {
    try {
      setLoading(true);
      const response = await getUsers();
      if (response.success && response.data) {
        setUsers(response.data);
      } else {
        setError(response.message || '获取用户列表失败');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '获取用户列表失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadUsers();
  }, []);

  // 创建用户
  const handleCreateUser = async () => {
    if (!newUsername.trim()) {
      setError('用户名不能为空');
      return;
    }

    try {
      setLoading(true);
      const response = await createUser({
        username: newUsername.trim(),
        password: newPassword || undefined,
        scopes: [],
      });

      if (response.success && response.data) {
        setUsers([...users, response.data]);
        setShowCreateModal(false);
        setNewUsername('');
        setNewPassword('');
        setError('');
      } else {
        setError(response.message || '创建用户失败');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '创建用户失败');
    } finally {
      setLoading(false);
    }
  };

  // 更新用户权限
  const handleUpdateUserScopes = async (username: string, scopes: string[]) => {
    try {
      setLoading(true);
      const response = await updateUserScopes(username, { scopes });

      if (response.success && response.data) {
        setUsers(users.map(u => u.username === username ? response.data! : u));
        setSelectedUser(response.data);
        setError('');
      } else {
        setError(response.message || '更新用户权限失败');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '更新用户权限失败');
    } finally {
      setLoading(false);
    }
  };

  // 删除用户
  const handleDeleteUser = async (username: string) => {
    if (!confirm(`确定要删除用户 ${username} 吗？`)) {
      return;
    }

    try {
      setLoading(true);
      const response = await deleteUser(username);

      if (response.success) {
        setUsers(users.filter(u => u.username !== username));
        if (selectedUser?.username === username) {
          setSelectedUser(null);
        }
        setError('');
      } else {
        setError(response.message || '删除用户失败');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '删除用户失败');
    } finally {
      setLoading(false);
    }
  };

  // 权限复选框变更处理
  const handleScopeChange = (scope: string, checked: boolean) => {
    if (!selectedUser) return;

    const newScopes = checked
      ? [...selectedUser.scopes, scope]
      : selectedUser.scopes.filter(s => s !== scope);

    // 乐观更新UI
    setSelectedUser({ ...selectedUser, scopes: newScopes });
    setUsers(users.map(u => u.username === selectedUser.username ? { ...u, scopes: newScopes } : u));

    // 调用API更新
    handleUpdateUserScopes(selectedUser.username, newScopes);
  };

  // 禁用用户
  const handleDisableUser = async (username: string) => {
    if (!confirm(`确定要禁用用户 ${username} 吗？禁用后该用户将无法登录。`)) {
      return;
    }

    try {
      setLoading(true);
      const response = await disableUser(username);

      if (response.success) {
        await loadUsers();
        setError('');
      } else {
        setError(response.message || '禁用用户失败');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '禁用用户失败');
    } finally {
      setLoading(false);
    }
  };

  // 启用用户
  const handleEnableUser = async (username: string) => {
    try {
      setLoading(true);
      const response = await enableUser(username);

      if (response.success) {
        await loadUsers();
        setError('');
      } else {
        setError(response.message || '启用用户失败');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '启用用户失败');
    } finally {
      setLoading(false);
    }
  };

  // 过滤用户列表
  const filteredUsers = useMemo(() => {
    return users.filter(user => {
      // 名称搜索
      if (searchText && !user.username.toLowerCase().includes(searchText.toLowerCase())) {
        return false;
      }
      // 来源筛选
      if (filterSource !== 'all') {
        const userSource = user.source || 'local';
        if (userSource !== filterSource) {
          return false;
        }
      }
      // 状态筛选
      if (filterStatus !== 'all') {
        const isDisabled = user.disabled === true;
        if (filterStatus === 'active' && isDisabled) {
          return false;
        }
        if (filterStatus === 'disabled' && !isDisabled) {
          return false;
        }
      }
      return true;
    });
  }, [users, searchText, filterSource, filterStatus]);

  return (
    <div className={`user-management ${className}`}>
      {error && (
        <div className="error-banner" style={{ 
          background: '#fee', 
          color: '#c00', 
          padding: '10px', 
          marginBottom: '10px', 
          borderRadius: '4px' 
        }}>
          {error}
        </div>
      )}

      <div className="user-management-container" style={{ display: 'flex', height: 'calc(100vh - 120px)' }}>
        {/* 左侧用户列表 */}
        <div className="user-list-panel" style={{
          width: '300px',
          borderRight: '1px solid #ddd',
          padding: '10px',
          overflow: 'auto'
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '10px' }}>
            <h3>用户列表</h3>
            <button
              onClick={() => setShowCreateModal(true)}
              disabled={loading}
              style={{
                padding: '5px 10px',
                backgroundColor: '#007bff',
                color: 'white',
                border: 'none',
                borderRadius: '4px',
                cursor: 'pointer'
              }}
            >
              新建用户
            </button>
          </div>

          {/* 搜索框 */}
          <div style={{ marginBottom: '10px' }}>
            <input
              type="text"
              placeholder="搜索用户名..."
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              style={{
                width: '100%',
                padding: '8px',
                border: '1px solid #ddd',
                borderRadius: '4px',
                fontSize: '14px'
              }}
            />
          </div>

          {/* 筛选下拉框 */}
          <div style={{ marginBottom: '10px', display: 'flex', gap: '8px' }}>
            <select
              value={filterSource}
              onChange={(e) => setFilterSource(e.target.value as UserSource | 'all')}
              style={{
                flex: 1,
                padding: '6px',
                border: '1px solid #ddd',
                borderRadius: '4px',
                fontSize: '13px'
              }}
            >
              <option value="all">全部来源</option>
              <option value="local">本地用户</option>
              <option value="external">外部用户</option>
            </select>
            <select
              value={filterStatus}
              onChange={(e) => setFilterStatus(e.target.value as 'all' | 'active' | 'disabled')}
              style={{
                flex: 1,
                padding: '6px',
                border: '1px solid #ddd',
                borderRadius: '4px',
                fontSize: '13px'
              }}
            >
              <option value="all">全部状态</option>
              <option value="active">正常</option>
              <option value="disabled">已禁用</option>
            </select>
          </div>

          {loading && users.length === 0 ? (
            <div>加载中...</div>
          ) : (
            <div className="user-list">
              {filteredUsers.map(user => {
                const isExternal = user.source === 'external';
                const isDisabled = user.disabled === true;
                
                return (
                  <div
                    key={user.username}
                    className={`user-item ${selectedUser?.username === user.username ? 'selected' : ''}`}
                    onClick={() => setSelectedUser(user)}
                    style={{
                      padding: '10px',
                      cursor: 'pointer',
                      backgroundColor: selectedUser?.username === user.username ? '#e3f2fd' : 'white',
                      borderRadius: '4px',
                      marginBottom: '5px',
                      border: '1px solid #ddd',
                      opacity: isDisabled ? 0.6 : 1
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <span style={{ fontWeight: 'bold' }}>{user.username}</span>
                      {isExternal && (
                        <span style={{
                          fontSize: '10px',
                          padding: '1px 5px',
                          backgroundColor: '#1890ff',
                          color: 'white',
                          borderRadius: '3px'
                        }}>
                          外部
                        </span>
                      )}
                      {isDisabled && (
                        <span style={{
                          fontSize: '10px',
                          padding: '1px 5px',
                          backgroundColor: '#ff4d4f',
                          color: 'white',
                          borderRadius: '3px'
                        }}>
                          已禁用
                        </span>
                      )}
                    </div>
                    {user.fullname && (
                      <div style={{ fontSize: '12px', color: '#333' }}>
                        {user.fullname}
                      </div>
                    )}
                    {isExternal && user.idp_name && (
                      <div style={{ fontSize: '11px', color: '#666' }}>
                        来源: {user.idp_name}
                      </div>
                    )}
                    {user.email && (
                      <div style={{ fontSize: '11px', color: '#666' }}>
                        邮箱: {user.email}
                      </div>
                    )}
                    <div style={{ fontSize: '11px', color: '#999' }}>
                      权限: {user.scopes.length} 个
                    </div>
                    <div style={{ fontSize: '11px', color: '#999' }}>
                      创建时间: {new Date(user.created_at).toLocaleDateString()}
                    </div>
                    {user.last_login_at && (
                      <div style={{ fontSize: '11px', color: '#999' }}>
                        最后登录: {new Date(user.last_login_at).toLocaleString()}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* 右侧权限设置 */}
        <div className="user-details-panel" style={{ flex: 1, padding: '10px' }}>
          {selectedUser ? (
            <div className="user-details">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
                <h3>用户权限设置: {selectedUser.username}</h3>
                <div style={{ display: 'flex', gap: '8px' }}>
                  {/* 本地用户显示删除按钮 */}
                  {selectedUser.source !== 'external' && (
                    <button
                      onClick={() => handleDeleteUser(selectedUser.username)}
                      disabled={loading}
                      style={{
                        padding: '5px 10px',
                        backgroundColor: '#dc3545',
                        color: 'white',
                        border: 'none',
                        borderRadius: '4px',
                        cursor: 'pointer'
                      }}
                    >
                      删除用户
                    </button>
                  )}
                  {/* 外部用户显示禁用/启用按钮 */}
                  {selectedUser.source === 'external' && (
                    selectedUser.disabled ? (
                      <button
                        onClick={() => handleEnableUser(selectedUser.username)}
                        disabled={loading}
                        style={{
                          padding: '5px 10px',
                          backgroundColor: '#52c41a',
                          color: 'white',
                          border: 'none',
                          borderRadius: '4px',
                          cursor: 'pointer'
                        }}
                      >
                        启用用户
                      </button>
                    ) : (
                      <button
                        onClick={() => handleDisableUser(selectedUser.username)}
                        disabled={loading}
                        style={{
                          padding: '5px 10px',
                          backgroundColor: '#faad14',
                          color: 'white',
                          border: 'none',
                          borderRadius: '4px',
                          cursor: 'pointer'
                        }}
                      >
                        禁用用户
                      </button>
                    )
                  )}
                </div>
              </div>

              <div className="user-info" style={{ marginBottom: '20px', padding: '10px', backgroundColor: '#f8f9fa', borderRadius: '4px' }}>
                <p><strong>用户名:</strong> {selectedUser.username}</p>
                {selectedUser.fullname && <p><strong>姓名:</strong> {selectedUser.fullname}</p>}
                {selectedUser.email && <p><strong>邮箱:</strong> {selectedUser.email}</p>}
                <p><strong>来源:</strong> {selectedUser.source === 'external' ? '外部用户' : '本地用户'}</p>
                {selectedUser.source === 'external' && selectedUser.idp_name && (
                  <p><strong>身份源:</strong> {selectedUser.idp_name}</p>
                )}
                {selectedUser.disabled !== undefined && (
                  <p><strong>状态:</strong> {selectedUser.disabled ? '已禁用' : '正常'}</p>
                )}
                <p><strong>创建时间:</strong> {new Date(selectedUser.created_at).toLocaleString()}</p>
                <p><strong>更新时间:</strong> {new Date(selectedUser.updated_at).toLocaleString()}</p>
                {selectedUser.last_login_at && (
                  <p><strong>最后登录:</strong> {new Date(selectedUser.last_login_at).toLocaleString()}</p>
                )}
                {selectedUser.synced_at && (
                  <p><strong>同步时间:</strong> {new Date(selectedUser.synced_at).toLocaleString()}</p>
                )}
              </div>

              <div className="scope-settings">
                <h4 style={{ marginBottom: '15px' }}>权限设置</h4>
                <div className="scope-checkboxes">
                  {AVAILABLE_SCOPES.map(scope => (
                    <div key={scope.value} style={{ marginBottom: '10px' }}>
                      <label style={{ display: 'flex', alignItems: 'center', cursor: 'pointer' }}>
                        <input
                          type="checkbox"
                          checked={selectedUser.scopes.includes(scope.value)}
                          onChange={(e) => handleScopeChange(scope.value, e.target.checked)}
                          disabled={loading}
                          style={{ marginRight: '8px' }}
                        />
                        <span>{scope.label}</span>
                      </label>
                    </div>
                  ))}
                </div>

                {loading && (
                  <div style={{ marginTop: '10px', color: '#666' }}>
                    正在更新权限...
                  </div>
                )}
              </div>

              {/* 项目角色面板 */}
              <div style={{ marginTop: '30px' }}>
                <UserProjectRolesPanel username={selectedUser.username} />
              </div>
            </div>
          ) : (
            <div style={{ textAlign: 'center', marginTop: '50px', color: '#666' }}>
              请选择一个用户来查看和编辑权限设置
            </div>
          )}
        </div>
      </div>

      {/* 创建用户模态框 */}
      {showCreateModal && (
        <div className="modal-overlay" style={{
          position: 'fixed',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundColor: 'rgba(0,0,0,0.5)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          zIndex: 1000
        }}>
          <div className="modal-content" style={{
            backgroundColor: 'white',
            padding: '20px',
            borderRadius: '8px',
            minWidth: '400px'
          }}>
            <h4>创建新用户</h4>
            
            <div style={{ marginBottom: '15px' }}>
              <label style={{ display: 'block', marginBottom: '5px' }}>用户名 *</label>
              <input
                type="text"
                value={newUsername}
                onChange={(e) => setNewUsername(e.target.value)}
                placeholder="输入用户名"
                style={{
                  width: '100%',
                  padding: '8px',
                  border: '1px solid #ddd',
                  borderRadius: '4px'
                }}
              />
            </div>

            <div style={{ marginBottom: '20px' }}>
              <label style={{ display: 'block', marginBottom: '5px' }}>密码 (留空使用默认密码)</label>
              <input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="输入密码或留空使用 neteye@123"
                style={{
                  width: '100%',
                  padding: '8px',
                  border: '1px solid #ddd',
                  borderRadius: '4px'
                }}
              />
            </div>

            <div style={{ display: 'flex', gap: '10px', justifyContent: 'flex-end' }}>
              <button
                onClick={() => {
                  setShowCreateModal(false);
                  setNewUsername('');
                  setNewPassword('');
                  setError('');
                }}
                disabled={loading}
                style={{
                  padding: '8px 16px',
                  backgroundColor: '#6c757d',
                  color: 'white',
                  border: 'none',
                  borderRadius: '4px',
                  cursor: 'pointer'
                }}
              >
                取消
              </button>
              <button
                onClick={handleCreateUser}
                disabled={loading}
                style={{
                  padding: '8px 16px',
                  backgroundColor: '#007bff',
                  color: 'white',
                  border: 'none',
                  borderRadius: '4px',
                  cursor: 'pointer'
                }}
              >
                {loading ? '创建中...' : '创建'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default UserManagement;