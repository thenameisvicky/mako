import { clearSession, getToken } from "./auth";
import type {
  Budget,
  ChatSession,
  ChatTranscript,
  Health,
  Me,
  Policy,
  UsageResponse,
} from "./types";

// The Mako proxy the dashboard reads from.
export const PROXY_URL =
  process.env.NEXT_PUBLIC_MAKO_PROXY_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  auth?: boolean;
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, auth = true } = opts;

  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";

  if (auth) {
    const token = getToken();
    if (!token) throw new ApiError("Not signed in", 401);
    // The proxy reserves Authorization for the caller's provider key.
    headers["X-Mako-Token"] = token;
  }

  let res: Response;
  try {
    res = await fetch(`${PROXY_URL}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    // A blocked CORS preflight and an unreachable server are indistinguishable
    // here: the browser deliberately hides CORS detail from JavaScript. Name
    // both causes, because the second is by far the easier one to overlook.
    throw new ApiError(
      `Cannot reach the Mako proxy at ${PROXY_URL}. Either it is not running, ` +
        `or it is not allowing requests from ${
          typeof window === "undefined" ? "this origin" : window.location.origin
        } — add that to its CORS_ALLOWED_ORIGINS.`,
      0,
    );
  }

  // A 401 means the JWT expired or was revoked; drop it so the guard redirects.
  if (res.status === 401 && auth) {
    clearSession();
    throw new ApiError("Session expired, please sign in again", 401);
  }

  let payload: Record<string, unknown> = {};
  try {
    payload = await res.json();
  } catch {
    throw new ApiError(`Mako proxy returned an unreadable response (${res.status})`, res.status);
  }

  if (!res.ok || payload.status === "error") {
    throw new ApiError(
      (payload.message as string) || `Request failed (${res.status})`,
      res.status,
    );
  }

  return payload as T;
}

export const api = {
  login: (
    username: string,
    password: string,
    opts: { isCreate?: boolean; teamId?: string; createTeam?: boolean; teamName?: string } = {},
  ) =>
    request<{ token: string }>("/api/v1/login", {
      method: "POST",
      auth: false,
      body: {
        username,
        password,
        isCreate: opts.isCreate ?? false,
        teamId: opts.teamId ?? "",
        createTeam: opts.createTeam ?? false,
        teamName: opts.teamName ?? "",
      },
    }),

  me: () => request<{ me: Me }>("/api/v1/me").then((r) => r.me),

  health: () => request<Health>("/api/v1/health", { auth: false }),

  getPolicy: () => request<{ policy: Policy }>("/api/v1/policy").then((r) => r.policy),

  savePolicy: (policy: Pick<Policy, "allowedModels" | "blockedModels" | "maxTokensPerRequest">) =>
    request<{ policy: Policy }>("/api/v1/policy", { method: "PUT", body: policy }).then(
      (r) => r.policy,
    ),

  getBudget: () => request<{ budget: Budget }>("/api/v1/budget").then((r) => r.budget),

  saveBudget: (budget: Pick<Budget, "period" | "tokenLimit">) =>
    request<{ budget: Budget }>("/api/v1/budget", { method: "PUT", body: budget }).then(
      (r) => r.budget,
    ),

  usage: () => request<UsageResponse>("/api/v1/usage"),

  chats: () =>
    request<{ sessions: ChatSession[] }>("/api/v1/chats").then((r) => r.sessions ?? []),

  chat: (sessionId: string) =>
    request<{ session: ChatTranscript }>(
      `/api/v1/chats/${encodeURIComponent(sessionId)}`,
    ).then((r) => r.session),
};
