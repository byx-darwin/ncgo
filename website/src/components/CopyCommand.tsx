import React, { useEffect, useRef, useState } from 'react';

export default function CopyCommand({ command, label = 'copy' }: { command: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  const resetTimer = useRef<number | undefined>(undefined);

  useEffect(() => () => {
    if (resetTimer.current !== undefined) {
      window.clearTimeout(resetTimer.current);
    }
  }, []);

  const onCopy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      if (resetTimer.current !== undefined) {
        window.clearTimeout(resetTimer.current);
      }
      resetTimer.current = window.setTimeout(() => {
        setCopied(false);
        resetTimer.current = undefined;
      }, 1600);
    } catch {
      setCopied(false);
    }
  };

  return (
    <div className="copy-command">
      <code className="copy-command-text">{command}</code>
      <button type="button" className="copy-command-btn" onClick={onCopy} aria-label={label}>
        {copied ? '✓' : '⧉'}
      </button>
    </div>
  );
}
