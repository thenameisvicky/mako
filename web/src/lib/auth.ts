// Session storage for the Mako dashboard.
//
// The session JWT lives in localStorage and travels in the X-Mako-Token
// header, matching the CLI. Authorization is deliberately left alone: on proxied
// routes the proxy treats it as the caller's own provider credential.

const TOKEN_KEY = "mako.token";
const USER_KEY = "mako.username";

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

export function setSession(token: string, username: string): void {
  try {
    window.localStorage.setItem(TOKEN_KEY, token);
    window.localStorage.setItem(USER_KEY, username);
  } catch {
    // Private-mode browsers refuse storage; the session simply won't persist.
  }
}

export function getStoredUsername(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(USER_KEY);
  } catch {
    return null;
  }
}

export function clearSession(): void {
  try {
    window.localStorage.removeItem(TOKEN_KEY);
    window.localStorage.removeItem(USER_KEY);
  } catch {
    // nothing to clear
  }
}

export function isSignedIn(): boolean {
  return !!getToken();
}
