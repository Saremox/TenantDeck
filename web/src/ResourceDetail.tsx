import { useEffect, useState } from "react";
import { ApiError, fetchResourceDetail, type ResourceKindKey } from "./api";

type LoadState =
  | { kind: "loading" }
  | { kind: "loaded"; item: Record<string, unknown> }
  | { kind: "forbidden" }
  | { kind: "unavailable" };

interface Props {
  namespace: string;
  kindKey: ResourceKindKey;
  kindLabel: string;
  name: string;
  onClose: () => void;
}

// One detail view for every resource kind: the same summary fields the
// list view already fetched, re-fetched by name and shown as a plain
// field list. Deliberately generic rather than bespoke per resource type
// for this v1 pass - see docs/route-allowlist.md.
export default function ResourceDetail({ namespace, kindKey, kindLabel, name, onClose }: Props) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    // See ResourceList.tsx: no synchronous setState here, callers remount
    // via `key` when namespace/kindKey/name changes.
    fetchResourceDetail<Record<string, unknown>>(namespace, kindKey, name)
      .then((item) => {
        if (!cancelled) setState({ kind: "loaded", item });
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
  }, [namespace, kindKey, name]);

  return (
    <section aria-label={`${kindLabel} detail`}>
      <button type="button" onClick={onClose}>
        ← Back to {kindLabel.toLowerCase()}
      </button>
      <h3>{name}</h3>
      {state.kind === "loading" && <p role="status">Loading…</p>}
      {state.kind === "forbidden" && <p role="alert">You don't have access to this resource.</p>}
      {state.kind === "unavailable" && <p role="alert">This resource is unavailable right now. Try again shortly.</p>}
      {state.kind === "loaded" && (
        <dl>
          {Object.entries(state.item).map(([field, value]) => (
            <div key={field}>
              <dt>{field}</dt>
              <dd>{formatFieldValue(value)}</dd>
            </div>
          ))}
        </dl>
      )}
    </section>
  );
}

function formatFieldValue(value: unknown): string {
  if (value === null || value === undefined) return "-";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
