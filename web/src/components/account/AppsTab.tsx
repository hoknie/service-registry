"use client";

import { useCallback } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { errorCode } from "@/lib/api";

import { useUiText } from "../UiText";
import { Panel } from "../ui/Panel";
import { AppList } from "./AppList";
import type { TokenLabels } from "./TokenTable";

type Props = { locale: Locale; labels: TokenLabels };

export function AppsTab({ locale, labels }: Props) {
  const { errors } = useUiText();
  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  return (
    <Panel title={labels.apps.title} description={labels.apps.lead} bodyClassName="p-0 sm:p-0">
      <AppList endpoint="/v1/account/apps" locale={locale} labels={labels} empty={labels.apps.empty} fail={fail} />
    </Panel>
  );
}
