"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type CatalogNode } from "@/lib/api";
import {
  MCP_BASE,
  type McpOrganizationSet,
  type McpServerPage,
  type McpServerRef,
  type McpSpace,
} from "@/lib/mcp";
import { loadOrgSet, loadSpace, saveOrgSet } from "@/lib/mcpApi";

import { ProxyForm } from "../mcp/ProxyForm";
import { ServerSetPicker } from "../mcp/ServerSetPicker";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { MarkdownField } from "../places/MarkdownField";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";

export function OrganizationMcp({
  node,
  locale,
  labels: t,
}: {
  node: Pick<CatalogNode, "id">;
  locale: Locale;
  labels: Messages["mcp"];
}) {
  const o = t.organization;
  const { errors } = useUiText();
  const [space, setSpace] = useState<McpSpace | null>(null);
  const [description, setDescription] = useState("");
  const [set, setSet] = useState<McpOrganizationSet | null>(null);
  const [selected, setSelected] = useState<string[] | null>(null);
  const [candidates, setCandidates] = useState<McpServerRef[] | undefined>(
    undefined,
  );
  const [note, setNote] = useState<Note>(null);
  const [setNoteState, setSetNote] = useState<Note>(null);
  const [busy, setBusy] = useState<"description" | "set" | null>(null);

  useEffect(() => {
    loadSpace(node.id)
      .then((s) => {
        setSpace(s);
        setDescription(s.description);
      })
      .catch((e: unknown) =>
        setNote({ kind: "error", text: errorText(errors, errorCode(e)) }),
      );
    loadOrgSet(node.id)
      .then((s) => {
        setSet(s);
        setSelected(s.mode === "all" ? null : s.servers.map((x) => x.id));
      })
      .catch(() => setSet(null));
    Promise.all([
      apiGet<McpServerPage>(
        `${MCP_BASE}/servers?organization_id=global&limit=200`,
      ),
      apiGet<McpServerPage>(
        `${MCP_BASE}/servers?organization_id=${node.id}&limit=200`,
      ),
    ])
      .then(([g, own]) =>
        setCandidates(
          [...own.items, ...g.items].map((s) => ({
            id: s.id,
            slug: s.slug,
            title: s.title,
          })),
        ),
      )
      .catch(() => setCandidates([]));
  }, [node.id, errors]);

  const saveDescription = async () => {
    setBusy("description");
    setNote(null);
    try {
      const s = await apiSend<McpSpace>(
        "PUT",
        `${MCP_BASE}/organizations/${node.id}`,
        { description },
      );
      setSpace(s);
      setDescription(s.description);
      setNote({ kind: "success", text: o.saved });
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(null);
    }
  };

  const saveSet = async () => {
    setBusy("set");
    setSetNote(null);
    try {
      const s = await saveOrgSet(node.id, {
        mode: selected === null ? "all" : "selected",
        servers: selected ?? set?.servers.map((x) => x.id) ?? [],
      });
      setSet(s);
      setSetNote({ kind: "success", text: o.saved });
    } catch (e) {
      setSetNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="grid gap-4">
      <Panel title={o.title} description={o.lead}>
        <div className="grid gap-4">
          <MarkdownField
            label={o.description}
            hint={o.descriptionHint}
            rows={4}
            maxLength={2000}
            value={description}
            readOnly={!space?.can_write}
            onChange={setDescription}
          />
          <Message note={note} />
          <div className="flex flex-wrap items-center gap-3">
            {space?.can_write && (
              <Button
                variant="primary"
                busy={busy === "description"}
                onClick={saveDescription}
                disabled={description === space.description}
              >
                {o.save}
              </Button>
            )}
            <Link
              className="text-sm text-signal hover:underline"
              href={`/${locale}/mcp/visibility?organization=${node.id}`}
            >
              {t.visibility.title}
            </Link>
          </div>
        </div>
      </Panel>
      {set && (
        <Panel title={o.setTitle} description={o.setLead}>
          <div className="grid gap-4">
            <ServerSetPicker
              value={selected}
              onChange={setSelected}
              labels={t.picker}
              servers={candidates}
              disabled={!set.can_manage}
            />
            <Message note={setNoteState} />
            {set.can_manage && (
              <div>
                <Button
                  variant="primary"
                  busy={busy === "set"}
                  onClick={saveSet}
                >
                  {o.saveSet}
                </Button>
              </div>
            )}
          </div>
        </Panel>
      )}
      {space?.can_manage && (
        <ProxyForm
          owner="organizations"
          id={node.id}
          organization={node.id}
          canEdit
          locale={locale}
          labels={t}
        />
      )}
    </div>
  );
}
