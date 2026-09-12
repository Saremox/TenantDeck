import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import ResourceDetail from "./ResourceDetail";

function mockFetchOnce(response: { status: number; body?: unknown }) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: response.status >= 200 && response.status < 300,
      status: response.status,
      json: async () => response.body,
    }),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("ResourceDetail", () => {
  it("shows a loading state before the fetch resolves", () => {
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise(() => {})));
    render(
      <ResourceDetail namespace="tenant-a-dev" kindKey="deployments" kindLabel="Deployments" name="web" onClose={() => {}} />,
    );
    expect(screen.getByRole("status")).toHaveTextContent(/loading/i);
  });

  it("renders every field of the loaded item as a definition list entry", async () => {
    mockFetchOnce({ status: 200, body: { name: "web", replicas: 3, readyReplicas: 2 } });
    render(
      <ResourceDetail namespace="tenant-a-dev" kindKey="deployments" kindLabel="Deployments" name="web" onClose={() => {}} />,
    );

    await waitFor(() => expect(screen.getByText("replicas")).toBeInTheDocument());
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("readyReplicas")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("renders null/undefined fields as a dash instead of throwing", async () => {
    mockFetchOnce({ status: 200, body: { name: "web", lastScheduleTime: null } });
    render(
      <ResourceDetail namespace="tenant-a-dev" kindKey="cronjobs" kindLabel="CronJobs" name="web" onClose={() => {}} />,
    );

    await waitFor(() => expect(screen.getByText("lastScheduleTime")).toBeInTheDocument());
    expect(screen.getByText("-")).toBeInTheDocument();
  });

  it("shows the forbidden state on a 403 response", async () => {
    mockFetchOnce({ status: 403 });
    render(
      <ResourceDetail namespace="tenant-a-dev" kindKey="deployments" kindLabel="Deployments" name="web" onClose={() => {}} />,
    );

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/don't have access/i));
  });

  it("shows the unavailable state on a 500 response", async () => {
    mockFetchOnce({ status: 500 });
    render(
      <ResourceDetail namespace="tenant-a-dev" kindKey="deployments" kindLabel="Deployments" name="web" onClose={() => {}} />,
    );

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/unavailable/i));
  });

  it("calls onClose when the back button is clicked", async () => {
    mockFetchOnce({ status: 200, body: { name: "web" } });
    const onClose = vi.fn();
    render(
      <ResourceDetail namespace="tenant-a-dev" kindKey="deployments" kindLabel="Deployments" name="web" onClose={onClose} />,
    );

    const button = await screen.findByRole("button", { name: /back to deployments/i });
    button.click();

    expect(onClose).toHaveBeenCalled();
  });
});
