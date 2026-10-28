import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import { SectionMenu } from "../ui/SectionMenu";

export type SettingsSection = { key: string; label: string; icon: LucideIcon };

type Props = { label: string; sections: SettingsSection[]; current: string; href: (key: string) => string; children: ReactNode };

export function SettingsSections({ label, sections, current, href, children }: Props) {
  const items = sections.map((s) => ({ key: s.key, href: href(s.key), label: s.label, icon: s.icon, current: s.key === current }));
  return (
    <SectionMenu label={label} groups={[{ key: "sections", items }]}>
      {children}
    </SectionMenu>
  );
}

export function pickSection<K extends string>(keys: K[], asked: string | null): K | null {
  return asked && (keys as string[]).includes(asked) ? (asked as K) : (keys[0] ?? null);
}
