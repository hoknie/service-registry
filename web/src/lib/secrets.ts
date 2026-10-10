import type { Messages } from "@/i18n/messages";

import type { ComboOption } from "./combobox";
import type { Credentials, Secret } from "./api";

export const NO_SECRET = "-";

export function secretsPath(nodeId: string | null): `/${string}` {
  return nodeId ? `/v1/catalog/nodes/${nodeId}/secrets` : "/v1/secrets";
}

export function secretFrom(s: Pick<Secret, "from" | "own">, t: Messages["secrets"]): string {
  if (s.own) return t.here;
  if (!s.from) return t.global;
  return t.inherited.replace("{name}", s.from.name);
}

export function secretOptions(items: Secret[], t: Messages["secrets"], none: boolean): ComboOption[] {
  const out: ComboOption[] = none ? [{ value: NO_SECRET, label: t.none }] : [];
  for (const s of items) {
    const detail = [secretFrom(s, t), s.fingerprint ?? s.ref ?? ""].filter(Boolean).join(" · ");
    out.push({ value: s.id, label: s.name, detail, keywords: [s.description] });
  }
  return out;
}

export type SecretDraft = { name: string; description: string; mode: "value" | "ref"; value: string; ref: string };

export function secretBody(d: SecretDraft, editing: boolean): Record<string, string> {
  const body: Record<string, string> = { name: d.name.trim(), description: d.description.trim() };
  if (d.mode === "ref") body.ref = d.ref.trim();
  else if (d.value || !editing) body.value = d.value;
  return body;
}

export function credentialsText(c: Credentials, t: Messages["secrets"]): string {
  if (c.kind === "secret") return c.secret.from ? `${c.secret.name} · ${c.secret.from.name}` : `${c.secret.name} · ${t.global}`;
  if (c.kind === "legacy") return t.legacyShort;
  return t.none;
}
