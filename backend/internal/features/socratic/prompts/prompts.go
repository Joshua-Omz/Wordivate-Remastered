// Purpose: Embeds and provides versioned system prompt templates for Socratic sessions.
// Architecture: backend/internal/features/socratic/prompts/prompts.go
package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed *.txt
var promptFS embed.FS

/// TemplateContext carries data interpolated into the Socratic system prompt.
type TemplateContext struct {
	WordText           string
	OfficialDefinition string
}

/// Render returns the hydrated system prompt for the specified version and context.
func Render(version string, ctx TemplateContext) (string, error) {
	filename := version
	if !strings.HasSuffix(filename, ".txt") {
		filename = version + ".txt"
	}

	content, err := promptFS.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("prompt template version %q not found: %w", version, err)
	}

	tmpl, err := template.New(filename).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("parsing prompt template %q: %w", version, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("executing prompt template %q: %w", version, err)
	}

	return buf.String(), nil
}
