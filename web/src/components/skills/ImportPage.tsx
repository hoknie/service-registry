"use client";

import { GitPullRequestArrow } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import { parseTreeLink } from "@/lib/hostingAddress";
import { can } from "@/lib/rights";
import { placeHref, type SkillFolder } from "@/lib/skills";
import { importSource, loadFolder } from "@/lib/skillsApi";

import { useOrganizations } from "../places/spaces";
import { useSession } from "../SessionProvider";
import { Crumbs, type Crumb } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { PageHeader, Panel } from "../ui/Panel";
import { Select } from "../ui/Select";
import { bodyOf, emptyDraft, SourceFields, type Draft } from "./SourceForm";

type Props = { locale: Locale; labels: Messages["skills"] };

function repoName(d: Draft): string {
  const raw = d.kind === "remote" ? (parseTreeLink(d.url)?.url ?? d.url) : d.path;
  return raw.replace(/\/+$/, "").split("/").pop()?.replace(/\.git$/, "") ?? "";
}

export function ImportPage({ locale, labels: t }: Props) {
  const i = t.importPage;
  const params = useSearchParams();
  const router = useRouter();
  const { errors } = useUiText();
  const { session } = useSession();
  const user = session.status === "signed-in" ? session.user : null;
  const folderId = params.get("folder");
  const spaceParam = params.get("space");
  const orgs = useOrganizations();
  const canGlobal = can(user, "skills", "create");
  const [folder, setFolder] = useState<SkillFolder | null>(null);
  const [scope, setScope] = useState(spaceParam === "global" ? "" : (spaceParam ?? ""));
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const effectiveScope = scope || (canGlobal || spaceParam === "global" ? "" : (orgs[0]?.id ?? ""));

  useEffect(() => {
    if (folderId)
      loadFolder(folderId)
        .then(setFolder)
        .catch(() => setFolder(null));
  }, [folderId]);

  const organization = folderId ? null : effectiveScope || null;
  const secretsOn = folder ? folder.organization_id : organization;
  const back = placeHref(locale, folderId ? { folder: folderId, space: null } : { folder: null, space: spaceParam ?? (effectiveScope || "global") });

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const out = await importSource({
        ...bodyOf(draft),
        parent_id: folderId,
        organization_id: organization,
        name: name.trim(),
        slug: slug.trim(),
      });
      router.push(`${placeHref(locale, { folder: out.folder.id, space: null })}&tab=settings&section=sources`);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
      setBusy(false);
    }
  };

  const crumbs: Crumb[] = [{ label: t.title, href: `/${locale}/skills` }];
  if (folder) {
    crumbs.push({ label: folder.organization?.name ?? t.scopeGlobal, href: placeHref(locale, { folder: null, space: folder.organization_id ?? "global" }) });
    for (const a of folder.ancestors) crumbs.push({ label: a.name, href: placeHref(locale, { folder: a.id, space: null }) });
    crumbs.push({ label: folder.name, href: back });
  } else if (spaceParam) {
    const org = orgs.find((o) => o.id === spaceParam);
    crumbs.push({ label: spaceParam === "global" ? t.scopeGlobal : (org?.name ?? "…"), href: back });
  }
  crumbs.push({ label: i.title });

  return (
    <>
      <Crumbs items={crumbs} />
      <PageHeader glyph={GitPullRequestArrow} title={i.title} lead={i.lead} />
      <Panel>
        <div className="grid gap-4 sm:grid-cols-2">
          {!folderId && !spaceParam && (
            <Field label={i.space} className="sm:col-span-2">
              {(f) => (
                <Select id={f.id} value={effectiveScope} onChange={(e) => setScope(e.target.value)}>
                  {canGlobal && <option value="">{t.scopeGlobal}</option>}
                  {orgs.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          )}
          <Field label={i.name} hint={format(i.nameHint, { name: repoName(draft) || "—" })}>
            {(f) => <Input {...f} value={name} onChange={(e) => setName(e.target.value)} placeholder={repoName(draft)} />}
          </Field>
          <Field label={t.folder.slug} hint={t.folder.slugHint}>
            {(f) => <Input {...f} className="font-mono" value={slug} onChange={(e) => setSlug(e.target.value)} />}
          </Field>
          <SourceFields draft={draft} setDraft={setDraft} folder={{ organization_id: secretsOn }} locale={locale} labels={t} />
          <Message note={error ? { kind: "error", text: error } : null} className="sm:col-span-2" />
          <div className="flex flex-wrap gap-2 sm:col-span-2">
            <Button variant="primary" busy={busy} onClick={submit}>
              {i.submit}
            </Button>
            <Button asChild>
              <Link href={back}>{i.cancel}</Link>
            </Button>
          </div>
        </div>
      </Panel>
    </>
  );
}
