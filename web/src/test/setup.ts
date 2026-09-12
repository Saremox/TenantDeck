import "@testing-library/jest-dom/vitest";
import { cleanup } from "@testing-library/react";
import { toHaveNoViolations } from "jest-axe";
import { afterEach, expect } from "vitest";

// jest-axe ships a plain Jest-style matcher object; vitest's expect.extend
// accepts the same shape, so this works without a vitest-specific axe
// wrapper (which isn't past pre-release yet - see docs/implementation-plan.md
// ADR notes on preferring mature maintained libraries).
expect.extend(toHaveNoViolations);

// Testing Library's automatic per-test cleanup only self-registers when it
// detects vitest's *global* afterEach (test.globals: true). We don't set
// that, so register it explicitly - otherwise each test's render() output
// accumulates in the jsdom document across the whole file.
afterEach(() => {
  cleanup();
});
