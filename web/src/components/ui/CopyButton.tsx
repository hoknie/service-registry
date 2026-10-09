"use client";

import { Check, Copy } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { cn } from "@/lib/cn";

import { useUiText } from "../UiText";
import { Button } from "./Button";

export function CopyButton({
  text,
  label,
  className,
  variant = "secondary",
}: {
  text: string;
  label?: string;
  className?: string;
  variant?: "secondary" | "ghost" | "primary";
}) {
  const { common } = useUiText();
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 2000);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
    } catch {
      toast.error(common.copyFailed);
    }
  };

  return (
    <Button size="sm" variant={variant} className={cn(className)} onClick={() => void copy()}>
      {copied ? <Check aria-hidden="true" className="text-signal" /> : <Copy aria-hidden="true" />}
      <span aria-live="polite">{copied ? common.copied : (label ?? common.copy)}</span>
    </Button>
  );
}
