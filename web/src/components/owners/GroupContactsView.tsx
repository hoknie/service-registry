"use client";

import { ExternalLink, Siren } from "lucide-react";

import type { Contact } from "@/lib/api";
import { contactLink, contactText, webLink } from "@/lib/owners";

import { Button } from "../ui/Button";

type Props = { contacts: Contact[]; onCallUrl: string | null; labels: { contacts: string; onCall: string } };

export function GroupContactsView({ contacts, onCallUrl, labels }: Props) {
  const onCall = webLink(onCallUrl);
  return (
    <>
      {contacts.length > 0 && (
        <ul aria-label={labels.contacts} className="flex flex-wrap gap-x-3 gap-y-1 text-sm">
          {contacts.map((c, i) => {
            const link = contactLink(c);
            return (
              <li key={i} className="min-w-0">
                {link ? (
                  <a
                    href={link.href}
                    {...(link.external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
                    className="inline-flex max-w-full items-center gap-1 text-signal hover:underline"
                  >
                    <span className="truncate">{contactText(c)}</span>
                    {link.external && <ExternalLink aria-hidden="true" className="size-3 shrink-0" />}
                  </a>
                ) : (
                  <span className="text-ink-2">{contactText(c)}</span>
                )}
              </li>
            );
          })}
        </ul>
      )}
      {onCall && (
        <Button size="sm" asChild className="justify-self-start">
          <a href={onCall} target="_blank" rel="noopener noreferrer">
            <Siren aria-hidden="true" />
            {labels.onCall}
          </a>
        </Button>
      )}
    </>
  );
}
