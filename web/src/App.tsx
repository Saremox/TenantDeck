import { useEffect, useState } from "react";
import { fetchSessionStatus, logout, type SessionStatus } from "./api";
import NamespaceList from "./NamespaceList";

type AppState = { kind: "loading" } | { kind: "ready"; status: SessionStatus };

export default function App() {
  const [state, setState] = useState<AppState>({ kind: "loading" });

  useEffect(() => {
    fetchSessionStatus().then((status) => setState({ kind: "ready", status }));
  }, []);

  if (state.kind === "loading") {
    return <p role="status">Loading…</p>;
  }

  if (!state.status.authenticated) {
    return (
      <main>
        <h1>TenantDeck</h1>
        <p>Your slice of Kubernetes.</p>
        <a href="/auth/login">Log in</a>
      </main>
    );
  }

  const { subject, csrfToken } = state.status;

  async function handleLogout() {
    if (!csrfToken) return;
    await logout(csrfToken);
    // The session cookie is now gone server-side; reload to re-evaluate
    // the logged-out state rather than trying to fake it client-side.
    window.location.assign("/");
  }

  return (
    <main>
      <h1>TenantDeck</h1>
      {subject && <p>Signed in as {subject}</p>}
      <button type="button" onClick={handleLogout}>
        Log out
      </button>
      <NamespaceList />
    </main>
  );
}
