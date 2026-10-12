"use client";

import type { AnchorHTMLAttributes, MouseEvent } from "react";

import { navigateDoc } from "@/lib/docPath";

type Props = AnchorHTMLAttributes<HTMLAnchorElement> & { href: string };

export function DocLink({ href, onClick, ...rest }: Props) {
  const click = (e: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(e);
    if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
    e.preventDefault();
    navigateDoc(href);
  };
  return <a href={href} onClick={click} {...rest} />;
}
