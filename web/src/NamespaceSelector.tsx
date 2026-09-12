import { useEffect, useState } from "react";
import { ApiError, fetchNamespaces, type Namespace } from "./api";

type LoadState =
  | { kind: "loading" }
  | { kind: "loaded"; namespaces: Namespace[] }
  | { kind: "forbidden" }
  | { kind: "unavailable" };

interface Props {
  onSelect: (namespace: string) => void;
}

// The required empty/loading/forbidden/unavailable-upstream states from
// docs/spec/02-product-and-scope.md - each one rendered distinctly, not
// collapsed into a generic "something went wrong."
export default function NamespaceSelector({ onSelect }: Props) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    fetchNamespaces()
      .then((namespaces) => {
        if (!cancelled) setState({ kind: "loaded", namespaces });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 403) {
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
        <li key={ns.name}>
          {/* React escapes text content by default - no dangerouslySetInnerHTML
              anywhere in this codebase; see docs/spec/04. */}
          <button type="button" onClick={() => onSelect(ns.name)}>
            {ns.name}
          </button>
        </li>
      ))}
    </ul>
  );
}
