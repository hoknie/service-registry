"use client";

import { KeyRound, LogIn, UserRound } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";

import { LoginMethodsTab } from "./account/LoginMethodsTab";
import { PasswordPanel } from "./account/PasswordPanel";
import { TokensTab } from "./account/TokensTab";
import { useSession } from "./SessionProvider";
import { Badge } from "./ui/Badge";
import { Panel } from "./ui/Panel";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "./ui/Tabs";

type Props = { locale: Locale; labels: Messages["account"]; tokenLabels: Messages["tokens"]; superadminLabel: string };

type Tab = "profile" | "login" | "tokens";

const tabs: Tab[] = ["profile", "login", "tokens"];

export function AccountView({ locale, labels, tokenLabels, superadminLabel }: Props) {
  const router = useRouter();
  const asked = useSearchParams().get("tab");
  const tab: Tab = tabs.find((t) => t === asked) ?? "profile";
  const selectTab = (next: string) =>
    router.replace(next === "profile" ? `/${locale}/account` : `/${locale}/account?tab=${next}`, { scroll: false });
  const { session } = useSession();
  if (session.status !== "signed-in") return null;
  return (
    <Tabs value={tab} onValueChange={selectTab} activationMode="manual">
      <TabsList aria-label={labels.tabs.label}>
        <TabsTrigger value="profile">
          <UserRound aria-hidden="true" />
          {labels.tabs.profile}
        </TabsTrigger>
        <TabsTrigger value="login">
          <LogIn aria-hidden="true" />
          {labels.tabs.login}
        </TabsTrigger>
        <TabsTrigger value="tokens">
          <KeyRound aria-hidden="true" />
          {labels.tabs.tokens}
        </TabsTrigger>
      </TabsList>
      <TabsContent value="profile">
        <Profile labels={labels} superadminLabel={superadminLabel} />
      </TabsContent>
      <TabsContent value="login">
        <LoginMethodsTab locale={locale} labels={labels} />
      </TabsContent>
      <TabsContent value="tokens">
        <TokensTab locale={locale} superadmin={session.user.is_superadmin} labels={tokenLabels} />
      </TabsContent>
    </Tabs>
  );
}

function Profile({ labels, superadminLabel }: { labels: Messages["account"]; superadminLabel: string }) {
  const { session } = useSession();
  if (session.status !== "signed-in") return null;
  const { user } = session;
  return (
    <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)]">
      <Panel title={labels.profile}>
        <div className="mb-5 flex items-center gap-4">
          <span aria-hidden="true" className="grid size-14 place-items-center rounded-full bg-signal-soft text-xl font-semibold text-signal">
            {Array.from(user.display_name.trim())[0]?.toUpperCase() ?? "?"}
          </span>
          <div className="min-w-0">
            <p className="truncate text-lg font-semibold text-ink">{user.display_name}</p>
            {user.is_superadmin && <Badge tone="amber">{superadminLabel}</Badge>}
          </div>
        </div>
        <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-2 text-sm">
          <dt className="text-muted">{labels.email}</dt>
          <dd className="truncate text-ink">{user.email}</dd>
          <dt className="text-muted">{labels.name}</dt>
          <dd className="truncate text-ink">{user.display_name}</dd>
        </dl>
      </Panel>
      <PasswordPanel labels={labels} />
    </div>
  );
}
