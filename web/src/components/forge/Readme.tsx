"use client";

import { ExternalLink } from "lucide-react";
import { useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import { apiGet, errorCode, type Readme as ReadmeData } from "@/lib/api";

import { useUiText } from "../UiText";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonPanel } from "../ui/Skeleton";
import type { CatalogLabels } from "../catalog/shared";

type Props = { projectId: string; labels: CatalogLabels["repoMeta"] };

export function Readme({ projectId, labels: t }: Props) {
  const { errors, common } = useUiText();
  const [html, setHtml] = useState<string | null>(null);
  const [data, setData] = useState<ReadmeData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    Promise.all([apiGet<ReadmeData>(`/v1/catalog/nodes/${projectId}/readme`), import("@/lib/readme")])
      .then(([readme, { renderReadme }]) => {
        if (!live) return;
        setData(readme);
        setHtml(renderReadme(readme.markdown, { webUrl: readme.web_url, branch: readme.default_branch, kind: readme.kind }));
      })
      .catch((e: unknown) => live && setError(errorText(errors, errorCode(e))));
    return () => {
      live = false;
    };
  }, [projectId, errors]);

  return (
    <Panel title={t.readme}>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!html && !error && <SkeletonPanel lines={4} label={common.loading} />}
      {html && (
        <>
          <div className="readme max-w-none animate-rise-in text-sm leading-relaxed text-ink-2" dangerouslySetInnerHTML={{ __html: html }} />
          {data?.truncated && (
            <p className="mt-4 text-sm text-muted">
              {t.readmeTruncated}{" "}
              <a href={data.web_url} target="_blank" rel="noopener nofollow" className="inline-flex items-center gap-1 text-signal hover:underline">
                {t.readmeFull}
                <ExternalLink aria-hidden="true" className="size-3.5" />
              </a>
            </p>
          )}
        </>
      )}
    </Panel>
  );
}
