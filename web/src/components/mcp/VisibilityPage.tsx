"use client";

import { Eye, UsersRound } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import type { McpVisibility } from "@/lib/mcp";
import { loadVisibility, saveOrgSet } from "@/lib/mcpApi";
import { toast } from "sonner";

import { Crumbs } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { EmptyState } from "../ui/EmptyState";
import { Checkbox, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { Select } from "../ui/Select";
import { SkeletonRows } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { StateBadge } from "./ServerCard";

export function VisibilityPage({
  locale,
  labels: t,
}: {
  locale: Locale;
  labels: Messages["mcp"];
}) {
  const v = t.visibility;
  const params = useSearchParams();
  const router = useRouter();
  const { errors } = useUiText();
  const org = params.get("organization");
  const q = params.get("q") ?? "";
  const [data, setData] = useState<McpVisibility | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setData(await loadVisibility(org));
      setError(null);
    } catch (e) {
      setError(
        errorCode(e) === "auth.forbidden"
          ? v.forbidden
          : errorText(errors, errorCode(e)),
      );
    }
  }, [org, errors, v.forbidden]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const route = (next: { organization?: string | null; q?: string }) => {
    const s = new URLSearchParams();
    const o = next.organization === undefined ? org : next.organization;
    const text = next.q ?? q;
    if (o) s.set("organization", o);
    if (text) s.set("q", text);
    router.replace(`/${locale}/mcp/visibility${s.size ? `?${s}` : ""}`);
  };

  const setOrg = async (
    orgId: string,
    mode: "all" | "selected",
    servers: string[],
  ) => {
    try {
      await saveOrgSet(orgId, { mode, servers });
      toast.success(v.saved);
      await load();
    } catch (e) {
      toast.error(errorText(errors, errorCode(e)));
    }
  };

  const header = (
    <>
      <Crumbs
        items={[{ label: t.title, href: `/${locale}/mcp` }, { label: v.title }]}
      />
      <PageHeader title={v.title} lead={v.lead} />
    </>
  );
  if (error && !data)
    return (
      <>
        {header}
        <EmptyState icon={Eye} title={error} />
      </>
    );
  if (!data) return <SkeletonRows rows={5} label={v.title} />;
  const servers = data.servers.filter((s) =>
    `${s.slug} ${s.title}`.toLowerCase().includes(q.toLowerCase()),
  );
  const members = (orgId: string) =>
    data.servers.filter((s) => s.in.includes(orgId)).map((s) => s.id);
  return (
    <>
      {header}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <Input
          className="w-72"
          type="search"
          placeholder={v.search}
          aria-label={v.search}
          defaultValue={q}
          onChange={(e) => route({ q: e.target.value })}
        />
        <Select
          className="w-64"
          aria-label={v.organization}
          value={org ?? ""}
          onChange={(e) => route({ organization: e.target.value || null })}
        >
          <option value="">{v.allOrganizations}</option>
          {data.organizations.map((o) => (
            <option key={o.id} value={o.id}>
              {o.name}
            </option>
          ))}
        </Select>
      </div>
      <Message
        note={error ? { kind: "error", text: error } : null}
        className="mb-4"
      />
      {servers.length === 0 ? (
        <EmptyState icon={Eye} title={v.empty} />
      ) : (
        <div className="overflow-x-auto rounded-card border border-line bg-surface shadow-elev-1">
          <Table>
            <thead>
              <Tr>
                <Th>{v.server}</Th>
                <Th>{v.access}</Th>
                {data.organizations.map((o) => (
                  <Th key={o.id} className="min-w-44">
                    <div className="grid gap-1">
                      <span>{o.name}</span>
                      <Select
                        aria-label={`${v.mode} ${o.name}`}
                        disabled={!o.can_manage}
                        value={o.mode}
                        onChange={(e) =>
                          setOrg(
                            o.id,
                            e.target.value as "all" | "selected",
                            members(o.id),
                          )
                        }
                      >
                        <option value="all">{v.modes.all}</option>
                        <option value="selected">{v.modes.selected}</option>
                      </Select>
                    </div>
                  </Th>
                ))}
              </Tr>
            </thead>
            <tbody>
              {servers.map((s) => (
                <Tr key={s.id}>
                  <Td>
                    <Link
                      className="font-medium text-ink hover:underline"
                      href={`/${locale}/mcp/server?id=${s.id}`}
                    >
                      {s.title}
                    </Link>
                    <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                      <span className="font-mono">{s.slug}</span>
                      <span>
                        {s.organization_id
                          ? (data.organizations.find(
                              (o) => o.id === s.organization_id,
                            )?.name ?? "")
                          : t.scopeGlobal}
                      </span>
                      <StateBadge server={s} labels={t} />
                    </div>
                  </Td>
                  <Td>
                    <Link
                      className="inline-flex items-center gap-1 text-sm hover:underline"
                      href={`/${locale}/mcp/server?id=${s.id}&tab=settings&section=access`}
                    >
                      <UsersRound aria-hidden="true" className="size-3.5" />
                      {s.access.everyone
                        ? v.everyone
                        : [
                            ...s.access.groups.map((g) => g.name),
                            ...s.access.users.map((u) => u.email),
                          ].join(", ") || v.nobody}
                    </Link>
                  </Td>
                  {data.organizations.map((o) => {
                    const own =
                      !s.organization_id || s.organization_id === o.id;
                    const inSet = s.in.includes(o.id);
                    return (
                      <Td key={o.id}>
                        {own ? (
                          <Checkbox
                            label={
                              <span className="sr-only">{`${s.title} — ${o.name}`}</span>
                            }
                            checked={inSet}
                            disabled={!o.can_manage || o.mode === "all"}
                            onChange={(e) =>
                              setOrg(
                                o.id,
                                "selected",
                                e.target.checked
                                  ? [...members(o.id), s.id]
                                  : members(o.id).filter((id) => id !== s.id),
                              )
                            }
                          />
                        ) : (
                          <span className="text-muted">—</span>
                        )}
                      </Td>
                    );
                  })}
                </Tr>
              ))}
            </tbody>
          </Table>
        </div>
      )}
    </>
  );
}
