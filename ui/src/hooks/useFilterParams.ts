// Filter state in the URL, so a filtered list can be shared and survives reloads.
// Writes replace rather than push, so typing in a search box does not flood history.
import { useCallback, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';

const EMPTY: string[] = [];

/** A single-valued filter — a search box, a scope, a sort order. */
export function useFilterParam<T extends string = string>(
  key: string,
  fallback: T,
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
        { replace: true },
      );
    },
    [fallback, key, setParams],
  );

  return [value, set];
}

/** A multi-valued facet as one repeated key (`?queue=a&queue=b`), so commas round-trip. */
export function useFilterList(key: string): [string[], (values: string[]) => void] {
  const [params, setParams] = useSearchParams();

  // getAll() returns a fresh array every call; tie it to params so memos downstream stay put.
  const values = useMemo(() => {
    const raw = params.getAll(key);
    return raw.length ? raw : EMPTY;
  }, [params, key]);

  const set = useCallback(
    (next: string[]) => {
      setParams(
        (prev) => {
          const out = new URLSearchParams(prev);
          out.delete(key);
          next.filter(Boolean).forEach((v) => out.append(key, v));
          return out;
        },
        { replace: true },
      );
    },
    [key, setParams],
  );

  return [values, set];
}
