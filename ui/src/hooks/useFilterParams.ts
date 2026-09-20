/**
 * Filter state that lives in the URL.
 *
 * A filtered list is a finding — "the release jobs that are failing" — and a
 * finding is worth sending to someone. Keeping the filters in the query string
 * makes the address bar the share link, survives a reload, and gives Back its
 * usual meaning instead of dropping the operator into an unfiltered page.
 *
 * Writes replace rather than push: typing into a search box should not bury the
 * previous page under thirty history entries.
 */
import { useCallback, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';

const EMPTY: string[] = [];

/** A single-valued filter — a search box, a scope, a sort order. */
export function useFilterParam<T extends string = string>(
  key: string,
  fallback: T
): [T, (value: T) => void] {
  const [params, setParams] = useSearchParams();
  const value = (params.get(key) as T | null) ?? fallback;

  const set = useCallback(
    (next: T) => {
      setParams(
        (prev) => {
          const out = new URLSearchParams(prev);
          if (!next || next === fallback) out.delete(key);
          else out.set(key, next);
          return out;
        },
        { replace: true }
      );
    },
    [fallback, key, setParams]
  );

  return [value, set];
}

/**
 * A multi-valued facet, carried as one repeated key (`?queue=a&queue=b`) so the
 * values stay readable and a value containing a comma still round-trips.
 */
export function useFilterList(key: string): [string[], (values: string[]) => void] {
  const [params, setParams] = useSearchParams();

  // getAll() hands back a fresh array every render, which would re-run every
  // memo downstream. Hold the last one while its contents are unchanged.
  const cache = useRef<{ key: string; values: string[] }>({
    key: '',
    values: EMPTY,
  });
  const raw = params.getAll(key);
  const identity = raw.join('\u0000');
  if (cache.current.key !== identity) {
    cache.current = { key: identity, values: raw.length ? raw : EMPTY };
  }

  const set = useCallback(
    (next: string[]) => {
      setParams(
        (prev) => {
          const out = new URLSearchParams(prev);
          out.delete(key);
          next.filter(Boolean).forEach((v) => out.append(key, v));
          return out;
        },
        { replace: true }
      );
    },
    [key, setParams]
  );

  return [cache.current.values, set];
}
