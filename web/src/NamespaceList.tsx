import { useEffect, useState } from "react";
import { fetchNamespaces, NamespacesError, type Namespace } from "./api";

type LoadState =
  | { kind: "loading" }
  | { kind: "loaded"; namespaces: Namespace[] }
  | { kind: "forbidden" }
  | { kind: "unavailable" };

// The required empty/loading/forbidden/unavailable-upstream states from
// docs/spec/02-product-and-scope.md - each one rendered distinctly, not
// collapsed into a generic "something went wrong."
export default function NamespaceList() {
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    fetchNamespaces()
      .then((namespaces) => {
        if (!cancelled) setState({ kind: "loaded", namespaces });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof NamespacesError && err.status === 403) {
          setState({ kind: "forbidden" });
        } else {
          setState({ kind: "unavailable" });
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (state.kind === "loading") {
    return <p role="status">Loading namespaces…</p>;
  }
  if (state.kind === "forbidden") {
    return <p role="alert">You don't have access to any namespaces.</p>;
  }
  if (state.kind === "unavailable") {
    return <p role="alert">Namespaces are unavailable right now. Try again shortly.</p>;
  }
  if (state.namespaces.length === 0) {
    return <p>No namespaces found for your account.</p>;
  }
  return (
    <ul aria-label="Namespaces">
      {state.namespaces.map((ns) => (
        // React escapes text content by default - this is how
        // docs/spec/04-auth-session-browser-security.md's "no unsafe HTML
        // rendering of Kubernetes content" requirement is met here: there
        // is no dangerouslySetInnerHTML anywhere in this codebase.
        <li key={ns.name}>{ns.name}</li>
      ))}
    </ul>
  );
}
