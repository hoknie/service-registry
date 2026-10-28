"use client";

import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { KeepPrevious } from "../ui/KeepPrevious";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../ui/Tabs";
import { PlaceBody } from "./PlaceBody";

export type PlaceTab = { key: string; label: string; icon: LucideIcon; content: ReactNode; accent?: boolean };

const ACCENT =
  "bg-signal-soft text-signal hover:bg-signal-soft hover:text-signal data-[state=active]:bg-primary data-[state=active]:text-primary-ink data-[state=active]:[&_svg]:text-primary-ink";

type Props = { id: string; pending: boolean; hero: ReactNode; tabs: PlaceTab[]; tab: string; onTab: (tab: string) => void; tabsLabel: string };

export function PlaceShell({ id, pending, hero, tabs, tab, onTab, tabsLabel }: Props) {
  const current = tabs.find((x) => x.key === tab) ?? tabs[0];
  return (
    <>
      <PlaceBody pending={pending}>{hero}</PlaceBody>
      <KeepPrevious id={id} pending={pending}>
        {tabs.length > 1 ? (
          <Tabs value={current?.key} onValueChange={onTab} activationMode="manual">
            <TabsList aria-label={tabsLabel}>
              {tabs.map((x) => (
                <TabsTrigger key={x.key} value={x.key} className={x.accent ? ACCENT : undefined}>
                  <x.icon aria-hidden="true" />
                  {x.label}
                </TabsTrigger>
              ))}
            </TabsList>
            {current && (
              <TabsContent value={current.key} className="mt-6">
                {current.content}
              </TabsContent>
            )}
          </Tabs>
        ) : (
          current?.content
        )}
      </KeepPrevious>
    </>
  );
}
