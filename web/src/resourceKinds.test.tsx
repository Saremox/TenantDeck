import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { IngressSummary } from "./api";
import { resourceKinds } from "./resourceKinds";

describe("resourceKinds ingress host column", () => {
  const ingressConfig = resourceKinds.find((k) => k.key === "ingresses");
  if (!ingressConfig) throw new Error("ingresses resource kind config not found");
  const hostsColumn = ingressConfig.columns[1];

  it("renders a well-formed hostname as a link", () => {
    const item: IngressSummary = { name: "ing", hosts: ["app.tenant-a.example.com"] };
    render(<>{hostsColumn.render(item)}</>);

    const link = screen.getByRole("link", { name: "app.tenant-a.example.com" });
    expect(link).toHaveAttribute("href", "https://app.tenant-a.example.com");
  });

  // Ingress host strings come from Kubernetes-sourced, effectively
  // attacker-controlled data, not from TenantDeck itself. A host that isn't
  // a plain DNS-ish string must render as inert text, never become a
  // clickable/javascript: link. See docs/spec/04 and Lens 6 of
  // tenantdeck-security-review.
  it("renders a hostile host value as plain text, never as a link", () => {
    const hostileHost = 'evil.example.com"><script>window.__pwned = true</script>';
    const item: IngressSummary = { name: "ing", hosts: [hostileHost] };
    render(<>{hostsColumn.render(item)}</>);

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText(hostileHost)).toBeInTheDocument();
    expect(document.querySelector("script")).not.toBeInTheDocument();
  });

  it("renders a javascript: scheme attempt as plain text, never as a link", () => {
    const item: IngressSummary = { name: "ing", hosts: ["javascript:alert(1)"] };
    render(<>{hostsColumn.render(item)}</>);

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText("javascript:alert(1)")).toBeInTheDocument();
  });
});
