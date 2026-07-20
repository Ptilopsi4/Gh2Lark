package transformer

import (
	"encoding/json"
	"testing"
)

func TestTransformGenericEvents(t *testing.T) {
	tests := []struct {
		name     string
		event    string
		payload  map[string]any
		color    string
		contains []string
	}{
		{
			name:  "branch protection rule",
			event: "branch_protection_rule",
			payload: genericPayload("edited", map[string]any{
				"rule": map[string]any{"name": "protect-main", "pattern": "main", "id": 12, "html_url": "https://github.com/octo/example/settings/branches/main"},
			}),
			color:    "blue",
			contains: []string{"Branch protection rule", "protect-main", "main", "View on GitHub"},
		},
		{
			name:  "workflow run failure",
			event: "workflow_run",
			payload: genericPayload("completed", map[string]any{
				"workflow_run": map[string]any{"name": "CI", "conclusion": "failure", "status": "completed", "head_branch": "main", "head_sha": "abcdef012345", "html_url": "https://github.com/octo/example/actions/runs/1"},
			}),
			color:    "red",
			contains: []string{"Workflow run", "CI", "failure", "main", "View on GitHub"},
		},
		{
			name:  "secret scanning alert keeps details",
			event: "secret_scanning_alert",
			payload: genericPayload("created", map[string]any{
				"alert": map[string]any{"number": 9, "html_url": "https://github.com/octo/example/security/secret-scanning/9", "secret_type_display_name": "GitHub token", "state": "open", "resolution_comment": "rotated by incident responder"},
			}),
			color:    "green",
			contains: []string{"Secret scanning alert", "GitHub token", "rotated by incident responder", "View on GitHub"},
		},
		{
			name:  "ruleset bypass keeps reason",
			event: "bypass_request_push_ruleset",
			payload: genericPayload("created", map[string]any{
				"bypass_request": map[string]any{"reason": "Urgent production rollback", "comment": "approved by on-call", "status": "pending"},
			}),
			color:    "orange",
			contains: []string{"Push ruleset bypass request", "Urgent production rollback", "approved by on-call", "pending"},
		},
		{
			name:  "discussion",
			event: "discussion",
			payload: genericPayload("answered", map[string]any{
				"discussion": map[string]any{"number": 5, "title": "Release planning", "state": "open", "html_url": "https://github.com/octo/example/discussions/5", "body": "Plan the next release", "category": map[string]any{"name": "Ideas"}},
			}),
			color:    "blue",
			contains: []string{"Discussion", "Release planning", "Ideas", "Plan the next release", "View on GitHub"},
		},
		{
			name:  "secret scanning scan",
			event: "secret_scanning_scan",
			payload: genericPayload("completed", map[string]any{
				"type":                "incremental",
				"source":              "push",
				"started_at":          "2026-07-20T12:00:00Z",
				"completed_at":        "2026-07-20T12:01:00Z",
				"secret_types":        []string{"github_pat", "aws_access_key"},
				"custom_pattern_name": "Internal token",
			}),
			color:    "green",
			contains: []string{"Secret scanning scan", "incremental", "github_pat, aws_access_key", "Internal token"},
		},
		{
			name:  "release",
			event: "release",
			payload: genericPayload("published", map[string]any{
				"release": map[string]any{"name": "Version 1.2", "tag_name": "v1.2.0", "target_commitish": "main", "body": "Stable release", "html_url": "https://github.com/octo/example/releases/tag/v1.2.0"},
			}),
			color:    "green",
			contains: []string{"Release", "Version 1.2", "v1.2.0", "Stable release", "View on GitHub"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(test.payload)
			if err != nil {
				t.Fatal(err)
			}
			card, err := Transform(test.event, body)
			if err != nil {
				t.Fatalf("Transform(%s) failed: %v", test.event, err)
			}
			if card.Card.Header.Template != test.color {
				t.Errorf("color = %s, want %s", card.Card.Header.Template, test.color)
			}
			for _, expected := range test.contains {
				if !contains(t, card, expected) {
					t.Errorf("card should contain %q", expected)
				}
			}
		})
	}
}

func TestTransformGenericEventInvalidJSON(t *testing.T) {
	if _, err := Transform("workflow_run", []byte("not json")); err == nil {
		t.Error("expected generic event invalid JSON to fail")
	}
}

func TestTransformUnknownEventUsesBoundedFallback(t *testing.T) {
	card, err := Transform("future_event", []byte(`{"action":"noticed","repository":{"full_name":"octo/example","html_url":"https://github.com/octo/example"},"sender":{"login":"octocat"},"future":"value"}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"future_event", "noticed", "octo/example", "octocat", "future"} {
		if !contains(t, card, expected) {
			t.Errorf("fallback card should contain %q", expected)
		}
	}
}

func genericPayload(action string, fields map[string]any) map[string]any {
	fields["action"] = action
	fields["repository"] = map[string]any{"full_name": "octo/example", "html_url": "https://github.com/octo/example"}
	fields["sender"] = map[string]any{"login": "octocat"}
	return fields
}
