import axios, { AxiosError } from 'axios';
import { message } from 'antd';

export interface LoginResponse { token: string; username: string; scopes: string[]; }
export interface StoredAuth { token: string; username: string; scopes: string[]; }
export interface IdentityProvider { id: string; name: string; type: string; }
export interface ApiErrorResponse { error: string; }

const STORAGE_KEY = 'auth_info_v1';

// Auth state change listeners
type AuthChangeListener = (auth: StoredAuth | null) => void;
const authChangeListeners: AuthChangeListener[] = [];

export function onAuthChange(listener: AuthChangeListener) {
  authChangeListeners.push(listener);
  return () => {
    const index = authChangeListeners.indexOf(listener);
    if (index > -1) authChangeListeners.splice(index, 1);
  };
}

function notifyAuthChange(auth: StoredAuth | null) {
  authChangeListeners.forEach(listener => listener(auth));
}

export function loadAuth(): StoredAuth | null {
  try { const raw = localStorage.getItem(STORAGE_KEY); if(!raw) return null; return JSON.parse(raw); } catch { return null; }
}

export function saveAuth(a: StoredAuth){ 
  localStorage.setItem(STORAGE_KEY, JSON.stringify(a)); 
  notifyAuthChange(a);
}

export function clearAuth(showMessage = true){ 
  localStorage.removeItem(STORAGE_KEY); 
  if (showMessage) {
    message.warning('登录已失效，请重新登录');
  }
  notifyAuthChange(null);
}

export async function login(username: string, password: string): Promise<StoredAuth> {
  const r = await axios.post<LoginResponse>('/api/v1/login', { Username: username, Password: password });
  const info: StoredAuth = { token: r.data.token, username: r.data.username, scopes: r.data.scopes };
  saveAuth(info); return info;
}

/**
 * 智能登录：先尝试所有启用的 LDAP 身份源，最后尝试本地认证
 * 任一成功即返回，全部失败则抛出最后一个错误
 */
export async function smartLogin(username: string, password: string): Promise<StoredAuth> {
  // 获取公开的身份源列表
  let ldapIdps: IdentityProvider[] = [];
  try {
    const res = await axios.get<{ success: boolean; data: IdentityProvider[] }>('/api/v1/identity-providers/public');
    if (res.data.success && res.data.data) {
      ldapIdps = res.data.data.filter(idp => idp.type === 'LDAP');
    }
  } catch {
    // 获取身份源失败，直接尝试本地登录
  }

  // 依次尝试 LDAP 认证
  for (const idp of ldapIdps) {
    try {
      const r = await axios.post<LoginResponse>('/api/v1/login', {
        Username: username,
        Password: password,
        idp_id: idp.id,
      });
      const info: StoredAuth = { token: r.data.token, username: r.data.username, scopes: r.data.scopes };
      saveAuth(info);
      console.log(`[Auth] Login successful via LDAP: ${idp.name}`);
      return info;
    } catch (err: unknown) {
      console.log(`[Auth] LDAP ${idp.name} auth failed, trying next...`);
      // 继续尝试下一个
    }
  }

  // 最后尝试本地认证
  const r = await axios.post<LoginResponse>('/api/v1/login', { Username: username, Password: password });
  const info: StoredAuth = { token: r.data.token, username: r.data.username, scopes: r.data.scopes };
  saveAuth(info);
  console.log('[Auth] Login successful via local auth');
  return info;
}

/**
 * 使用身份源登录（支持 LDAP 等外部认证）
 * @param username 用户名
 * @param password 密码
 * @param idpId 身份源ID（可选，不传则使用本地认证）
 */
export async function loginWithIdP(username: string, password: string, idpId?: string): Promise<StoredAuth> {
  const payload: { Username: string; Password: string; idp_id?: string } = {
    Username: username,
    Password: password,
  };
  if (idpId) {
    payload.idp_id = idpId;
  }
  const r = await axios.post<LoginResponse>('/api/v1/login', payload);
  const info: StoredAuth = { token: r.data.token, username: r.data.username, scopes: r.data.scopes };
  saveAuth(info);
  return info;
}

/**
 * 从 JWT token 字符串解析用户信息并保存
 * 用于 OIDC 回调后处理
 * @param token JWT token 字符串
 */
export function saveAuthFromToken(token: string): StoredAuth {
  // 解析 JWT payload（不验证签名，仅提取信息）
  const parts = token.split('.');
  if (parts.length !== 3) {
    throw new Error('Invalid token format');
  }
  
  try {
    // Base64 URL decode
    const payloadBase64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const payloadJson = atob(payloadBase64);
    const payload = JSON.parse(payloadJson);
    
    const info: StoredAuth = {
      token: token,
      username: payload.username || payload.sub || '',
      scopes: payload.scopes || [],
    };
    
    saveAuth(info);
    return info;
  } catch (e) {
    throw new Error('Failed to parse token: ' + (e instanceof Error ? e.message : 'unknown error'));
  }
}

export async function refreshToken(): Promise<StoredAuth> {

  const r = await authedApi.get<LoginResponse>('/me/token');
  const info: StoredAuth = { token: r.data.token, username: r.data.username, scopes: r.data.scopes };
  saveAuth(info); return info;
}

// Axios instance with auth header injection
export const authedApi = axios.create({ baseURL: '/api/v1' });

authedApi.interceptors.request.use(cfg => {
  const a = loadAuth();
  if(a && cfg.headers) cfg.headers.Authorization = `Bearer ${a.token}`;
  return cfg;
});

authedApi.interceptors.response.use(r=>r, (err: AxiosError)=>{
  if(err.response?.status === 401){ 
    const errorData = err.response?.data as ApiErrorResponse;
    const errorMsg = errorData?.error || '';
    
    // 只在真正的认证失败时清除登录状态
    // 如果是权限不足（403）或其他业务错误，不清除登录
    if (errorMsg.includes('invalid token') || errorMsg.includes('missing bearer token')) {
      clearAuth();
    } else {
      // 其他401错误，不清除登录，只显示错误信息
      console.error('Authentication error:', errorMsg);
      message.error(errorMsg || '认证错误');
    }
  } else if (err.response?.status === 403) {
    // 403 权限不足错误，不显示错误提示
    // 让组件自己决定如何处理权限错误
    console.warn('Permission denied:', err.config?.url);
    // 不调用 message.error()，避免显示错误提示
  } else if (err.response?.status === 404) {
    // 404 资源不存在错误
    // 对于会议文档（topic, meeting-context, feature-list等），404是正常的（文档未生成）
    // 不显示错误提示，让组件自己决定如何处理
    const url = err.config?.url || '';
    const isOptionalDocument = [
      '/topic', 
      '/meeting-context', 
      '/meeting-summary',
      '/feature-list', 
      '/summary', 
      '/polish',
      '/merged_all',
      '/architecture-design',
      '/segments',
      '/mapped'
    ].some(path => url.includes(path));
    
    if (isOptionalDocument) {
      // 可选文档的404是正常的，静默处理
      console.log('Optional document not found (404):', url);
    } else {
      // 其他404可能是错误，输出警告但不显示提示
      console.warn('Resource not found (404):', url);
    }
  }
  return Promise.reject(err);
});
