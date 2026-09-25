// The schema being authored plus the values for the next build, together so the editor can preview live.
// Mutators read `fields` from the closure: they update schema and values at once, and a setState
// updater runs twice under StrictMode, which would migrate a renamed value twice.
import { useCallback, useEffect, useMemo, useState } from 'react';
import { initialValue, initialValues, typeDef, validateAll } from './registry';
import { envFromName, newDraftId } from './yaml';
import type { ParameterField, ParameterType, ParameterValue } from '@/types/jobs';

/** A parameter plus a client-side identity that survives renames. */
export type DraftField = ParameterField & { id: string };

function withIds(fields: ParameterField[]): DraftField[] {
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
  const [values, setValues] = useState<Record<string, ParameterValue>>(() => initialValues(source));
  const [submitted, setSubmitted] = useState(false);

  // Reseed on content, not identity, so a background refetch does not wipe what was typed.
  const sourceKey = useMemo(() => JSON.stringify(source), [source]);
  useEffect(() => {
    setFields(withIds(source));
    setValues(initialValues(source));
    setSubmitted(false);
    // sourceKey stands in for source.
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
        // A rename carries the value across; a type change resets it.
        const carried = next[before.name];
        if (renamed) delete next[before.name];
        next[after.name] = retyped ? initialValue(after) : (carried ?? initialValue(after));
        return next;
      });
    },
    [fields],
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
    [fields],
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
    [fields],
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
  // Errors stay quiet until the first submit.
  const valid = Object.keys(localErrors).length === 0;

  /** True once the draft schema differs from what Git gave us. */
  const dirty = useMemo(
    () => JSON.stringify(stripIds(fields)) !== JSON.stringify(source),
    [fields, source],
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
