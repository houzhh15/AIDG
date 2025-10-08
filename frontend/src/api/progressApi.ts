import { ApiResponse } from '../types';
import { authedApi } from './auth';

export interface Week {
  week_number: string;       // YYYY-WW格式 (例如 "2025-05")
  week_number_int: number;   // 周编号整数
  range: string;             // 周范围 (例如 "01/29-02/04")
  summary: string;           // Markdown内容
}

export interface Month {
  month: string;             // 月份字符串 (例如 "01")
  month_number: number;      // 月份数字 (1-12)
  name: string;              // 月份名称 (例如 "2025年1月")
  summary: string;           // Markdown内容
  weeks: Week[];
}

export interface Quarter {
  quarter: string;           // 季度字符串 (例如 "Q1")
  quarter_number: number;    // 季度数字 (1-4)
  summary: string;           // Markdown内容
  months: Month[];
}

export interface YearProgress {
  year: number;
  quarters: Quarter[];
}

export interface WeekProgress {
  year: number;
  week_number: string;       // YYYY-WW格式
  week_range: string;        // 周范围
  quarter: Quarter;
  month: Month;
  week: Week;
}

export interface UpdateWeekProgressRequest {
  quarter_summary?: string;
  month_summary?: string;
  week_summary?: string;
}

const BASE_URL = '/projects';

// 获取指定周的进展
// weekNumber格式: "2025-05" (ISO 8601周编号)
export async function fetchWeekProgress(
  projectId: string,
  weekNumber: string
): Promise<ApiResponse<WeekProgress>> {
  const response = await authedApi.get<ApiResponse<WeekProgress>>(
    `${BASE_URL}/${projectId}/progress/week/${weekNumber}`
  );
  return response.data;
}

// 更新指定周的进展
// weekNumber格式: "2025-05" (ISO 8601周编号)
export async function updateWeekProgress(
  projectId: string,
  weekNumber: string,
  data: UpdateWeekProgressRequest
): Promise<ApiResponse<void>> {
  const response = await authedApi.put<ApiResponse<void>>(
    `${BASE_URL}/${projectId}/progress/week/${weekNumber}`,
    data
  );
  return response.data;
}

// 获取指定年的完整进展树
export async function fetchYearProgress(
  projectId: string,
  year: number
): Promise<ApiResponse<YearProgress>> {
  const response = await authedApi.get<ApiResponse<YearProgress>>(
    `${BASE_URL}/${projectId}/progress/year/${year}`
  );
  return response.data;
}
