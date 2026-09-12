import { useEffect, useRef, useState } from "react";
import { streamPodLogs } from "./api";

interface Props {
  namespace: string;
  podName: string;
  onClose: () => void;
}

type Status = "streaming" | "done" | "error";

// Bounded pod logs with follow mode and cancellation/reconnection
// (docs/spec/02-product-and-scope.md, docs/route-allowlist.md). Rendered
// inside <pre> as plain text only - log content is hostile input, same as
// events; see docs/spec/04 and Lens 6 of tenantdeck-security-review.
export default function PodLogsViewer({ namespace, podName, onClose }: Props) {
  const [follow, setFollow] = useState(false);
  const [lines, setLines] = useState("");
  const [status, setStatus] = useState<Status>("streaming");
  const controllerRef = useRef<AbortController | null>(null);

  // Starts (or continues) a stream on an already-created controller.
  // Every state update here happens inside a promise callback or the
  // onChunk callback, never synchronously - safe to call from useEffect.
  function runStream(withFollow: boolean, controller: AbortController) {
    streamPodLogs(
      namespace,
      podName,
      { tailLines: 200, follow: withFollow },
      (chunk) => {
        setLines((prev) => prev + chunk);
      },
      controller.signal,
    )
      .then(() => {
        if (!controller.signal.aborted) setStatus("done");
      })
      .catch(() => {
        if (!controller.signal.aborted) setStatus("error");
      });
  }

  useEffect(() => {
    const controller = new AbortController();
    controllerRef.current = controller;
    runStream(follow, controller);
    return () => controller.abort();
    // Mount-only: this effect starts the initial stream for this
    // pod/namespace. Toggling follow or reconnecting goes through
    // restart() below, triggered from event handlers (where a
    // synchronous setState reset is allowed), not by re-running this
    // effect from a `follow` dependency.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [namespace, podName]);

  // Resets visible state and starts a fresh stream. Only ever called
  // from event handlers (Reconnect click, Follow checkbox change) - the
  // synchronous setState calls here are fine there, just not in an effect.
  function restart(withFollow: boolean) {
    controllerRef.current?.abort();
    const controller = new AbortController();
    controllerRef.current = controller;
    setFollow(withFollow);
    setLines("");
    setStatus("streaming");
    runStream(withFollow, controller);
  }

  function cancel() {
    controllerRef.current?.abort();
    setStatus("done");
  }

  return (
    <section aria-label={`Logs for ${podName}`}>
      <button type="button" onClick={onClose}>
        ← Back to pods
      </button>
      <h3>Logs: {podName}</h3>
      <label>
        <input type="checkbox" checked={follow} onChange={(e) => restart(e.target.checked)} />
        Follow
      </label>
      <button type="button" onClick={() => restart(follow)}>
        Reconnect
      </button>
      <button type="button" onClick={cancel} disabled={status !== "streaming"}>
        Cancel
      </button>
      {status === "error" && <p role="alert">The log stream failed. Try reconnecting.</p>}
      <pre aria-label="Log output">{lines}</pre>
    </section>
  );
}
