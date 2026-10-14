import type { Suggestion } from "./labelSuggest";

export function branchSuggestions(names: string[], draft: string, def: string | null, defaultLabel: string, limit = 20): Suggestion[] {
  const q = draft.trim().toLowerCase();
  const seen = new Set<string>();
  const ordered = def && names.includes(def) ? [def, ...names] : names;
  const out: Suggestion[] = [];
  for (const name of ordered) {
    if (seen.has(name) || (q && !name.toLowerCase().startsWith(q))) continue;
    seen.add(name);
    out.push(name === def ? { value: name, label: name, detail: defaultLabel } : { value: name, label: name });
    if (out.length >= limit) break;
  }
  return out;
}
