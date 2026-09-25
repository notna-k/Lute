import { useState } from 'react';

async function writeClipboard(text: string) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    // The Clipboard API needs a secure context; a panel served over plain HTTP falls back.
    const ta = document.createElement('textarea');
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand('copy');
    } finally {
      document.body.removeChild(ta);
    }
  }
}

/** Copies text and reports `copied` for a moment afterwards. */
export function useCopy(): [boolean, (text: string) => Promise<void>] {
  const [copied, setCopied] = useState(false);
  async function copy(text: string) {
    await writeClipboard(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }
  return [copied, copy];
}
