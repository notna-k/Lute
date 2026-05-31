import { useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { Button, IconButton, Input } from '@/components/ui';

const LABEL_KEY_RE = /^[a-zA-Z0-9_\-.]{1,63}$/;

interface LabelRow {
  key: string;
  value: string;
}

interface LabelEditorProps {
  initialLabels?: Record<string, string>;
  onSave: (labels: Record<string, string>) => void;
  saving?: boolean;
}

function labelsToRows(labels: Record<string, string> = {}): LabelRow[] {
  return Object.entries(labels).map(([key, value]) => ({ key, value }));
}

function rowsToLabels(rows: LabelRow[]): Record<string, string> {
  const out: Record<string, string> = {};
  rows.forEach(({ key, value }) => {
    if (key.trim()) out[key.trim()] = value;
  });
  return out;
}

function validateRow(row: LabelRow): string | null {
  if (!row.key.trim()) return null; // empty key = will be skipped
  if (!LABEL_KEY_RE.test(row.key.trim()))
    return `Key "${row.key}" is invalid — use letters, numbers, _ - . only (1–63 chars)`;
  if (row.value.length > 255)
    return `Value for "${row.key}" exceeds 255 characters`;
  return null;
}

export function LabelEditor({ initialLabels, onSave, saving }: LabelEditorProps) {
  const [rows, setRows] = useState<LabelRow[]>(labelsToRows(initialLabels));
  const [validationError, setValidationError] = useState<string | null>(null);

  const addRow = () => setRows((r) => [...r, { key: '', value: '' }]);
  const removeRow = (i: number) => setRows((r) => r.filter((_, idx) => idx !== i));
  const updateRow = (i: number, field: 'key' | 'value', value: string) =>
    setRows((r) => r.map((row, idx) => (idx === i ? { ...row, [field]: value } : row)));

  const handleSave = () => {
    if (rows.length > 32) {
      setValidationError('Too many labels — max 32 allowed');
      return;
    }
    for (const row of rows) {
      const err = validateRow(row);
      if (err) {
        setValidationError(err);
        return;
      }
    }
    setValidationError(null);
    onSave(rowsToLabels(rows));
  };

  return (
    <div className='flex flex-col gap-3'>
      {rows.length > 0 ? (
        <div className='flex flex-col gap-2'>
          {rows.map((row, i) => (
            <div key={i} className='flex items-center gap-2'>
              <Input
                placeholder='key'
                value={row.key}
                onChange={(e) => updateRow(i, 'key', e.target.value)}
                className='flex-1 font-mono text-sm'
              />
              <span className='text-fg-muted'>=</span>
              <Input
                placeholder='value'
                value={row.value}
                onChange={(e) => updateRow(i, 'value', e.target.value)}
                className='flex-1 font-mono text-sm'
              />
              <IconButton
                label='Remove label'
                variant='ghost'
                onClick={() => removeRow(i)}
                className='text-danger hover:bg-danger/10'
              >
                <Trash2 className='h-4 w-4' />
              </IconButton>
            </div>
          ))}
        </div>
      ) : (
        <p className='text-sm text-fg-muted'>No labels — add one below.</p>
      )}

      {validationError && (
        <p className='text-sm text-danger'>{validationError}</p>
      )}

      <div className='flex items-center gap-2'>
        <Button
          type='button'
          variant='ghost'
          size='sm'
          leftIcon={<Plus className='h-4 w-4' />}
          onClick={addRow}
        >
          Add label
        </Button>
        <Button
          type='button'
          size='sm'
          onClick={handleSave}
          loading={saving}
          disabled={saving}
        >
          Save labels
        </Button>
      </div>
    </div>
  );
}

export default LabelEditor;
