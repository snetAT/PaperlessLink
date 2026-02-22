// Package uploader handles uploading a file to Paperless-ngx via its
// POST /api/documents/post_document/ endpoint, and performs the configured
// post-upload action (delete or move to backup directory).
package uploader

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"paperlesslink/config"
)

// Upload uploads filePath to Paperless-ngx using the provided config and
// performs the configured post-upload action.
func Upload(cfg *config.Config, filePath string) error {
	slog.Info("starting upload", "file", filePath)

	// Resolve the actual file to upload (may be a UUID-named temp copy).
	uploadPath := filePath
	originalName := filepath.Base(filePath)

	if cfg.RenameToUUID {
		ext := filepath.Ext(filePath)
		uuidName := uuid.New().String() + ext
		uploadPath = filepath.Join(os.TempDir(), uuidName)
		if err := copyFile(filePath, uploadPath); err != nil {
			return fmt.Errorf("uuid copy: %w", err)
		}
		slog.Info("file copied with UUID name for upload",
			"uuid_name", uuidName,
			"original", originalName,
		)
		defer func() {
			if err := os.Remove(uploadPath); err != nil && !os.IsNotExist(err) {
				slog.Warn("could not remove temp uuid file", "path", uploadPath, "error", err)
			}
		}()
	}

	// Title = original filename stem (without extension).
	stem := strings.TrimSuffix(originalName, filepath.Ext(originalName))

	if err := postDocument(cfg, uploadPath, stem); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	slog.Info("upload successful", "file", filePath, "title", stem)
	return postUploadAction(cfg, filePath)
}

// postDocument performs the multipart POST to Paperless-ngx.
func postDocument(cfg *config.Config, filePath, title string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Build the multipart body in memory so we can set Content-Length.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	// --- document field -------------------------------------------------------
	// Use the correct MIME type for the file extension (same behaviour as curl -F @file).
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filePath)))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="document"; filename="%s"`, filepath.Base(filePath)))
	h.Set("Content-Type", mimeType)
	part, err := mw.CreatePart(h)
	if err != nil {
		return fmt.Errorf("create form file part: %w", err)
	}
	slog.Debug("document part mime type", "mime", mimeType)
	n, err := io.Copy(part, f)
	if err != nil {
		return fmt.Errorf("write file content to form: %w", err)
	}
	slog.Debug("file content written to form", "bytes", n)

	// --- title field ----------------------------------------------------------
	if err := mw.WriteField("title", title); err != nil {
		return fmt.Errorf("write title field: %w", err)
	}

	// Close MUST be called before reading body.Body (writes boundary epilogue).
	contentType := mw.FormDataContentType()
	if err := mw.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	endpoint := strings.TrimRight(cfg.PaperlessURL, "/") + "/api/documents/post_document/"
	slog.Debug("posting to paperless", "endpoint", endpoint, "title", title, "body_bytes", body.Len())

	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+cfg.Token)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "curl/7.81.0")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	slog.Info("paperless response", "status", resp.StatusCode, "body", string(respBody))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("paperless returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// postUploadAction deletes or backs up the original file after a successful upload.
func postUploadAction(cfg *config.Config, filePath string) error {
	switch cfg.AfterUpload {
	case config.AfterUploadDelete:
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("delete after upload: %w", err)
		}
		slog.Info("file deleted after upload", "file", filePath)

	case config.AfterUploadBackup:
		dst := filepath.Join(cfg.BackupDir, filepath.Base(filePath))
		if err := moveFile(filePath, dst); err != nil {
			return fmt.Errorf("backup after upload: %w", err)
		}
		slog.Info("file moved to backup", "src", filePath, "dst", dst)
	}
	return nil
}

// moveFile moves src to dst, falling back to copy+delete for cross-device moves.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// copyFile copies src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
