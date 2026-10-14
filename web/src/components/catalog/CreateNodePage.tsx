"use client";

import { FolderTree } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { apiGet, apiSend, errorCode, type CatalogNode, type NodeKind, type SourceCheck } from "@/lib/api";
import { catalogChanged } from "@/lib/catalogEvents";
import { labelsOf } from "@/lib/tags";

import { useSession } from "../SessionProvider";
import { Crumbs } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { PageHeader, Panel } from "../ui/Panel";
import { SkeletonRows } from "../ui/Skeleton";
import { hasInvalidTags, LabelsInput } from "../ui/TagInput";
import { emptyDraft, sourceBody, type SourceDraft } from "../knowledge/SourceFields";
import { knownBranches } from "./BranchInput";
import { KeyDialog } from "./KeyDialog";
import { KindIcon } from "./KindIcon";
import { DescriptionField, labelError, noRepo, ObservationField, RepoSection, slugify, type Repo, type RepoMode } from "./NodeFields";
import { catalogHref, childKinds, type CatalogLabels } from "./shared";

const KINDS: NodeKind[] = ["organization", "folder", "project"];

function checkable(mode: RepoMode, d: SourceDraft): boolean {
  if (mode === "link") return false;
  if (mode === "remote") return !!d.forge && d.url.trim().length > 0;
  return d.path.trim().length > 0;
}

export function CreateNodePage({ locale, labels }: { locale: Locale; labels: CatalogLabels }) {
  const params = useSearchParams();
  const parentId = params.get("parent");
  const kindParam = params.get("kind");
  const kind = KINDS.find((k) => k === kindParam) ?? null;
  const router = useRouter();
  const { session } = useSession();
  const { errors, common } = useUiText();
  const t = labels.create;
  const fail = (e: unknown) => errorText(errors, errorCode(e));

  const [parent, setParent] = useState<CatalogNode | null | undefined>(parentId ? undefined : null);
  const [loadError, setLoadError] = useState(false);
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [slugError, setSlugError] = useState<string | null>(null);
  const [description, setDescription] = useState("");
  const [labelLines, setLabelLines] = useState("");
  const [labelDraft, setLabelDraft] = useState("");
  const [repo, setRepo] = useState<Repo>(noRepo);
  const [branchTouched, setBranchTouched] = useState(false);
  const [mode, setMode] = useState<RepoMode>("link");
  const [draft, setDraft] = useState<SourceDraft>(emptyDraft);
  const [observe, setObserve] = useState(true);
  const [check, setCheck] = useState<SourceCheck | null>(null);
  const [checkNote, setCheckNote] = useState<Note>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState<CatalogNode | null>(null);

  useEffect(() => {
    if (!parentId) return;
    let live = true;
    apiGet<CatalogNode>(`/v1/catalog/nodes/${parentId}`)
      .then((n) => live && setParent(n))
      .catch(() => {
        if (!live) return;
        setParent(null);
        setLoadError(true);
      });
    return () => {
      live = false;
    };
  }, [parentId]);

  const body = mode !== "link" && kind === "project" ? JSON.stringify(sourceBody(draft)) : null;
  const ready = kind === "project" && parentId !== null && checkable(mode, draft);
  useEffect(() => {
    if (!ready || !body || !parentId) return;
    const abort = new AbortController();
    const timer = setTimeout(() => {
      apiSend<SourceCheck>("POST", `/v1/catalog/nodes/${parentId}/knowledge/source/check`, JSON.parse(body) as Record<string, unknown>)
        .then((res) => {
          if (abort.signal.aborted) return;
          setCheck(res);
          setCheckNote(res.ok ? null : { kind: "error", text: errorText(errors, res.error_code ?? "unknown") });
          if (res.ok && res.default_branch) {
            const def = res.default_branch;
            setRepo((r) => (branchTouched || r.default_branch ? r : { ...r, default_branch: def }));
          }
        })
        .catch((e) => {
          if (abort.signal.aborted) return;
          setCheck(null);
          setCheckNote({ kind: "error", text: errorText(errors, errorCode(e)) });
        });
    }, 600);
    return () => {
      abort.abort();
      clearTimeout(timer);
    };
  }, [ready, body, parentId, errors, branchTouched]);

  const branchSuggest = useMemo(
    () => (check?.ok && check.branch_names?.length ? knownBranches(check.branch_names, check.default_branch ?? null, labels.branches.badges.default) : undefined),
    [check, labels.branches.badges.default],
  );

  const superadmin = session.status === "signed-in" && session.user.is_superadmin;
  const rootCrumb = { label: labels.root, href: catalogHref(locale, null), node: "root" };
  const backHref = catalogHref(locale, parentId);

  if (parent === undefined || session.status === "loading") return <SkeletonRows rows={6} label={common.loading} />;

  const allowed =
    !!kind &&
    !loadError &&
    childKinds(parent ? parent.kind : null).includes(kind) &&
    (parent ? parent.access === "read" && (parent.permissions ?? []).includes("catalog.write") : superadmin);

  const crumbs = [
    rootCrumb,
    ...(parent?.path ?? []).map((p) => ({ label: p.name, href: catalogHref(locale, p.id), node: p.id })),
    ...(parent ? [{ label: parent.name, href: catalogHref(locale, parent.id), node: parent.id }] : []),
    { label: kind ? t.newTitle[kind] : t.title },
  ];

  if (!allowed || !kind) {
    return (
      <>
        <Crumbs items={crumbs} />
        <EmptyState
          icon={FolderTree}
          title={t.notAllowed}
          action={
            <Button asChild>
              <Link href={backHref}>{t.back}</Link>
            </Button>
          }
          className="mt-4"
        />
      </>
    );
  }

  const open = (node: CatalogNode) => router.push(catalogHref(locale, node.id));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setNote(null);
    setSlugError(null);
    try {
      const node = await apiSend<CatalogNode>("POST", "/v1/catalog/nodes", {
        kind,
        parent_id: parent?.id ?? null,
        slug,
        name,
        description,
        labels: labelsOf(labelLines, labelDraft),
        ...(kind === "project"
          ? {
              ...(mode === "link" ? repo : { default_branch: repo.default_branch, source: sourceBody(draft) }),
              cluster_observation: observe,
            }
          : {}),
      });
      toast.success(t.done);
      catalogChanged();
      if (node.key?.secret) setCreated(node);
      else open(node);
    } catch (e) {
      const code = errorCode(e);
      if (code === "conflict.slug_taken" || code === "validation.invalid_slug") setSlugError(errorText(errors, code));
      else setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  const invalidLabels = hasInvalidTags(labelLines, labelError);

  return (
    <>
      <Crumbs items={crumbs} />
      <PageHeader title={t.newTitle[kind]} lead={parent ? parent.name : undefined} icon={<KindIcon kind={kind} tile className="size-10" />} />
      <Panel>
        <form className="grid gap-4" onSubmit={submit}>
          <div className="grid gap-4 md:grid-cols-2">
            <Field label={t.name}>
              {(p) => (
                <Input
                  {...p}
                  required
                  autoFocus
                  value={name}
                  onChange={(e) => {
                    setName(e.target.value);
                    if (!slugTouched) setSlug(slugify(e.target.value));
                  }}
                />
              )}
            </Field>
            <Field label={t.slug} hint={t.slugHint} error={slugError ?? undefined}>
              {(p) => (
                <Input
                  {...p}
                  required
                  className="font-mono"
                  value={slug}
                  onChange={(e) => {
                    setSlugTouched(true);
                    setSlug(e.target.value);
                  }}
                />
              )}
            </Field>
          </div>
          <DescriptionField labels={labels} value={description} onChange={setDescription} rows={4} />
          <Field label={t.labels} hint={t.labelsHint}>
            {(p) => <LabelsInput {...p} value={labelLines} onChange={setLabelLines} onDraft={setLabelDraft} />}
          </Field>
          {kind === "project" && (
            <RepoSection
              labels={labels}
              mode={mode}
              onMode={(m) => {
                setMode(m);
                setCheck(null);
                setCheckNote(null);
              }}
              repo={repo}
              onRepo={(r) => {
                if (r.default_branch !== repo.default_branch) setBranchTouched(true);
                setRepo(r);
              }}
              draft={draft}
              onDraft={setDraft}
              scope={{ listOn: parent?.id ?? null, parent: parent?.id ?? null, canCreate: !!parent?.permissions?.includes("catalog.access") }}
              branchSuggest={branchSuggest}
              branchNote={mode !== "link" && <Message note={checkNote} />}
            />
          )}
          {kind === "project" && <ObservationField labels={labels} value={observe} onChange={setObserve} />}
          <Message note={note} />
          <div className="flex flex-wrap items-center justify-end gap-3">
            {invalidLabels && <span className="mr-auto text-sm text-danger">{common.fixLabels}</span>}
            <Button asChild>
              <Link href={backHref}>{common.cancel}</Link>
            </Button>
            <Button type="submit" variant="primary" busy={busy} disabled={invalidLabels}>
              {t.submit}
            </Button>
          </div>
        </form>
      </Panel>
      {created?.key?.secret && <KeyDialog secret={created.key.secret} labels={labels.keys.dialog} onClose={() => open(created)} />}
    </>
  );
}
