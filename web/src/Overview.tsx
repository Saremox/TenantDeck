import { useEffect, useState } from "react";
import { ApiError, fetchOverview, type Overview as OverviewData } from "./api";

type LoadState =
  | { kind: "loading" }
  | { kind: "loaded"; data: OverviewData }
  | { kind: "forbidden" }
  | { kind: "unavailable" };

interface Props {
  namespace: string;
}

export default function Overview({ namespace }: Props) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    // See ResourceList.tsx: no synchronous setState here. Overview's
    // namespace prop is stable for the component's lifetime - Workspace
    // (its parent) itself remounts per namespace, see App.tsx.
    fetchOverview(namespace)
      .then((data) => {
        if (!cancelled) setState({ kind: "loaded", data });
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
  }, [namespace]);

  if (state.kind === "loading") {
    return <p role="status">Loading overview…</p>;
  }
  if (state.kind === "forbidden") {
    return <p role="alert">You don't have access to this namespace's overview.</p>;
  }
  if (state.kind === "unavailable") {
    return <p role="alert">The overview is unavailable right now. Try again shortly.</p>;
  }

  const { resourceQuotas, limitRanges } = state.data;

  return (
    <section aria-label="Namespace overview">
      <h3>Resource quotas</h3>
      {resourceQuotas.length === 0 ? (
        <p>No resource quotas set for this namespace.</p>
      ) : (
        resourceQuotas.map((q) => (
          <table key={q.name}>
            <caption>{q.name}</caption>
            <thead>
              <tr>
                <th>Resource</th>
                <th>Used</th>
                <th>Hard limit</th>
              </tr>
            </thead>
            <tbody>
              {Object.keys(q.hard).map((resource) => (
                <tr key={resource}>
                  <td>{resource}</td>
                  <td>{q.used[resource] ?? "0"}</td>
                  <td>{q.hard[resource]}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ))
      )}

      <h3>Limit ranges</h3>
      {limitRanges.length === 0 ? (
        <p>No limit ranges set for this namespace.</p>
      ) : (
        limitRanges.map((lr) => (
          <table key={lr.name}>
            <caption>{lr.name}</caption>
            <thead>
              <tr>
                <th>Type</th>
                <th>Default</th>
                <th>Default request</th>
                <th>Min</th>
                <th>Max</th>
              </tr>
            </thead>
            <tbody>
              {lr.limits.map((limit, i) => (
                <tr key={`${lr.name}-${i}`}>
                  <td>{limit.type}</td>
                  <td>{formatLimitMap(limit.default)}</td>
                  <td>{formatLimitMap(limit.defaultRequest)}</td>
                  <td>{formatLimitMap(limit.min)}</td>
                  <td>{formatLimitMap(limit.max)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ))
      )}
    </section>
  );
}

function formatLimitMap(m?: Record<string, string>): string {
  if (!m || Object.keys(m).length === 0) return "-";
  return Object.entries(m)
    .map(([k, v]) => `${k}: ${v}`)
    .join(", ");
}
