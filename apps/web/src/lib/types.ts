// Definición de tipos comunes de la API de BattOS

import type {
  ApiAgent,
  ApiDomain,
  ApiGoal,
  ApiProject,
  ApiProvider,
  ApiRepository,
  ApiRun,
  ApiRunApproval,
  ApiRunLog,
  ApiRuntimeAdapter,
  ApiSkill,
  ApiStatusResponse,
  ApiTask,
  ApiUsageOverviewItem,
} from './generated/api-contract';

export type OpenAPIBackedTypes = {
  status: ApiStatusResponse;
  project: ApiProject;
  domain: ApiDomain;
  goal: ApiGoal;
  task: ApiTask;
  agent: ApiAgent;
  skill: ApiSkill;
  runtimeAdapter: ApiRuntimeAdapter;
  provider: ApiProvider;
  run: ApiRun;
  runApproval: ApiRunApproval;
  runLog: ApiRunLog;
  repository: ApiRepository;
  usageOverviewItem: ApiUsageOverviewItem;
};

export interface VersionResponse {
  version: string;
  commit: string;
  buildDate: string;
  goVersion: string;
}

export interface SubsystemHealth {
  name: string;
  status: string;
  detail: string;
  latencyMs?: number;
}

export interface TopProcess {
  pid: number;
  name: string;
  cpuPercent: number;
  memMB: number;
}

export interface SystemMetrics {
  cpuPercent: number;
  memPercent: number;
  memUsedMB: number;
  memTotalMB: number;
  netUploadKBps: number;
  netDownloadKBps: number;
  diskPercent: number;
  diskUsedGB: number;
  diskTotalGB: number;
  topProcesses: TopProcess[];
}

export interface StatusResponse {
  version: VersionResponse;
  overall: string;
  subsystems: SubsystemHealth[];
  metrics: SystemMetrics;
  timestamp: string;
}

export interface Project {
  id: string;
  slug: string;
  name: string;
  description?: string;
  status: string;
  monthlyBudgetUSD?: number;
}

export interface Domain {
  id: string;
  slug: string;
  name: string;
  description?: string;
  status: string;
}

export interface Goal {
  id: string;
  projectId: string;
  title: string;
  description?: string;
  status: string;
}

export interface Task {
  id: string;
  projectId: string;
  goalId?: string;
  title: string;
  description?: string;
  assignedAgentId?: string;
  status: string;
  boardPosition: number;
}

export interface Agent {
  id: string;
  slug: string;
  name: string;
  role?: string;
  description?: string;
  runtimeId?: string;
  systemPrompt?: string;
  riskLevel?: string;
  isLead: boolean;
  isMeta: boolean;
  status: string;
}

export interface Skill {
  id: string;
  slug: string;
  name: string;
  description?: string;
  category?: string;
  status: string;
}

export interface AgentRuntime {
  id: string;
  name: string;
  kind?: string;
  command?: string;
  status: string;
  version?: string;
  executable?: string;
  approvalRequired?: boolean;
  approvedForExecution?: boolean;
  requiresAuth: boolean;
  lastDetectedAt?: string;
}

// CLI tools instalables en el host. El contrato OpenAPI todavía no expone
// estos schemas en `generated/`, por eso se tipan a mano siguiendo el patrón
// del resto de la API (snake_case en el wire, camelCase tras camelizeResponse).
export type CliToolStatus = 'detected' | 'not_detected' | 'broken';

export interface CliTool {
  id: string;
  name: string;
  command: string;
  kind?: string;
  status: CliToolStatus | string;
  detectedPath?: string;
  version?: string;
  runtimeId?: string;
  riskLevel?: string;
  requiresAuth?: boolean;
  installCommand?: string;
  installUrl?: string;
  lastDetectedAt?: string;
}

export type CliToolInstallStatus =
  | 'pending_approval'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'rejected';

export interface CliToolInstall {
  id: string;
  cliToolId: string;
  installCommand: string;
  status: CliToolInstallStatus | string;
  reason?: string | null;
  output?: string | null;
  requestedAt?: string;
  decidedAt?: string | null;
  completedAt?: string | null;
}

export interface CliToolInstallDecision {
  decision: 'approved' | 'rejected';
  reason?: string;
}

export interface Provider {
  id: string;
  name: string;
  kind: string;
  status: string;
}

export type ExecutionMode = 'sandbox' | 'direct' | 'connected';

export interface Run {
  id: string;
  projectId: string;
  taskId: string;
  agentId: string;
  skillId?: string;
  runtimeAdapterId: string;
  repositoryId?: string;
  prompt: string;
  requestedNetwork: boolean;
  networkEnabled: boolean;
  hostSessionEnabled: boolean;
  executionMode: ExecutionMode;
  status: string;
  branchName?: string;
  resultSummary?: string;
  errorMessage?: string;
  estimatedCostUSD: number;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
}

export interface RunApproval {
  id: string;
  runId: string;
  kind: 'execute' | 'network' | 'commit' | 'push' | 'remember';
  decision: 'approved' | 'rejected';
  reason?: string;
  decidedAt: string;
}

export interface RunLog {
  id: number;
  runId: string;
  stream: 'system' | 'stdout' | 'stderr';
  message: string;
  createdAt: string;
}

export interface Repository {
  id: string;
  projectId: string;
  kind: string;
  name: string;
  defaultBranch: string;
}

export interface UsageOverviewItem {
  projectId: string;
  projectName: string;
  projectMonthlyBudgetUSD: number;
  agentId: string;
  modelId: string;
  providerId: string;
  totalInputTokens: number;
  totalOutputTokens: number;
  totalCachedTokens: number;
  totalRequests: number;
  totalCostUSD: number;
  costPrecision: 'exact' | 'estimated' | 'not_reported';
}

export interface UsageEvent {
  id: string;
  runId: string;
  providerId: string;
  modelId: string;
  projectId: string;
  agentId: string;
  skillId?: string;
  inputTokens: number;
  outputTokens: number;
  cachedTokens: number;
  requestCount: number;
  estimatedCostUSD: number;
  createdAt: string;
}

export interface MemoryObservation {
  id: number;
  topicKey: string;
  content: string;
  scope: string;
  createdAt: string;
}

export interface MemoryStats {
  totalItems: number;
  itemsLast24h: number;
}

export interface Conversation {
  id: string;
  startedAt: string;
  messageCount: number;
  totalInputTokens: number;
  totalOutputTokens: number;
  totalCostUSD: number;
}

export interface Message {
  id: string;
  conversationId: string;
  role: string;
  content: string;
  tokensIn: number;
  tokensOut: number;
  createdAt: string;
}
