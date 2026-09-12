import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import Overview from "./Overview";

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

describe("Overview", () => {
  it("shows a loading state before the fetch resolves", () => {
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise(() => {})));
    render(<Overview namespace="tenant-a-dev" />);
    expect(screen.getByRole("status")).toHaveTextContent(/loading overview/i);
  });

  it("renders resource quota and limit range tables once loaded", async () => {
    mockFetchOnce({
      status: 200,
      body: {
        resourceQuotas: [{ name: "compute-quota", hard: { "requests.cpu": "4" }, used: { "requests.cpu": "1" } }],
        limitRanges: [{ name: "defaults", limits: [{ type: "Container", default: { cpu: "500m" } }] }],
      },
    });
    render(<Overview namespace="tenant-a-dev" />);

    await waitFor(() => expect(screen.getByText("compute-quota")).toBeInTheDocument());
    expect(screen.getByText("requests.cpu")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("defaults")).toBeInTheDocument();
    expect(screen.getByText("cpu: 500m")).toBeInTheDocument();
  });

  it("shows the empty-state text when there are no quotas or limit ranges", async () => {
    mockFetchOnce({ status: 200, body: { resourceQuotas: [], limitRanges: [] } });
    render(<Overview namespace="tenant-a-dev" />);

    await waitFor(() => expect(screen.getByText(/no resource quotas set/i)).toBeInTheDocument());
    expect(screen.getByText(/no limit ranges set/i)).toBeInTheDocument();
  });

  it("shows the forbidden state on a 403 response", async () => {
    mockFetchOnce({ status: 403 });
    render(<Overview namespace="tenant-a-dev" />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/don't have access/i));
  });

  it("shows the unavailable state on a 500 response", async () => {
    mockFetchOnce({ status: 500 });
    render(<Overview namespace="tenant-a-dev" />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/unavailable/i));
  });
});
