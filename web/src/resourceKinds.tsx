import type { ReactNode } from "react";
import type {
  CronJobSummary,
  DaemonSetSummary,
  DeploymentSummary,
  IngressSummary,
  JobSummary,
  PersistentVolumeClaimSummary,
  PodSummary,
  ResourceKindKey,
  ServiceSummary,
  StatefulSetSummary,
} from "./api";

export interface ColumnDef<T> {
  header: string;
  // Method-shorthand signature rather than a `render: (item: T) => ReactNode`
  // property: TypeScript checks method parameters bivariantly, which is what
  // lets a heterogeneous array of `ResourceKindConfig<Specific>` literals
  // (see resourceKinds below) satisfy `ResourceKindConfig[]` (= `<unknown>`)
  // without an `any` escape hatch.
  render(item: T): ReactNode;
}

export interface ResourceKindConfig<T = unknown> {
  key: ResourceKindKey;
  label: string;
  hasDetail: boolean;
  columns: ColumnDef<T>[];
}

const nameColumn = <T extends { name: string }>(): ColumnDef<T> => ({
  header: "Name",
  render: (item) => item.name,
});

// isSafeHostname is deliberately strict: only letters, digits, '.', and
// '-' - no scheme, no whitespace, no control characters. An ingress host
// that doesn't match this renders as plain text instead of a link. See
// docs/spec/04-auth-session-browser-security.md ("validate URL schemes")
// and Lens 6 of tenantdeck-security-review - the host string comes from
// Kubernetes-sourced, effectively attacker-controlled data, not from
// TenantDeck.
function isSafeHostname(host: string): boolean {
  return /^[a-zA-Z0-9.-]+$/.test(host);
}

export const resourceKinds: ResourceKindConfig[] = [
  {
    key: "deployments",
    label: "Deployments",
    hasDetail: true,
    columns: [
      nameColumn<DeploymentSummary>(),
      { header: "Ready", render: (d: DeploymentSummary) => `${d.readyReplicas}/${d.replicas}` },
      { header: "Updated", render: (d: DeploymentSummary) => d.updatedReplicas },
      { header: "Available", render: (d: DeploymentSummary) => d.availableReplicas },
    ],
  },
  {
    key: "statefulsets",
    label: "StatefulSets",
    hasDetail: true,
    columns: [
      nameColumn<StatefulSetSummary>(),
      { header: "Ready", render: (s: StatefulSetSummary) => `${s.readyReplicas}/${s.replicas}` },
      { header: "Current", render: (s: StatefulSetSummary) => s.currentReplicas },
    ],
  },
  {
    key: "daemonsets",
    label: "DaemonSets",
    hasDetail: true,
    columns: [
      nameColumn<DaemonSetSummary>(),
      { header: "Ready", render: (d: DaemonSetSummary) => `${d.numberReady}/${d.desiredNumberScheduled}` },
      { header: "Available", render: (d: DaemonSetSummary) => d.numberAvailable },
    ],
  },
  {
    key: "pods",
    label: "Pods",
    hasDetail: true,
    columns: [
      nameColumn<PodSummary>(),
      { header: "Phase", render: (p: PodSummary) => p.phase },
      { header: "Ready", render: (p: PodSummary) => (p.ready ? "Yes" : "No") },
      {
        header: "Restarts",
        render: (p: PodSummary) => p.containers.reduce((sum, c) => sum + c.restartCount, 0),
      },
    ],
  },
  {
    key: "jobs",
    label: "Jobs",
    hasDetail: true,
    columns: [
      nameColumn<JobSummary>(),
      { header: "Active", render: (j: JobSummary) => j.active },
      { header: "Succeeded", render: (j: JobSummary) => j.succeeded },
      { header: "Failed", render: (j: JobSummary) => j.failed },
    ],
  },
  {
    key: "cronjobs",
    label: "CronJobs",
    hasDetail: true,
    columns: [
      nameColumn<CronJobSummary>(),
      { header: "Schedule", render: (c: CronJobSummary) => c.schedule },
      { header: "Suspended", render: (c: CronJobSummary) => (c.suspend ? "Yes" : "No") },
      { header: "Last run", render: (c: CronJobSummary) => c.lastScheduleTime ?? "never" },
    ],
  },
  {
    key: "services",
    label: "Services",
    hasDetail: true,
    columns: [
      nameColumn<ServiceSummary>(),
      { header: "Type", render: (s: ServiceSummary) => s.type },
      { header: "Cluster IP", render: (s: ServiceSummary) => s.clusterIP },
      {
        header: "Ports",
        render: (s: ServiceSummary) => s.ports.map((p) => `${p.port}/${p.protocol}`).join(", "),
      },
    ],
  },
  {
    key: "ingresses",
    label: "Ingresses",
    hasDetail: true,
    columns: [
      nameColumn<IngressSummary>(),
      {
        header: "Hosts",
        render: (ing: IngressSummary) => (
          <>
            {ing.hosts.map((host, i) => (
              <span key={host}>
                {i > 0 && ", "}
                {isSafeHostname(host) ? (
                  <a href={`https://${host}`} target="_blank" rel="noreferrer noopener">
                    {host}
                  </a>
                ) : (
                  host
                )}
              </span>
            ))}
          </>
        ),
      },
    ],
  },
  {
    key: "persistentvolumeclaims",
    label: "Persistent volume claims",
    hasDetail: true,
    columns: [
      nameColumn<PersistentVolumeClaimSummary>(),
      { header: "Phase", render: (p: PersistentVolumeClaimSummary) => p.phase },
      { header: "Capacity", render: (p: PersistentVolumeClaimSummary) => p.capacity ?? "-" },
      { header: "Access modes", render: (p: PersistentVolumeClaimSummary) => p.accessModes.join(", ") },
    ],
  },
];
