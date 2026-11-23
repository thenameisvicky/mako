// Payload shapes mirrored from pkg/models on the Go side.

export interface Me {
  userId: string;
  username: string;
  email: string;
  teamId: string;
  teamName: string;
  role: string;
}

export interface Policy {
  allowedModels: string[];
  blockedModels: string[];
  maxTokensPerRequest: number;
  configured: boolean;
  updatedAt?: string;
}

export interface Budget {
  period: "daily" | "weekly" | "monthly";
  tokenLimit: number;
  configured: boolean;
  updatedAt?: string;
}

export interface ModelUsage {
  model: string;
  tokens: number;
  count: number;
}

export interface DailyUsage {
  day: string;
  tokens: number;
}

export interface TeamUsage {
  totalTokens: number;
  inputTokens: number;
  outputTokens: number;
  requests: number;
  byModel: ModelUsage[];
  daily: DailyUsage[];
}

export interface UsageResponse {
  period: string;
  usage: TeamUsage;
}

export interface ChatSession {
  sessionId: string;
  projectPath: string;
  title: string;
  messageCount: number;
  sizeBytes: number;
  checksum: string;
  syncedAt: string;
  mine: boolean;
}

export interface ChatTranscript {
  sessionId: string;
  projectPath: string;
  title: string;
  transcript: string;
  messageCount: number;
  checksum: string;
}

export interface Health {
  status: string;
  latency: { p50: string; p95: string; p99: string };
}
