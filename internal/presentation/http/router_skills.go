package http

import "svc-registry/internal/presentation/http/handlers"

func (r routes) skills(api *handlers.Handlers) {
	if api.Skills == nil {
		return
	}
	r.add("/api/v1/skills", true, get(api.ListSkills), post(api.CreateSkill))
	r.add("/api/v1/skills/tags", true, get(api.SkillTags))
	r.add("/api/v1/skills/folders", true, get(api.ListSkillFolders), post(api.CreateSkillFolder))
	r.add("/api/v1/skills/folders/:id", true, get(api.GetSkillFolder), patch(api.UpdateSkillFolder), del(api.DeleteSkillFolder))
	r.add("/api/v1/skills/folders/:id/sources", true, get(api.ListSkillSources), post(api.CreateSkillSource))
	r.add("/api/v1/skills/sources/:sid", true, get(api.GetSkillSource), patch(api.UpdateSkillSource), del(api.DeleteSkillSource))
	r.add("/api/v1/skills/sources/:sid/sync", true, post(api.SyncSkillSource))
	r.add("/api/v1/skills/organizations/:id", true, get(api.OrganizationSkillSpace), put(api.PutOrganizationSkillSpace))
	r.add("/api/v1/skills/organizations/:id/version-policy", true, get(api.OrganizationSkillPolicy), put(api.PutOrganizationSkillPolicy))
	r.add("/api/v1/skills/:id", true, get(api.GetSkill), patch(api.PatchSkill), del(api.DeleteSkill))
	r.add("/api/v1/skills/:id/content", true, put(api.UploadSkillVersion))
	r.add("/api/v1/skills/:id/files", true, get(api.SkillFiles))
	r.add("/api/v1/skills/:id/file", true, get(api.SkillFile))
	r.add("/api/v1/skills/:id/raw", true, get(api.SkillRawFile))
	r.add("/api/v1/skills/:id/archive", true, get(api.SkillArchive))
	r.add("/api/v1/skills/:id/image", true, get(api.SkillImage), put(api.PutSkillImage), del(api.DeleteSkillImage))
	r.add("/api/v1/skills/:id/versions", true, get(api.ListSkillVersions))
	r.add("/api/v1/skills/:id/versions/:version", true, put(api.YankSkillVersion))
	r.add("/api/v1/skills/:id/version-policy", true, get(api.SkillPolicy), put(api.PutSkillPolicy))
}

func skillUpload(method, path string) bool {
	if method == "POST" && path == "/api/v1/skills" {
		return true
	}
	rest, ok := cutSkillsPrefix(path)
	return ok && method == "PUT" && len(rest) > len("/content") && rest[len(rest)-len("/content"):] == "/content"
}

func cutSkillsPrefix(path string) (string, bool) {
	const prefix = "/api/v1/skills/"
	if len(path) <= len(prefix) || path[:len(prefix)] != prefix {
		return "", false
	}
	return path[len(prefix):], true
}
