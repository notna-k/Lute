/**
 * The schema being authored, plus the values for the next build.
 *
 * Both live in one hook on purpose: it is what lets the editor show a live
 * preview of the form while you shape it.
 *
 * Style note: the mutators read `fields` from the closure rather than using a
 * functional updater. They touch two pieces of state at once (schema and
 * values), and a `setState` updater must stay pure — StrictMode invokes it
 * twice, which would run a rename's value migration against already-migrated
 * state and drop the value.
 */
import { useCallback, useEffect, useMemo, useState } from 'react';
import { initialValue, initialValues, typeDef, validateAll } from './registry';
import { envFromName, newDraftId } from './yaml';
import type { ParameterField, ParameterType, ParameterValue } from '@/types/jobs';

/** A parameter plus a client-side identity that survives renames. */
export type DraftField = ParameterField & { id: string };

export function withIds(fields: ParameterField[]): DraftField[] {
  return fields.map((f) => ({ ...f, id: newDraftId() }));
}

/** Strips the client-side id before anything leaves the panel. */
export function stripIds(fields: DraftField[]): ParameterField[] {
  return fields.map((field) => {
    const rest = { ...field } as Partial<DraftField>;
    delete rest.id;
    return rest as ParameterField;
  });
}

export function useSchemaDraft(source: ParameterField[]) {
  const [fields, setFields] = useState<DraftField[]>(() => withIds(source));
  const [values, setValues] = useState<Record<string, ParameterValue>>(() =>
    initialValues(source)
  );
  const [submitted, setSubmitted] = useState(false);

  // Reseed when the definition itself changes (a refetch after a Git sync).
  // Keyed on content, not array identity: an equal-but-new array from a
  // background refetch must not wipe what the user has typed.
  const sourceKey = useMemo(() => JSON.stringify(source), [source]);
  useEffect(() => {
    setFields(withIds(source));
    setValues(initialValues(source));
    setSubmitted(false);
    // `source` is covered by sourceKey; depending on it directly would defeat
    // the content check above.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sourceKey]);

  const setValue = useCallback((name: string, value: ParameterValue) => {
    setValues((prev) => ({ ...prev, [name]: value }));
  }, []);

  const setAllValues = useCallback((next: Record<string, ParameterValue>) => {
    setValues(next);
    setSubmitted(false);
  }, []);

  const resetValues = useCallback(() => {
    setValues(initialValues(fields));
    setSubmitted(false);
  }, [fields]);

  /** Throws away schema edits and goes back to what Git gave us. */
  const resetFields = useCallback(() => {
    setFields(withIds(source));
    setValues(initialValues(source));
    setSubmitted(false);
  }, [source]);

  const updateField = useCallback(
    (id: string, patch: Partial<ParameterField>) => {
      const before = fields.find((f) => f.id === id);
      if (!before) return;
      const after = { ...before, ...patch };
      setFields(fields.map((f) => (f.id === id ? after : f)));

      const renamed = patch.name !== undefined && patch.name !== before.name;
      const retyped = patch.type !== undefined && patch.type !== before.type;
      if (!renamed && !retyped) return;

      setValues((vs) => {
        const next = { ...vs };
        // A rename carries the value across so the form does not blank out
        // mid-edit; a type change invalidates the value's shape, so it resets.
        const carried = next[before.name];
        if (renamed) delete next[before.name];
        next[after.name] = retyped ? initialValue(after) : (carried ?? initialValue(after));
        return next;
      });
    },
    [fields]
  );

  const addField = useCallback(
    (type: ParameterType = 'string'): string => {
      const id = newDraftId();
      const taken = new Set(fields.map((f) => f.name));
      let n = fields.length + 1;
      while (taken.has(`param_${n}`)) n += 1;
      const name = `param_${n}`;

      const field: DraftField = {
        id,
        type,
        name,
        label: `New ${typeDef(type).label.toLowerCase()}`,
        envVar: envFromName(name),
        ...typeDef(type).seed(),
      };
      setFields([...fields, field]);
      setValues((vs) => ({ ...vs, [field.name]: initialValue(field) }));
      return id;
    },
    [fields]
  );

  const duplicateField = useCallback(
    (id: string) => {
      const i = fields.findIndex((f) => f.id === id);
      if (i < 0) return;
      const src = fields[i];
      const taken = new Set(fields.map((f) => f.name));
      let name = `${src.name}_copy`;
      let n = 2;
      while (taken.has(name)) name = `${src.name}_copy${n++}`;

      const copy: DraftField = { ...src, id: newDraftId(), name, envVar: envFromName(name) };
      const next = [...fields];
      next.splice(i + 1, 0, copy);
      setFields(next);
      setValues((vs) => ({ ...vs, [copy.name]: initialValue(copy) }));
    },
    [fields]
  );

  const removeField = useCallback((id: string) => {
    setFields((prev) => prev.filter((f) => f.id !== id));
  }, []);

  const moveField = useCallback((id: string, delta: number) => {
    setFields((prev) => {
      const i = prev.findIndex((f) => f.id === id);
      const j = i + delta;
      if (i < 0 || j < 0 || j >= prev.length) return prev;
      const next = [...prev];
      [next[i], next[j]] = [next[j], next[i]];
      return next;
    });
  }, []);

  /** Drag-and-drop reorder: lift the field out, drop it back at `to`. */
  const reorderField = useCallback((from: number, to: number) => {
    setFields((prev) => {
      if (from === to || from < 0 || to < 0 || from >= prev.length || to >= prev.length) {
        return prev;
      }
      const next = [...prev];
      const [lifted] = next.splice(from, 1);
      next.splice(to, 0, lifted);
      return next;
    });
  }, []);

  const localErrors = useMemo(() => validateAll(fields, values), [fields, values]);
  // Errors stay quiet until the first submit — a form that shouts before you
  // have touched it reads as broken.
  const valid = Object.keys(localErrors).length === 0;

  /** True once the draft schema differs from what Git gave us. */
  const dirty = useMemo(
    () => JSON.stringify(stripIds(fields)) !== JSON.stringify(source),
    [fields, source]
  );

  return {
    fields,
    values,
    setValue,
    setAllValues,
    resetValues,
    resetFields,
    localErrors,
    valid,
    submitted,
    setSubmitted,
    dirty,
    addField,
    updateField,
    duplicateField,
    removeField,
    moveField,
    reorderField,
  };
}

export type SchemaDraft = ReturnType<typeof useSchemaDraft>;
