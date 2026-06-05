// Cliente HTTP para consumir los endpoints de BattOS API

import type {
  ApiApprovalInput,
  ApiAgent,
  ApiAgentInput,
  ApiArtifact,
  ApiArtifactInput,
  ApiChatRequest,
  ApiChatResponse,
  ApiGoal,
  ApiGoalInput,
  ApiJournal,
  ApiJournalInput,
  ApiKnowledgeWorkspace,
  ApiKnowledgeWorkspaceInput,
  ApiMemoryObservationInput,
  ApiMemorySearchRequest,
  ApiNovaCoreConversation,
  ApiNovaCoreMessage,
  ApiProject,
  ApiProjectInput,
  ApiRun,
  ApiRunLog,
  ApiRunProposal,
  ApiRepository,
  ApiRepositoryInput,
  ApiRuntimeAdapter,
  ApiSkill,
  ApiStatusResponse,
  ApiTask,
  ApiTaskInput,
  ApiTaskPatch,
  ApiUsageOverviewItem,
} from './generated/api-contract';

const BASE_URL = process.env.NEXT_PUBLIC_BATTOS_API_URL || 'http://localhost:8000';

export function getApiBaseUrl(): string {
  return BASE_URL;
}

export function getApiToken(): string {
  if (typeof window !== 'undefined') {
    return localStorage.getItem('BATTOS_API_TOKEN') || '';
  }
  return '';
}

export function setApiToken(token: string) {
  if (typeof window !== 'undefined') {
    localStorage.setItem('BATTOS_API_TOKEN', token);
  }
}

export function clearApiToken() {
  if (typeof window !== 'undefined') {
    localStorage.removeItem('BATTOS_API_TOKEN');
  }
}

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

function toCamelCase(value: string): string {
  return value
    .replace(/_([a-z])/g, (_, letter: string) => letter.toUpperCase())
    .replace(/Usd$/g, 'USD')
    .replace(/Kbps$/g, 'KBps')
    .replace(/Mb$/g, 'MB');
}

function toSnakeCase(value: string): string {
  return value
    .replace(/USD/g, 'Usd')
    .replace(/KBps/g, 'Kbps')
    .replace(/MB/g, 'Mb')
    .replace(/[A-Z]/g, letter => `_${letter.toLowerCase()}`);
}

function snakeizeBody(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(item => snakeizeBody(item));
  }

  if (value && typeof value === 'object' && Object.getPrototypeOf(value) === Object.prototype) {
    const out: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) {
      out[toSnakeCase(key)] = snakeizeBody(item);
    }
    return out;
  }

  return value;
}

export function camelizeResponse<T>(value: unknown): T {
  if (Array.isArray(value)) {
    return value.map(item => camelizeResponse(item)) as T;
  }

  if (value && typeof value === 'object' && Object.getPrototypeOf(value) === Object.prototype) {
    const out: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) {
      out[toCamelCase(key)] = camelizeResponse(item);
    }
    return out as T;
  }

  return value as T;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getApiToken();
  const headers = new Headers(options.headers || {});
  
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  headers.set('Content-Type', 'application/json');

  const url = `${BASE_URL}${path}`;
  const response = await fetch(url, {
    ...options,
    headers,
  });

  if (!response.ok) {
    let errMsg = `Request failed with status ${response.status}`;
    try {
      const errJSON = await response.json();
      if (errJSON.error && errJSON.error.message) {
        errMsg = errJSON.error.message;
      }
    } catch {
      // Ignorar fallo al decodificar error
    }
    throw new ApiError(response.status, errMsg);
  }

  if (response.status === 204) {
    return {} as T;
  }

  const contentType = response.headers.get('Content-Type') || '';
  if (contentType.includes('text/plain')) {
    return response.text() as Promise<T>;
  }

  const data = await response.json();
  return camelizeResponse<T>(data);
}

export const api = {
  get: <T>(path: string, options?: RequestInit) => 
    request<T>(path, { ...options, method: 'GET' }),
  
  post: <T>(path: string, body?: unknown, options?: RequestInit) => 
    request<T>(path, { 
      ...options, 
      method: 'POST', 
      body: body ? JSON.stringify(body) : undefined 
    }),
  
  patch: <T>(path: string, body?: unknown, options?: RequestInit) => 
    request<T>(path, { 
      ...options, 
      method: 'PATCH', 
      body: body ? JSON.stringify(body) : undefined 
    }),
  
  delete: <T>(path: string, options?: RequestInit) => 
    request<T>(path, { ...options, method: 'DELETE' }),

  getEventSourceUrl: (path: string): string => {
    const url = new URL(`${BASE_URL}${path}`);
    // Si la API no soporta Auth headers en EventSource, podemos pasar el token por query param
    // pero nuestra API espera Bearer Token en Auth Header. Next.js puede conectarse y autorizarse.
    // Para simplificar EventSource en navegadores sin Headers personalizados, pasamos el token por query param
    // o usamos una librería, pero dado que en Go definimos authMiddleware, éste lee Bearer del header.
    // En JS, para meter headers a EventSource, es común usar event-source-polyfill o pasar el token por query param
    // si el middleware de Go lo soporta. Pero en router.go el authMiddleware sólo busca en `r.Header.Get("Authorization")`.
    // Por suerte, para SSE en React podemos usar fetch e ir leyendo la respuesta por streams para soportar headers personalizados!
    // Esto es mucho más moderno, estándar y no requiere EventSource que tiene el problema de no soportar headers.
    return url.toString();
  }
};

export const apiClient = {
  getStatus: () => api.get<ApiStatusResponse>('/status'),
  listProjects: () => api.get<ApiProject[]>('/projects'),
  listGoals: () => api.get<ApiGoal[]>('/goals'),
  listTasks: () => api.get<ApiTask[]>('/tasks'),
  listAgents: () => api.get<ApiAgent[]>('/agents'),
  createAgent: (body: ApiAgentInput) => api.post<ApiAgent>('/agents', snakeizeBody(body)),
  listSkills: () => api.get<ApiSkill[]>('/skills'),
  listRuntimeAdapters: () => api.get<ApiRuntimeAdapter[]>('/runtime-adapters'),
  listRepositories: () => api.get<ApiRepository[]>('/repositories'),
  connectRepository: (body: ApiRepositoryInput) => api.post<ApiRepository>('/repositories', snakeizeBody(body)),
  listRuns: () => api.get<ApiRun[]>('/runs'),
  getRun: (id: string) => api.get<ApiRun>(`/runs/${id}`),
  listRunLogs: (id: string) => api.get<ApiRunLog[]>(`/runs/${id}/logs`),
  getRunDiff: (id: string) => api.get<string>(`/runs/${id}/diff`),
  listUsageOverview: () => api.get<ApiUsageOverviewItem[]>('/usage/overview'),
  createProject: (body: ApiProjectInput) => api.post<ApiProject>('/projects', snakeizeBody(body)),
  createGoal: (body: ApiGoalInput) => api.post<ApiGoal>('/goals', snakeizeBody(body)),
  createTask: (body: ApiTaskInput) => api.post<ApiTask>('/tasks', snakeizeBody(body)),
  updateTask: (id: string, body: ApiTaskPatch) => api.patch<ApiTask>(`/tasks/${id}`, snakeizeBody(body)),
  proposeRun: (body: ApiRunProposal) => api.post<ApiRun>('/runs', snakeizeBody(body)),
  approveRunAction: (id: string, body: ApiApprovalInput) => api.post<{ run: ApiRun; approval: unknown }>(`/runs/${id}/approvals`, snakeizeBody(body)),
  cancelRun: (id: string) => api.post<ApiRun>(`/runs/${id}/cancel`),
  searchMemory: (body: ApiMemorySearchRequest) => api.post<{ results: unknown[]; count: number; query: string }>('/memory/search', snakeizeBody(body)),
  saveMemoryObservation: (body: ApiMemoryObservationInput) => api.post('/memory/save', snakeizeBody(body)),
  createKnowledgeWorkspace: (body: ApiKnowledgeWorkspaceInput) => api.post<ApiKnowledgeWorkspace>('/knowledge/workspaces', snakeizeBody(body)),
  createJournal: (body: ApiJournalInput) => api.post<ApiJournal>('/journals', snakeizeBody(body)),
  createArtifact: (body: ApiArtifactInput) => api.post<ApiArtifact>('/artifacts', snakeizeBody(body)),
  listNovaCoreConversations: () => api.get<ApiNovaCoreConversation[]>('/novacore/conversations'),
  listNovaCoreMessages: (id: string) => api.get<ApiNovaCoreMessage[]>(`/novacore/conversations/${id}/messages`),
  chatNovaCore: (body: ApiChatRequest) => api.post<ApiChatResponse>('/novacore/chat', snakeizeBody(body)),
};
