"use client";

import { Globe } from "lucide-react";
import { useEffect, useState } from "react";

import { apiGet, type Tree } from "@/lib/api";
import { cn } from "@/lib/cn";

import { KindIcon } from "../catalog/KindIcon";

export type SpaceRef = { id: string; slug: string; name: string };

export function useOrganizations(): SpaceRef[] {
  const [orgs, setOrgs] = useState<SpaceRef[]>([]);
  useEffect(() => {
    apiGet<Tree>("/v1/catalog/tree?depth=1")
      .then((t) => setOrgs(t.nodes.filter((n) => n.parent_id === null).map((n) => ({ id: n.id, slug: n.slug, name: n.name }))))
      .catch(() => setOrgs([]));
  }, []);
  return orgs;
}

export function SpaceIcon({ space, size, className }: { space: string; size: "sm" | "lg" | "xs"; className?: string }) {
  const box = size === "lg" ? "size-14 rounded-2xl shadow-elev-1 [&>svg]:size-7" : size === "sm" ? "size-9 rounded-lg [&>svg]:size-4" : "";
  if (space === "global") {
    return (
      <span aria-hidden="true" className={cn("grid shrink-0 place-items-center bg-cobalt-soft text-cobalt ring-1 ring-line ring-inset", box || "size-4 rounded-sm [&>svg]:size-3", className)}>
        <Globe />
      </span>
    );
  }
  return size === "xs" ? <KindIcon kind="organization" className={className} /> : <KindIcon kind="organization" tile className={cn(box, className)} />;
}
