import { useEffect, useRef, useState } from 'react';
import { connectSSE } from '../api/client';
import type { ActiveRunSnapshot } from '../types/api';

export function useSSE({
  enabled = true,
  onRunCompleted,
  onJobUpdated,
  onRunStarted,
  onError,
}: {
  enabled?: boolean;
  onRunCompleted?: () => void;
  onJobUpdated?: () => void;
  onRunStarted?: () => void;
  onError?: () => void;
}) {
  const [activeRuns, setActiveRuns] = useState<ActiveRunSnapshot[]>([]);
  const [connected, setConnected] = useState(false);

  const callbacksRef = useRef({ onRunCompleted, onJobUpdated, onRunStarted, onError });
  callbacksRef.current = { onRunCompleted, onJobUpdated, onRunStarted, onError };

  useEffect(() => {
    if (!enabled) {
      setActiveRuns([]);
      setConnected(false);
      return;
    }

    return connectSSE({
      onStatusChange: setConnected,
      onError: () => callbacksRef.current.onError?.(),
      onSnapshot: (runs) => setActiveRuns(runs),
      onRunStarted: (run) => {
        setActiveRuns((prev) => [...prev.filter((r) => r.run_id !== run.run_id), run]);
        callbacksRef.current.onRunStarted?.();
      },
      onProgress: (run) => {
        setActiveRuns((prev) => {
          const exists = prev.some((r) => r.run_id === run.run_id);
          return exists ? prev.map((r) => (r.run_id === run.run_id ? run : r)) : [...prev, run];
        });
      },
      onRunCompleted: (run) => {
        setActiveRuns((prev) => prev.filter((r) => r.run_id !== run.id));
        callbacksRef.current.onRunCompleted?.();
      },
      onJobUpdated: () => {
        callbacksRef.current.onJobUpdated?.();
      },
    });
  }, [enabled]);

  return { activeRuns, connected };
}
