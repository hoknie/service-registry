"use client";

import { Check, Languages } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";

import { localeNames, locales, type Locale } from "@/i18n/config";
import { cn } from "@/lib/cn";

import { Button } from "./ui/Button";
import { Menu, MenuContent, MenuItem, MenuLabel, MenuTrigger } from "./ui/Menu";

export function LanguageSwitcher({ current, label }: { current: Locale; label: string }) {
  const pathname = usePathname() ?? `/${current}`;
  const rest = pathname.split("/").slice(2).join("/");
  const [search, setSearch] = useState("");

  return (
    <Menu onOpenChange={(open) => open && setSearch(window.location.search)}>
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
            <Link
              href={`/${locale}${rest ? `/${rest}` : ""}${search}`}
              hrefLang={locale}
              lang={locale}
              aria-current={locale === current ? "true" : undefined}
              prefetch={false}
              className={cn("pr-8", locale === current && "font-medium")}
            >
              {localeNames[locale]}
              {locale === current && <Check aria-hidden="true" className="ml-auto !text-signal" />}
            </Link>
          </MenuItem>
        ))}
      </MenuContent>
    </Menu>
  );
}
