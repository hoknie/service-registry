import type { RefKind } from "./refs";

export type PublicationVersion = { pattern: string; kind: RefKind };

export type PublicationOrigin = { id: string; kind: string; name: string } | null;

export type Publication = {
  publish: boolean;
  versions: PublicationVersion[];
  default_branch: boolean;
  current_release: boolean;
  name: string | null;
  host: string | null;
  own: Partial<Pick<Publication, "publish" | "versions" | "default_branch" | "current_release" | "name" | "host">>;
  from: Record<"publish" | "versions" | "default_branch" | "current_release", PublicationOrigin>;
  entries: { kind: "path" | "host"; url: string }[];
  can_edit: boolean;
};

export type PublicationDraft = {
  publish: { set: boolean; value: boolean };
  versions: { set: boolean; rows: PublicationVersion[] };
  default_branch: { set: boolean; value: boolean };
  current_release: { set: boolean; value: boolean };
  name: string;
  host: string;
};

export function draftOf(p: Publication): PublicationDraft {
  return {
    publish: { set: "publish" in p.own, value: p.own.publish ?? p.publish },
    versions: { set: "versions" in p.own, rows: (p.own.versions ?? []).map((v) => ({ ...v })) },
    default_branch: { set: "default_branch" in p.own, value: p.own.default_branch ?? p.default_branch },
    current_release: { set: "current_release" in p.own, value: p.own.current_release ?? p.current_release },
    name: p.own.name ?? "",
    host: p.own.host ?? "",
  };
}

export function publicationBody(d: PublicationDraft, withName: boolean): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  if (d.publish.set) body.publish = d.publish.value;
  if (d.versions.set) body.versions = d.versions.rows.map((r) => ({ pattern: r.pattern.trim(), kind: r.kind })).filter((r) => r.pattern !== "");
  if (d.default_branch.set) body.default_branch = d.default_branch.value;
  if (d.current_release.set) body.current_release = d.current_release.value;
  if (withName && d.name.trim()) body.name = d.name.trim().toLowerCase();
  if (d.host.trim()) body.host = d.host.trim();
  return body;
}

export function unreachable(p: Publication): boolean {
  return p.publish && p.entries.length === 0;
}
