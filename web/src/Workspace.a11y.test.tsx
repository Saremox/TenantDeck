import { render } from "@testing-library/react";
import { axe } from "jest-axe";
import { afterEach, describe, expect, it, vi } from "vitest";
import Workspace from "./Workspace";

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return {
    ...actual,
    streamPodLogs: vi.fn().mockImplementation((_ns: string, _pod: string, _opts: unknown, onChunk: (c: string) => void) => {
      onChunk("a log line\n");
      return new Promise(() => {});
    }),
  };
});

function mockApiFetch() {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockImplementation((url: string) => {
      if (url.endsWith("/overview")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({
            resourceQuotas: [{ name: "compute-quota", hard: { "requests.cpu": "4" }, used: { "requests.cpu": "1" } }],
            limitRanges: [],
          }),
        });
      }
      if (url.endsWith("/events")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({
            items: [
              {
                type: "Warning",
                reason: "BackOff",
                involvedObject: "pod/web-1",
                message: "container failed to start",
                count: 1,
                lastTimestamp: "2026-01-01T00:00:00Z",
              },
            ],
          }),
        });
      }
      if (/\/deployments\/[^/]+$/.test(url)) {
        return Promise.resolve({ ok: true, status: 200, json: async () => ({ name: "web", replicas: 1 }) });
      }
      if (url.endsWith("/deployments")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({
            items: [{ name: "web", replicas: 1, readyReplicas: 1, updatedReplicas: 1, availableReplicas: 1 }],
          }),
        });
      }
      if (url.endsWith("/pods")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({ items: [{ name: "web-1", phase: "Running", ready: true, containers: [] }] }),
        });
      }
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ items: [] }) });
    }),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

// Automated accessibility gate for the per-namespace views added in
// Phase 3 - see the tenantdeck-testing skill ("every required view/page...
// must pass with zero serious/critical violations").
describe("Workspace accessibility", () => {
  it("has no axe violations in the overview view", async () => {
    mockApiFetch();
    const { container, findByText } = render(<Workspace namespace="tenant-a-dev" onBack={() => {}} />);
    await findByText("compute-quota");

    expect(await axe(container)).toHaveNoViolations();
  });

  it("has no axe violations in a resource list view", async () => {
    mockApiFetch();
    const { container, findByRole } = render(<Workspace namespace="tenant-a-dev" onBack={() => {}} />);
    (await findByRole("button", { name: "Deployments" })).click();
    await findByRole("cell", { name: "web" });

    expect(await axe(container)).toHaveNoViolations();
  });

  it("has no axe violations in a resource detail view", async () => {
    mockApiFetch();
    const { container, findByRole } = render(<Workspace namespace="tenant-a-dev" onBack={() => {}} />);
    (await findByRole("button", { name: "Deployments" })).click();
    (await findByRole("button", { name: /details/i })).click();
    await findByRole("heading", { name: "web" });

    expect(await axe(container)).toHaveNoViolations();
  });

  it("has no axe violations in the events view", async () => {
    mockApiFetch();
    const { container, findByRole, findByText } = render(<Workspace namespace="tenant-a-dev" onBack={() => {}} />);
    (await findByRole("button", { name: "Events" })).click();
    await findByText("BackOff");

    expect(await axe(container)).toHaveNoViolations();
  });

  it("has no axe violations in the pod logs view", async () => {
    mockApiFetch();
    const { container, findByRole, findByLabelText } = render(
      <Workspace namespace="tenant-a-dev" onBack={() => {}} />,
    );
    (await findByRole("button", { name: "Pods" })).click();
    (await findByRole("button", { name: /view logs/i })).click();
    await findByLabelText(/log output/i);

    expect(await axe(container)).toHaveNoViolations();
  });
});
