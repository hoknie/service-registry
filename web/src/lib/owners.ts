import type { Contact, Owner, OwnerBadge, SubjectKind } from "./api";

export type ContactLink = { href: string; external: boolean };

const WEB = /^https?:\/\//i;

export function contactLink(c: Contact): ContactLink | null {
  switch (c.kind) {
    case "email":
      return { href: `mailto:${c.value}`, external: false };
    case "phone":
      return { href: `tel:${c.value.replace(/[\s-]/g, "")}`, external: false };
    default:
      return WEB.test(c.value) ? { href: c.value, external: true } : null;
  }
}

export function webLink(url: string | null): string | null {
  return url && /^https:\/\//i.test(url) ? url : null;
}

export function contactText(c: Contact): string {
  return c.label ?? c.value;
}

export type DraftOwner = { subject_kind: SubjectKind; subject: string; name: string; detail: string | null };

export function draftOf(o: Owner): DraftOwner {
  return o.subject_kind === "group"
    ? { subject_kind: "group", subject: o.name, name: o.name, detail: null }
    : { subject_kind: "user", subject: o.email ?? o.name, name: o.name, detail: o.email };
}

export function ownersBody(draft: DraftOwner[]): { owners: { subject_kind: SubjectKind; subject: string }[] } {
  return { owners: draft.map((d) => ({ subject_kind: d.subject_kind, subject: d.subject })) };
}

export function moveItem<T>(list: T[], at: number, by: -1 | 1): T[] {
  const to = at + by;
  if (at < 0 || at >= list.length || to < 0 || to >= list.length) return list;
  const out = [...list];
  [out[at], out[to]] = [out[to]!, out[at]!];
  return out;
}

export function sameSubject(a: DraftOwner, b: DraftOwner): boolean {
  return a.subject_kind === b.subject_kind && a.subject.toLowerCase() === b.subject.toLowerCase();
}

export function badgeNames(owners: OwnerBadge[], max = 2): { names: string[]; more: number } {
  return { names: owners.slice(0, max).map((o) => o.name), more: Math.max(0, owners.length - max) };
}

export function ownerFilter(kind: SubjectKind, id: string): string {
  return `${kind}:${id}`;
}
