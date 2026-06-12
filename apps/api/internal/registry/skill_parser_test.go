package registry

import (
	"strings"
	"testing"
)

func TestParseSkillMD_basic(t *testing.T) {
	input := `---
name: my-skill
description: Does something useful
author: Nico
version: "1.0"
---

# My Skill
Body content in markdown...
`
	name, description, author, version, body, err := ParseSkillMD(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "my-skill" {
		t.Errorf("name: got %q, want %q", name, "my-skill")
	}
	if description != "Does something useful" {
		t.Errorf("description: got %q, want %q", description, "Does something useful")
	}
	if author != "Nico" {
		t.Errorf("author: got %q, want %q", author, "Nico")
	}
	if version != "1.0" {
		t.Errorf("version: got %q, want %q", version, "1.0")
	}
	if !strings.Contains(body, "Body content in markdown") {
		t.Errorf("body missing expected content, got: %q", body)
	}
}

func TestParseSkillMD_minimal(t *testing.T) {
	// Solo name es obligatorio.
	input := "---\nname: bare-minimum\n---\n\nAlgun body."
	name, description, author, version, body, err := ParseSkillMD(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "bare-minimum" {
		t.Errorf("name: got %q, want %q", name, "bare-minimum")
	}
	if description != "" {
		t.Errorf("description should be empty, got %q", description)
	}
	if author != "" {
		t.Errorf("author should be empty, got %q", author)
	}
	if version != "" {
		t.Errorf("version should be empty, got %q", version)
	}
	if !strings.Contains(body, "Algun body") {
		t.Errorf("body missing expected content, got: %q", body)
	}
}

func TestParseSkillMD_missingFrontmatter(t *testing.T) {
	input := "# Just a markdown file\nNo frontmatter here."
	_, _, _, _, _, err := ParseSkillMD(input)
	if err == nil {
		t.Fatal("expected error for missing frontmatter, got nil")
	}
}

func TestParseSkillMD_unclosedFrontmatter(t *testing.T) {
	input := "---\nname: test\ndescription: no close"
	_, _, _, _, _, err := ParseSkillMD(input)
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter, got nil")
	}
}

func TestParseSkillMD_missingName(t *testing.T) {
	input := "---\ndescription: no name here\n---\n\nBody."
	_, _, _, _, _, err := ParseSkillMD(input)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
}

func TestParseSkillMD_singleQuoteVersion(t *testing.T) {
	input := "---\nname: quoted\nversion: '2.0'\n---\n\nContent."
	name, _, _, version, _, err := ParseSkillMD(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "quoted" {
		t.Errorf("name: got %q, want %q", name, "quoted")
	}
	if version != "2.0" {
		t.Errorf("version: got %q, want %q", version, "2.0")
	}
}

func TestParseSkillMD_CRLF(t *testing.T) {
	// Simular archivo con line endings de Windows.
	input := "---\r\nname: crlf-skill\r\ndescription: Windows line endings\r\n---\r\n\r\nBody."
	name, description, _, _, body, err := ParseSkillMD(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "crlf-skill" {
		t.Errorf("name: got %q, want %q", name, "crlf-skill")
	}
	if description != "Windows line endings" {
		t.Errorf("description: got %q, want %q", description, "Windows line endings")
	}
	if !strings.Contains(body, "Body") {
		t.Errorf("body missing expected content, got: %q", body)
	}
}
