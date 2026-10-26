"use client";

import { Maximize2, Minimize2 } from "lucide-react";
import { createContext, useContext, type ReactNode } from "react";

import { Button } from "../ui/Button";
import { Tooltip } from "../ui/Tooltip";

type Wide = { wide: boolean; setWide: (wide: boolean) => void; wideLabel: string; normalLabel: string };

const Context = createContext<Wide | null>(null);

export function DocWideProvider({ value, children }: { value: Wide; children: ReactNode }) {
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function WideToggle() {
  const ctx = useContext(Context);
  if (!ctx) return null;
  const label = ctx.wide ? ctx.normalLabel : ctx.wideLabel;
  return (
    <Tooltip content={label}>
      <Button variant="secondary" size="icon" className="hidden lg:inline-flex" aria-pressed={ctx.wide} aria-label={label} onClick={() => ctx.setWide(!ctx.wide)}>
        {ctx.wide ? <Minimize2 aria-hidden="true" /> : <Maximize2 aria-hidden="true" />}
      </Button>
    </Tooltip>
  );
}
