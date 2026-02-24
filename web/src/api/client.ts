// web/src/api/client.ts
import axios, { AxiosError } from "axios";
import { getAccessToken, getRefreshToken, setTokens, clearTokens } from "./auth";

const baseURL = (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? "http://localhost:8080";

export const api = axios.create({
  baseURL,
  headers: { Accept: "application/json" },
});

// Attach Authorization header when we have an access token
api.interceptors.request.use((config) => {
  const token = getAccessToken();
  if (token) {
    config.headers = config.headers ?? {};
    (config.headers as any).Authorization = `Bearer ${token}`;
  }
  return config;
});

let isRefreshing = false;
let refreshWaiters: Array<(token: string | null) => void> = [];

function notifyRefreshWaiters(token: string | null) {
  refreshWaiters.forEach((cb) => cb(token));
  refreshWaiters = [];
}

async function refreshTokens(): Promise<string | null> {
  const rt = getRefreshToken();
  if (!rt) return null;

  const form = new FormData();
  form.append("refresh_token", rt);

  const { data } = await api.post<RefreshResponse>("/refresh", form);
  setTokens({ accessToken: data.access_token, refreshToken: data.refresh_token });
  return data.access_token;
}

// On 401, try refresh once, then retry original request
api.interceptors.response.use(
  (res) => res,
  async (err: AxiosError) => {
    const status = err.response?.status;
    const original: any = err.config;
    if (status !== 401 || !original || original._retry) throw err;

    original._retry = true;

    if (isRefreshing) {
      const token = await new Promise<string | null>((resolve) => refreshWaiters.push(resolve));
      if (!token) throw err;
      original.headers.Authorization = `Bearer ${token}`;
      return api.request(original);
    }

    isRefreshing = true;
    try {
      const token = await refreshTokens();
      notifyRefreshWaiters(token);
      if (!token) {
        clearTokens();
        throw err;
      }
      original.headers.Authorization = `Bearer ${token}`;
      return api.request(original);
    } finally {
      isRefreshing = false;
    }
  }
);

/** Types (keep minimal until you want strict models) */
export type HealthResponse = { status: string };

export type Book = {
  id: number;
  title: string;
  author: string;
  year: number;
};

export type Paginated<T> = {
  data: T[];
  page: number;
  limit: number;
  // some endpoints may omit total; keep optional
  total?: number;
};

export type PopularBook = {
  id: number;
  title: string;
  author: string;
  likes: number;
};

export type LoginResponse = {
  access_token: string;
  refresh_token: string;
  user: Record<string, unknown>;
};

export type RefreshResponse = { access_token: string; refresh_token: string };
export type LogoutResponse = { message: string };

/** Endpoints */
export async function getHealth(): Promise<HealthResponse> {
  const { data } = await api.get<HealthResponse>("/healthz");
  return data;
}

export async function listBooks(params?: { page?: number; limit?: number }): Promise<Paginated<Book>> {
  const { data } = await api.get<Paginated<Book>>("/books", { params });
  return data;
}

export async function listPopularBooks(): Promise<PopularBook[]> {
  const { data } = await api.get<PopularBook[]>("/books/popular");
  return data;
}

export async function searchBooks(params: {
  q?: string;
  author?: string;
  year_from?: number;
  year_to?: number;
  sort?: string;
  page?: number;
  limit?: number;
}): Promise<Paginated<Book>> {
  const { data } = await api.get<Paginated<Book>>("/books/search", { params });
  return data;
}

export async function getStats(): Promise<Record<string, unknown>> {
  const { data } = await api.get<Record<string, unknown>>("/stats");
  return data;
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  const form = new FormData();
  form.append("email", email);
  form.append("password", password);

  const { data } = await api.post<LoginResponse>("/login", form);
  setTokens({ accessToken: data.access_token, refreshToken: data.refresh_token });
  return data;
}

export async function logout(): Promise<LogoutResponse> {
  const rt = getRefreshToken();
  const form = new FormData();
  form.append("refresh_token", rt ?? "");

  const { data } = await api.post<LogoutResponse>("/logout", form);
  clearTokens();
  return data;
}

export async function getRecommendations(userId: number) {
  const { data } = await api.get(`/recommendations/${userId}`);
  return data;
}

export async function logoutAll(): Promise<LogoutResponse> {
  const { data } = await api.post<LogoutResponse>("/logout-all");
  clearTokens();
  return data;
}
