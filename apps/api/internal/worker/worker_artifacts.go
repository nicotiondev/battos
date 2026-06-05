package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func (w *Worker) writeManagedArtifact(relativePath, content string) error {
	root, err := filepath.Abs(w.ArtifactsDir)
	if err != nil {
		return fmt.Errorf("artifacts root: %w", err)
	}
	target := filepath.Join(root, filepath.FromSlash(relativePath))
	if !pathWithin(root, target) {
		return fmt.Errorf("artifact path outside root")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("artifact mkdir: %w", err)
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		return fmt.Errorf("artifact write: %w", err)
	}
	return nil
}

func pathWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative == "." || (!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != "..")
}

func managedArtifactFilename(name, kind string) string {
	return fmt.Sprintf("%s-%s%s", time.Now().UTC().Format("20060102T150405"), safePathSegment(name), artifactExtension(kind))
}

func artifactExtension(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "image":
		return ".bin"
	case "link":
		return ".url"
	case "diff":
		return ".diff"
	case "build_report":
		return ".md"
	default:
		return ".md"
	}
}

var unsafePathChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safePathSegment(value string) string {
	cleaned := strings.Trim(unsafePathChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(value)), "-"), ".-")
	if cleaned == "" {
		return "artifact"
	}
	return cleaned
}
