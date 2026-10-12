"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { AnchorHTMLAttributes } from "react";

import { isDocPath } from "@/lib/docPath";

import { DocLink } from "../docs/DocLink";

type Props = AnchorHTMLAttributes<HTMLAnchorElement> & { href: string };

export function NavAnchor({ href, ...rest }: Props) {
  const pathname = usePathname() ?? "";
  if (isDocPath(href)) {
    return isDocPath(pathname) ? <DocLink href={href} {...rest} /> : <a href={href} {...rest} />;
  }
  return <Link href={href} {...rest} />;
}
