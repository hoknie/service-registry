import { apiGet, type Items, type LabelKey, type LabelValue } from "./api";
import { labelQuery, labelsPath, labelSuggestions, type Suggestion } from "./labelSuggest";

export async function suggestLabels(draft: string, signal?: AbortSignal): Promise<Suggestion[]> {
  const q = labelQuery(draft);
  if (q.key === "") return [];
  if (q.key === null) return labelSuggestions(q, (await apiGet<Items<LabelKey>>(labelsPath(q), signal)).items, null);
  return labelSuggestions(q, null, (await apiGet<Items<LabelValue>>(labelsPath(q), signal)).items);
}
