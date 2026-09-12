import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import Workspace from "./Workspace";

function mockApiFetch() {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockImplementation((url: string) => {
      if (url.endsWith("/overview")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({ resourceQuotas: [], limitRanges: [] }),
        });
      }
      if (url.endsWith("/events")) {
        return Promise.resolve({ ok: true, status: 200, json: async () => ({ items: [] }) });
      }
      if (/\/deployments\/[^/]+$/.test(url)) {
        return Promise.resolve({ ok: true, status: 200, json: async () => ({ name: "web" }) });
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
      return Promise.resolve({ ok: true, status: 200, json: async () => ({ items: [] }) });
    }),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

// Workspace wires the navigation between the per-namespace views
// (docs/route-allowlist.md); the views' own loading/forbidden/empty states
// are covered by each view's own test file, so these tests focus on
// navigation itself.
describe("Workspace", () => {
  it("shows the overview by default", async () => {
    mockApiFetch();
    render(<Workspace namespace="tenant-a-dev" onBack={() => {}} />);

    await waitFor(() => expect(screen.getByText(/no resource quotas set/i)).toBeInTheDocument());
  });

  it("switches to the Events view when its nav button is clicked", async () => {
    mockApiFetch();
    render(<Workspace namespace="tenant-a-dev" onBack={() => {}} />);
    await waitFor(() => expect(screen.getByText(/no resource quotas set/i)).toBeInTheDocument());

    screen.getByRole("button", { name: "Events" }).click();

    await waitFor(() => expect(screen.getByText(/no events found/i)).toBeInTheDocument());
  });

  it("switches to a resource list view and on to its detail view", async () => {
    mockApiFetch();
    render(<Workspace namespace="tenant-a-dev" onBack={() => {}} />);
    await waitFor(() => expect(screen.getByText(/no resource quotas set/i)).toBeInTheDocument());

    screen.getByRole("button", { name: "Deployments" }).click();
    const detailsButton = await screen.findByRole("button", { name: /details/i });
    detailsButton.click();

    await waitFor(() => expect(screen.getByRole("heading", { name: "web" })).toBeInTheDocument());
  });

  it("calls onBack when the back-to-namespaces button is clicked", () => {
    mockApiFetch();
    const onBack = vi.fn();
    render(<Workspace namespace="tenant-a-dev" onBack={onBack} />);

    screen.getByRole("button", { name: /back to namespaces/i }).click();

    expect(onBack).toHaveBeenCalled();
  });
});
