import { apiGet, apiSend } from "./api";
import {
  SKILLS_BASE,
  skillUrl,
  type Skill,
  type SkillFolder,
  type SkillPolicy,
  type SkillVersion,
} from "./skills";

export const loadSkill = (id: string) => apiGet<Skill>(skillUrl(id));

export const loadFolder = (id: string) =>
  apiGet<SkillFolder>(`${SKILLS_BASE}/folders/${encodeURIComponent(id)}`);

export const loadFolders = (parent: string | null) =>
  apiGet<{ items: SkillFolder[] }>(
    `${SKILLS_BASE}/folders${parent ? `?parent_id=${encodeURIComponent(parent)}` : ""}`,
  );

export const loadVersions = (id: string, preview?: SkillPolicy) => {
  const q = new URLSearchParams();
  if (preview?.range) q.set("range", preview.range);
  if (preview?.prerelease !== null && preview?.prerelease !== undefined)
    q.set("prerelease", String(preview.prerelease));
  for (const v of preview?.exclude ?? []) q.append("exclude", v);
  const s = q.toString();
  return apiGet<{ items: SkillVersion[] }>(
    `${skillUrl(id, "/versions")}${s ? `?${s}` : ""}` as `/${string}`,
  );
};

export const patchSkill = (id: string, body: Record<string, unknown>) =>
  apiSend<Skill>("PATCH", skillUrl(id), body);
