import axios, { AxiosError } from 'axios';
import { message } from 'antd';

export interface LoginResponse { token: string; username: string; scopes: string[]; }
export interface StoredAuth { token: string; username: string; scopes: string[]; }

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
    const errorData = err.response?.data as any;
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
  }
  return Promise.reject(err);
});
