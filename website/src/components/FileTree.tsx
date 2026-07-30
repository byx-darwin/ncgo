import React, { useEffect, useState } from 'react';

export type TreeRow = { text: string; depth: number; ai?: boolean };

function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

export default function FileTree({ rows, className = '' }: { rows: TreeRow[]; className?: string }) {
  const [mounted, setMounted] = useState(false);
  const [visible, setVisible] = useState(rows.length);

  useEffect(() => setMounted(true), []);
  const animate = mounted && !prefersReducedMotion();

  useEffect(() => {
    if (!animate) return;
    setVisible(0);
    const timers = rows.map((_, i) =>
      window.setTimeout(() => setVisible(i + 1), 130 * (i + 1)),
    );
    return () => timers.forEach((t) => window.clearTimeout(t));
  }, [animate, rows]);

  return (
    <div className={`filetree ${className}`.trim()}>
      {rows.slice(0, visible).map((r, i) => (
        <div
          key={`r-${i}`}
          className={`filetree-row ${r.ai ? 'filetree-ai' : ''}`.trim()}
          style={{ paddingLeft: `${r.depth * 18 + 14}px` }}
        >
          <span className="filetree-text">{r.text}</span>
          {r.ai ? <span className="filetree-badge">AI</span> : null}
        </div>
      ))}
    </div>
  );
}
