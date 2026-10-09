"use client";

import { createContext, useContext, type ReactNode } from "react";

import type { Messages } from "@/i18n/messages";

export type UiText = { common: Messages["common"]; errors: Messages["errors"] };

const Context = createContext<UiText | null>(null);

export function UiTextProvider({ value, children }: { value: UiText; children: ReactNode }) {
  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useUiText(): UiText {
  const value = useContext(Context);
  if (!value) throw new Error("useUiText outside <UiTextProvider>");
  return value;
}
