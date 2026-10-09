import type { Locale } from "@/i18n/config";
import type { LinkKind, LinkStatus } from "@/lib/api";

import type { Tone } from "../ui/Badge";

export function kindName(kinds: LinkKind[], key: string, locale: Locale): string {
  return kinds.find((k) => k.key === key)?.names[locale] ?? key;
}

export function statusTone(status: LinkStatus): Tone {
  switch (status) {
    case "ok":
      return "signal";
    case "auth_required":
    case "redirect":
      return "amber";
    case "blocked":
    case "unexpected":
      return "outline";
    default:
      return "danger";
  }
}

export function linkQuery(branch: string | null, environment: string): string {
  const q = new URLSearchParams();
  if (branch) q.set("branch", branch);
  if (environment) q.set("environment", environment);
  const s = q.toString();
  return s ? `?${s}` : "";
}
