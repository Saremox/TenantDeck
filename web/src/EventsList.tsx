import { useEffect, useState } from "react";
import { ApiError, fetchEvents, type EventSummary } from "./api";

type LoadState =
  | { kind: "loading" }
  | { kind: "loaded"; events: EventSummary[] }
  | { kind: "forbidden" }
  | { kind: "unavailable" };

interface Props {
  namespace: string;
}

export default function EventsList({ namespace }: Props) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    // See ResourceList.tsx / Overview.tsx: no synchronous setState here.
    fetchEvents(namespace)
      .then((events) => {
        if (!cancelled) setState({ kind: "loaded", events });
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
    return <p role="status">Loading events…</p>;
  }
  if (state.kind === "forbidden") {
    return <p role="alert">You don't have access to this namespace's events.</p>;
  }
  if (state.kind === "unavailable") {
    return <p role="alert">Events are unavailable right now. Try again shortly.</p>;
  }
  if (state.events.length === 0) {
    return <p>No events found in this namespace.</p>;
  }

  return (
    <table>
      <caption>Events</caption>
      <thead>
        <tr>
          <th>Type</th>
          <th>Reason</th>
          <th>Object</th>
          <th>Message</th>
          <th>Count</th>
          <th>Last seen</th>
        </tr>
      </thead>
      <tbody>
        {state.events.map((ev, i) => (
          // Event message/reason text is hostile input (whatever produced
          // the event, not TenantDeck or Kubernetes itself) - rendered as
          // plain text cell content only, never dangerouslySetInnerHTML.
          // See docs/spec/04 and Lens 6 of tenantdeck-security-review.
          <tr key={`${ev.involvedObject}-${ev.reason}-${i}`}>
            <td>{ev.type}</td>
            <td>{ev.reason}</td>
            <td>{ev.involvedObject}</td>
            <td>{ev.message}</td>
            <td>{ev.count}</td>
            <td>{ev.lastTimestamp}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
