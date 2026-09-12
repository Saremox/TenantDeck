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

// ApiError distinguishes the UI states docs/spec/02-product-and-scope.md
// requires (forbidden vs. upstream unavailable) from a generic failure -
// every resource fetch in this file throws this on a non-2xx response.
export class ApiError extends Error {
  constructor(public readonly status: number) {
    super(`request failed with status ${status}`);
  }
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { credentials: "same-origin" });
  if (!res.ok) {
    throw new ApiError(res.status);
  }
  return (await res.json()) as T;
}

export async function fetchNamespaces(): Promise<Namespace[]> {
  const data = await getJSON<{ namespaces: Namespace[] }>("/api/namespaces");
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

// --- Namespace overview ---

export interface ResourceQuotaSummary {
  name: string;
  hard: Record<string, string>;
  used: Record<string, string>;
}

export interface LimitRangeItemSummary {
  type: string;
  default?: Record<string, string>;
  defaultRequest?: Record<string, string>;
  max?: Record<string, string>;
  min?: Record<string, string>;
}

export interface LimitRangeSummary {
  name: string;
  limits: LimitRangeItemSummary[];
}

export interface Overview {
  resourceQuotas: ResourceQuotaSummary[];
  limitRanges: LimitRangeSummary[];
}

export function fetchOverview(namespace: string): Promise<Overview> {
  return getJSON<Overview>(`/api/namespaces/${encodeURIComponent(namespace)}/overview`);
}

// --- Generic namespaced resource list/detail (docs/route-allowlist.md) ---

export type ResourceKindKey =
  | "deployments"
  | "statefulsets"
  | "daemonsets"
  | "pods"
  | "jobs"
  | "cronjobs"
  | "services"
  | "ingresses"
  | "persistentvolumeclaims";

export function fetchResourceList<T>(namespace: string, kind: ResourceKindKey): Promise<T[]> {
  return getJSON<{ items: T[] }>(`/api/namespaces/${encodeURIComponent(namespace)}/${kind}`).then(
    (data) => data.items ?? [],
  );
}

export function fetchResourceDetail<T>(namespace: string, kind: ResourceKindKey, name: string): Promise<T> {
  return getJSON<T>(`/api/namespaces/${encodeURIComponent(namespace)}/${kind}/${encodeURIComponent(name)}`);
}

export interface ContainerSummary {
  name: string;
  image: string;
  ready: boolean;
  restartCount: number;
}

export interface PodSummary {
  name: string;
  phase: string;
  ready: boolean;
  containers: ContainerSummary[];
}

export interface DeploymentSummary {
  name: string;
  replicas: number;
  readyReplicas: number;
  updatedReplicas: number;
  availableReplicas: number;
}

export interface StatefulSetSummary {
  name: string;
  replicas: number;
  readyReplicas: number;
  currentReplicas: number;
}

export interface DaemonSetSummary {
  name: string;
  desiredNumberScheduled: number;
  numberReady: number;
  numberAvailable: number;
}

export interface JobSummary {
  name: string;
  active: number;
  succeeded: number;
  failed: number;
}

export interface CronJobSummary {
  name: string;
  schedule: string;
  suspend: boolean;
  lastScheduleTime?: string;
}

export interface ServicePortSummary {
  name?: string;
  port: number;
  targetPort?: string;
  protocol: string;
}

export interface ServiceSummary {
  name: string;
  type: string;
  clusterIP: string;
  ports: ServicePortSummary[];
}

export interface IngressSummary {
  name: string;
  ingressClassName?: string;
  hosts: string[];
}

export interface PersistentVolumeClaimSummary {
  name: string;
  phase: string;
  capacity?: string;
  accessModes: string[];
  storageClassName?: string;
}

// --- Events ---

export interface EventSummary {
  message: string;
  reason: string;
  type: string;
  count: number;
  lastTimestamp: string;
  involvedObject: string;
}

export function fetchEvents(namespace: string): Promise<EventSummary[]> {
  return getJSON<{ items: EventSummary[] }>(`/api/namespaces/${encodeURIComponent(namespace)}/events`).then(
    (data) => data.items ?? [],
  );
}

// --- Pod logs ---

export interface LogStreamOptions {
  tailLines?: number;
  follow?: boolean;
  container?: string;
}

function logsURL(namespace: string, podName: string, opts: LogStreamOptions): string {
  const params = new URLSearchParams();
  if (opts.tailLines) params.set("tailLines", String(opts.tailLines));
  if (opts.follow) params.set("follow", "true");
  if (opts.container) params.set("container", opts.container);
  const query = params.toString();
  return `/api/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(podName)}/logs${query ? "?" + query : ""}`;
}

// streamPodLogs reads the bounded/optionally-following log response chunk
// by chunk, calling onChunk as text arrives - this is what makes "follow"
// show new lines as they're produced rather than only after the stream
// ends. The caller's AbortSignal is what stops it (cancel button, or
// leaving the view) - see docs/route-allowlist.md on cancellation.
export async function streamPodLogs(
  namespace: string,
  podName: string,
  opts: LogStreamOptions,
  onChunk: (text: string) => void,
  signal: AbortSignal,
): Promise<void> {
  const res = await fetch(logsURL(namespace, podName, opts), {
    credentials: "same-origin",
    signal,
  });
  if (!res.ok || !res.body) {
    throw new ApiError(res.status);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) return;
      onChunk(decoder.decode(value, { stream: true }));
    }
  } finally {
    reader.cancel().catch(() => {});
  }
}
