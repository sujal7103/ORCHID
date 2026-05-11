import { api } from './api';
import type {
  AdminStatusResponse,
  OverviewStats,
  ProviderAnalytics,
  ModelAnalytics,
  ChatAnalytics,
  AgentAnalytics,
  AdminProviderView,
  CreateProviderRequest,
  UpdateProviderRequest,
  AdminModelView,
  CreateModelRequest,
  UpdateModelRequest,
  ProvidersConfig,
  CapabilityTestResult,
  BenchmarkResults,
  CreateAliasRequest,
  UpdateAliasRequest,
  ModelAliasView,
  BulkUpdateAgentsRequest,
  BulkUpdateVisibilityRequest,
  BulkUpdateTierRequest,
  TierAssignment,
  UserListResponse,
  GDPRDataPolicy,
  HealthDashboardResponse,
  InsightsOverview,
  DailyMetricsResponse,
  HealthDistribution,
  ActivationFunnel,
  FeedbackStreamResponse,
  AutoPilotContext,
  CollectionStats,
  BackfillResponse,
  AutoPilotAnalysisResponse,
} from '@/types/admin';
import type { Model } from '@/types/websocket';

export const adminService = {
  // Admin Status
  getAdminStatus(): Promise<AdminStatusResponse> {
    return api.get<AdminStatusResponse>('/api/admin/me');
  },

  // Analytics
  getOverviewStats(): Promise<OverviewStats> {
    return api.get<OverviewStats>('/api/admin/analytics/overview');
  },

  getProviderAnalytics(): Promise<ProviderAnalytics[]> {
    return api.get<ProviderAnalytics[]>('/api/admin/analytics/providers');
  },

  getModelAnalytics(): Promise<ModelAnalytics[]> {
    return api.get<ModelAnalytics[]>('/api/admin/analytics/models');
  },

  getChatAnalytics(): Promise<ChatAnalytics> {
    return api.get<ChatAnalytics>('/api/admin/analytics/chats');
  },

  getAgentAnalytics(): Promise<AgentAnalytics> {
    return api.get<AgentAnalytics>('/api/admin/analytics/agents');
  },

  migrateChatSessionTimestamps(): Promise<{
    success: boolean;
    message: string;
    sessions_updated: number;
  }> {
    return api.post('/api/admin/analytics/migrate-timestamps', {});
  },

  // Provider Management
  getProviders(): Promise<ProvidersConfig> {
    return api.get<ProvidersConfig>('/api/admin/providers');
  },

  addProvider(data: CreateProviderRequest): Promise<AdminProviderView> {
    return api.post<AdminProviderView>('/api/admin/providers', data);
  },

  updateProvider(id: string, data: UpdateProviderRequest): Promise<AdminProviderView> {
    return api.put<AdminProviderView>(`/api/admin/providers/${id}`, data);
  },

  deleteProvider(id: string): Promise<void> {
    return api.delete(`/api/admin/providers/${id}`);
  },

  toggleProvider(id: string, enabled: boolean): Promise<void> {
    return api.put(`/api/admin/providers/${id}/toggle`, { enabled });
  },

  // Model Management - CRUD
  getModels(): Promise<AdminModelView[]> {
    return api.get<AdminModelView[]>('/api/admin/models');
  },

  createModel(data: CreateModelRequest): Promise<Model> {
    return api.post<Model>('/api/admin/models', data);
  },

  updateModel(modelId: string, data: UpdateModelRequest): Promise<Model> {
    return api.put<Model>(`/api/admin/models/by-id?model_id=${encodeURIComponent(modelId)}`, data);
  },

  deleteModel(modelId: string): Promise<void> {
    return api.delete(`/api/admin/models/by-id?model_id=${encodeURIComponent(modelId)}`);
  },

  // Model Fetching
  fetchModelsFromProvider(
    providerId: number
  ): Promise<{ success: boolean; models_fetched: number; message: string }> {
    return api.post(`/api/admin/providers/${providerId}/fetch`, {});
  },

  syncProviderToJSON(providerId: number): Promise<{ success: boolean; message: string }> {
    return api.post(`/api/admin/providers/${providerId}/sync`, {});
  },

  // Model Testing
  testModelConnection(modelId: string): Promise<{
    success: boolean;
    passed: boolean;
    latency_ms: number;
    error?: string;
    message: string;
  }> {
    return api.post(
      `/api/admin/models/by-id/test/connection?model_id=${encodeURIComponent(modelId)}`,
      {}
    );
  },

  testModelCapability(modelId: string): Promise<CapabilityTestResult> {
    return api.post<CapabilityTestResult>(
      `/api/admin/models/by-id/test/capability?model_id=${encodeURIComponent(modelId)}`,
      {}
    );
  },

  runModelBenchmark(modelId: string): Promise<BenchmarkResults> {
    return api.post<BenchmarkResults>(
      `/api/admin/models/by-id/benchmark?model_id=${encodeURIComponent(modelId)}`,
      {}
    );
  },

  getModelTestResults(modelId: string): Promise<BenchmarkResults> {
    return api.get<BenchmarkResults>(
      `/api/admin/models/by-id/test-results?model_id=${encodeURIComponent(modelId)}`
    );
  },

  // Alias Management
  getModelAliases(modelId: string): Promise<ModelAliasView[]> {
    return api.get<ModelAliasView[]>(
      `/api/admin/models/by-id/aliases?model_id=${encodeURIComponent(modelId)}`
    );
  },

  createModelAlias(
    modelId: string,
    data: Omit<CreateAliasRequest, 'model_id'>
  ): Promise<{ success: boolean; message: string }> {
    return api.post(
      `/api/admin/models/by-id/aliases?model_id=${encodeURIComponent(modelId)}`,
      data
    );
  },

  updateModelAlias(
    modelId: string,
    aliasName: string,
    data: UpdateAliasRequest
  ): Promise<ModelAliasView> {
    return api.put<ModelAliasView>(
      `/api/admin/models/by-id/aliases/${encodeURIComponent(aliasName)}?model_id=${encodeURIComponent(modelId)}`,
      data
    );
  },

  deleteModelAlias(
    modelId: string,
    aliasName: string,
    providerId: number
  ): Promise<{ success: boolean; message: string }> {
    return api.delete(
      `/api/admin/models/by-id/aliases/${encodeURIComponent(aliasName)}?model_id=${encodeURIComponent(modelId)}&provider_id=${providerId}`
    );
  },

  importAliasesFromJSON(): Promise<{ success: boolean; message: string }> {
    return api.post('/api/admin/models/import-aliases', {});
  },

  // Bulk Operations
  bulkUpdateAgentsEnabled(data: BulkUpdateAgentsRequest): Promise<{ message: string }> {
    return api.put('/api/admin/models/bulk/agents-enabled', data);
  },

  bulkUpdateVisibility(data: BulkUpdateVisibilityRequest): Promise<{ message: string }> {
    return api.put('/api/admin/models/bulk/visibility', data);
  },

  bulkUpdateTier(data: BulkUpdateTierRequest): Promise<{ message: string }> {
    return api.put('/api/admin/models/bulk/tier', data);
  },

  // Global Tier Management
  getTiers(): Promise<Record<string, TierAssignment>> {
    return api
      .get<{ tiers: Record<string, TierAssignment> }>('/api/admin/tiers')
      .then(res => res.tiers);
  },

  setModelTier(modelId: string, providerId: number, tier: string): Promise<{ message: string }> {
    return api.post(`/api/admin/models/by-id/tier?model_id=${encodeURIComponent(modelId)}`, {
      provider_id: providerId,
      tier,
    });
  },

  clearModelTier(tier: string): Promise<{ message: string }> {
    return api.deleteWithBody(`/api/admin/models/by-id/tier`, { tier });
  },

  // User Management (GDPR-Compliant)
  getUsers(params?: {
    page?: number;
    page_size?: number;
    tier?: string;
    search?: string;
  }): Promise<UserListResponse> {
    const queryParams = new URLSearchParams();
    if (params?.page) queryParams.set('page', params.page.toString());
    if (params?.page_size) queryParams.set('page_size', params.page_size.toString());
    if (params?.tier) queryParams.set('tier', params.tier);
    if (params?.search) queryParams.set('search', params.search);

    const query = queryParams.toString();
    return api.get<UserListResponse>(`/api/admin/users${query ? `?${query}` : ''}`);
  },

  getGDPRPolicy(): Promise<GDPRDataPolicy> {
    return api.get<GDPRDataPolicy>('/api/admin/gdpr-policy');
  },

  // Health Dashboard
  getHealthDashboard(): Promise<HealthDashboardResponse> {
    return api.get<HealthDashboardResponse>('/api/admin/health');
  },

  // Insights Dashboard
  getInsightsOverview(): Promise<InsightsOverview> {
    return api.get<InsightsOverview>('/api/admin/insights/overview');
  },

  getInsightsMetrics(days: number = 30): Promise<DailyMetricsResponse> {
    return api.get<DailyMetricsResponse>(`/api/admin/insights/metrics?days=${days}`);
  },

  getHealthDistribution(): Promise<HealthDistribution> {
    return api.get<HealthDistribution>('/api/admin/insights/health-distribution');
  },

  getActivationFunnel(): Promise<ActivationFunnel> {
    return api.get<ActivationFunnel>('/api/admin/insights/activation-funnel');
  },

  getFeedbackStream(limit: number = 20, type: string = 'all'): Promise<FeedbackStreamResponse> {
    return api.get<FeedbackStreamResponse>(
      `/api/admin/insights/feedback-stream?limit=${limit}&type=${type}`
    );
  },

  // Collection Stats (direct from raw collections)
  getCollectionStats(): Promise<CollectionStats> {
    return api.get<CollectionStats>('/api/admin/insights/collection-stats');
  },

  // Backfill daily metrics from historical data
  backfillMetrics(days: number = 90): Promise<BackfillResponse> {
    return api.post<BackfillResponse>(`/api/admin/insights/backfill?days=${days}`, {});
  },

  // AutoPilot Dashboard
  getAutoPilotContext(): Promise<AutoPilotContext> {
    return api.get<AutoPilotContext>('/api/admin/autopilot/context');
  },

  analyzeWithAI(modelId: string): Promise<AutoPilotAnalysisResponse> {
    return api.post<AutoPilotAnalysisResponse>('/api/admin/autopilot/analyze', { modelId });
  },
};
