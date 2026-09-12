import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi, type Mock } from "vitest";
import { streamPodLogs } from "./api";
import PodLogsViewer from "./PodLogsViewer";

vi.mock("./api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("./api")>();
  return { ...actual, streamPodLogs: vi.fn() };
});

const mockStreamPodLogs = streamPodLogs as unknown as Mock;

afterEach(() => {
  vi.clearAllMocks();
});

function pendingPromise(): Promise<void> {
  return new Promise(() => {});
}

describe("PodLogsViewer", () => {
  it("renders log chunks as they arrive", async () => {
    mockStreamPodLogs.mockImplementation((_ns: string, _pod: string, _opts: unknown, onChunk: (c: string) => void) => {
      onChunk("line one\n");
      onChunk("line two\n");
      return pendingPromise();
    });

    render(<PodLogsViewer namespace="tenant-a-dev" podName="web-1" onClose={() => {}} />);

    await waitFor(() => expect(screen.getByLabelText(/log output/i)).toHaveTextContent("line one"));
    expect(screen.getByLabelText(/log output/i)).toHaveTextContent("line two");
  });

  // Pod log content is hostile input, same as Kubernetes events - it must
  // render inside <pre> as plain text only, never be interpreted as
  // markup. See docs/spec/04 and Lens 6 of tenantdeck-security-review.
  it("renders hostile log content as plain text instead of executing it", async () => {
    const hostileLine = '<img src=x onerror="window.__pwned = true">';
    mockStreamPodLogs.mockImplementation((_ns: string, _pod: string, _opts: unknown, onChunk: (c: string) => void) => {
      onChunk(hostileLine + "\n");
      return pendingPromise();
    });

    render(<PodLogsViewer namespace="tenant-a-dev" podName="web-1" onClose={() => {}} />);

    await waitFor(() => expect(screen.getByLabelText(/log output/i)).toHaveTextContent(hostileLine));
    expect(document.querySelector("img")).not.toBeInTheDocument();
    expect((window as typeof window & { __pwned?: boolean }).__pwned).toBeUndefined();
  });

  it("shows an error message when the stream fails", async () => {
    mockStreamPodLogs.mockRejectedValue(new Error("boom"));
    render(<PodLogsViewer namespace="tenant-a-dev" podName="web-1" onClose={() => {}} />);

    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent(/log stream failed/i));
  });

  it("aborts the in-flight stream's signal when Cancel is clicked", async () => {
    let capturedSignal: AbortSignal | undefined;
    mockStreamPodLogs.mockImplementation(
      (_ns: string, _pod: string, _opts: unknown, _onChunk: unknown, signal: AbortSignal) => {
        capturedSignal = signal;
        return pendingPromise();
      },
    );

    render(<PodLogsViewer namespace="tenant-a-dev" podName="web-1" onClose={() => {}} />);
    await waitFor(() => expect(mockStreamPodLogs).toHaveBeenCalledTimes(1));

    screen.getByRole("button", { name: /cancel/i }).click();

    expect(capturedSignal?.aborted).toBe(true);
  });

  it("starts a fresh stream when Reconnect is clicked", async () => {
    mockStreamPodLogs.mockImplementation(() => pendingPromise());
    render(<PodLogsViewer namespace="tenant-a-dev" podName="web-1" onClose={() => {}} />);
    await waitFor(() => expect(mockStreamPodLogs).toHaveBeenCalledTimes(1));

    screen.getByRole("button", { name: /reconnect/i }).click();

    await waitFor(() => expect(mockStreamPodLogs).toHaveBeenCalledTimes(2));
  });

  it("restarts the stream with follow enabled when the Follow checkbox is checked", async () => {
    mockStreamPodLogs.mockImplementation(() => pendingPromise());
    render(<PodLogsViewer namespace="tenant-a-dev" podName="web-1" onClose={() => {}} />);
    await waitFor(() => expect(mockStreamPodLogs).toHaveBeenCalledTimes(1));

    screen.getByRole("checkbox", { name: /follow/i }).click();

    await waitFor(() => expect(mockStreamPodLogs).toHaveBeenCalledTimes(2));
    const secondCallOpts = mockStreamPodLogs.mock.calls[1][2] as { follow?: boolean };
    expect(secondCallOpts.follow).toBe(true);
  });

  it("calls onClose when the back button is clicked", async () => {
    mockStreamPodLogs.mockImplementation(() => pendingPromise());
    const onClose = vi.fn();
    render(<PodLogsViewer namespace="tenant-a-dev" podName="web-1" onClose={onClose} />);

    screen.getByRole("button", { name: /back to pods/i }).click();

    expect(onClose).toHaveBeenCalled();
  });
});
