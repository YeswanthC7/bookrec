import { useEffect, useState } from "react";
import {
  getHealth,
  getStats,
  listBooks,
  listPopularBooks,
  searchBooks,
  login,
  logout,
  getRecommendations,
  logoutAll,
  type HealthResponse,
  type Book,
  type Paginated,
  type PopularBook,
} from "./api/client";

type State = {
  health?: HealthResponse;
  stats?: Record<string, unknown>;
  books?: Paginated<Book>;
  popular?: PopularBook[];
  search?: Paginated<Book>;
};

export default function App() {
  const [data, setData] = useState<State>({});
  const [error, setError] = useState<string | null>(null);
  const [msg, setMsg] = useState<string>("");

  const [email, setEmail] = useState("test@example.com");
  const [password, setPassword] = useState("pass123");
  const [userId, setUserId] = useState<number>(1);

  useEffect(() => {
    (async () => {
      try {
        const [health, stats, books, popular, search] = await Promise.all([
          getHealth(),
          getStats(),
          listBooks({ page: 1, limit: 5 }),
          listPopularBooks(),
          searchBooks({ q: "harry", page: 1, limit: 5, sort: "relevance" }),
        ]);

        setData({ health, stats, books, popular, search });
      } catch (e: any) {
        setError(e?.message ?? "Request failed");
      }
    })();
  }, []);

  async function onLogin() {
    setError(null);
    setMsg("");
    try {
      const res = await login(email, password);
      const maybeId = (res.user as any)?.id;
      if (typeof maybeId === "number") setUserId(maybeId);
      setMsg("Logged in ✅");
    } catch (e: any) {
      setError(e?.message ?? "Login failed");
    }
  }

  async function onRecs() {
    setError(null);
    setMsg("");
    try {
      const r = await getRecommendations(userId);
      setMsg(`Recommendations ✅ (count: ${Array.isArray(r) ? r.length : "?"})`);
      console.log("recommendations:", r);
    } catch (e: any) {
      setError(e?.message ?? "Recommendations failed");
    }
  }

  async function onLogout() {
    setError(null);
    setMsg("");
    try {
      await logout();
      setMsg("Logged out ✅");
    } catch (e: any) {
      setError(e?.message ?? "Logout failed");
    }
  }

  async function onLogoutAll() {
    setError(null);
    setMsg("");
    try {
      const r = await logoutAll();
      setMsg(`Logout-all ✅: ${r.message}`);
    } catch (e: any) {
      setError(e?.message ?? "Logout-all failed");
    }
  }

  return (
    <div style={{ padding: 24, fontFamily: "system-ui, sans-serif" }}>
      <h1>BookRec Web</h1>

      {error && <p>❌ {error}</p>}
      {msg && <p>✅ {msg}</p>}

      {data.health && <p>✅ API health: {data.health.status}</p>}

      <h2>Auth test</h2>
      <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
        <input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="email" />
        <input
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="password"
          type="password"
        />
        <button onClick={onLogin}>Login</button>

        <input
          value={userId}
          onChange={(e) => setUserId(Number(e.target.value))}
          placeholder="user_id"
          type="number"
          style={{ width: 110 }}
        />
        <button onClick={onRecs}>Get recommendations</button>

        <button onClick={onLogout}>Logout</button>
        <button onClick={onLogoutAll}>Logout all</button>
      </div>

      <h2>Stats</h2>
      <pre style={{ background: "#f6f6f6", padding: 12, borderRadius: 8 }}>
        {JSON.stringify(data.stats ?? null, null, 2)}
      </pre>

      <h2>Books (page 1, limit 5)</h2>
      <pre style={{ background: "#f6f6f6", padding: 12, borderRadius: 8 }}>
        {JSON.stringify(data.books ?? null, null, 2)}
      </pre>

      <h2>Popular</h2>
      <pre style={{ background: "#f6f6f6", padding: 12, borderRadius: 8 }}>
        {JSON.stringify(data.popular ?? null, null, 2)}
      </pre>

      <h2>Search (q=harry)</h2>
      <pre style={{ background: "#f6f6f6", padding: 12, borderRadius: 8 }}>
        {JSON.stringify(data.search ?? null, null, 2)}
      </pre>
    </div>
  );
}
