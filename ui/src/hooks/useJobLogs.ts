/**
 * Tail-following log pagination for one engine job (a build's run).
 *
 * The API serves a window from the end of the log file with a cursor for the
 * lines before it, so the viewer starts at the tail — where a running build is
 * interesting — and pages backwards as the user scrolls up. The scroll anchor is
 * restored after a prepend, otherwise loading older lines would yank the reader
 * away from the line they were reading.
 */
import { useCallback, useEffect, useRef, useState } from 'react';
import { jobService } from '@/services/jobService';

const PAGE = 200;

export interface UseJobLogsOptions {
  /** Poll while the run is still producing output. */
  live?: boolean;
  intervalMs?: number;
}

export function useJobLogs(
  jobId: string | undefined,
  { live = false, intervalMs = 3000 }: UseJobLogsOptions = {},
) {
  const boxRef = useRef<HTMLDivElement>(null);
  const loadingOlderRef = useRef(false);
  const [lines, setLines] = useState<string[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    async (olderCursor?: string) => {
      if (!jobId) return;
      const prepend = Boolean(olderCursor);
      if (prepend) {
        if (loadingOlderRef.current) return;
        loadingOlderRef.current = true;
        setLoadingOlder(true);
      } else {
        setLoading(true);
      }
      setError(null);
      try {
        const res = await jobService.getJobLogs(jobId, {
          direction: 'tail',
          limit: PAGE,
          cursor: olderCursor,
        });
        const el = boxRef.current;
        const prevHeight = prepend && el ? el.scrollHeight : 0;
        const prevTop = prepend && el ? el.scrollTop : 0;
        const chunk = Array.isArray(res.lines) ? res.lines : [];

        setLines((prev) => (prepend ? [...chunk, ...prev] : chunk));
        setCursor(res.next_cursor ?? null);
        setHasMore(res.has_more);
        if (res.error) setError(res.error);

        requestAnimationFrame(() => {
          const box = boxRef.current;
          if (!box) return;
          box.scrollTop = prepend ? box.scrollHeight - prevHeight + prevTop : box.scrollHeight;
        });
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load logs');
        if (!prepend) {
          setLines([]);
          setHasMore(false);
          setCursor(null);
        }
      } finally {
        setLoading(false);
        setLoadingOlder(false);
        loadingOlderRef.current = false;
      }
    },
    [jobId],
  );

  // A new run starts from an empty pane rather than the previous build's tail.
  useEffect(() => {
    setLines([]);
    setCursor(null);
    setHasMore(false);
    setError(null);
    if (jobId) void load();
  }, [jobId, load]);

  useEffect(() => {
    if (!live || !jobId) return;
    const timer = window.setInterval(() => void load(), intervalMs);
    return () => window.clearInterval(timer);
  }, [live, jobId, intervalMs, load]);

  /** Wire to the log pane's onScroll: pages backwards near the top. */
  const onScroll = useCallback(() => {
    const el = boxRef.current;
    if (!el || loading || loadingOlderRef.current || !hasMore || !cursor) return;
    if (el.scrollTop < 64) void load(cursor);
  }, [cursor, hasMore, load, loading]);

  return {
    boxRef,
    lines,
    loading,
    loadingOlder,
    hasMore,
    error,
    onScroll,
    reload: () => void load(),
  };
}
