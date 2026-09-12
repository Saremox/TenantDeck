// Thin fetch wrappers for the BFF's own API (docs/route-allowlist.md).
// Every call is same-origin; none of these ever see or set a token - only
// the opaque session cookie the browser already holds, per
// docs/spec/03-architecture.md.

export interface SessionStatus {
  authenticated: boolean;
  subject?: string;
  csrfToken?: string;
}

export async function fetchSessionStatus(): Promise<SessionStatus> {
  const res = await fetch("/auth/session", { credentials: "same-origin" });
  if (!res.ok) {
    return { authenticated: false };
  }
  return (await res.json()) as SessionStatus;
}

export interface Namespace {
  name: string;
}

// NamespacesError distinguishes the UI states docs/spec/02-product-and-scope.md
// requires (forbidden vs. upstream unavailable) from a generic failure.
export class NamespacesError extends Error {
  constructor(public readonly status: number) {
    super(`fetching namespaces failed with status ${status}`);
  }
}

export async function fetchNamespaces(): Promise<Namespace[]> {
  const res = await fetch("/api/namespaces", { credentials: "same-origin" });
  if (!res.ok) {
    throw new NamespacesError(res.status);
  }
  const data = (await res.json()) as { namespaces: Namespace[] };
  return data.namespaces ?? [];
}

export async function logout(csrfToken: string): Promise<void> {
  const res = await fetch("/auth/logout", {
    method: "POST",
    credentials: "same-origin",
    headers: { "X-CSRF-Token": csrfToken },
  });
  if (!res.ok) {
    throw new Error(`logout failed with status ${res.status}`);
  }
}
