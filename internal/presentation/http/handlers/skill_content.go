package handlers

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	accesstypes "svc-registry/internal/feature/access/types"
	skillsservices "svc-registry/internal/feature/skills/services"
	skillstypes "svc-registry/internal/feature/skills/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) skillUpload(c fiber.Ctx) (requests.SkillUpload, error) {
	up, err := requests.ParseSkillUpload(c.Get(fiber.HeaderContentType), c.Body(), a.Config.Skills.MaxUploadBytes)
	if errors.Is(err, requests.ErrNotMultipart) {
		return up, InvalidBody(err.Error())
	}
	return up, err
}

func optionalID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, InvalidBody("folder_id and organization_id are UUIDs")
	}
	return &id, nil
}

func (a *Handlers) CreateSkill(c fiber.Ctx) error {
	return a.skillCall(c, "", http.StatusCreated, func(p accesstypes.Principal, _ uuid.UUID) (any, error) {
		up, err := a.skillUpload(c)
		if err != nil {
			return nil, err
		}
		folder, err := optionalID(up.Fields["folder_id"])
		if err != nil {
			return nil, err
		}
		org, err := optionalID(up.Fields["organization_id"])
		if err != nil {
			return nil, err
		}
		up.Version = up.Fields["version"]
		out, err := a.Skills.Content.CreateSkill(c.Context(), p, skillsservices.CreateInput{Upload: up.Upload, FolderID: folder,
			OrganizationID: org, Status: up.Fields["status"]})
		return responses.SkillOf(out), err
	})
}

func (a *Handlers) UploadSkillVersion(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		up, err := a.skillUpload(c)
		if err != nil {
			return nil, err
		}
		up.Version = up.Fields["version"]
		out, err := a.Skills.Content.UploadVersion(c.Context(), p, id, up.Upload)
		return responses.SkillOf(out), err
	})
}

func (a *Handlers) SkillFiles(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Content.SkillFiles(c.Context(), p, id, c.Query("version"))
		return responses.SkillFilesOf(out), err
	})
}

func (a *Handlers) SkillFile(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		out, err := a.Skills.Content.SkillFile(c.Context(), p, id, c.Query("path"), c.Query("version"))
		return responses.SkillFileContentOf(out), err
	})
}

func sendSkillBytes(c fiber.Ctx, contentType, disposition string, data []byte) error {
	c.Set(fiber.HeaderContentType, contentType)
	c.Set(fiber.HeaderContentSecurityPolicy, "sandbox")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	c.Set(fiber.HeaderContentDisposition, disposition)
	return c.Status(http.StatusOK).Send(data)
}

func (a *Handlers) SkillRawFile(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	f, data, err := a.Skills.Content.RawFile(c.Context(), p, id, c.Query("path"), c.Query("version"))
	if err != nil {
		return responses.Fail(c, err)
	}
	contentType, disposition := "text/plain; charset=utf-8", "attachment"
	switch f.Kind {
	case skillstypes.KindImage:
		contentType, disposition = f.ContentType, "inline"
	case skillstypes.KindBinary:
		contentType = "application/octet-stream"
	}
	return sendSkillBytes(c, contentType, mime.FormatMediaType(disposition, map[string]string{"filename": baseName(f.Path)}), data)
}

func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

func (a *Handlers) SkillArchive(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	sel, files, err := a.Skills.Content.Archive(c.Context(), p, id, c.Query("version"), c.Method() == fiber.MethodGet)
	if err != nil {
		return responses.Fail(c, err)
	}
	name := sel.Skill.Name + "-" + sel.Version.Version + ".zip"
	c.Set(fiber.HeaderContentType, "application/zip")
	c.Set(fiber.HeaderContentDisposition, mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	c.Set("X-Skill-Version", sel.Version.Version)
	c.Status(http.StatusOK)
	prefix := sel.Skill.Name + "/"
	return c.SendStreamWriter(func(w *bufio.Writer) {
		z := zip.NewWriter(w)
		for _, f := range files {
			out, err := z.Create(prefix + f.Path)
			if err != nil {
				return
			}
			if _, err := out.Write(f.Data); err != nil {
				return
			}
		}
		_ = z.Close()
		_ = w.Flush()
	})
}

func (a *Handlers) SkillImage(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	contentType, data, err := a.Skills.Images.Image(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "private, max-age=300")
	return sendSkillBytes(c, contentType, "inline", data)
}

func (a *Handlers) PutSkillImage(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusOK, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		header, err := c.FormFile("image")
		if err != nil {
			return nil, InvalidBody("a multipart form with an image part is required")
		}
		if header.Size > skillstypes.MaxImageBytes {
			return nil, skillstypes.ImageTooLarge
		}
		f, err := header.Open()
		if err != nil {
			return nil, InvalidBody("the image cannot be read")
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, skillstypes.MaxImageBytes+1))
		if err != nil {
			return nil, InvalidBody("the image cannot be read")
		}
		out, err := a.Skills.Images.PutImage(c.Context(), p, id, data)
		return responses.SkillOf(out), err
	})
}

func (a *Handlers) DeleteSkillImage(c fiber.Ctx) error {
	return a.skillCall(c, "id", http.StatusNoContent, func(p accesstypes.Principal, id uuid.UUID) (any, error) {
		return nil, a.Skills.Images.DeleteImage(c.Context(), p, id)
	})
}

func (a *Handlers) SkillFolderArchive(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	head := c.Method() == fiber.MethodHead
	plan, err := a.Skills.Archive.FolderPlan(c.Context(), p, id, !head)
	if err != nil {
		return responses.Fail(c, err)
	}
	c.Set(fiber.HeaderContentType, "application/zip")
	c.Set(fiber.HeaderContentDisposition, mime.FormatMediaType("attachment", map[string]string{"filename": plan.Folder.Slug + ".zip"}))
	c.Set("X-Skills-Count", strconv.Itoa(len(plan.Skills)))
	c.Set("X-Skills-Skipped", strconv.Itoa(plan.Skipped))
	if head {
		return c.SendStatus(http.StatusOK)
	}
	if err := a.Skills.Archive.CountDownloads(c.Context(), plan); err != nil {
		return responses.Fail(c, err)
	}
	ctx := context.WithoutCancel(c.Context())
	c.Status(http.StatusOK)
	return c.SendStreamWriter(func(w *bufio.Writer) {
		z := zip.NewWriter(w)
		for _, sel := range plan.Skills {
			for _, f := range sel.Files {
				data, err := a.Skills.Archive.ReadFile(ctx, f)
				if err != nil {
					return
				}
				out, err := z.Create(sel.Skill.Name + "/" + f.Path)
				if err != nil {
					return
				}
				if _, err := out.Write(data); err != nil {
					return
				}
			}
		}
		_ = z.Close()
		_ = w.Flush()
	})
}
