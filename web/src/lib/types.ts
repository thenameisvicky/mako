// Payload shapes mirrored from pkg/models on the Go side.

export interface Me {
  userId: string;
  username: string;
  email: string;
  teamId: string;
  teamName: string;
  role: string;
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
