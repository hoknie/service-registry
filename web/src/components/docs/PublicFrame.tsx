"use client";

import { BookOpen, Search } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import { Toaster } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiGet, errorCode, type SearchSegment } from "@/lib/api";
import { DOC_NAVIGATE, docBase, docHref, navigateDoc, publicRoot, setDocBase, splitPath } from "@/lib/docPath";
import { ensureSpace, searchUrl, setDocSource, spaceEntry, useDocStore } from "@/lib/docSpaceStore";

import { LanguageSwitcher } from "../LanguageSwitcher";
import { CrumbsProvider, useCrumbTrail } from "../shell/crumbs";
import { ThemeSwitcher, useTheme } from "../shell/ThemeSwitcher";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { Select } from "../ui/Select";
import { SkeletonPanel } from "../ui/Skeleton";
import { DocLink } from "./DocLink";
import { DocSpace, type DocLabels } from "./DocSpace";

type Text = Messages["publicDocs"];

type Props = { locale: Locale; labels: DocLabels; text: Text; theme: Messages["header"]["theme"]; language: string; skip: string };

type Hit = { project: { path: string; name: string }; branch: string; ref_kind: "branch" | "tag"; path: string; title: string | null; snippet: SearchSegment[] };

function queryOf(): string {
  return new URLSearchParams(window.location.search).get("q") ?? "";
}

export function PublicFrame({ locale, labels, text: t, theme, language, skip }: Props) {
  const [ready, setReady] = useState<{ root: string | null; hosted: boolean } | null>(null);
  const [q, setQ] = useState("");
  const [draft, setDraft] = useState("");

  useEffect(() => {
    const { root, base } = publicRoot(window.location.pathname);
    const hosted = !window.location.pathname.startsWith("/public/docs");
    setDocBase(base);
    setDocSource({ kind: "public", root });
    const sync = () => {
      setQ(queryOf());
      setDraft(queryOf());
    };
    sync();
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setReady({ root, hosted });
    window.addEventListener("popstate", sync);
    window.addEventListener(DOC_NAVIGATE, sync);
    return () => {
      window.removeEventListener("popstate", sync);
      window.removeEventListener(DOC_NAVIGATE, sync);
    };
  }, []);

  const search = (e: FormEvent) => {
    e.preventDefault();
    const value = draft.trim();
    navigateDoc(value ? `${docHref([])}?q=${encodeURIComponent(value)}` : docHref([]));
  };

  return (
    <CrumbsProvider>
      <a
        href="#main"
        className="sr-only z-[60] rounded-md bg-primary px-3 py-2 text-primary-ink focus:not-sr-only focus:fixed focus:top-3 focus:left-3"
      >
        {skip}
      </a>
      <div className="flex min-h-dvh flex-col">
        <header className="sticky top-0 z-30 border-b border-line/80 bg-canvas/80 backdrop-blur-xl">
          <div className="mx-auto flex w-full max-w-[84rem] [.doc-wide_&]:max-w-none flex-wrap items-center gap-3 px-4 py-2.5 sm:px-8">
            <EntryTitle fallback={t.title} />
            <form onSubmit={search} className="ml-auto flex min-w-0 items-center gap-2" role="search">
              <Input
                aria-label={t.search}
                placeholder={t.search}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                className="w-48 sm:w-64"
              />
              <Button type="submit" size="sm" aria-label={t.search}>
                <Search aria-hidden="true" />
              </Button>
            </form>
            <LanguageSwitcher current={locale} label={language} cookieOnly={!!ready?.hosted} />
            <ThemeSwitcher labels={theme} />
          </div>
          <div className="mx-auto flex w-full max-w-[84rem] [.doc-wide_&]:max-w-none flex-wrap items-center gap-3 px-4 pb-2 sm:px-8">
            <div className="min-w-0 flex-1">
              <Trail />
            </div>
            {ready && (ready.root || ready.hosted) && <ProjectPicker label={t.project} />}
          </div>
        </header>
        <main id="main" tabIndex={-1} className="mx-auto w-full max-w-[84rem] [.doc-wide_&]:max-w-none min-w-0 flex-1 px-4 py-6 focus:outline-none sm:px-8">
          {!ready ? (
            <SkeletonPanel lines={6} label={t.title} />
          ) : !ready.root && !ready.hosted ? (
            <EmptyState icon={BookOpen} title={t.noRoot} />
          ) : q ? (
            <Results q={q} text={t} />
          ) : (
            <DocSpace locale={locale} labels={labels} />
          )}
        </main>
      </div>
      <PublicToaster />
    </CrumbsProvider>
  );
}

function PublicToaster() {
  const [theme] = useTheme();
  return <Toaster theme={theme} position="bottom-right" closeButton duration={5000} />;
}

function EntryTitle({ fallback }: { fallback: string }) {
  useDocStore();
  const title = spaceEntry("")?.value?.entry?.title;
  return (
    <a href={docHref([])} className="flex min-w-0 items-center gap-2 font-semibold text-ink">
      <BookOpen aria-hidden="true" className="size-5 shrink-0 text-signal" />
      <span className="truncate">{title ?? fallback}</span>
    </a>
  );
}

function ProjectPicker({ label }: { label: string }) {
  useDocStore();
  useEffect(() => {
    ensureSpace("");
  }, []);
  const projects = spaceEntry("")?.value?.entry?.projects ?? [];
  const current = splitPath(window.location.pathname.slice(docBase().length)).join("/");
  const selected = projects.find((p) => p.path && (current === p.path || current.startsWith(`${p.path}/`)))?.path ?? "";
  if (projects.length < 2) return null;
  return (
    <Select aria-label={label} value={selected} placeholder={label} onChange={(e) => navigateDoc(docHref(splitPath(e.target.value)))} className="h-8 w-56 text-sm">
      {projects.map((p) => (
        <option key={p.path} value={p.path}>
          {p.name}
        </option>
      ))}
    </Select>
  );
}

function Trail() {
  const items = useCrumbTrail();
  if (items.length === 0) return null;
  return (
    <nav aria-label="breadcrumbs" className="flex min-w-0 flex-wrap items-center gap-1 text-sm text-muted">
      {items.map((c, i) => (
        <span key={`${i}:${c.label}`} className="flex min-w-0 items-center gap-1">
          {i > 0 && <span aria-hidden="true">/</span>}
          {c.href && i < items.length - 1 ? (
            <DocLink href={c.href} className="truncate hover:text-ink hover:underline">
              {c.label}
            </DocLink>
          ) : (
            <span className="truncate text-ink">{c.label}</span>
          )}
        </span>
      ))}
    </nav>
  );
}

function Results({ q, text: t }: { q: string; text: Text }) {
  const { errors } = useUiText();
  const [state, setState] = useState<{ q: string; hits: Hit[] | null; error: string | null }>({ q: "", hits: null, error: null });
  useEffect(() => {
    let live = true;
    apiGet<{ items: Hit[] }>(searchUrl(q))
      .then((r) => live && setState({ q, hits: r.items, error: null }))
      .catch((e: unknown) => live && setState({ q, hits: null, error: errorText(errors, errorCode(e)) }));
    return () => {
      live = false;
    };
  }, [q, errors]);
  const current = state.q === q ? state : null;
  return (
    <section aria-label={t.results} className="grid gap-4">
      <h1 className="text-lg font-semibold text-ink">{t.results}</h1>
      {current?.error && <Message note={{ kind: "error", text: current.error }} />}
      {!current && <SkeletonPanel lines={4} label={t.results} />}
      {current?.hits && current.hits.length === 0 && <p className="text-sm text-muted">{t.noResults}</p>}
      <ul className="grid gap-3">
        {current?.hits?.map((h) => {
          const href = docHref([...splitPath(h.project.path), ...splitPath(h.path)], h.ref_kind === "tag" ? `tag:${h.branch}` : null);
          return (
            <li key={`${h.project.path}:${h.path}`} className="rounded-card border border-line bg-surface p-3">
              <DocLink href={href} className="font-medium text-signal hover:underline">
                {h.title || h.path}
              </DocLink>
              <p className="mt-0.5 text-xs text-muted">
                {h.project.name} · {h.path}
              </p>
              <p className="mt-1 text-sm text-ink-2">
                {h.snippet.map((s, i) =>
                  s.match ? (
                    <mark key={i} className="rounded-sm bg-signal-soft px-0.5 text-ink">
                      {s.text}
                    </mark>
                  ) : (
                    <span key={i}>{s.text}</span>
                  ),
                )}
              </p>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
