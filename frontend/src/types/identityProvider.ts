/**
 * 身份源类型定义
 * @module types/identityProvider
 */

// 身份源类型枚举
export type IdPType = 'OIDC' | 'LDAP';

// 身份源状态枚举
export type IdPStatus = 'Enabled' | 'Disabled';

// 同步状态枚举
export type SyncStatusType = 'running' | 'completed' | 'failed';

// 冲突策略类型
export type ConflictPolicy = 'override' | 'ignore';

// 同步间隔类型
export type SyncInterval = '1h' | '6h' | '12h' | '24h';

/**
 * OIDC 配置接口
 */
export interface OIDCConfig {
  issuer_url: string;
  client_id: string;
  client_secret?: string;      // 创建时必填，编辑时可选
  redirect_uri: string;
  scopes: string[];
  username_claim: string;
  auto_create_user: boolean;
  default_scopes: string[];
}

/**
 * LDAP 配置接口
 */
export interface LDAPConfig {
  server_url: string;
  base_dn: string;
  bind_dn: string;
  bind_password?: string;      // 创建时必填，编辑时可选
  user_filter: string;
  group_filter?: string;
  username_attribute: string;
  email_attribute: string;
  fullname_attribute: string;
  use_tls: boolean;
  skip_verify: boolean;
  auto_create_user: boolean;
  default_scopes: string[];
}

/**
 * 同步配置接口
 */
export interface SyncConfig {
  sync_enabled: boolean;
  sync_interval: SyncInterval;
  conflict_policy: ConflictPolicy;
  disable_on_remove: boolean;
}

/**
 * 身份源完整模型接口
 */
export interface IdentityProvider {
  id: string;
  name: string;
  type: IdPType;
  status: IdPStatus;
  priority: number;
  config: OIDCConfig | LDAPConfig;
  sync?: SyncConfig;           // 仅 LDAP 类型
  created_at: string;
  updated_at: string;
}

/**
 * 公开身份源接口（无需认证获取，用于登录页展示）
 */
export interface PublicIdentityProvider {
  id: string;
  name: string;
  type: IdPType;
  priority: number;
}

/**
 * 连接测试结果接口
 */
export interface TestResult {
  success: boolean;
  message: string;
  details?: Record<string, any>;
}

/**
 * 同步统计接口
 */
export interface SyncStats {
  total_fetched: number;
  created: number;
  updated: number;
  disabled: number;
  skipped: number;
  errors: number;
}

/**
 * 同步日志接口
 */
export interface SyncLog {
  sync_id: string;
  idp_id: string;
  started_at: string;
  finished_at: string;
  status: SyncStatusType;
  stats: SyncStats;
  error?: string;
}

/**
 * 同步状态接口
 */
export interface SyncStatus {
  is_running: boolean;
  last_sync?: SyncLog;
}

/**
 * 创建身份源请求接口
 */
export interface CreateIdPRequest {
  name: string;
  type: IdPType;
  status: IdPStatus;
  priority: number;
  config: OIDCConfig | LDAPConfig;
  sync?: SyncConfig;
}

/**
 * 更新身份源请求接口
 */
export interface UpdateIdPRequest extends Partial<CreateIdPRequest> {}

/**
 * 测试连接请求接口
 */
export interface TestConnectionRequest {
  id?: string;  // 可选：编辑模式下传入已保存的身份源 ID，用于复用密码
  type: IdPType;
  config: OIDCConfig | LDAPConfig;
}

/**
 * 同步结果接口
 */
export interface SyncResult {
  sync_id: string;
  status: SyncStatusType;
  stats: SyncStats;
  error?: string;
}
