"use client";

import { KeyRound, Search } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type Page, type PersonalToken } from "@/lib/api";

import { TokenTable, type TokenLabels } from "../account/TokenTable";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { Pager } from "../ui/Pager";
import { SkeletonTable } from "../ui/Skeleton";

const PAGE_SIZE = 50;
const PREFIX_MAX = 12;

type Props = { locale: Locale; labels: Messages["admin"]; tokenLabels: TokenLabels };

export function TokensAdmin({ locale, labels, tokenLabels }: Props) {
  const { errors, common } = useUiText();
  const t = labels.tokens;
  const [offset, setOffset] = useState(0);
  const [draft, setDraft] = useState("");
  const [prefix, setPrefix] = useState("");
  const [page, setPage] = useState<Page<PersonalToken> | null>(null);
  const [listError, setListError] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<PersonalToken | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const load = useCallback(
    async (at: number, start: string) => {
      try {
        const query = new URLSearchParams({ limit: String(PAGE_SIZE), offset: String(at) });
        if (start) query.set("prefix", start);
        setPage(await apiGet<Page<PersonalToken>>(`/v1/tokens?${query}`));
        setListError(null);
      } catch (e) {
        setListError(fail(e));
      }
    },
    [fail],
  );

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load(offset, prefix);
  }, [load, offset, prefix]);

  const search = (event: FormEvent) => {
    event.preventDefault();
    setOffset(0);
    setPrefix(draft.trim());
  };

  return (
    <>
      <PageHeader glyph={KeyRound} title={t.title} lead={t.lead} />
      <form className="mb-4 grid gap-1.5" onSubmit={search} role="search">
        <div className="flex flex-wrap items-end gap-2">
          <Field label={t.search} className="w-72 max-w-full">
            {(p) => (
              <Input
                {...p}
                aria-describedby="token-prefix-hint"
                maxLength={PREFIX_MAX}
                spellCheck={false}
                autoComplete="off"
                placeholder="svcp_"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
              />
            )}
          </Field>
          <Button type="submit">
            <Search aria-hidden="true" />
            {t.searchSubmit}
          </Button>
        </div>
        <p id="token-prefix-hint" className="text-xs text-muted">
          {t.searchHint}
        </p>
      </form>
      {listError && <Message note={{ kind: "error", text: listError }} className="mb-4" />}
      {!page && !listError && <SkeletonTable rows={6} label={common.loading} />}
      {page && page.items.length === 0 && <EmptyState icon={KeyRound} title={t.empty} />}
      {page && page.items.length > 0 && (
        <>
          <TokenTable tokens={page.items} locale={locale} labels={tokenLabels} showOwner onRevoke={setRevoking} />
          <Pager offset={offset} shown={page.items.length} total={page.total} size={PAGE_SIZE} labels={labels.users.pager} onChange={setOffset} />
        </>
      )}
      {revoking && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setRevoking(null)}
          title={tokenLabels.revokeTitle}
          text={format(tokenLabels.confirmRevoke, { name: revoking.name, prefix: revoking.prefix })}
          confirm={tokenLabels.revoke}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `/v1/tokens/${revoking.id}`);
              toast.success(tokenLabels.revoked);
              await load(offset, prefix);
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </>
  );
}
