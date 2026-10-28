package requests

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/url"
	"strings"

	skillsservices "svc-registry/internal/feature/skills/services"
	skillstypes "svc-registry/internal/feature/skills/types"
)

var ErrNotMultipart = errors.New("a multipart/form-data body with an archive part or file parts is required")

type SkillUpload struct {
	skillsservices.Upload
	Fields map[string]string
}

const UploadSlack = 1 << 20

func ParseSkillUpload(contentType string, body []byte, limit int64) (SkillUpload, error) {
	if int64(len(body)) > limit {
		return SkillUpload{}, skillstypes.UploadTooLarge
	}
	media, params, err := mime.ParseMediaType(contentType)
	if err != nil || media != "multipart/form-data" || params["boundary"] == "" {
		return SkillUpload{}, ErrNotMultipart
	}
	out := SkillUpload{Fields: map[string]string{}}
	r := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := r.NextRawPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return SkillUpload{}, skillstypes.ArchiveInvalid
		}
		if err := out.add(part, limit); err != nil {
			return SkillUpload{}, err
		}
	}
	if out.Archive == nil && len(out.Files) == 0 {
		return SkillUpload{}, ErrNotMultipart
	}
	return out, nil
}

func (u *SkillUpload) add(part *multipart.Part, limit int64) error {
	defer part.Close()
	_, params, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
	if err != nil {
		return skillstypes.ArchiveInvalid
	}
	data, err := io.ReadAll(io.LimitReader(part, limit+1))
	if err != nil || int64(len(data)) > limit {
		return skillstypes.UploadTooLarge
	}
	switch params["name"] {
	case "archive":
		u.Archive = data
	case "file":
		name := params["filename"]
		if decoded, err := url.PathUnescape(name); err == nil && strings.Contains(name, "%") {
			name = decoded
		}
		u.Files = append(u.Files, skillstypes.Entry{Path: name, Open: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		}})
	default:
		u.Fields[params["name"]] = string(data)
	}
	return nil
}
