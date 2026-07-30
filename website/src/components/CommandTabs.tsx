import React, { useState } from 'react';
import Terminal, { TermLine } from './Terminal';

export type CommandTab = { id: string; label: string; lines: TermLine[] };

export default function CommandTabs({ tabs }: { tabs: CommandTab[] }) {
  const [active, setActive] = useState<string | undefined>(tabs[0]?.id);
  const current = tabs.find((t) => t.id === active) ?? tabs[0];

  return (
    <div className="command-tabs">
      <div role="tablist" className="command-tabs-list">
        {tabs.map((t) => (
          <button
            key={t.id}
            role="tab"
            type="button"
            aria-selected={t.id === active}
            className={`command-tab ${t.id === active ? 'command-tab-active' : ''}`.trim()}
            onClick={() => setActive(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>
      {current ? <Terminal lines={current.lines} title={current.label} /> : null}
    </div>
  );
}
