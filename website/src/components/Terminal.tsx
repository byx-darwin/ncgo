import React, { useEffect, useRef, useState } from 'react';

export type TermLine = {
  kind: 'command' | 'output' | 'success' | 'highlight' | 'blank';
  text: string;
};

type Props = {
  lines: TermLine[];
  title?: string;
  className?: string;
};

const KIND_CLASS: Record<TermLine['kind'], string> = {
  command: 'term-command',
  output: 'term-output',
  success: 'term-success',
  highlight: 'term-highlight',
  blank: 'term-blank',
};

function prefersReducedMotion(): boolean {
  return (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );
}

export default function Terminal({ lines, title = 'terminal', className = '' }: Props) {
  const [mounted, setMounted] = useState(false);
  const [visible, setVisible] = useState(lines.length);
  const [partial, setPartial] = useState('');
  const timers = useRef<number[]>([]);

  useEffect(() => setMounted(true), []);
  const animate = mounted && !prefersReducedMotion();

  useEffect(() => {
    if (!animate) return;
    const clearAll = () => {
      timers.current.forEach((t) => window.clearTimeout(t));
      timers.current = [];
    };
    clearAll();
    setVisible(0);
    setPartial('');

    let line = 0;
    const schedule = (fn: () => void, delay: number) => {
      timers.current.push(window.setTimeout(fn, delay));
    };

    const process = () => {
      if (line >= lines.length) return;
      const cur = lines[line];
      if (cur.kind === 'command') {
        let char = 0;
        const typeChar = () => {
          char += 2;
          if (char >= cur.text.length) {
            setPartial('');
            setVisible(line + 1);
            line += 1;
            schedule(process, 160);
          } else {
            setPartial(cur.text.slice(0, char));
            schedule(typeChar, 26);
          }
        };
        schedule(typeChar, 60);
      } else {
        setVisible(line + 1);
        line += 1;
        schedule(process, cur.kind === 'blank' ? 40 : 100);
      }
    };
    schedule(process, 350);
    return clearAll;
  }, [animate, lines]);

  const rows: React.ReactNode[] = [];
  for (let i = 0; i < Math.min(visible, lines.length); i++) {
    const l = lines[i];
    rows.push(
      <div key={`l-${i}`} className={`term-line ${KIND_CLASS[l.kind]}`}>
        {l.kind === 'command' ? <span className="term-prompt">$ </span> : null}
        {l.text}
      </div>,
    );
  }
  if (partial) {
    rows.push(
      <div key="partial" className="term-line term-command">
        <span className="term-prompt">$ </span>
        {partial}
        <span className="term-caret" aria-hidden="true" />
      </div>,
    );
  }

  return (
    <div className={`terminal ${className}`.trim()}>
      <div className="terminal-bar">
        <span className="terminal-dot terminal-dot-r" />
        <span className="terminal-dot terminal-dot-y" />
        <span className="terminal-dot terminal-dot-g" />
        <span className="terminal-title">{title}</span>
      </div>
      <div className="terminal-body" role="log" aria-live="polite">
        {rows}
      </div>
    </div>
  );
}
