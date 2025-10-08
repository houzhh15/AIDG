/**
 * Environment Configuration Module
 * Provides type-safe access to environment variables
 */

interface EnvConfig {
  apiBaseUrl: string;
  appTitle: string;
  appVersion: string;
  logLevel: 'debug' | 'info' | 'warn' | 'error';
  isDevelopment: boolean;
  isProduction: boolean;
}

/**
 * Get environment configuration
 * Supports both build-time (Vite env) and runtime (window.CONFIG) configuration
 */
export function getEnvConfig(): EnvConfig {
  // Check for runtime configuration override
  const runtimeConfig = (window as any).CONFIG;
  
  return {
    apiBaseUrl: runtimeConfig?.apiBaseUrl || import.meta.env.VITE_API_BASE_URL || '/api',
    appTitle: runtimeConfig?.appTitle || import.meta.env.VITE_APP_TITLE || 'AIDG',
    appVersion: runtimeConfig?.appVersion || import.meta.env.VITE_APP_VERSION || '1.0.0',
    logLevel: (runtimeConfig?.logLevel || import.meta.env.VITE_LOG_LEVEL || 'info') as EnvConfig['logLevel'],
    isDevelopment: import.meta.env.DEV,
    isProduction: import.meta.env.PROD,
  };
}

/**
 * Singleton config instance
 */
let config: EnvConfig | null = null;

/**
 * Get or create config instance
 */
export function getConfig(): EnvConfig {
  if (!config) {
    config = getEnvConfig();
    
    // Log config in development mode
    if (config.isDevelopment) {
      console.log('[Config] Environment configuration loaded:', config);
    }
  }
  
  return config;
}

/**
 * Export individual config values for convenience
 */
export const env = {
  get apiBaseUrl() {
    return getConfig().apiBaseUrl;
  },
  get appTitle() {
    return getConfig().appTitle;
  },
  get appVersion() {
    return getConfig().appVersion;
  },
  get logLevel() {
    return getConfig().logLevel;
  },
  get isDevelopment() {
    return getConfig().isDevelopment;
  },
  get isProduction() {
    return getConfig().isProduction;
  },
};

export default env;
