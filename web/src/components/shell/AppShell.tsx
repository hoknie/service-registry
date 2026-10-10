"use client";

import { ChevronRight, Menu as MenuIcon, PanelLeftClose, PanelLeftOpen, Search, X } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Dialog as D } from "radix-ui";
import { useEffect, useState, useSyncExternalStore, type ReactNode } from "react";
import { Toaster } from "sonner";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { cn } from "@/lib/cn";
import { saveSidebar, sidebarCollapsed, watchSidebar } from "@/lib/sidebar";

import { LanguageSwitcher } from "../LanguageSwitcher";
import { useSession } from "../SessionProvider";
import { Button } from "../ui/Button";
import { Kbd } from "../ui/Kbd";
import { Tooltip } from "../ui/Tooltip";
import { UserMenu } from "../UserMenu";
import { BrandMark } from "./BrandMark";
import { CommandPalette } from "./CommandPalette";
import { CrumbMenu } from "./CrumbMenu";
import { CrumbsProvider, useCrumbTrail } from "./crumbs";
import { isActive, navItems, type NavItem } from "./nav";
import { useShortcutLabel } from "./shortcut";
import { ThemeSwitcher, useTheme } from "./ThemeSwitcher";

type Labels = { header: Messages["header"]; palette: Messages["palette"] };

function useSidebarCollapsed(): boolean {
  return useSyncExternalStore(watchSidebar, sidebarCollapsed, () => false);
}

function toggleSidebar() {
  saveSidebar(sidebarCollapsed() ? "expanded" : "collapsed");
}

function typing(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName);
}

export function AppShell({ locale, labels, children }: { locale: Locale; labels: Labels; children: ReactNode }) {
  const { session } = useSession();
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [theme] = useTheme();
  const shortcut = useShortcutLabel();
  const h = labels.header;
  const signedIn = session.status === "signed-in";
  const items = navItems(locale, h.nav, signedIn && session.user.is_superadmin);

  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === "b" && !typing(e.target)) {
        e.preventDefault();
        toggleSidebar();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <CrumbsProvider>
      <a
        href="#main"
        className="sr-only z-[60] rounded-md bg-primary px-3 py-2 text-primary-ink focus:not-sr-only focus:fixed focus:top-3 focus:left-3"
      >
        {h.skip}
      </a>
      {signedIn ? (
        <div className="min-h-dvh transition-[grid-template-columns] duration-200 ease-out lg:grid lg:grid-cols-[var(--sidebar-w)_minmax(0,1fr)]">
          <aside className="sticky top-0 hidden h-dvh overflow-hidden border-r border-line bg-surface/80 backdrop-blur-xl lg:block">
            <Sidebar locale={locale} items={items} labels={h} onSearch={() => setPaletteOpen(true)} rail />
          </aside>
          <div className="flex min-w-0 flex-col">
            <header className="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-line/80 bg-canvas/70 px-3 backdrop-blur-xl backdrop-saturate-150 supports-[not(backdrop-filter:blur(1px))]:bg-canvas sm:px-6">
              <MobileNav locale={locale} items={items} labels={h} onSearch={() => setPaletteOpen(true)} />
              <Breadcrumbs label={h.crumbs} locale={locale} navigator={h.navigator} />
              <div className="ml-auto flex items-center gap-1">
                <Button
                  variant="secondary"
                  size="sm"
                  className="hidden w-56 justify-start text-muted md:inline-flex"
                  onClick={() => setPaletteOpen(true)}
                >
                  <Search aria-hidden="true" />
                  <span className="flex-1 text-left font-normal">{h.search}</span>
                  <Kbd>{shortcut}</Kbd>
                </Button>
                <Button variant="ghost" size="icon" className="md:hidden" aria-label={h.search} onClick={() => setPaletteOpen(true)}>
                  <Search aria-hidden="true" />
                </Button>
                <LanguageSwitcher current={locale} label={h.language} />
                <ThemeSwitcher labels={h.theme} />
                <UserMenu locale={locale} nav={h.nav} labels={h.user} />
              </div>
            </header>
            <main id="main" tabIndex={-1} className="mx-auto w-full max-w-[84rem] min-w-0 flex-1 px-4 py-6 focus:outline-none sm:px-8 sm:py-8">
              {children}
            </main>
          </div>
          <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} locale={locale} pages={items} labels={labels.palette} />
        </div>
      ) : (
        <div className="flex min-h-dvh flex-col">
          <header className="flex h-14 items-center gap-2 px-4 sm:px-6">
            <Link href={`/${locale}`} className="flex items-center gap-2.5 rounded-md font-semibold text-ink">
              <BrandMark className="size-7" />
              <span>{h.brand}</span>
            </Link>
            <div className="ml-auto flex items-center gap-1">
              <LanguageSwitcher current={locale} label={h.language} />
              <ThemeSwitcher labels={h.theme} />
              <UserMenu locale={locale} nav={h.nav} labels={h.user} />
            </div>
          </header>
          <main id="main" tabIndex={-1} className="flex min-w-0 flex-1 flex-col px-4 pb-10 focus:outline-none sm:px-6">
            {children}
          </main>
        </div>
      )}
      <Toaster theme={theme} position="bottom-right" closeButton duration={5000} />
    </CrumbsProvider>
  );
}

function Sidebar({
  locale,
  items,
  labels,
  onSearch,
  onNavigate,
  rail,
}: {
  locale: Locale;
  items: NavItem[];
  labels: Messages["header"];
  onSearch: () => void;
  onNavigate?: () => void;
  rail?: boolean;
}) {
  const pathname = usePathname() ?? `/${locale}`;
  const shortcut = useShortcutLabel();
  const collapsed = useSidebarCollapsed() && !!rail;
  const tip = (label: string) => (collapsed ? label : null);
  const main = items.filter((i) => !i.admin);
  const admin = items.filter((i) => i.admin);

  const link = (item: NavItem) => {
    const active = isActive(pathname, item.href, locale);
    return (
      <li key={item.key}>
        <Tooltip content={tip(item.label)} side="right">
          <Link
            href={item.href}
            onClick={onNavigate}
            aria-current={active ? "page" : undefined}
            className={cn(
              "motion-control relative flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium rail:justify-center rail:px-0",
              active
                ? "bg-gradient-to-r from-signal-soft to-transparent text-ink shadow-elev-1 ring-1 ring-line ring-inset"
                : "text-ink-2 hover:bg-surface-2 hover:text-ink",
            )}
          >
            {active && <span aria-hidden="true" className="absolute inset-y-1.5 -left-3 w-[3px] rounded-r bg-signal rail:-left-2" />}
            <item.icon aria-hidden="true" className={cn("size-4 shrink-0", active ? "text-signal" : "text-muted")} />
            <span className="truncate rail:sr-only">{item.label}</span>
          </Link>
        </Tooltip>
      </li>
    );
  };

  return (
    <div className="flex h-full flex-col">
      <Link
        href={`/${locale}`}
        onClick={onNavigate}
        className="mx-3 mt-3 mb-4 flex items-center gap-2.5 rounded-md px-2 py-2 font-semibold text-ink rail:justify-center rail:px-0"
      >
        <BrandMark className="size-7 shrink-0" />
        <span className="truncate leading-tight rail:sr-only">{labels.brand}</span>
      </Link>
      <nav aria-label={labels.nav.label} className="flex-1 overflow-x-hidden overflow-y-auto px-3">
        <ul className="grid gap-0.5">{main.map(link)}</ul>
      </nav>
      <div className="grid gap-0.5 border-t border-line p-3">
        {admin.length > 0 && (
          <nav aria-label={labels.nav.admin}>
            <ul className="grid gap-0.5">{admin.map(link)}</ul>
          </nav>
        )}
        <Tooltip content={tip(labels.search)} side="right">
          <button
            type="button"
            onClick={onSearch}
            className="motion-control flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted hover:bg-surface-2 hover:text-ink rail:justify-center rail:px-0"
          >
            <Search aria-hidden="true" className="size-4 shrink-0" />
            <span className="flex-1 text-left rail:sr-only">{labels.search}</span>
            <Kbd className="rail:hidden">{shortcut}</Kbd>
          </button>
        </Tooltip>
        {rail && (
          <Tooltip content={tip(labels.sidebar.expand)} side="right">
            <button
              type="button"
              onClick={toggleSidebar}
              aria-expanded={!collapsed}
              className="motion-control flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-muted hover:bg-surface-2 hover:text-ink rail:justify-center rail:px-0"
            >
              {collapsed ? (
                <PanelLeftOpen aria-hidden="true" className="size-4 shrink-0" />
              ) : (
                <PanelLeftClose aria-hidden="true" className="size-4 shrink-0" />
              )}
              <span className="flex-1 text-left rail:sr-only">{collapsed ? labels.sidebar.expand : labels.sidebar.collapse}</span>
            </button>
          </Tooltip>
        )}
      </div>
    </div>
  );
}

function MobileNav({
  locale,
  items,
  labels,
  onSearch,
}: {
  locale: Locale;
  items: NavItem[];
  labels: Messages["header"];
  onSearch: () => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <D.Root open={open} onOpenChange={setOpen}>
      <D.Trigger asChild>
        <Button variant="ghost" size="icon" className="lg:hidden" aria-label={labels.nav.open}>
          <MenuIcon aria-hidden="true" />
        </Button>
      </D.Trigger>
      <D.Portal>
        <D.Overlay className="fixed inset-0 z-50 animate-fade-in bg-overlay lg:hidden" />
        <D.Content className="fixed inset-y-0 left-0 z-50 w-72 max-w-[85vw] animate-slide-in border-r border-line bg-surface shadow-pop focus:outline-none lg:hidden">
          <D.Title className="sr-only">{labels.nav.label}</D.Title>
          <D.Description className="sr-only">{labels.brand}</D.Description>
          <D.Close asChild>
            <Button variant="ghost" size="icon" className="absolute top-4 right-3" aria-label={labels.nav.close}>
              <X aria-hidden="true" />
            </Button>
          </D.Close>
          <Sidebar
            locale={locale}
            items={items}
            labels={labels}
            onNavigate={() => setOpen(false)}
            onSearch={() => {
              setOpen(false);
              onSearch();
            }}
          />
        </D.Content>
      </D.Portal>
    </D.Root>
  );
}

function Breadcrumbs({ label, locale, navigator }: { label: string; locale: Locale; navigator: Messages["header"]["navigator"] }) {
  const crumbs = useCrumbTrail();
  if (crumbs.length === 0) return null;
  return (
    <nav aria-label={label} className="min-w-0">
      <ol className="flex min-w-0 items-center gap-1 text-sm">
        {crumbs.map((c, i) => {
          const last = i === crumbs.length - 1;
          return (
            <li
              key={`${i}-${c.label}`}
              className={cn("flex min-w-0 items-center gap-1", !last && i < crumbs.length - 2 && "hidden sm:flex")}
            >
              {i > 0 && <ChevronRight aria-hidden="true" className="size-3.5 shrink-0 text-muted" />}
              {c.href && !last ? (
                <Link href={c.href} className="motion-control truncate rounded px-1 text-muted hover:text-ink">
                  {c.label}
                </Link>
              ) : (
                <span aria-current={last ? "page" : undefined} className={cn("truncate px-1", last ? "font-medium text-ink" : "text-muted")}>
                  {c.label}
                </span>
              )}
              {c.node && <CrumbMenu node={c.node} name={c.label} last={last} locale={locale} labels={navigator} />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
