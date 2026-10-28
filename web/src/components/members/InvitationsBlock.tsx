"use client";

import { toast } from "sonner";
import { useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import { errorCode } from "@/lib/api";
import { stateTone, type Invitation, type IssuedInvitation, type MemberRole } from "@/lib/members";
import { resendInvitation, revokeInvitation } from "@/lib/membersApi";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Panel } from "../ui/Panel";
import { Table, Td, Th, Tr } from "../ui/Table";
import { IssuedLink } from "./InviteDialog";

type Props = {
  org: string;
  items: Invitation[];
  locale: Locale;
  roles: Record<MemberRole, string>;
  labels: Messages["catalog"]["members"];
  onChanged: () => void;
};

export function InvitationsBlock({ org, items, locale, roles, labels: t, onChanged }: Props) {
  const { errors } = useUiText();
  const [issued, setIssued] = useState<IssuedInvitation | null>(null);
  const fail = (e: unknown) => toast.error(errorText(errors, errorCode(e)));
  if (items.length === 0) return null;
  return (
    <Panel title={t.invitationsTitle} bodyClassName="p-0 sm:p-0">
      <Table>
        <thead>
          <Tr>
            <Th>{t.email}</Th>
            <Th>{t.role}</Th>
            <Th className="hidden md:table-cell">{t.invitedBy}</Th>
            <Th>{t.expires}</Th>
            <Th />
          </Tr>
        </thead>
        <tbody>
          {items.map((i) => (
            <Tr key={i.id}>
              <Td className="font-medium text-ink">{i.email}</Td>
              <Td>{roles[i.role]}</Td>
              <Td className="hidden text-muted md:table-cell">{i.invited_by?.name ?? "—"}</Td>
              <Td>
                {i.state === "expired" ? (
                  <Badge tone={stateTone(i.state)}>{t.expired}</Badge>
                ) : (
                  <span title={when(i.expires_at, locale, "")}>{ago(i.expires_at, locale, "")}</span>
                )}
              </Td>
              <Td>
                <span className="flex flex-wrap justify-end gap-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      resendInvitation(org, i.id)
                        .then((r) => {
                          setIssued(r);
                          onChanged();
                        })
                        .catch(fail)
                    }
                  >
                    {t.resend}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() =>
                      revokeInvitation(org, i.id)
                        .then(() => {
                          toast.success(t.revoked);
                          onChanged();
                        })
                        .catch(fail)
                    }
                  >
                    {t.revoke}
                  </Button>
                </span>
              </Td>
            </Tr>
          ))}
        </tbody>
      </Table>
      <Dialog open={issued !== null} onOpenChange={(o) => !o && setIssued(null)} title={t.resend} footer={<Button onClick={() => setIssued(null)}>{t.done}</Button>}>
        {issued && <IssuedLink issued={issued} labels={t} />}
      </Dialog>
    </Panel>
  );
}
