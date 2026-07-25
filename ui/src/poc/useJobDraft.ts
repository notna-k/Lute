/**
 * Shared state for the PoCs: a job draft (the schema being authored) plus the
 * values for the next build (the schema being filled in).
 *
 * Keeping both in one hook is deliberate — it is what lets every layout show a
 * live preview of the form while you edit it, which is the thing Jenkins can't
 * do at all.
 *
 * Note on style: the mutators read `job` from the closure rather than using a
 * functional updater. They need to touch two pieces of state at once (schema
 * and values), and a `setState` updater must stay pure — StrictMode invokes it
 * twice, which would run a rename's value-migration a second time against
 * already-migrated state and drop the value.
 */
import { useCallback, useMemo, useState } from 'react';
import { initialValue, initialValues, typeDef, validateAll } from './params/registry';
import { envFromName, newId } from './params/yaml';
import type { JobDef, ParamSpec, ParamTypeId, ParamValue } from './params/types';

export function useJobDraft(initial: JobDef) {
  const [job, setJob] = useState<JobDef>(initial);
  const [values, setValues] = useState<Record<string, ParamValue>>(() =>
    initialValues(initial.params)
  );
  const [submitted, setSubmitted] = useState(false);

  const setValue = useCallback((name: string, value: ParamValue) => {
    setValues((prev) => ({ ...prev, [name]: value }));
  }, []);

  const updateParam = useCallback(
    (id: string, patch: Partial<ParamSpec>) => {
      const before = job.params.find((p) => p.id === id);
      if (!before) return;
      const after = { ...before, ...patch };
      setJob({ ...job, params: job.params.map((p) => (p.id === id ? after : p)) });

      const renamed = patch.name !== undefined && patch.name !== before.name;
      const retyped = patch.type !== undefined && patch.type !== before.type;
      if (!renamed && !retyped) return;

      setValues((vs) => {
        const next = { ...vs };
        // A rename carries the value across, so the form does not blank out
        // mid-edit and leave the author thinking they broke something. A type
        // change invalidates the old value's shape, so it resets instead.
        const carried = next[before.name];
        if (renamed) delete next[before.name];
        next[after.name] = retyped ? initialValue(after) : (carried ?? initialValue(after));
        return next;
      });
    },
    [job]
  );

  const addParam = useCallback(
    (type: ParamTypeId, at?: number): string => {
      const id = newId();
      // Names must be unique; a bare counter collides once anything is deleted.
      const taken = new Set(job.params.map((p) => p.name));
      let n = job.params.length + 1;
      while (taken.has(`param_${n}`)) n += 1;
      const name = `param_${n}`;

      const spec: ParamSpec = {
        id,
        type,
        name,
        label: `New ${typeDef(type).label.toLowerCase()}`,
        env: envFromName(name),
        ...typeDef(type).seed(),
      };
      const params = [...job.params];
      params.splice(at ?? params.length, 0, spec);
      setJob({ ...job, params });
      setValues((vs) => ({ ...vs, [spec.name]: initialValue(spec) }));
      return id;
    },
    [job]
  );

  const duplicateParam = useCallback(
    (id: string) => {
      const i = job.params.findIndex((p) => p.id === id);
      if (i < 0) return;
      const src = job.params[i];
      const taken = new Set(job.params.map((p) => p.name));
      let name = `${src.name}_copy`;
      let n = 2;
      while (taken.has(name)) name = `${src.name}_copy${n++}`;

      const copy: ParamSpec = { ...src, id: newId(), name, env: envFromName(name) };
      const params = [...job.params];
      params.splice(i + 1, 0, copy);
      setJob({ ...job, params });
      setValues((vs) => ({ ...vs, [copy.name]: initialValue(copy) }));
    },
    [job]
  );

  const removeParam = useCallback((id: string) => {
    setJob((prev) => ({ ...prev, params: prev.params.filter((p) => p.id !== id) }));
  }, []);

  const moveParam = useCallback((id: string, delta: number) => {
    setJob((prev) => {
      const i = prev.params.findIndex((p) => p.id === id);
      const j = i + delta;
      if (i < 0 || j < 0 || j >= prev.params.length) return prev;
      const params = [...prev.params];
      [params[i], params[j]] = [params[j], params[i]];
      return { ...prev, params };
    });
  }, []);

  /** Drag-and-drop reorder: lift the param out, drop it back at `to`. */
  const reorderParam = useCallback((from: number, to: number) => {
    setJob((prev) => {
      if (from === to || from < 0 || to < 0) return prev;
      if (from >= prev.params.length || to >= prev.params.length) return prev;
      const params = [...prev.params];
      const [lifted] = params.splice(from, 1);
      params.splice(to, 0, lifted);
      return { ...prev, params };
    });
  }, []);

  const resetValues = useCallback(() => {
    setValues(initialValues(job.params));
    setSubmitted(false);
  }, [job.params]);

  const errors = useMemo(() => validateAll(job.params, values), [job.params, values]);
  // Errors stay quiet until the first submit — a form that shouts before you
  // have touched it reads as broken.
  const shownErrors = submitted ? errors : {};
  const valid = Object.keys(errors).length === 0;

  return {
    job,
    setJob,
    values,
    setValue,
    errors: shownErrors,
    allErrors: errors,
    valid,
    submitted,
    setSubmitted,
    resetValues,
    addParam,
    updateParam,
    removeParam,
    moveParam,
    reorderParam,
    duplicateParam,
  };
}

export type JobDraft = ReturnType<typeof useJobDraft>;
