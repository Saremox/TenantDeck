import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import EventsList from "./EventsList";

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

describe("EventsList", () => {
  it("shows a loading state before the fetch resolves", () => {
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise(() => {})));
    render(<EventsList namespace="tenant-a-dev" />);
    expect(screen.getByRole("status")).toHaveTextContent(/loading events/i);
  });

  it("renders a row per event", async () => {
    mockFetchOnce({
      status: 200,
      body: {
        items: [
          {
            type: "Warning",
            reason: "BackOff",
            involvedObject: "pod/web-1",
            message: "container failed to start",
            count: 3,
            lastTimestamp: "2026-01-01T00:00:00Z",
          },
        ],
      },
    });
    render(<EventsList namespace="tenant-a-dev" />);

    await waitFor(() => expect(screen.getByText("BackOff")).toBeInTheDocument());
    expect(screen.getByText("container failed to start")).toBeInTheDocument();
    expect(screen.getByText("pod/web-1")).toBeInTheDocument();
  });

  // Event message/reason text comes from whatever produced the event, not
  // TenantDeck or Kubernetes itself - treated as hostile input and rendered
  // as plain text only. See docs/spec/04 and Lens 6 of
  // tenantdeck-security-review.
  it("renders a hostile event message as plain text instead of executing it", async () => {
    const hostileMessage = '<img src=x onerror="window.__pwned = true">';
    mockFetchOnce({
      status: 200,
      body: {
        items: [
          {
            type: "Warning",
            reason: "Unhealthy",
            involvedObject: "pod/web-1",
            message: hostileMessage,
            count: 1,
            lastTimestamp: "2026-01-01T00:00:00Z",
          },
        ],
      },
    });
    render(<EventsList namespace="tenant-a-dev" />);

    await waitFor(() => expect(screen.getByText(hostileMessage)).toBeInTheDocument());
    expect(document.querySelector("img")).not.toBeInTheDocument();
    expect((window as { __pwned?: boolean }).__pwned).toBeUndefined();
  });

  it("shows the forbidden state on a 403 response", async () => {
    mockFetchOnce({ status: 403 });
    render(<EventsList namespace="tenant-a-dev" />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/don't have access/i));
  });

  it("shows the unavailable state on a 500 response", async () => {
    mockFetchOnce({ status: 500 });
    render(<EventsList namespace="tenant-a-dev" />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/unavailable/i));
  });

  it("shows the empty state when there are no events", async () => {
    mockFetchOnce({ status: 200, body: { items: [] } });
    render(<EventsList namespace="tenant-a-dev" />);

    await waitFor(() => expect(screen.getByText(/no events found/i)).toBeInTheDocument());
  });
});
