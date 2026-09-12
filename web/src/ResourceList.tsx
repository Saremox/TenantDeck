import { useEffect, useState } from "react";
import type { ReactNode } from "react";
import { ApiError, fetchResourceList } from "./api";
import type { ResourceKindConfig } from "./resourceKinds";

type LoadState<T> =
  | { kind: "loading" }
  | { kind: "loaded"; items: T[] }
  | { kind: "forbidden" }
  | { kind: "unavailable" };

interface Props<T extends { name: string }> {
  namespace: string;
  config: ResourceKindConfig<T>;
  onSelect?: (name: string) => void;
  renderRowActions?: (item: T) => ReactNode;
}

// One table for every resource kind in docs/route-allowlist.md - the
// required empty/loading/forbidden/unavailable-upstream states
// (docs/spec/02-product-and-scope.md) are handled exactly once here
// rather than re-implemented per resource type.
export default function ResourceList<T extends { name: string }>({
  namespace,
  config,
  onSelect,
  renderRowActions,
}: Props<T>) {
  const [state, setState] = useState<LoadState<T>>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    // No synchronous setState here: a fresh mount already starts at
    // { kind: "loading" } via useState's initial value. Callers remount
    // this component (via a `key` keyed on namespace/resource kind) when
    // identity changes, rather than this effect resetting state in place.
    fetchResourceList<T>(namespace, config.key)
      .then((items) => {
        if (!cancelled) setState({ kind: "loaded", items });
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
  }, [namespace, config.key]);

  const label = config.label.toLowerCase();

  if (state.kind === "loading") {
    return <p role="status">Loading {label}…</p>;
  }
  if (state.kind === "forbidden") {
    return <p role="alert">You don't have access to {label} in this namespace.</p>;
  }
  if (state.kind === "unavailable") {
    return <p role="alert">{config.label} are unavailable right now. Try again shortly.</p>;
  }
  if (state.items.length === 0) {
    return <p>No {label} found in this namespace.</p>;
  }

  return (
    <table>
      <caption>{config.label}</caption>
      <thead>
        <tr>
          {config.columns.map((col) => (
            <th key={col.header}>{col.header}</th>
          ))}
          {renderRowActions && <th>Actions</th>}
        </tr>
      </thead>
      <tbody>
        {state.items.map((item) => (
          <tr key={item.name}>
            {config.columns.map((col) => (
              <td key={col.header}>{col.render(item)}</td>
            ))}
            {renderRowActions && <td>{renderRowActions(item)}</td>}
            {config.hasDetail && onSelect && (
              <td>
                <button type="button" onClick={() => onSelect(item.name)}>
                  Details
                </button>
              </td>
            )}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
