"use client";

import { FileSearch, FileText, Search } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { format } from "@/i18n/format";
import { apiGet, errorCode, type SearchHit, type SearchMode, type SearchModes, type SearchPage } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { Segmented } from "../ui/Segmented";
import { SkeletonTable } from "../ui/Skeleton";
import { catalogHref } from "../catalog/shared";

type Props = { locale: Locale; labels: Messages["search"]; kinds: Messages["catalog"]["docs"]["kinds"] };

export function SearchView({ locale, labels: t, kinds }: Props) {
  const { errors, common } = useUiText();
  const router = useRouter();
  const params = useSearchParams();
  const q = params.get("q") ?? "";
  const branch = params.get("branch") ?? "";
  const project = params.get("project") ?? "";
  const mode = params.get("mode") ?? "";
  const [modes, setModes] = useState<SearchModes | null>(null);
  const [draft, setDraft] = useState(q);
  const [draftBranch, setDraftBranch] = useState(branch);
  const [hits, setHits] = useState<SearchHit[] | null>(null);
  const [next, setNext] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const fetchPage = useCallback(
    async (cursor: string | null) => {
      const query = new URLSearchParams({ q });
      if (branch) query.set("branch", branch);
      if (project) query.set("project", project);
      if (mode) query.set("mode", mode);
      if (cursor) query.set("cursor", cursor);
      return apiGet<SearchPage>(`/v1/knowledge/search?${query}`);
    },
    [q, branch, project, mode],
  );

  useEffect(() => {
    let live = true;
    apiGet<SearchModes>("/v1/knowledge/search/modes")
      .then((m) => live && setModes(m))
      .catch(() => live && setModes(null));
    return () => {
      live = false;
    };
  }, []);

  useEffect(() => {
    if (!q) return;
    let live = true;
    fetchPage(null)
      .then((page) => {
        if (!live) return;
        setHits(page.items);
        setNext(page.next_cursor);
        setError(null);
      })
      .catch((e: unknown) => live && setError(errorText(errors, errorCode(e))));
    return () => {
      live = false;
    };
  }, [q, fetchPage, errors]);

  const go = (nextMode: string) => {
    const query = new URLSearchParams();
    if (draft.trim()) query.set("q", draft.trim());
    if (draftBranch.trim()) query.set("branch", draftBranch.trim());
    if (project) query.set("project", project);
    if (nextMode) query.set("mode", nextMode);
    setHits(null);
    router.replace(`/${locale}/search?${query}`, { scroll: false });
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    go(mode);
  };

  const current: SearchMode | "" = modes ? (modes.modes.includes(mode as SearchMode) ? (mode as SearchMode) : modes.default) : "";

  const more = async () => {
    if (!next) return;
    setBusy(true);
    try {
      const page = await fetchPage(next);
      setHits((h) => [...(h ?? []), ...page.items]);
      setNext(page.next_cursor);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="grid gap-6">
      <form className="flex flex-wrap items-end gap-3" onSubmit={submit} role="search">
        <Field label={t.title} className="min-w-0 flex-[2_1_20rem]">
          {(p) => <Input {...p} type="search" value={draft} placeholder={t.placeholder} onChange={(e) => setDraft(e.target.value)} />}
        </Field>
        <Field label={t.branch} hint={t.branchHint} className="min-w-0 flex-[1_1_12rem]">
          {(p) => <Input {...p} value={draftBranch} onChange={(e) => setDraftBranch(e.target.value)} />}
        </Field>
        <Button type="submit" variant="primary">
          <Search aria-hidden="true" />
          {t.submit}
        </Button>
      </form>

      {modes && modes.modes.length > 1 && (
        <div className="flex flex-wrap items-center gap-3">
          <Segmented
            label={t.mode}
            value={current}
            options={modes.modes.map((m) => ({ value: m, label: t.modes[m] }))}
            onChange={(m) => go(m === modes.default ? "" : m)}
          />
          {modes.index && modes.index.pending > 0 && (
            <span className="text-sm text-muted" role="status">
              {format(t.indexing, { count: modes.index.pending })}
            </span>
          )}
        </div>
      )}

      {error && <Message note={{ kind: "error", text: error }} />}
      {!q && <EmptyState icon={FileSearch} title={t.start} />}
      {q && !hits && !error && <SkeletonTable rows={5} label={common.loading} />}
      {q && hits && hits.length === 0 && <EmptyState icon={FileSearch} title={t.empty} />}
      {hits && hits.length > 0 && (
        <ul className="grid gap-3">
          {hits.map((h) => (
            <li key={`${h.project.id}:${h.branch}:${h.path}`} className="rounded-lg border border-line bg-surface p-4">
              <Link
                href={catalogHref(locale, h.project.id, "docs", branch ? h.branch : null, h.path)}
                className="flex flex-wrap items-center gap-2 font-medium text-ink hover:text-signal"
              >
                <FileText aria-hidden="true" className="size-4 text-muted" />
                <span className="font-mono text-sm">{h.path}</span>
                <Badge tone="outline">{kinds[h.kind]}</Badge>
              </Link>
              <p className="mt-1 text-xs text-muted">
                {h.project.path} · {h.branch}
              </p>
              {h.snippet.length > 0 && (
                <p className="mt-2 text-sm leading-relaxed text-ink-2">
                  {h.snippet.map((s, i) =>
                    s.match ? (
                      <mark key={i} className="rounded bg-signal-soft px-0.5 text-signal">
                        {s.text}
                      </mark>
                    ) : (
                      <span key={i}>{s.text}</span>
                    ),
                  )}
                </p>
              )}
            </li>
          ))}
        </ul>
      )}
      {next && (
        <div>
          <Button onClick={() => void more()} disabled={busy}>
            {t.more}
          </Button>
        </div>
      )}
    </div>
  );
}
