import {
  api,
  type LogoutResponse,
  type RefreshResponse,
  type LoginResponse,
  type Paginated,
  type Book,
  type PopularBook,
} from "./client";

/**
 * System
 */
export async function healthz() {
  const { data } = await api.get<{ status: string }>("/healthz");
  return data;
}

export async function stats() {
  const { data } = await api.get<Record<string, unknown>>("/stats");
  return data;
}

/**
 * Books
 */
export async function booksList(params?: { page?: number; limit?: number }) {
  const { data } = await api.get<Paginated<Book>>("/books", { params });
  return data;
}

export async function booksPopular() {
  const { data } = await api.get<PopularBook[]>("/books/popular");
  return data;
}

export async function booksSearch(params: {
  q?: string;
  author?: string;
  year_from?: number;
  year_to?: number;
  sort?: "newest" | "popular" | "relevance" | string;
  page?: number;
  limit?: number;
}) {
  const { data } = await api.get<Paginated<Book>>("/books/search", { params });
  return data;
}

/**
 * Auth
 */
export async function authLogin(email: string, password: string): Promise<LoginResponse> {
  const body = new URLSearchParams();
  body.set("email", email);
  body.set("password", password);

  const { data } = await api.post<LoginResponse>("/login", body, {
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
  });

  return data;
}

export async function authRefresh(refresh_token: string): Promise<RefreshResponse> {
  const body = new URLSearchParams();
  body.set("refresh_token", refresh_token);

  const { data } = await api.post<RefreshResponse>("/refresh", body, {
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
  });

  return data;
}

export async function authLogout(refresh_token: string): Promise<LogoutResponse> {
  const body = new URLSearchParams();
  body.set("refresh_token", refresh_token);

  const { data } = await api.post<LogoutResponse>("/logout", body, {
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
  });

  return data;
}

export async function authLogoutAll(): Promise<LogoutResponse> {
  const { data } = await api.post<LogoutResponse>("/logout-all");
  return data;
}

/**
 * Users / Recommendations
 */
export async function usersList() {
  const { data } = await api.get("/users");
  return data;
}

export async function usersCreate(email: string, handle: string, password: string) {
  const body = new URLSearchParams();
  body.set("email", email);
  body.set("handle", handle);
  body.set("password", password);

  const { data } = await api.post("/users", body, {
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
  });

  return data;
}

export async function userHistory(userId: number) {
  const { data } = await api.get(`/users/${userId}/history`);
  return data;
}

export async function recommendations(userId: number) {
  const { data } = await api.get(`/recommendations/${userId}`);
  return data;
}
