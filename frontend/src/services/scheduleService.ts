/**
 * Schedule service
 * Handles backend API calls for agent scheduling (cron-based execution)
 */

import { api } from './api';

// ============================================================================
// Types
// ============================================================================

// Backend returns camelCase field names
export interface Schedule {
  id: string;
  agentId: string;
  userId?: string;
  cronExpression: string;
  timezone: string;
  enabled: boolean;
  inputTemplate: Record<string, unknown>;
  nextRunAt: string | null;
  lastRunAt: string | null;
  totalRuns: number;
  failedRuns: number;
  createdAt: string;
  updatedAt: string;
}

// Backend expects camelCase field names
export interface CreateScheduleRequest {
  cron_expression: string;
  timezone?: string;
  input_template?: Record<string, unknown>;
  enabled?: boolean;
}

export interface UpdateScheduleRequest {
  cron_expression?: string;
  timezone?: string;
  input_template?: Record<string, unknown>;
  enabled?: boolean;
}

export interface TriggerScheduleResponse {
  execution_id?: string;
  message: string;
}

export interface ScheduleUsage {
  active: number;
  paused: number;
  total: number;
  limit: number;
  canCreate: boolean;
}

// ============================================================================
// Common cron presets for UI
// ============================================================================

export const CRON_PRESETS = [
  { label: 'Every hour', value: '0 * * * *' },
  { label: 'Every 6 hours', value: '0 */6 * * *' },
  { label: 'Daily at 9 AM', value: '0 9 * * *' },
  { label: 'Daily at midnight', value: '0 0 * * *' },
  { label: 'Weekly on Monday', value: '0 9 * * 1' },
  { label: 'Monthly on the 1st', value: '0 9 1 * *' },
  { label: 'Custom', value: 'custom' },
];

export const COMMON_TIMEZONES = [
  // UTC
  { label: 'UTC (Coordinated Universal Time)', value: 'UTC' },

  // Americas
  { label: 'US Eastern (New York)', value: 'America/New_York' },
  { label: 'US Central (Chicago)', value: 'America/Chicago' },
  { label: 'US Mountain (Denver)', value: 'America/Denver' },
  { label: 'US Pacific (Los Angeles)', value: 'America/Los_Angeles' },
  { label: 'US Alaska', value: 'America/Anchorage' },
  { label: 'US Hawaii', value: 'Pacific/Honolulu' },
  { label: 'Canada Eastern (Toronto)', value: 'America/Toronto' },
  { label: 'Canada Pacific (Vancouver)', value: 'America/Vancouver' },
  { label: 'Mexico City', value: 'America/Mexico_City' },
  { label: 'São Paulo (Brazil)', value: 'America/Sao_Paulo' },
  { label: 'Buenos Aires (Argentina)', value: 'America/Argentina/Buenos_Aires' },

  // Europe
  { label: 'London (UK)', value: 'Europe/London' },
  { label: 'Paris (France)', value: 'Europe/Paris' },
  { label: 'Berlin (Germany)', value: 'Europe/Berlin' },
  { label: 'Amsterdam (Netherlands)', value: 'Europe/Amsterdam' },
  { label: 'Rome (Italy)', value: 'Europe/Rome' },
  { label: 'Madrid (Spain)', value: 'Europe/Madrid' },
  { label: 'Moscow (Russia)', value: 'Europe/Moscow' },
  { label: 'Istanbul (Turkey)', value: 'Europe/Istanbul' },

  // Asia
  { label: 'Dubai (UAE)', value: 'Asia/Dubai' },
  { label: 'Mumbai (India)', value: 'Asia/Kolkata' },
  { label: 'Bangalore (India)', value: 'Asia/Kolkata' },
  { label: 'Bangkok (Thailand)', value: 'Asia/Bangkok' },
  { label: 'Singapore', value: 'Asia/Singapore' },
  { label: 'Hong Kong', value: 'Asia/Hong_Kong' },
  { label: 'Shanghai (China)', value: 'Asia/Shanghai' },
  { label: 'Tokyo (Japan)', value: 'Asia/Tokyo' },
  { label: 'Seoul (South Korea)', value: 'Asia/Seoul' },
  { label: 'Jakarta (Indonesia)', value: 'Asia/Jakarta' },
  { label: 'Manila (Philippines)', value: 'Asia/Manila' },

  // Oceania
  { label: 'Sydney (Australia)', value: 'Australia/Sydney' },
  { label: 'Melbourne (Australia)', value: 'Australia/Melbourne' },
  { label: 'Perth (Australia)', value: 'Australia/Perth' },
  { label: 'Auckland (New Zealand)', value: 'Pacific/Auckland' },

  // Africa / Middle East
  { label: 'Cairo (Egypt)', value: 'Africa/Cairo' },
  { label: 'Johannesburg (South Africa)', value: 'Africa/Johannesburg' },
  { label: 'Lagos (Nigeria)', value: 'Africa/Lagos' },
  { label: 'Tel Aviv (Israel)', value: 'Asia/Tel_Aviv' },
];

// ============================================================================
// API Functions
// ============================================================================

/**
 * Get schedule for an agent
 */
export async function getSchedule(agentId: string): Promise<Schedule | null> {
  try {
    const response = await api.get<Schedule>(`/api/agents/${agentId}/schedule`);
    return response;
  } catch (error) {
    // 404 means no schedule exists, which is valid
    if (error instanceof Error && error.message.includes('404')) {
      return null;
    }
    console.error('Failed to fetch schedule:', error);
    return null;
  }
}

/**
 * Get schedule usage stats for the current user
 */
export async function getScheduleUsage(): Promise<ScheduleUsage> {
  try {
    const response = await api.get<ScheduleUsage>('/api/schedules/usage');
    return response;
  } catch (error) {
    console.error('Failed to fetch schedule usage:', error);
    // Return default values on error
    return {
      active: 0,
      paused: 0,
      total: 0,
      limit: 5,
      canCreate: true,
    };
  }
}

/**
 * Create a schedule for an agent
 */
export async function createSchedule(
  agentId: string,
  data: CreateScheduleRequest
): Promise<Schedule> {
  // Backend expects camelCase field names
  const response = await api.post<Schedule>(`/api/agents/${agentId}/schedule`, {
    cronExpression: data.cron_expression,
    timezone: data.timezone || 'UTC',
    inputTemplate: data.input_template || {},
    enabled: data.enabled ?? true,
  });
  return response;
}

/**
 * Update an existing schedule
 */
export async function updateSchedule(
  agentId: string,
  data: UpdateScheduleRequest
): Promise<Schedule> {
  // Backend expects camelCase field names
  const payload: Record<string, unknown> = {};
  if (data.cron_expression !== undefined) payload.cronExpression = data.cron_expression;
  if (data.timezone !== undefined) payload.timezone = data.timezone;
  if (data.input_template !== undefined) payload.inputTemplate = data.input_template;
  if (data.enabled !== undefined) payload.enabled = data.enabled;

  const response = await api.put<Schedule>(`/api/agents/${agentId}/schedule`, payload);
  return response;
}

/**
 * Delete a schedule
 */
export async function deleteSchedule(agentId: string): Promise<void> {
  await api.delete(`/api/agents/${agentId}/schedule`);
}

/**
 * Trigger an immediate execution of a scheduled agent
 */
export async function triggerScheduleNow(agentId: string): Promise<TriggerScheduleResponse> {
  const response = await api.post<TriggerScheduleResponse>(
    `/api/agents/${agentId}/schedule/run`,
    {}
  );
  return response;
}

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Parse a cron expression into human-readable format
 */
export function parseCronToHuman(cron: string): string {
  const parts = cron.split(' ');
  if (parts.length !== 5) return cron;

  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts;

  // Check common patterns
  if (minute === '0' && hour === '*' && dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    return 'Every hour';
  }

  if (
    minute === '0' &&
    hour.startsWith('*/') &&
    dayOfMonth === '*' &&
    month === '*' &&
    dayOfWeek === '*'
  ) {
    const interval = hour.slice(2);
    return `Every ${interval} hours`;
  }

  if (minute === '0' && dayOfMonth === '*' && month === '*' && dayOfWeek === '*') {
    const hourNum = parseInt(hour);
    if (!isNaN(hourNum)) {
      const ampm = hourNum >= 12 ? 'PM' : 'AM';
      const hour12 = hourNum === 0 ? 12 : hourNum > 12 ? hourNum - 12 : hourNum;
      return `Daily at ${hour12}:00 ${ampm}`;
    }
  }

  if (minute === '0' && dayOfMonth === '*' && month === '*' && dayOfWeek !== '*') {
    const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
    const dayNum = parseInt(dayOfWeek);
    if (!isNaN(dayNum) && dayNum >= 0 && dayNum <= 6) {
      return `Weekly on ${days[dayNum]}`;
    }
  }

  return cron;
}

/**
 * Format next run time for display
 */
export function formatNextRun(nextRunAt: string | null): string {
  if (!nextRunAt) return 'Not scheduled';

  const date = new Date(nextRunAt);
  const now = new Date();
  const diffMs = date.getTime() - now.getTime();
  const diffMins = Math.floor(diffMs / (1000 * 60));
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffMs < 0) return 'Overdue';
  if (diffMins < 1) return 'Less than a minute';
  if (diffMins < 60) return `In ${diffMins} minute${diffMins !== 1 ? 's' : ''}`;
  if (diffHours < 24) return `In ${diffHours} hour${diffHours !== 1 ? 's' : ''}`;
  if (diffDays === 1) return 'Tomorrow';
  if (diffDays < 7) return `In ${diffDays} days`;

  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}
