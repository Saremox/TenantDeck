import { useState } from "react";
import EventsList from "./EventsList";
import Overview from "./Overview";
import PodLogsViewer from "./PodLogsViewer";
import ResourceDetail from "./ResourceDetail";
import ResourceList from "./ResourceList";
import type { PodSummary, ResourceKindKey } from "./api";
import { resourceKinds } from "./resourceKinds";

interface Props {
  namespace: string;
  onBack: () => void;
}

type View =
  | { kind: "overview" }
  | { kind: "events" }
  | { kind: "list"; resourceKind: ResourceKindKey }
  | { kind: "detail"; resourceKind: ResourceKindKey; name: string }
  | { kind: "logs"; podName: string };

// Everything inside one selected namespace: overview, the resource kinds
// from docs/route-allowlist.md, events, and pod logs. Deliberately a
// handful of React state, not a router library - the view set is small
// and fixed for v1; see CLAUDE.md "Code style" on not over-engineering.
export default function Workspace({ namespace, onBack }: Props) {
  const [view, setView] = useState<View>({ kind: "overview" });

  function showList(resourceKind: ResourceKindKey) {
    setView({ kind: "list", resourceKind });
  }

  return (
    <div>
      <button type="button" onClick={onBack}>
        ← Back to namespaces
      </button>
      <h2>{namespace}</h2>

      <nav aria-label="Namespace views">
        <button type="button" onClick={() => setView({ kind: "overview" })}>
          Overview
        </button>
        {resourceKinds.map((k) => (
          <button key={k.key} type="button" onClick={() => showList(k.key)}>
            {k.label}
          </button>
        ))}
        <button type="button" onClick={() => setView({ kind: "events" })}>
          Events
        </button>
      </nav>

      {view.kind === "overview" && <Overview namespace={namespace} />}
      {view.kind === "events" && <EventsList namespace={namespace} />}

      {view.kind === "list" &&
        (() => {
          const config = resourceKinds.find((k) => k.key === view.resourceKind);
          if (!config) return null;
          const resourceKind = view.resourceKind;
          return (
            <ResourceList
              key={resourceKind}
              namespace={namespace}
              config={config}
              onSelect={(name) => setView({ kind: "detail", resourceKind, name })}
              renderRowActions={
                resourceKind === "pods"
                  ? (item: PodSummary) => (
                      <button type="button" onClick={() => setView({ kind: "logs", podName: item.name })}>
                        View logs
                      </button>
                    )
                  : undefined
              }
            />
          );
        })()}

      {view.kind === "detail" &&
        (() => {
          const config = resourceKinds.find((k) => k.key === view.resourceKind);
          const resourceKind = view.resourceKind;
          return (
            <ResourceDetail
              key={`${resourceKind}-${view.name}`}
              namespace={namespace}
              kindKey={resourceKind}
              kindLabel={config?.label ?? resourceKind}
              name={view.name}
              onClose={() => showList(resourceKind)}
            />
          );
        })()}

      {view.kind === "logs" && (
        <PodLogsViewer namespace={namespace} podName={view.podName} onClose={() => showList("pods")} />
      )}
    </div>
  );
}
