import axios, { AxiosInstance, AxiosError } from 'axios';

export interface AnnetConfig {
  apiUrl: string;
  authToken: string;
  timeout?: number;
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

export class AnnetOilClient {
  private client: AxiosInstance;

  constructor(private config: AnnetConfig) {
    this.client = axios.create({
      baseURL: `${config.apiUrl}/api/v0`,
      timeout: config.timeout || 30000,
      headers: {
        'Authorization': `Bearer ${config.authToken}`,
        'Content-Type': 'application/json',
      },
    });
  }

  async gen(request: CommandRequest): Promise<CommandResponse> {
    try {
      const response = await this.client.post('/gen', request);
      return response.data;
    } catch (error) {
      throw this.handleError(error);
    }
  }

  async diff(request: CommandRequest): Promise<CommandResponse> {
    try {
      const response = await this.client.post('/diff', request);
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

  async executeCommand(request: CommandRequest): Promise<CommandResponse> {
    try {
      // The execute endpoint expects a single host and command
      // If filters is provided, use the first filter as the host
      const host = request.filters && request.filters.length > 0 ? request.filters[0] : '';

      const executeRequest = {
        host: host,
        command: request.command || '',
      };

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