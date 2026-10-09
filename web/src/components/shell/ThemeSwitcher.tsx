"use client";

import { Monitor, Moon, Sun } from "lucide-react";
import { useEffect, useSyncExternalStore } from "react";

import type { Messages } from "@/i18n/messages";
import { applyTheme, saveTheme, storedTheme, themes, watchSystemTheme, type Theme } from "@/lib/theme";

import { Button } from "../ui/Button";
import { Menu, MenuContent, MenuLabel, MenuRadioGroup, MenuRadioItem, MenuTrigger } from "../ui/Menu";

const EVENT = "svcr-theme-change";
const icons = { light: Sun, dark: Moon, system: Monitor } as const;

function subscribe(onChange: () => void) {
  window.addEventListener(EVENT, onChange);
  window.addEventListener("storage", onChange);
  return () => {
    window.removeEventListener(EVENT, onChange);
    window.removeEventListener("storage", onChange);
  };
}

export function useTheme(): [Theme, (theme: Theme) => void] {
  const theme = useSyncExternalStore(subscribe, storedTheme, () => "system" as Theme);
  const set = (next: Theme) => {
    saveTheme(next);
    window.dispatchEvent(new Event(EVENT));
  };
  return [theme, set];
}

export function ThemeSwitcher({ labels }: { labels: Messages["header"]["theme"] }) {
  const [theme, setTheme] = useTheme();

  useEffect(() => {
    applyTheme(theme);
    if (theme !== "system") return;
    return watchSystemTheme(() => applyTheme("system"));
  }, [theme]);

  const Icon = icons[theme];
  return (
    <Menu>
      <MenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={`${labels.label}: ${labels[theme]}`}>
          <Icon aria-hidden="true" />
        </Button>
      </MenuTrigger>
      <MenuContent className="min-w-40">
        <MenuLabel>{labels.label}</MenuLabel>
        <MenuRadioGroup value={theme} onValueChange={(value) => setTheme(value as Theme)}>
          {themes.map((t) => {
            const ItemIcon = icons[t];
            return (
              <MenuRadioItem key={t} value={t}>
                <ItemIcon aria-hidden="true" />
                {labels[t]}
              </MenuRadioItem>
            );
          })}
        </MenuRadioGroup>
      </MenuContent>
    </Menu>
  );
}
