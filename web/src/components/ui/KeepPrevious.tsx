"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";

import { cn } from "@/lib/cn";

const WAIT = 2000;

type Shown = { id: string; el: ReactNode };

function Slot({ mode, onReady, children }: { mode: "shown" | "dim" | "next"; onReady?: () => void; children: ReactNode }) {
  const ref = useRef<HTMLDivElement>(null);
  const next = mode === "next";

  useEffect(() => {
    const el = ref.current;
    if (!next || !el || !onReady) return;
    let done = false;
    let frame = 0;
    const finish = () => {
      if (done) return;
      done = true;
      onReady();
    };
    const check = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        if (!el.querySelector("[data-skeleton]")) finish();
      });
    };
    const observer = new MutationObserver(check);
    observer.observe(el, { childList: true, subtree: true });
    const timer = setTimeout(finish, WAIT);
    check();
    return () => {
      done = true;
      observer.disconnect();
      clearTimeout(timer);
      cancelAnimationFrame(frame);
    };
  }, [next, onReady]);

  if (next) {
    return (
      <div ref={ref} aria-hidden="true" inert className="invisible relative h-0 overflow-hidden">
        {children}
      </div>
    );
  }
  const dim = mode === "dim";
  return (
    <div ref={ref} aria-busy={dim || undefined} inert={dim} className={cn("motion-control", dim && "opacity-60 delay-150")}>
      {children}
    </div>
  );
}

export function KeepPrevious({ id, pending = false, children }: { id: string; pending?: boolean; children: ReactNode }) {
  const [shown, setShown] = useState<Shown>({ id, el: children });
  const onReady = useCallback(() => setShown({ id, el: null }), [id]);

  if (shown.id === id && !pending && shown.el !== children) setShown({ id, el: children });

  const slots =
    shown.id === id
      ? [
          <Slot key={id} mode={pending ? "dim" : "shown"}>
            {pending ? shown.el : children}
          </Slot>,
        ]
      : [
          <Slot key={shown.id} mode="dim">
            {shown.el}
          </Slot>,
          <Slot key={id} mode="next" onReady={onReady}>
            {children}
          </Slot>,
        ];
  return <>{slots}</>;
}
