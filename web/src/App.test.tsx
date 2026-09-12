import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import App from "./App";

afterEach(() => {
  vi.unstubAllGlobals();
});

function mockSessionFetch(status: { authenticated: boolean; subject?: string; csrfToken?: string }) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockImplementation((url: string) => {
      if (url === "/auth/session") {
        return Promise.resolve({ ok: true, json: async () => status });
      }
      return Promise.resolve({ ok: true, json: async () => ({ namespaces: [] }) });
    }),
  );
}

describe("App", () => {
  // Session-expired / logged-out handling: no session means a login link,
  // not a broken or blank page.
  it("shows a login link when there is no session", async () => {
    mockSessionFetch({ authenticated: false });
    render(<App />);

    await waitFor(() => expect(screen.getByRole("link", { name: /log in/i })).toBeInTheDocument());
    expect(screen.getByRole("link", { name: /log in/i })).toHaveAttribute("href", "/auth/login");
  });

  it("shows the signed-in subject and a logout control when authenticated", async () => {
    mockSessionFetch({ authenticated: true, subject: "alice", csrfToken: "csrf-token" });
    render(<App />);

    await waitFor(() => expect(screen.getByText(/signed in as alice/i)).toBeInTheDocument());
    expect(screen.getByRole("button", { name: /log out/i })).toBeInTheDocument();
  });
});
