export interface A2APart {
  kind: 'text' | 'file' | 'data';
  text?: string;
  data?: Record<string, unknown>;
  file?: { name?: string; mimeType?: string; data?: string; uri?: string };
  metadata?: Record<string, unknown>;
}

export interface A2AMessage {
  role: 'user' | 'agent';
  parts: A2APart[];
  messageId?: string;
  taskId?: string;
  contextId?: string;
}

export interface A2ATaskStatus {
  state: 'submitted' | 'working' | 'input-required' | 'completed' | 'failed' | 'canceled';
  message?: A2AMessage;
  timestamp?: string;
}

export interface A2AArtifact {
  artifactId: string;
  name?: string;
  parts: A2APart[];
}

export interface A2ATask {
  id: string;
  contextId: string;
  status: A2ATaskStatus;
  artifacts?: A2AArtifact[];
  history?: A2AMessage[];
  kind: 'task';
  metadata?: Record<string, unknown>;
}

export interface A2AStatusUpdate {
  taskId: string;
  contextId: string;
  status: A2ATaskStatus;
  final?: boolean;
  kind: 'status-update';
}

export interface A2AArtifactUpdate {
  taskId: string;
  contextId: string;
  artifact: A2AArtifact;
  append?: boolean;
  lastChunk?: boolean;
  kind: 'artifact-update';
}

export interface A2ADirectMessage {
  messageId: string;
  contextId: string;
  role: 'agent';
  parts: A2APart[];
  kind: 'message';
  metadata?: Record<string, unknown>;
}

export type A2AEvent = A2ATask | A2AStatusUpdate | A2AArtifactUpdate | A2ADirectMessage;

export interface A2AAgentCard {
  name: string;
  description?: string;
  url?: string;
  version?: string;
  capabilities?: {
    streaming?: boolean;
    pushNotifications?: boolean;
  };
  skills?: Array<{
    id: string;
    name: string;
    description?: string;
  }>;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'agent';
  text: string;
  timestamp: Date;
  taskId?: string;
  status?: A2ATaskStatus['state'];
  artifacts?: A2AArtifact[];
  isStreaming?: boolean;
}

export interface A2AEventRecord {
  id: string;
  timestamp: Date;
  kind: A2AEvent['kind'];
  raw: A2AEvent;
  taskId?: string;
  linkedMessageId?: string;
}
