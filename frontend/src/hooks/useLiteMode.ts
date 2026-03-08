import { useState, useEffect } from 'react';
import axios from 'axios';

interface AppConfig {
  lite_mode: boolean;
  version: string;
}

let cachedConfig: AppConfig | null = null;
let configPromise: Promise<AppConfig> | null = null;

async function fetchAppConfig(): Promise<AppConfig> {
  if (cachedConfig) return cachedConfig;
  if (configPromise) return configPromise;
  
  configPromise = axios.get<AppConfig>('/api/v1/app/config')
    .then(res => {
      cachedConfig = res.data;
      return cachedConfig;
    })
    .catch(() => {
      // If endpoint doesn't exist, it's not lite mode
      cachedConfig = { lite_mode: false, version: '1.0.0' };
      return cachedConfig;
    });
  
  return configPromise;
}

/**
 * Hook to detect if the server is running in lite mode.
 * In lite mode:
 * - No header bar
 * - Only project view
 * - Auto-login with default credentials
 * - Simplified UI (no prompts, history, linked docs in task view)
 * - Only feature list and architecture design in project deliverables
 */
export function useLiteMode() {
  const [liteMode, setLiteMode] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchAppConfig().then(config => {
      setLiteMode(config.lite_mode);
      setLoading(false);
    });
  }, []);

  return { liteMode: liteMode === true, loading };
}

/**
 * Non-hook version for use outside React components.
 */
export function isLiteMode(): boolean {
  return cachedConfig?.lite_mode === true;
}

export { fetchAppConfig };
