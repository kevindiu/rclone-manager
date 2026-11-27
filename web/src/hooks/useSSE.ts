import { useEffect, useRef, useState } from 'react';
import { connectSSE } from '../api/client';
import type { ActiveRunSnapshot } from '../types/api';

export function useSSE({
  onRunCompleted,
  onJobUpdated,
  onRunStarted,
}: {
  onRunCompleted?: () => void;
  onJobUpdated?: () => void;
  onRunStarted?: () => void;
}) {
  const [activeRuns, setActiveRuns] = useState<ActiveRunSnapshot[]>([]);
  const [connected, setConnected] = useState(false);

  const callbacksRef = useRef({ onRunCompleted, onJobUpdated, onRunStarted });
  callbacksRef.current = { onRunCompleted, onJobUpdated, onRunStarted };

  useEffect(() => {
    return connectSSE({
      onStatusChange: setConnected,
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
  }, []);

  return { activeRuns, connected };
}
