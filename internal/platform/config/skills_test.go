package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"svc-registry/internal/platform/config"
)

func TestSkillsDefaults(t *testing.T) {
	t.Parallel()
	c, err := config.Load[config.SkillsConfig](lookup())
	if err != nil {
		t.Fatal(err)
	}
	if !c.Enabled || c.Storage != "local" || c.Dir != filepath.Clean("data/skills") || c.MaxUploadBytes != 10<<20 ||
		c.KeepVersions != 0 || c.SyncIntervalSecs != 3600 || c.MaxFiles != 500 {
		t.Fatalf("defaults %+v", c)
	}
}

func TestSkillsVariablesAreChecked(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		env  []string
		want []string
	}{
		{[]string{"SKILLS_STORAGE", "s3", "SKILLS_S3_ENDPOINT", "https://s3.example.com"}, []string{"SKILLS_S3_BUCKET"}},
		{[]string{"SKILLS_STORAGE", "s3", "SKILLS_S3_ENDPOINT", "https://s3.example.com", "SKILLS_S3_BUCKET", "skills",
			"SKILLS_S3_ACCESS_KEY", "key"}, []string{"SKILLS_S3_SECRET_KEY"}},
		{[]string{"SKILLS_STORAGE", "s3", "SKILLS_S3_ENDPOINT", "http://s3.example.com", "SKILLS_S3_BUCKET", "skills"}, []string{"SKILLS_S3_ENDPOINT"}},
		{[]string{"SKILLS_STORAGE", "s3", "SKILLS_S3_ENDPOINT", "https://s3.example.com/path", "SKILLS_S3_BUCKET", "Skills"},
			[]string{"SKILLS_S3_BUCKET", "SKILLS_S3_ENDPOINT"}},
		{[]string{"SKILLS_STORAGE", "disk"}, []string{"SKILLS_STORAGE"}},
		{[]string{"SKILLS_DIR", "../skills"}, []string{"SKILLS_DIR"}},
		{[]string{"SKILLS_ENABLED", "maybe"}, []string{"SKILLS_ENABLED"}},
		{[]string{"SKILLS_MAX_FILES", "0", "SKILLS_SYNC_INTERVAL_SECS", "60", "SKILLS_KEEP_VERSIONS", "1001"},
			[]string{"SKILLS_KEEP_VERSIONS", "SKILLS_MAX_FILES", "SKILLS_SYNC_INTERVAL_SECS"}},
	} {
		_, err := config.Load[config.SkillsConfig](lookup(c.env...))
		if got := vars(t, err); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%v: %v", c.env, got)
		}
	}
}

func TestDisabledSkillsSkipTheStorageChecks(t *testing.T) {
	t.Parallel()
	c, err := config.Load[config.SkillsConfig](lookup("SKILLS_ENABLED", "false", "SKILLS_STORAGE", "s3", "SKILLS_DIR", "../x"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled {
		t.Fatal("enabled")
	}
}

func TestSkillsS3KeysFromReferences(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(file, []byte("the-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load[config.SkillsConfig](lookup("SKILLS_STORAGE", "s3", "SKILLS_S3_ENDPOINT", "http://localhost:9010",
		"SKILLS_S3_BUCKET", "skills", "SKILLS_S3_ACCESS_KEY", "minio", "SKILLS_S3_SECRET_KEY", "file:"+file, "SKILLS_S3_PATH_STYLE", "true"))
	if err != nil {
		t.Fatal(err)
	}
	o := c.S3()
	if o.Endpoint != "localhost:9010" || o.Secure || o.AccessKey != "minio" || o.SecretKey != "the-secret" || !o.PathStyle || o.Prefix != "skills/" {
		t.Fatalf("options %+v", o)
	}
}
