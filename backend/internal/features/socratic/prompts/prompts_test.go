// Purpose: Unit tests for versioned Socratic system prompt loading and interpolation.
// Architecture: backend/internal/features/socratic/prompts/prompts_test.go
package prompts_test

import (
	"strings"
	"testing"

	"github.com/Joshua-Omz/wordivate/backend/internal/features/socratic/prompts"
)

func TestRender_V1(t *testing.T) {
	ctx := prompts.TemplateContext{
		WordText:           "ephemeral",
		OfficialDefinition: "Lasting for a very short time.",
	}

	rendered, err := prompts.Render("v1.0", ctx)
	if err != nil {
		t.Fatalf("unexpected error rendering v1.0 prompt: %v", err)
	}

	if !strings.Contains(rendered, "TARGET HEADWORD: ephemeral") {
		t.Errorf("expected word 'ephemeral' in rendered prompt, got: %s", rendered)
	}
	if !strings.Contains(rendered, "Lasting for a very short time.") {
		t.Errorf("expected definition in rendered prompt, got: %s", rendered)
	}
	if !strings.Contains(rendered, "Two-Strike Direct Explanation Fallback") {
		t.Errorf("expected two-strike rule directive in prompt")
	}
}

func TestRender_NotFound(t *testing.T) {
	_, err := prompts.Render("v99.9", prompts.TemplateContext{})
	if err == nil {
		t.Fatalf("expected error for non-existent prompt version, got nil")
	}
}
