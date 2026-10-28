"use client";

import { Download, Upload } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { when } from "@/i18n/time";
import { apiSend, errorCode } from "@/lib/api";
import {
  apiHref,
  humanSize,
  skillUrl,
  withVersion,
  type Skill,
  type SkillVersion,
} from "@/lib/skills";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Table, Td, Th, Tr } from "../ui/Table";

type Props = {
  skill: Skill;
  versions: SkillVersion[];
  locale: Locale;
  labels: Messages["skills"];
  onOpen: (version: string) => void;
  onChanged: () => void;
};

export function SkillVersions({
  skill,
  versions,
  locale,
  labels: t,
  onOpen,
  onChanged,
}: Props) {
  const v = t.versions;
  const { errors } = useUiText();
  const [error, setError] = useState<string | null>(null);
  const yank = async (version: string, yanked: boolean) => {
    try {
      await apiSend(
        "PUT",
        skillUrl(skill.id, `/versions/${encodeURIComponent(version)}`),
        { yanked },
      );
      onChanged();
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    }
  };
  return (
    <Panel
      title={t.page.versions}
      actions={
        skill.can_write &&
        !skill.managed && (
          <Button asChild variant="primary">
            <Link href={`/${locale}/skills/upload?skill=${skill.id}`}>
              <Upload aria-hidden="true" />
              {t.page.uploadVersion}
            </Link>
          </Button>
        )
      }
    >
      <Message
        note={error ? { kind: "error", text: error } : null}
        className="mb-3"
      />
      {versions.length === 0 ? (
        <p className="text-sm text-muted">{v.empty}</p>
      ) : (
        <Table>
          <thead>
            <Tr>
              <Th>{v.version}</Th>
              <Th>{v.created}</Th>
              <Th>{v.origin}</Th>
              <Th>{v.author}</Th>
              <Th numeric>{v.files}</Th>
              <Th numeric>{v.size}</Th>
              <Th>{v.available}</Th>
              <Th />
            </Tr>
          </thead>
          <tbody>
            {versions.map((x) => (
              <Tr key={x.version}>
                <Td>
                  <button
                    type="button"
                    className="font-mono text-signal hover:underline"
                    onClick={() => onOpen(x.version)}
                  >
                    {x.version}
                  </button>
                  {x.prerelease && (
                    <Badge tone="outline" className="ml-2">
                      {t.page.prerelease}
                    </Badge>
                  )}
                </Td>
                <Td className="text-xs">{when(x.created_at, locale, "")}</Td>
                <Td className="text-xs">
                  {x.source === "git"
                    ? `${v.git} ${x.tag ?? x.commit?.slice(0, 8) ?? ""}`
                    : v.upload}
                </Td>
                <Td className="text-xs">{x.created_by?.name ?? ""}</Td>
                <Td numeric>{x.files}</Td>
                <Td numeric>{humanSize(x.size)}</Td>
                <Td>
                  {x.yanked ? (
                    <Badge tone="danger">{t.page.yanked}</Badge>
                  ) : x.available ? (
                    v.yes
                  ) : (
                    <Badge tone="amber">{t.page.unavailable}</Badge>
                  )}
                </Td>
                <Td>
                  <span className="flex flex-wrap justify-end gap-1">
                    {(x.available || skill.can_write) && (
                      <Button asChild size="sm" variant="ghost">
                        <a href={apiHref(withVersion(skillUrl(skill.id, "/archive"), x.version))} download>
                          <Download aria-hidden="true" />
                          {t.page.download}
                        </a>
                      </Button>
                    )}
                    {skill.can_write && (
                      <Button size="sm" variant="ghost" onClick={() => yank(x.version, !x.yanked)}>
                        {x.yanked ? v.restore : v.yank}
                      </Button>
                    )}
                  </span>
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
    </Panel>
  );
}
