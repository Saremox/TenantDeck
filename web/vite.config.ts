import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Go BFF serves these assets itself (go:embed) - base stays "/" since
// there's exactly one origin, per docs/spec/03-architecture.md.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "dist",
    // false, not the default true: emptying dist/ would delete the
    // committed dist/.gitkeep placeholder go:embed relies on for a
    // pre-build `go build` to work (see web/embed.go). Stale hashed
    // assets from older builds can accumulate locally over many builds -
    // harmless, since every CI/production build starts from a clean
    // checkout; run `make clean` locally if it ever matters.
    emptyOutDir: false,
  },
});
