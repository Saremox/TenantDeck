import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import NamespaceSelector from "./NamespaceSelector";

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

describe("NamespaceSelector", () => {
  it("shows a loading state before the fetch resolves", () => {
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise(() => {})));
    render(<NamespaceSelector onSelect={() => {}} />);
    expect(screen.getByRole("status")).toHaveTextContent(/loading/i);
  });

  it("renders a button per namespace once loaded", async () => {
    mockFetchOnce({ status: 200, body: { namespaces: [{ name: "tenant-a-dev" }] } });
    render(<NamespaceSelector onSelect={() => {}} />);

    await waitFor(() => expect(screen.getByRole("button", { name: "tenant-a-dev" })).toBeInTheDocument());
  });

  it("calls onSelect with the namespace name when clicked", async () => {
    mockFetchOnce({ status: 200, body: { namespaces: [{ name: "tenant-a-dev" }] } });
    const onSelect = vi.fn();
    render(<NamespaceSelector onSelect={onSelect} />);

    const button = await screen.findByRole("button", { name: "tenant-a-dev" });
    button.click();

    expect(onSelect).toHaveBeenCalledWith("tenant-a-dev");
  });

  it("shows the forbidden state on a 403 response", async () => {
    mockFetchOnce({ status: 403 });
    render(<NamespaceSelector onSelect={() => {}} />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/don't have access/i));
  });

  it("shows the empty state when there are no namespaces", async () => {
    mockFetchOnce({ status: 200, body: { namespaces: [] } });
    render(<NamespaceSelector onSelect={() => {}} />);

    await waitFor(() => expect(screen.getByText(/no namespaces found/i)).toBeInTheDocument());
  });
});
