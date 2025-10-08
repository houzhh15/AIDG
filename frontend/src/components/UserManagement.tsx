import React, { useState, useEffect } from 'react';
import { User, getUsers, createUser, updateUserScopes, deleteUser, AVAILABLE_SCOPES } from '../api/users';
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

    handleUpdateUserScopes(selectedUser.username, newScopes);
  };

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

          {loading && users.length === 0 ? (
            <div>加载中...</div>
          ) : (
            <div className="user-list">
              {users.map(user => (
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
                    border: '1px solid #ddd'
                  }}
                >
                  <div style={{ fontWeight: 'bold' }}>{user.username}</div>
                  <div style={{ fontSize: '12px', color: '#666' }}>
                    权限: {user.scopes.length} 个
                  </div>
                  <div style={{ fontSize: '11px', color: '#999' }}>
                    创建时间: {new Date(user.created_at).toLocaleDateString()}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* 右侧权限设置 */}
        <div className="user-details-panel" style={{ flex: 1, padding: '10px' }}>
          {selectedUser ? (
            <div className="user-details">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px' }}>
                <h3>用户权限设置: {selectedUser.username}</h3>
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
              </div>

              <div className="user-info" style={{ marginBottom: '20px', padding: '10px', backgroundColor: '#f8f9fa', borderRadius: '4px' }}>
                <p><strong>用户名:</strong> {selectedUser.username}</p>
                <p><strong>创建时间:</strong> {new Date(selectedUser.created_at).toLocaleString()}</p>
                <p><strong>更新时间:</strong> {new Date(selectedUser.updated_at).toLocaleString()}</p>
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