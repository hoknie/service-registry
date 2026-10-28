package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	SkillsStorageLocal = "local"
	SkillsStorageS3    = "s3"
	SkillsStorageOff   = "off"
)

var s3Bucket = regexp.MustCompile(`^[a-z0-9.-]{3,63}$`)

type SkillsConfig struct {
	Enabled          bool   `env:"SKILLS_ENABLED" envDefault:"true"`
	Storage          string `env:"SKILLS_STORAGE" envDefault:"local"`
	DirRaw           string `env:"SKILLS_DIR" envDefault:"data/skills"`
	S3Endpoint       string `env:"SKILLS_S3_ENDPOINT"`
	S3Region         string `env:"SKILLS_S3_REGION" envDefault:"us-east-1"`
	S3Bucket         string `env:"SKILLS_S3_BUCKET"`
	S3Prefix         string `env:"SKILLS_S3_PREFIX" envDefault:"skills/"`
	S3AccessKeyRaw   string `env:"SKILLS_S3_ACCESS_KEY"`
	S3SecretKeyRaw   string `env:"SKILLS_S3_SECRET_KEY"`
	S3PathStyle      bool   `env:"SKILLS_S3_PATH_STYLE" envDefault:"false"`
	MaxUploadBytes   int64  `env:"SKILLS_MAX_UPLOAD_BYTES" envDefault:"10485760"`
	MaxTotalBytes    int64  `env:"SKILLS_MAX_TOTAL_BYTES" envDefault:"52428800"`
	MaxFileBytes     int64  `env:"SKILLS_MAX_FILE_BYTES" envDefault:"5242880"`
	MaxFiles         int64  `env:"SKILLS_MAX_FILES" envDefault:"500"`
	KeepVersions     int64  `env:"SKILLS_KEEP_VERSIONS" envDefault:"0"`
	SyncIntervalSecs int64  `env:"SKILLS_SYNC_INTERVAL_SECS" envDefault:"3600"`
	Dir              string `env:"-"`
	S3AccessKey      string `env:"-"`
	S3SecretKey      string `env:"-"`
}

type S3Options struct {
	Endpoint  string
	Secure    bool
	Region    string
	Bucket    string
	Prefix    string
	AccessKey string
	SecretKey string
	PathStyle bool
}

type skillsRange struct {
	name     string
	value    int64
	min, max int64
}

func (c SkillsConfig) SyncInterval() time.Duration {
	return time.Duration(c.SyncIntervalSecs) * time.Second
}

func (c *SkillsConfig) check() Errors {
	if !c.Enabled {
		return nil
	}
	errs := c.checkRanges()
	switch c.Storage {
	case SkillsStorageOff:
	case SkillsStorageLocal:
		errs = append(errs, c.checkDir()...)
	case SkillsStorageS3:
		errs = append(errs, c.checkDir()...)
		errs = append(errs, c.CheckS3()...)
	default:
		errs = append(errs, &Error{Var: "SKILLS_STORAGE", Reason: fmt.Sprintf("must be local, s3 or off, got %q", c.Storage)})
	}
	return errs
}

func (c *SkillsConfig) checkRanges() Errors {
	var errs Errors
	for _, r := range []skillsRange{
		{"SKILLS_MAX_UPLOAD_BYTES", c.MaxUploadBytes, 65536, 268435456},
		{"SKILLS_MAX_TOTAL_BYTES", c.MaxTotalBytes, 65536, 1073741824},
		{"SKILLS_MAX_FILE_BYTES", c.MaxFileBytes, 1024, 104857600},
		{"SKILLS_MAX_FILES", c.MaxFiles, 1, 10000},
		{"SKILLS_KEEP_VERSIONS", c.KeepVersions, 0, 1000},
		{"SKILLS_SYNC_INTERVAL_SECS", c.SyncIntervalSecs, 300, 86400},
	} {
		if r.value < r.min || r.value > r.max {
			errs = append(errs, &Error{Var: r.name, Reason: fmt.Sprintf("must be from %d to %d, got %d", r.min, r.max, r.value)})
		}
	}
	return errs
}

func (c *SkillsConfig) checkDir() Errors {
	if slices.Contains(strings.Split(filepath.ToSlash(c.DirRaw), "/"), "..") {
		return Errors{{Var: "SKILLS_DIR", Reason: "must not contain .."}}
	}
	c.Dir = filepath.Clean(c.DirRaw)
	return nil
}

func (c *SkillsConfig) CheckS3() Errors {
	var errs Errors
	if c.S3Endpoint == "" {
		errs = append(errs, &Error{Var: "SKILLS_S3_ENDPOINT", Reason: "is required by SKILLS_STORAGE=s3"})
	} else if _, _, err := s3Endpoint(c.S3Endpoint); err != nil {
		errs = append(errs, &Error{Var: "SKILLS_S3_ENDPOINT", Reason: err.Error()})
	}
	if c.S3Bucket == "" {
		errs = append(errs, &Error{Var: "SKILLS_S3_BUCKET", Reason: "is required by SKILLS_STORAGE=s3"})
	} else if !s3Bucket.MatchString(c.S3Bucket) {
		errs = append(errs, &Error{Var: "SKILLS_S3_BUCKET", Reason: "must be 3 to 63 of a-z, 0-9, . and -"})
	}
	if n := len(c.S3Region); n < 1 || n > 64 {
		errs = append(errs, &Error{Var: "SKILLS_S3_REGION", Reason: "must be 1 to 64 characters"})
	}
	if len(c.S3Prefix) > 200 || strings.HasPrefix(c.S3Prefix, "/") {
		errs = append(errs, &Error{Var: "SKILLS_S3_PREFIX", Reason: "must be at most 200 characters without a leading /"})
	}
	return append(errs, c.checkKeys()...)
}

func (c *SkillsConfig) checkKeys() Errors {
	access, secret := strings.TrimSpace(c.S3AccessKeyRaw), strings.TrimSpace(c.S3SecretKeyRaw)
	switch {
	case access == "" && secret == "":
		return nil
	case access == "":
		return Errors{{Var: "SKILLS_S3_ACCESS_KEY", Reason: "is required with SKILLS_S3_SECRET_KEY"}}
	case secret == "":
		return Errors{{Var: "SKILLS_S3_SECRET_KEY", Reason: "is required with SKILLS_S3_ACCESS_KEY"}}
	}
	var errs Errors
	var err error
	if c.S3AccessKey, err = secretValue(access); err != nil {
		errs = append(errs, &Error{Var: "SKILLS_S3_ACCESS_KEY", Reason: "reference cannot be resolved: " + err.Error()})
	}
	if c.S3SecretKey, err = secretValue(secret); err != nil {
		errs = append(errs, &Error{Var: "SKILLS_S3_SECRET_KEY", Reason: "reference cannot be resolved: " + err.Error()})
	}
	return errs
}

func secretValue(raw string) (string, error) {
	if strings.HasPrefix(raw, "env:") || strings.HasPrefix(raw, "file:") {
		return ResolveSecretRef(raw)
	}
	return raw, nil
}

func s3Endpoint(raw string) (string, bool, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", false, fmt.Errorf("must be https://host[:port] without path, query or credentials")
	}
	switch {
	case u.Scheme == "https":
		return u.Host, true, nil
	case u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"):
		return u.Host, false, nil
	}
	return "", false, fmt.Errorf("must use https (http only for localhost)")
}

func (c SkillsConfig) S3() S3Options {
	host, secure, _ := s3Endpoint(c.S3Endpoint)
	return S3Options{Endpoint: host, Secure: secure, Region: c.S3Region, Bucket: c.S3Bucket, Prefix: c.S3Prefix,
		AccessKey: c.S3AccessKey, SecretKey: c.S3SecretKey, PathStyle: c.S3PathStyle}
}
