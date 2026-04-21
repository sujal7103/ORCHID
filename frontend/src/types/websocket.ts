// Minimal websocket types for Orchid

export interface Model {
  id: string;
  provider_id: number;
  provider_name: string;
  provider_favicon?: string;
  name: string;
  display_name: string;
  description?: string;
  context_length?: number;
  supports_tools: boolean;
  supports_streaming: boolean;
  supports_vision: boolean;
  agents_enabled: boolean;
  provider_secure?: boolean;
  is_visible: boolean;
  smart_tool_router?: boolean;
  fetched_at?: string;
  recommendation_tier?: {
    tier: 'tier1' | 'tier2' | 'tier3' | 'tier4' | 'tier5';
    label: string;
    description: string;
    icon: string;
  };
  structured_output_support?: 'excellent' | 'good' | 'poor' | 'unknown';
  structured_output_compliance?: number;
  structured_output_warning?: string;
  structured_output_speed_ms?: number;
  structured_output_badge?: string;
}

export interface ModelsResponse {
  models: Model[];
  count: number;
  tier?: 'anonymous' | 'authenticated';
}

// ─── File attachment types (used by uploadService + agent blocks) ─────────────

export interface DataPreview {
  headers: string[];
  rows: string[][];
  row_count: number;
  col_count: number;
}

export interface ImageAttachment {
  type: 'image';
  file_id: string;
  url: string;
  mime_type: string;
  size: number;
  filename?: string;
  expired?: boolean;
}

export interface DocumentAttachment {
  type: 'document';
  file_id: string;
  url: string;
  mime_type: string;
  size: number;
  filename?: string;
  page_count?: number;
  word_count?: number;
  preview?: string;
  expired?: boolean;
}

export interface DataAttachment {
  type: 'data';
  file_id: string;
  url: string;
  mime_type: string;
  size: number;
  filename?: string;
  expired?: boolean;
  data_preview?: DataPreview;
}

export type Attachment = ImageAttachment | DocumentAttachment | DataAttachment;
