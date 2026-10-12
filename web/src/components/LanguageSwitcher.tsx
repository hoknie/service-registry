"use client";

import { Check, Languages } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, type AnchorHTMLAttributes, type ComponentProps } from "react";

import { localeNames, locales, type Locale } from "@/i18n/config";
import { cn } from "@/lib/cn";
import { isDocPath } from "@/lib/docPath";

import { Button } from "./ui/Button";
import { Menu, MenuContent, MenuItem, MenuLabel, MenuTrigger } from "./ui/Menu";

function LocaleLink({ docs, prefetch, ...rest }: ComponentProps<typeof Link> & { docs: boolean; href: string }) {
  return docs ? <a {...(rest as AnchorHTMLAttributes<HTMLAnchorElement>)} /> : <Link prefetch={prefetch} {...rest} />;
}

export function LanguageSwitcher({ current, label }: { current: Locale; label: string }) {
  const routed = usePathname() ?? `/${current}`;
  const [here, setHere] = useState<{ pathname: string; search: string } | null>(null);
  const pathname = here?.pathname ?? routed;
  const search = here?.search ?? "";
  const docs = isDocPath(pathname);
  const rest = docs ? pathname.slice(1) : pathname.split("/").slice(2).join("/");

  return (
    <Menu onOpenChange={(open) => open && setHere({ pathname: window.location.pathname, search: window.location.search })}>
      <MenuTrigger asChild>
        <Button variant="ghost" size="sm" aria-label={`${label}: ${localeNames[current]}`}>
          <Languages aria-hidden="true" />
          <span lang={current} className="hidden sm:inline">
            {localeNames[current]}
          </span>
        </Button>
      </MenuTrigger>
      <MenuContent className="min-w-44">
        <MenuLabel>{label}</MenuLabel>
        {locales.map((locale) => (
          <MenuItem key={locale} asChild>
            <LocaleLink
              docs={docs}
              href={`/${locale}${rest ? `/${rest}` : ""}${search}`}
              hrefLang={locale}
              lang={locale}
              aria-current={locale === current ? "true" : undefined}
              prefetch={false}
              className={cn("pr-8", locale === current && "font-medium")}
            >
              {localeNames[locale]}
              {locale === current && <Check aria-hidden="true" className="ml-auto !text-signal" />}
            </LocaleLink>
          </MenuItem>
        ))}
      </MenuContent>
    </Menu>
  );
}
