"use client";

import { ChevronDown, LogIn, LogOut, UserRound } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";

import { useSession } from "./SessionProvider";
import { Badge } from "./ui/Badge";
import { Button } from "./ui/Button";
import { Menu, MenuContent, MenuItem, MenuSeparator, MenuTrigger } from "./ui/Menu";

type Props = {
  locale: Locale;
  nav: Messages["header"]["nav"];
  labels: Messages["header"]["user"];
};

function initials(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  const letters = words.slice(0, 2).map((w) => Array.from(w)[0] ?? "");
  return letters.join("").toUpperCase() || "?";
}

export function UserMenu({ locale, nav, labels }: Props) {
  const { session, signOut } = useSession();
  const router = useRouter();

  if (session.status === "loading") return <span aria-hidden="true" className="size-8" />;

  if (session.status === "anonymous") {
    return (
      <Button asChild variant="primary" size="sm">
        <Link href={`/${locale}/login`}>
          <LogIn aria-hidden="true" />
          {nav.login}
        </Link>
      </Button>
    );
  }

  const { user } = session;
  const logout = async () => {
    await signOut();
    router.replace(`/${locale}/login`);
  };

  return (
    <Menu>
      <MenuTrigger asChild>
        <Button variant="ghost" size="sm" className="gap-2 pl-1" aria-label={`${labels.label}: ${user.display_name}`}>
          <span aria-hidden="true" className="grid size-6 place-items-center rounded-full bg-signal-soft text-[0.7rem] font-semibold text-signal">
            {initials(user.display_name)}
          </span>
          <span className="hidden max-w-40 truncate md:inline">{user.display_name}</span>
          <ChevronDown aria-hidden="true" className="text-muted" />
        </Button>
      </MenuTrigger>
      <MenuContent className="w-64">
        <div className="px-2.5 py-2">
          <p className="truncate font-medium text-ink">{user.display_name}</p>
          <p className="truncate text-xs text-muted">{user.email}</p>
          {user.login_method === "password" && <p className="mt-1 text-xs text-muted">{labels.viaPassword}</p>}
          {user.login_method.startsWith("oauth:") && (
            <p className="mt-1 truncate text-xs text-muted">
              {format(labels.viaProvider, { provider: user.login_method.slice("oauth:".length) })}
            </p>
          )}
          {user.is_superadmin && (
            <Badge tone="amber" className="mt-2">
              {labels.superadmin}
            </Badge>
          )}
        </div>
        <MenuSeparator />
        <MenuItem asChild>
          <Link href={`/${locale}/account`}>
            <UserRound aria-hidden="true" />
            {labels.account}
          </Link>
        </MenuItem>
        <MenuItem onSelect={() => void logout()}>
          <LogOut aria-hidden="true" />
          {labels.logout}
        </MenuItem>
      </MenuContent>
    </Menu>
  );
}
