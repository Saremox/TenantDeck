import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import NamespaceList from "./NamespaceList";

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

describe("NamespaceList", () => {
  it("shows a loading state before the fetch resolves", () => {
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise(() => {})));
    render(<NamespaceList />);
    expect(screen.getByRole("status")).toHaveTextContent(/loading/i);
  });

  it("renders each namespace name once the fetch resolves", async () => {
    mockFetchOnce({ status: 200, body: { namespaces: [{ name: "tenant-a-dev" }, { name: "tenant-a-prod" }] } });
    render(<NamespaceList />);

    await waitFor(() => expect(screen.getByText("tenant-a-dev")).toBeInTheDocument());
    expect(screen.getByText("tenant-a-prod")).toBeInTheDocument();
  });

  it("shows the empty state distinctly from the loading state when there are no namespaces", async () => {
    mockFetchOnce({ status: 200, body: { namespaces: [] } });
    render(<NamespaceList />);

    await waitFor(() => expect(screen.getByText(/no namespaces found/i)).toBeInTheDocument());
  });

  // This is the API-error-mapping requirement from the testing skill: a
  // 403 from the BFF must render the forbidden state, not a generic error.
  it("shows the forbidden state on a 403 response", async () => {
    mockFetchOnce({ status: 403 });
    render(<NamespaceList />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/don't have access/i));
  });

  it("shows the unavailable-upstream state on a 502 response", async () => {
    mockFetchOnce({ status: 502 });
    render(<NamespaceList />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/unavailable/i));
  });

  // Hostile-content rendering (docs/spec/04, Lens 6 of tenantdeck-security-review):
  // a namespace name containing markup must render as literal text, never
  // as executed HTML.
  it("renders a hostile namespace name as literal text, not as markup", async () => {
    const hostileName = '<img src=x onerror="window.__pwned=true">';
    mockFetchOnce({ status: 200, body: { namespaces: [{ name: hostileName }] } });
    render(<NamespaceList />);

    await waitFor(() => expect(screen.getByText(hostileName)).toBeInTheDocument());
    expect(document.querySelector("img")).toBeNull();
    expect((window as unknown as { __pwned?: boolean }).__pwned).toBeUndefined();
  });
});
