import { render } from "@testing-library/react";
import { axe } from "jest-axe";
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
      return Promise.resolve({ ok: true, json: async () => ({ namespaces: [{ name: "tenant-a-dev" }] }) });
    }),
  );
}

// Automated accessibility gate - see the tenantdeck-testing skill
// ("Accessibility: automated checks (axe-core)"). Zero serious/critical
// violations required on every required view; this is the first one.
describe("App accessibility", () => {
  it("has no axe violations in the logged-out state", async () => {
    mockSessionFetch({ authenticated: false });
    const { container, findByRole } = render(<App />);
    await findByRole("link", { name: /log in/i });

    const results = await axe(container);
    expect(results).toHaveNoViolations();
  });

  it("has no axe violations in the logged-in state", async () => {
    mockSessionFetch({ authenticated: true, subject: "alice", csrfToken: "token" });
    const { container, findByText } = render(<App />);
    await findByText(/signed in as alice/i);

    const results = await axe(container);
    expect(results).toHaveNoViolations();
  });
});
