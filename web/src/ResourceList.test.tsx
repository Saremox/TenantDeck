import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import ResourceList from "./ResourceList";
import type { ResourceKindConfig } from "./resourceKinds";

interface TestItem {
  name: string;
  phase: string;
}

const testConfig: ResourceKindConfig<TestItem> = {
  key: "deployments",
  label: "Widgets",
  hasDetail: true,
  columns: [
    { header: "Name", render: (item) => item.name },
    { header: "Phase", render: (item) => item.phase },
  ],
};

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

describe("ResourceList", () => {
  it("shows a loading state before the fetch resolves", () => {
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise(() => {})));
    render(<ResourceList namespace="tenant-a-dev" config={testConfig} />);
    expect(screen.getByRole("status")).toHaveTextContent(/loading widgets/i);
  });

  it("renders one row per item with the configured columns", async () => {
    mockFetchOnce({ status: 200, body: { items: [{ name: "web", phase: "Running" }] } });
    render(<ResourceList namespace="tenant-a-dev" config={testConfig} />);

    await waitFor(() => expect(screen.getByRole("cell", { name: "web" })).toBeInTheDocument());
    expect(screen.getByRole("cell", { name: "Running" })).toBeInTheDocument();
  });

  it("calls onSelect with the item's name when its Details button is clicked", async () => {
    mockFetchOnce({ status: 200, body: { items: [{ name: "web", phase: "Running" }] } });
    const onSelect = vi.fn();
    render(<ResourceList namespace="tenant-a-dev" config={testConfig} onSelect={onSelect} />);

    const button = await screen.findByRole("button", { name: /details/i });
    button.click();

    expect(onSelect).toHaveBeenCalledWith("web");
  });

  it("renders custom row actions alongside the Details button", async () => {
    mockFetchOnce({ status: 200, body: { items: [{ name: "web", phase: "Running" }] } });
    render(
      <ResourceList
        namespace="tenant-a-dev"
        config={testConfig}
        onSelect={() => {}}
        renderRowActions={(item) => <button type="button">Logs for {item.name}</button>}
      />,
    );

    await waitFor(() => expect(screen.getByRole("button", { name: "Logs for web" })).toBeInTheDocument());
  });

  it("shows the forbidden state on a 403 response", async () => {
    mockFetchOnce({ status: 403 });
    render(<ResourceList namespace="tenant-a-dev" config={testConfig} />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/don't have access/i));
  });

  it("shows the unavailable state on a 500 response", async () => {
    mockFetchOnce({ status: 500 });
    render(<ResourceList namespace="tenant-a-dev" config={testConfig} />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/unavailable/i));
  });

  it("shows the empty state when there are no items", async () => {
    mockFetchOnce({ status: 200, body: { items: [] } });
    render(<ResourceList namespace="tenant-a-dev" config={testConfig} />);

    await waitFor(() => expect(screen.getByText(/no widgets found/i)).toBeInTheDocument());
  });
});
