import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { ago } from "@/i18n/time";
import type { DependencySource } from "@/lib/dependencies";

import { Badge, type Tone } from "../ui/Badge";
import { Tooltip } from "../ui/Tooltip";
import type { CatalogLabels } from "../catalog/shared";

const tones: Record<DependencySource["source"], Tone> = { event: "signal", cluster: "cobalt", repository: "amber", manual: "neutral" };

type Props = { sources: DependencySource[]; locale: Locale; labels: CatalogLabels };

export function SourceBadges({ sources, locale, labels }: Props) {
  const t = labels.dependencies;
  return (
    <span className="flex flex-wrap gap-1">
      {sources.map((s, i) => {
        const origin = s.source === "manual" ? (s.note ?? t.sources.manual) : s.origin;
        return (
          <Tooltip key={`${s.source}-${s.origin}-${i}`} content={format(t.seen, { origin, when: ago(s.seen_at, locale, "") })}>
            <Badge tone={tones[s.source]} tabIndex={0}>
              {t.sources[s.source]}
            </Badge>
          </Tooltip>
        );
      })}
    </span>
  );
}
