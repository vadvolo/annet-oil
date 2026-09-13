import axios, { AxiosInstance, AxiosError } from 'axios';
import { ResponseCache } from './cache.js';
import { logger } from './logger.js';

export interface AnnetConfig {
  apiUrl: string;
  authToken: string;
  timeout?: number;
  cacheTtl?: number;
}

export interface CommandRequest {
  command?: string;
  filters?: string[];
  generators?: string[];
  exclude_generators?: string[];
  container?: string;
  dry_run?: boolean;
  parallel?: boolean;
  timeout?: number;
  vendor?: string;
  quiet?: boolean;
  extra_args?: string[];
  environment?: Record<string, string>;
}

export interface CommandResult {
  container: string;
  exit_code: number;
  stdout: string;
  stderr: string;
  error?: string;
  error_code?: string;
  duration?: string;
}

export interface CommandResponse {
  success: boolean;
  results?: Record<string, CommandResult>;
  error?: string;
  total_hosts: number;
  success_hosts: number;
  failed_hosts: number;
}

export interface CheckeastError {
  type: string;
  message: string;
}

export interface CheckeastArtifact {
  key: string;
  location: string;
  content_type: string;
  size: number;
  error?: CheckeastError;
}

export interface CheckeastReport {
  run_id: string;
  timestamp: string;
  bucket: string;
  combined?: CheckeastArtifact;
  per_host?: Record<string, CheckeastArtifact>;
  stored_hosts: number;
  failed_hosts: number;
  success: boolean;
  error?: CheckeastError;
}

export interface CheckeastResponse {
  diff: CommandResponse;
  archive: CheckeastReport;
}

export interface AuditEvent {
  id: number;
  timestamp: string;
  actor: string;
  actor_role?: string;
  source: string;
  action: string;
  devices?: string[];
  command?: string;
  params?: Record<string, unknown>;
  success: boolean;
  duration_ms?: number;
  error?: { type: string; message: string };
  request_id?: string;
}

export interface AuditListResponse {
  events: AuditEvent[];
  total: number;
}

export interface AuditFilter {
  actor?: string;
  device?: string;
  action?: string;
  source?: string;
  from?: string;
  to?: string;
  success?: boolean;
  limit?: number;
  offset?: number;
}

export interface ContainerStatus {
  name: string;
  container_name: string;
  running: boolean;
  configured: boolean;
  status: string;
  created?: string;
  state?: string;
  error?: string;
}

export interface RoutingInfo {
  hostname: string;
  container?: string;
  routes?: Array<{
    prefix: string;
    container: string;
  }>;
}

export interface DeviceInfo {
  hostname: string;
  ip: string;
  port: number;
  vendor: string;
  platform: string;
  description?: string;
  aliases?: string[];
}

export interface InventoryResponse {
  devices: DeviceInfo[];
  total: number;
}

export interface ReloadResponse {
  status: string;
  path: string;
  devices: number;
  reloaded_at: string;
}

export interface CheckPortResult {
  port: number;
  open: boolean;
  latency_ms?: number;
  error?: string;
}

export interface CheckResult {
  hostname: string;
  ip: string;
  vendor?: string;
  timestamp: string;
  duration_ms: number;
  reachable: boolean;
  login: string; // "ok" | "failed" | "skipped"
  ports: CheckPortResult[];
  error?: {
    type: string;
    message: string;
  };
}

export interface CheckRequest {
  host: string;
  ports?: number[];
  login?: boolean;
  timeout?: number;
}

export interface FeatureMode {
  name: string;
  support: string; // "supported" | "unsupported" | "partial" | "unknown"
  notes?: string;
}

export interface FeatureInfo {
  name: string;
  category?: string;
  title?: string;
  support: string;
  notes?: string;
  modes?: FeatureMode[];
  refs?: string[];
}

export interface FeatureSetResult {
  vendor: string;
  model: string;
  version?: string;
  family?: string;
  platform?: string;
  matched: boolean;
  features: FeatureInfo[];
  warnings?: string[];
}

export interface FeatureSetRequest {
  vendor?: string;
  model?: string;
  version?: string;
  feature?: string;
  host?: string;
}

export interface OpStateRequest {
  host: string;
  vendor?: string;
  states?: string[];
  force?: boolean;
}

export interface CreateRFCRequest {
  summary: string;
  description?: string;
  devices: string[];
  priority?: string;
}

export interface CreateRFCResponse {
  ticket_key: string;
  url: string;
}

export interface RFCStatusResponse {
  ticket_key: string;
  summary: string;
  status: string;
  transitions: string[];
}

export class AnnetOilClient {
  private client: AxiosInstance;
  private cache: ResponseCache<CommandResponse>;

  constructor(private config: AnnetConfig) {
    this.client = axios.create({
      baseURL: `${config.apiUrl}/api/v0`,
      timeout: config.timeout || 30000,
      headers: {
        'Authorization': `Bearer ${config.authToken}`,
        'Content-Type': 'application/json',
      },
    });
    this.cache = new ResponseCache<CommandResponse>(config.cacheTtl || 300);
  }

  async gen(request: CommandRequest): Promise<CommandResponse> {
    const cacheKey = ResponseCache.hashRequest({ op: 'gen', ...request });
    const cached = this.cache.get(cacheKey);
    if (cached) {
      logger.debug({ op: 'gen', cache: 'hit' }, 'cache hit');
      return cached.value;
    }

    try {
      const response = await this.client.post('/gen', request);
      this.cache.set(cacheKey, response.data);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async diff(request: CommandRequest): Promise<CommandResponse> {
    const cacheKey = ResponseCache.hashRequest({ op: 'diff', ...request });
    const cached = this.cache.get(cacheKey);
    if (cached) {
      logger.debug({ op: 'diff', cache: 'hit' }, 'cache hit');
      return cached.value;
    }

    try {
      const response = await this.client.post('/diff', request);
      this.cache.set(cacheKey, response.data);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async audit(filter: AuditFilter): Promise<AuditListResponse> {
    try {
      const response = await this.client.get('/audit', { params: filter });
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async checkeast(request: CommandRequest): Promise<CheckeastResponse> {
    // Not cached: every call runs a fresh diff and writes to S3.
    try {
      const response = await this.client.post('/checkeast', request);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async patch(request: CommandRequest): Promise<CommandResponse> {
    try {
      const response = await this.client.post('/patch', request);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async deploy(request: CommandRequest): Promise<CommandResponse> {
    try {
      const response = await this.client.post('/deploy', request);
      this.cache.invalidateAll();
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async getContainers(): Promise<Record<string, ContainerStatus>> {
    try {
      const response = await this.client.get('/containers');
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async getRouting(hostname?: string): Promise<RoutingInfo | RoutingInfo[]> {
    try {
      const params = hostname ? { hostname } : {};
      const response = await this.client.get('/routing', { params });
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async health(): Promise<{ status: string }> {
    try {
      const response = await this.client.get('/health');
      return { status: response.data || 'OK' };
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async getInventory(vendor?: string, platform?: string, pattern?: string): Promise<InventoryResponse> {
    try {
      const params: Record<string, string> = {};
      if (vendor) params.vendor = vendor;
      if (platform) params.platform = platform;
      if (pattern) params.pattern = pattern;
      const response = await this.client.get('/inventory', { params });
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async reloadInventory(): Promise<ReloadResponse> {
    try {
      const response = await this.client.post('/inventory/reload');
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async check(request: CheckRequest): Promise<CheckResult> {
    try {
      const response = await this.client.post('/check', request);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async featureSet(request: FeatureSetRequest): Promise<FeatureSetResult> {
    try {
      const response = await this.client.post('/featureset', request);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async state(request: OpStateRequest): Promise<Record<string, unknown>> {
    try {
      const response = await this.client.post('/state', request);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async topologyGraph(host?: string, depth?: number): Promise<Record<string, unknown>> {
    try {
      const params: Record<string, unknown> = {};
      if (host) params.host = host;
      if (depth !== undefined) params.depth = depth;
      const response = await this.client.get('/topology', { params });
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async topologyCollect(request: {
    hosts: string[];
    sources?: string[];
    vendor?: string;
    force?: boolean;
  }): Promise<Record<string, unknown>> {
    try {
      const response = await this.client.post('/topology/collect', request);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async createRFC(request: CreateRFCRequest): Promise<CreateRFCResponse> {
    try {
      const response = await this.client.post('/rfc/create', request);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async postRFCComment(ticketKey: string, comment: string): Promise<void> {
    try {
      await this.client.post('/rfc/comment', { ticket_key: ticketKey, comment });
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async getRFCStatus(ticketKey: string): Promise<RFCStatusResponse> {
    try {
      const response = await this.client.get(`/rfc/status/${ticketKey}`);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async submitRFCForReview(ticketKey: string, comment?: string): Promise<void> {
    try {
      await this.client.post(`/rfc/submit/${ticketKey}`, { comment });
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async closeRFC(ticketKey: string, resolution?: string): Promise<void> {
    try {
      await this.client.post(`/rfc/close/${ticketKey}`, { resolution });
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async executeCommand(request: CommandRequest): Promise<CommandResponse> {
    try {
      // The execute endpoint expects a single host and command
      // If filters is provided, use the first filter as the host
      const host = request.filters && request.filters.length > 0 ? request.filters[0] : '';

      const executeRequest: {
        host: string;
        command: string;
        timeout_s?: number;
        vendor?: string;
      } = {
        host: host,
        command: request.command || '',
      };
      // Forward an optional per-command timeout for large-output diagnostics.
      if (request.timeout && request.timeout > 0) {
        executeRequest.timeout_s = request.timeout;
      }
      // Forward an optional vendor to select the right gnetcli profile.
      if (request.vendor) {
        executeRequest.vendor = request.vendor;
      }

      const response = await this.client.post('/execute', executeRequest);

      // Transform the response to match CommandResponse format
      return {
        success: response.data.status === 0,
        results: {
          [host]: {
            container: request.container || 'default',
            exit_code: response.data.status || 0,
            stdout: response.data.output || '',
            stderr: response.data.error || '',
            error: response.data.error,
            error_code: response.data.error_code,
          }
        },
        total_hosts: 1,
        success_hosts: response.data.status === 0 ? 1 : 0,
        failed_hosts: response.data.status === 0 ? 0 : 1,
      };
    } catch (error) {
      throw this.handleError(error);
    }
  }

  private handleError(error: unknown): Error {
    if (axios.isAxiosError(error)) {
      const axiosError = error as AxiosError;
      if (axiosError.response) {
        return new Error(
          `API Error: ${axiosError.response.status} - ${JSON.stringify(axiosError.response.data)}`
        );
      } else if (axiosError.request) {
        return new Error('No response from API server');
      }
    }
    return error instanceof Error ? error : new Error('Unknown error');
  }
}