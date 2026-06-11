import type { components } from './openapi';

type ApiSchema<Name extends keyof components['schemas']> = components['schemas'][Name];

type CamelCaseKey<Value extends string> =
  Value extends `${infer Head}_${infer Tail}`
    ? PreserveAcronyms<`${Head}${Capitalize<CamelCaseKey<Tail>>}`>
    : PreserveAcronyms<Value>;

type PreserveAcronyms<Value extends string> =
  Value extends `${infer Prefix}Usd`
    ? `${Prefix}USD`
    : Value extends `${infer Prefix}Kbps`
      ? `${Prefix}KBps`
      : Value extends `${infer Prefix}Mb`
        ? `${Prefix}MB`
        : Value;

export type Camelize<T> =
  T extends readonly (infer Item)[]
    ? Camelize<Item>[]
    : T extends object
      ? { [Key in keyof T as Key extends string ? CamelCaseKey<Key> : Key]: Camelize<T[Key]> }
      : T;

export type ApiStatusResponse = Camelize<ApiSchema<'StatusResponse'>>;
export type ApiMemoryObservationInput = Camelize<ApiSchema<'MemoryObservationInput'>>;
export type ApiMemorySearchRequest = Camelize<ApiSchema<'MemorySearchRequest'>>;
export type ApiProject = Camelize<ApiSchema<'Project'>>;
export type ApiProjectInput = Camelize<ApiSchema<'ProjectInput'>>;
export type ApiDomain = Camelize<ApiSchema<'Domain'>>;
export type ApiGoal = Camelize<ApiSchema<'Goal'>>;
export type ApiGoalInput = Camelize<ApiSchema<'GoalInput'>>;
export type ApiTask = Camelize<ApiSchema<'Task'>>;
export type ApiTaskInput = Camelize<ApiSchema<'TaskInput'>>;
export type ApiTaskPatch = Camelize<ApiSchema<'TaskPatch'>>;
export type ApiAgent = Camelize<ApiSchema<'Agent'>>;
export type ApiAgentInput = Camelize<ApiSchema<'AgentInput'>>;
export type ApiSkill = Camelize<ApiSchema<'Skill'>>;
export type ApiRuntimeAdapter = Camelize<ApiSchema<'RuntimeAdapter'>>;
export type ApiProvider = Camelize<ApiSchema<'Provider'>>;
export type ApiExecutionMode = ApiSchema<'ExecutionMode'>;
export type ApiRun = Camelize<ApiSchema<'Run'>>;
export type ApiRunProposal = Camelize<ApiSchema<'RunProposal'>>;
export type ApiApprovalInput = Camelize<ApiSchema<'ApprovalInput'>>;
export type ApiRunApproval = Camelize<ApiSchema<'Approval'>>;
export type ApiRunLog = Camelize<ApiSchema<'RunLog'>>;
export type ApiRepository = Camelize<ApiSchema<'Repository'>>;
export type ApiRepositoryInput = Camelize<ApiSchema<'RepositoryInput'>>;
export type ApiUsageOverviewItem = Camelize<ApiSchema<'UsageOverviewItem'>>;
export type ApiArtifact = Camelize<ApiSchema<'Artifact'>>;
export type ApiArtifactInput = Camelize<ApiSchema<'ArtifactInput'>>;
export type ApiJournal = Camelize<ApiSchema<'Journal'>>;
export type ApiJournalInput = Camelize<ApiSchema<'JournalInput'>>;
export type ApiKnowledgeWorkspace = Camelize<ApiSchema<'KnowledgeWorkspace'>>;
export type ApiKnowledgeWorkspaceInput = Camelize<ApiSchema<'KnowledgeWorkspaceInput'>>;
export type ApiChatRequest = Camelize<ApiSchema<'ChatRequest'>>;
export type ApiChatResponse = Camelize<ApiSchema<'ChatResponse'>>;
export type ApiNovaCoreConversation = Camelize<ApiSchema<'NovaCoreConversation'>>;
export type ApiNovaCoreMessage = Camelize<ApiSchema<'NovaCoreMessage'>>;
