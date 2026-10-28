"use client";

import { useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiSend, errorCode } from "@/lib/api";
import {
  MCP_BASE,
  serverUrl,
  type McpAuthKind,
  type McpServer,
  type McpStatus,
} from "@/lib/mcp";
import { checkServer } from "@/lib/mcpApi";
import { slugOf } from "@/lib/slug";

import { SecretPicker } from "../secrets/SecretPicker";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { MarkdownField } from "../places/MarkdownField";
import { Checkbox, Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { IconPicker } from "../ui/IconPicker";
import { Segmented } from "../ui/Segmented";
import { Select } from "../ui/Select";
import { TagInput } from "../ui/TagInput";

type Draft = {
  title: string;
  slug: string;
  description: string;
  tags: string[];
  iconMode: "auto" | "set" | "url";
  icon: string;
  iconUrl: string;
  url: string;
  transport: "streamable" | "sse";
  auth: McpAuthKind;
  header: string;
  secret: string | null;
  status: McpStatus;
  logPayloads: boolean;
};

function draftOf(s?: McpServer): Draft {
  return {
    title: s?.title ?? "",
    slug: s?.slug ?? "",
    description: s?.description ?? "",
    tags: s?.tags ?? [],
    iconMode: s?.icon_url ? "url" : s?.icon ? "set" : "auto",
    icon: s?.icon ?? "plug",
    iconUrl: s?.icon_url ?? "",
    url: s?.url ?? "",
    transport: s?.transport ?? "streamable",
    auth: s?.auth.kind ?? "none",
    header: s?.auth.header ?? "",
    secret: s?.auth.credentials?.secret.id ?? null,
    status: s?.status ?? "draft",
    logPayloads: s?.log_payloads ?? true,
  };
}

type Props = {
  server?: McpServer;
  folder?: string | null;
  space?: string | null;
  locale: Locale;
  labels: Messages["mcp"];
  onSaved: (s: McpServer) => void;
  part?: "general" | "connection";
};

export function ServerForm({
  server,
  folder,
  space,
  locale,
  labels: t,
  onSaved,
  part,
}: Props) {
  const general = part !== "connection";
  const connection = part !== "general";
  const f = t.form;
  const { errors } = useUiText();
  const [d, setD] = useState<Draft>(() => draftOf(server));
  const [slugTouched, setSlugTouched] = useState(Boolean(server));
  const [busy, setBusy] = useState<"save" | "check" | null>(null);
  const [note, setNote] = useState<Note>(null);
  const set = (patch: Partial<Draft>) => setD((cur) => ({ ...cur, ...patch }));
  const org = server
    ? server.organization_id
    : space && space !== "global"
      ? space
      : null;

  const generalBody = () => ({
    title: d.title,
    description: d.description,
    tags: d.tags,
    icon_url:
      d.iconMode === "url" && d.iconUrl.trim() ? d.iconUrl.trim() : null,
    icon: d.iconMode === "set" ? d.icon : null,
    status: d.status,
    log_payloads: d.logPayloads,
  });
  const connectionBody = () => ({
    url: d.url,
    transport: d.transport,
    auth:
      d.auth === "none"
        ? { kind: "none" }
        : {
            kind: d.auth,
            header: d.auth === "header" ? d.header : undefined,
            credentials: { secret_id: d.secret },
          },
  });
  const body = () => ({
    ...(general ? generalBody() : {}),
    ...(connection ? connectionBody() : {}),
  });

  const save = async (check: boolean) => {
    setBusy(check ? "check" : "save");
    setNote(null);
    try {
      let saved = server
        ? await apiSend<McpServer>("PATCH", serverUrl(server.id), body())
        : await apiSend<McpServer>("POST", `${MCP_BASE}/servers`, {
            ...body(),
            slug: d.slug,
            folder_id: folder ?? null,
            organization_id: folder ? null : org,
          });
      if (check) saved = await checkServer(saved.id);
      setNote({ kind: "success", text: server ? f.saved : f.created });
      onSaved(saved);
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(null);
    }
  };

  const secretsHref = org
    ? `/${locale}/catalog?node=${org}&tab=settings&section=secrets`
    : `/${locale}/admin/secrets`;
  return (
    <form
      className="grid gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        void save(false);
      }}
    >
      {general && (
        <>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={f.title}>
              {(p) => (
                <Input
                  {...p}
                  required
                  value={d.title}
                  onChange={(e) => {
                    set({ title: e.target.value });
                    if (!slugTouched)
                      set({
                        title: e.target.value,
                        slug: slugOf(e.target.value, 32),
                      });
                  }}
                />
              )}
            </Field>
            <Field label={f.slug} hint={f.slugHint}>
              {(p) => (
                <Input
                  {...p}
                  required
                  className="font-mono"
                  disabled={Boolean(server)}
                  value={d.slug}
                  onChange={(e) => {
                    setSlugTouched(true);
                    set({ slug: e.target.value.toLowerCase() });
                  }}
                />
              )}
            </Field>
          </div>
          <MarkdownField
            label={f.description}
            rows={4}
            value={d.description}
            onChange={(description) => set({ description })}
          />
          <div className="grid gap-4">
            <Field label={f.tags}>
              {(p) => (
                <TagInput
                  {...p}
                  value={d.tags.join("\n")}
                  onChange={(v) => set({ tags: v.split("\n").filter(Boolean) })}
                  placeholder={f.tags}
                />
              )}
            </Field>
          </div>
          <div className="grid gap-1.5">
            <span className="text-sm font-medium text-ink-2">{f.icon}</span>
            <Segmented
              label={f.icon}
              value={d.iconMode}
              onChange={(v) => set({ iconMode: v as Draft["iconMode"] })}
              options={[
                { value: "auto", label: f.iconAuto },
                { value: "set", label: f.iconSet },
                { value: "url", label: f.iconUrl },
              ]}
            />
            {d.iconMode === "auto" && (
              <p className="text-xs text-muted">{f.iconAutoHint}</p>
            )}
            {d.iconMode === "set" && (
              <IconPicker
                value={d.icon}
                onChange={(icon) => set({ icon })}
                labels={{
                  search: f.iconSearch,
                  empty: f.iconEmpty,
                  choose: f.iconChoose,
                }}
              />
            )}
            {d.iconMode === "url" && (
              <Input
                aria-label={f.iconUrl}
                type="url"
                className="font-mono"
                value={d.iconUrl}
                onChange={(e) => set({ iconUrl: e.target.value })}
                placeholder="https://"
              />
            )}
          </div>
        </>
      )}
      {connection && (
        <>
          <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_12rem]">
            <Field label={f.url} hint={f.urlHint}>
              {(p) => (
                <Input
                  {...p}
                  required
                  type="url"
                  className="font-mono"
                  value={d.url}
                  onChange={(e) => set({ url: e.target.value })}
                />
              )}
            </Field>
            <Field label={f.transport}>
              {(p) => (
                <Select
                  {...p}
                  value={d.transport}
                  onChange={(e) =>
                    set({ transport: e.target.value as Draft["transport"] })
                  }
                >
                  <option value="streamable">{f.transports.streamable}</option>
                  <option value="sse">{f.transports.sse}</option>
                </Select>
              )}
            </Field>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={f.auth}>
              {(p) => (
                <Select
                  {...p}
                  value={d.auth}
                  onChange={(e) => set({ auth: e.target.value as McpAuthKind })}
                >
                  <option value="none">{f.auths.none}</option>
                  <option value="bearer">{f.auths.bearer}</option>
                  <option value="header">{f.auths.header}</option>
                </Select>
              )}
            </Field>
            {d.auth === "header" && (
              <Field label={f.header} hint={f.headerHint}>
                {(p) => (
                  <Input
                    {...p}
                    required
                    className="font-mono"
                    value={d.header}
                    onChange={(e) => set({ header: e.target.value })}
                  />
                )}
              </Field>
            )}
          </div>
          {d.auth !== "none" && (
            <SecretPicker
              listOn={org}
              createOn={org ?? false}
              value={d.secret}
              onChange={(secret) => set({ secret })}
              manageHref={secretsHref}
              required
            />
          )}
        </>
      )}
      {general && (
        <>
          <Field label={f.status}>
            {(p) => (
              <Select
                {...p}
                className="w-56"
                value={d.status}
                onChange={(e) => set({ status: e.target.value as McpStatus })}
              >
                <option value="draft">{t.statuses.draft}</option>
                <option value="published">{t.statuses.published}</option>
                <option value="disabled">{t.statuses.disabled}</option>
              </Select>
            )}
          </Field>
          <div className="grid gap-1">
            <Checkbox
              label={f.logPayloads}
              checked={d.logPayloads}
              onChange={(e) => set({ logPayloads: e.target.checked })}
            />
            <p className="text-xs text-muted">{f.logPayloadsHint}</p>
          </div>
        </>
      )}
      <Message note={note} />
      <div className="flex flex-wrap gap-2">
        <Button
          type="submit"
          variant="primary"
          busy={busy === "save"}
          disabled={busy !== null || !d.title || !d.slug || !d.url}
        >
          {server ? f.save : f.create}
        </Button>
        {connection && (
          <Button
            type="button"
            busy={busy === "check"}
            disabled={busy !== null || !d.title || !d.slug || !d.url}
            onClick={() => save(true)}
          >
            {f.check}
          </Button>
        )}
      </div>
    </form>
  );
}
