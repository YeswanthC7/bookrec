// web/src/api/auth.ts
export type Tokens = {
  accessToken: string | null;
  refreshToken: string | null;
};

let accessTokenMem: string | null = null;

const REFRESH_KEY = "bookrec_refresh_token";

export function getAccessToken(): string | null {
  return accessTokenMem;
}

export function getRefreshToken(): string | null {
  return sessionStorage.getItem(REFRESH_KEY);
}

export function setTokens(tokens: Tokens) {
  accessTokenMem = tokens.accessToken;
  if (tokens.refreshToken) sessionStorage.setItem(REFRESH_KEY, tokens.refreshToken);
}

export function clearTokens() {
  accessTokenMem = null;
  sessionStorage.removeItem(REFRESH_KEY);
}
