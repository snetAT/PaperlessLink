package config

import (
	"errors"
	"strings"
	"time"
)

// AfterUpload defines what to do with a file after a successful upload.
type AfterUpload string

const (
	AfterUploadDelete AfterUpload = "delete"
	AfterUploadBackup AfterUpload = "backup"
)

// Config holds all runtime configuration for PaperlessLink.
type Config struct {
	WatchDir     string
	PaperlessURL string
	Token        string

	// AllowedExts is the set of lower-cased extensions (without leading dot)
	// that are accepted. Empty means all extensions are accepted.
	AllowedExts map[string]struct{}

	RenameToUUID bool
	AfterUpload  AfterUpload
	BackupDir    string

	LogFile      string
	PollInterval time.Duration
}

// Validate checks that required fields are present and combinations are valid.
func (c *Config) Validate() error {
	if c.WatchDir == "" {
		return errors.New("flag -dir is required")
	}
	if c.PaperlessURL == "" {
		return errors.New("flag -url is required")
	}
	if c.Token == "" {
		return errors.New("flag -token is required")
	}
	switch c.AfterUpload {
	case AfterUploadDelete, AfterUploadBackup:
	default:
		return errors.New("flag -after-upload must be 'delete' or 'backup'")
	}
	if c.AfterUpload == AfterUploadBackup && c.BackupDir == "" {
		return errors.New("flag -backup-dir is required when -after-upload=backup")
	}
	return nil
}

// ParseExtensions converts a comma-separated extension string (e.g. "pdf,png,jpg")
// into a normalised set: lowercase, no leading dot.
func ParseExtensions(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	if raw == "" {
		return result
	}
	for _, e := range strings.Split(raw, ",") {
		e = strings.TrimSpace(e)
		e = strings.ToLower(e)
		e = strings.TrimPrefix(e, ".")
		if e != "" {
			result[e] = struct{}{}
		}
	}
	return result
}
