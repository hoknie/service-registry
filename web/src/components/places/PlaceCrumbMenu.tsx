"use client";

import { Plug, Sparkles } from "lucide-react";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { placeEntries, type PlaceCrumb, type PlaceEntry } from "@/lib/placeCrumbs";
import { loadPlaceLevel } from "@/lib/placeCrumbsApi";

import { KindIcon } from "../catalog/KindIcon";
import { CrumbMenuView, type MenuEntry } from "../shell/CrumbMenu";
import { SpaceIcon } from "./spaces";

type Props = { place: PlaceCrumb; name: string; last: boolean; locale: Locale; labels: Messages["header"]["navigator"] };

function icon(e: PlaceEntry, section: PlaceCrumb["section"]) {
  if (e.kind === "space") return <SpaceIcon space={e.id} size="xs" />;
  if (e.kind === "folder") return <KindIcon kind="folder" />;
  const Icon = section === "skills" ? Sparkles : Plug;
  return <Icon aria-hidden="true" className="size-4 shrink-0 text-signal" />;
}

export function PlaceCrumbMenu({ place, name, last, locale, labels }: Props) {
  const [open, setOpen] = useState(false);
  const [state, setState] = useState<{ entries: MenuEntry[] | null; error: unknown }>({ entries: null, error: null });
  const key = JSON.stringify([place, last]);

  useEffect(() => {
    if (!open || state.entries) return;
    let live = true;
    loadPlaceLevel(place, last)
      .then((level) => {
        if (!live) return;
        const entries = placeEntries(place, last, level, locale).map((e) => ({ ...e, icon: icon(e, place.section) }));
        setState({ entries, error: null });
      })
      .catch((error: unknown) => live && setState({ entries: null, error }));
    return () => {
      live = false;
    };
  }, [open, state.entries, place, last, locale]);

  const [seenKey, setSeenKey] = useState(key);
  if (seenKey !== key) {
    setSeenKey(key);
    setState({ entries: null, error: null });
  }

  return (
    <CrumbMenuView
      open={open}
      onOpenChange={setOpen}
      current={place.id}
      name={name}
      entries={state.entries ?? []}
      loading={!state.entries && !state.error}
      error={state.error}
      labels={labels}
      top={labels.spaces}
    />
  );
}
