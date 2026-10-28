import { ApiError, apiGet, apiHead, apiSend } from "./api";
import {
  SKILLS_BASE,
  skillUrl,
  type Skill,
  type SkillFolder,
  archiveErrorCode,
  type SkillPolicy,
  type SkillSet,
  type SkillSource,
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

export const patchFolder = (id: string, body: Record<string, unknown>) =>
  apiSend<SkillFolder>("PATCH", `${SKILLS_BASE}/folders/${encodeURIComponent(id)}`, body);

export const importSource = (body: Record<string, unknown>) =>
  apiSend<{ folder: SkillFolder; source: SkillSource }>("POST", `${SKILLS_BASE}/imports`, body);

export const loadSkillSet = (org: string) => apiGet<SkillSet>(`${SKILLS_BASE}/organizations/${encodeURIComponent(org)}/set`);

export const putSkillSet = (org: string, body: { mode: string; folders: string[]; skills: string[] }) =>
  apiSend<SkillSet>("PUT", `${SKILLS_BASE}/organizations/${encodeURIComponent(org)}/set`, body);

export async function folderArchiveCount(id: string): Promise<number> {
  try {
    const h = await apiHead(`${SKILLS_BASE}/folders/${encodeURIComponent(id)}/archive`);
    return Number(h.get("X-Skills-Count") ?? "0");
  } catch (e) {
    if (e instanceof ApiError && e.status > 0) throw new ApiError(e.status, archiveErrorCode(e.status), e.message);
    throw e;
  }
}
