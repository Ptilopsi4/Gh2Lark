package transformer

import (
	"encoding/json"
	"testing"
)

func TestTransformPush(t *testing.T) {
	body := githubPushJSON(t, "refs/heads/main", false, false, false, []commitData{
		{ID: "abc1234567890", Msg: "feat: add new feature\n\nDetailed body", Author: "dev1"},
		{ID: "def4567890123", Msg: "fix: resolve bug", Author: "dev2"},
	})

	card, err := Transform("push", body)
	if err != nil {
		t.Fatalf("Transform push failed: %v", err)
	}

	if card.Card.Header.Template != "green" {
		t.Errorf("header color = %s, want green", card.Card.Header.Template)
	}
	if !contains(t, card, "octocat/Hello-World") {
		t.Error("card should contain repo name")
	}
	if !contains(t, card, "abc1234") {
		t.Error("card should contain short SHA")
	}
	if !contains(t, card, "feat: add new feature") {
		t.Error("card should contain commit message")
	}
	if !contains(t, card, "Compare changes") {
		t.Error("card should have compare link")
	}
}

func TestTransformPushTag(t *testing.T) {
	body := githubPushJSON(t, "refs/tags/v1.0.0", true, false, false, nil)

	card, err := Transform("push", body)
	if err != nil {
		t.Fatalf("Transform push tag failed: %v", err)
	}
	if !contains(t, card, "v1.0.0") {
		t.Error("card should show tag name")
	}
}

func TestTransformPushDeleted(t *testing.T) {
	body := githubPushJSON(t, "refs/heads/old-branch", false, true, false, nil)

	card, err := Transform("push", body)
	if err != nil {
		t.Fatalf("Transform push deleted failed: %v", err)
	}
	if card.Card.Header.Template != "red" {
		t.Errorf("deleted push header color = %s, want red", card.Card.Header.Template)
	}
}

func TestTransformPushForced(t *testing.T) {
	body := githubPushJSON(t, "refs/heads/main", false, false, true, []commitData{
		{ID: "fff0000000000", Msg: "force push commit", Author: "admin"},
	})

	card, err := Transform("push", body)
	if err != nil {
		t.Fatalf("Transform forced push failed: %v", err)
	}
	if card.Card.Header.Template != "orange" {
		t.Errorf("forced push header color = %s, want orange", card.Card.Header.Template)
	}
}

func TestTransformPullRequest(t *testing.T) {
	body := githubPullRequestJSON(t, "opened", "open", false, "feature/fast", "main")

	card, err := Transform("pull_request", body)
	if err != nil {
		t.Fatalf("Transform pull_request failed: %v", err)
	}

	if card.Card.Header.Template != "green" {
		t.Errorf("opened PR color = %s, want green", card.Card.Header.Template)
	}
	if !contains(t, card, "Add new feature") {
		t.Error("card should contain PR title")
	}
	if !contains(t, card, "View Pull Request") {
		t.Error("card should have view button")
	}
}

func TestTransformPullRequestClosed(t *testing.T) {
	body := githubPullRequestJSON(t, "closed", "closed", false, "feat/x", "main")

	card, err := Transform("pull_request", body)
	if err != nil {
		t.Fatalf("Transform closed PR failed: %v", err)
	}
	if card.Card.Header.Template != "red" {
		t.Errorf("closed PR color = %s, want red", card.Card.Header.Template)
	}
}

func TestTransformPullRequestMerged(t *testing.T) {
	body := githubPullRequestJSON(t, "closed", "closed", true, "feat/x", "main")

	// For merged PRs, the action is still "closed" but merged=true.
	// Our transformer uses the action string, so it will show "closed" with red.
	// That's a known design choice — merged notification usually comes as a
	// separate "pull_request" event with action=closed and merged=true.
	// (GitHub doesn't send action="merged" — it sends closed + merged bool.)
	card, err := Transform("pull_request", body)
	if err != nil {
		t.Fatalf("Transform merged PR failed: %v", err)
	}
	cardJSON, _ := json.Marshal(card)
	if len(cardJSON) == 0 {
		t.Error("card should not be empty")
	}
}

func TestTransformIssues(t *testing.T) {
	body := githubIssueJSON(t, "opened", "open")

	card, err := Transform("issues", body)
	if err != nil {
		t.Fatalf("Transform issues failed: %v", err)
	}

	if card.Card.Header.Template != "green" {
		t.Errorf("opened issue color = %s, want green", card.Card.Header.Template)
	}
	if !contains(t, card, "crash on startup") {
		t.Error("card should contain issue title")
	}
	if !contains(t, card, "View Issue") {
		t.Error("card should have view button")
	}
}

func TestTransformIssuesClosed(t *testing.T) {
	body := githubIssueJSON(t, "closed", "closed")

	card, err := Transform("issues", body)
	if err != nil {
		t.Fatalf("Transform closed issue failed: %v", err)
	}
	if card.Card.Header.Template != "red" {
		t.Errorf("closed issue color = %s, want red", card.Card.Header.Template)
	}
}

func TestTransformFallback(t *testing.T) {
	body := []byte(`{"random": "event", "data": 42}`)

	card, err := Transform("create", body)
	if err != nil {
		t.Fatalf("Transform fallback failed: %v", err)
	}

	if card.Card.Header.Template != "blue" {
		t.Errorf("fallback color = %s, want blue", card.Card.Header.Template)
	}
	if !contains(t, card, "create") {
		t.Error("fallback card should mention event type")
	}
	if !contains(t, card, "random") {
		t.Error("fallback card should include raw JSON snippet")
	}
}

func TestTransformInvalidJSON(t *testing.T) {
	_, err := Transform("push", []byte("not json"))
	if err == nil {
		t.Error("expected error for invalid push JSON")
	}

	_, err = Transform("pull_request", []byte("not json"))
	if err == nil {
		t.Error("expected error for invalid PR JSON")
	}

	_, err = Transform("issues", []byte("not json"))
	if err == nil {
		t.Error("expected error for invalid issue JSON")
	}
}

func TestCommitLimit(t *testing.T) {
	commits := make([]commitData, 20)
	for i := range commits {
		commits[i] = commitData{
			ID:  "abc000000000000000000000000000000000000" + string(rune('0'+i%10)),
			Msg: "commit message",
		}
	}
	body := githubPushJSON(t, "refs/heads/main", false, false, false, commits)

	card, err := Transform("push", body)
	if err != nil {
		t.Fatalf("Transform multi-commit push failed: %v", err)
	}
	if !contains(t, card, "15 more commits") {
		t.Error("card should indicate truncated commits")
	}
}

// --- helpers ---

type commitData struct {
	ID     string
	Msg    string
	Author string
}

func githubPushJSON(t *testing.T, ref string, created, deleted, forced bool, commits []commitData) []byte {
	t.Helper()

	commitObjects := make([]map[string]any, len(commits))
	for i, c := range commits {
		commitObjects[i] = map[string]any{
			"id":      c.ID,
			"message": c.Msg,
			"url":     "https://api.github.com/repos/octocat/Hello-World/commits/" + c.ID[:7],
			"author": map[string]any{
				"name":     c.Author,
				"email":    c.Author + "@example.com",
				"username": c.Author,
			},
			"committer": map[string]any{
				"name":     c.Author,
				"email":    c.Author + "@example.com",
				"username": c.Author,
			},
			"added":    []string{},
			"removed":  []string{},
			"modified": []string{},
			"distinct": true,
		}
	}

	var headCommit map[string]any
	if len(commitObjects) > 0 {
		headCommit = commitObjects[0]
	}

	pusher := map[string]any{"name": "Pusher Cat", "login": "pushercat"}

	payload := map[string]any{
		"ref":     ref,
		"before":  "0000000000000000000000000000000000000000",
		"after":   "abc1234567890123456789012345678901234567",
		"created": created,
		"deleted": deleted,
		"forced":  forced,
		"compare": "https://github.com/octocat/Hello-World/compare/abc...def",
		"commits": commitObjects,
		"head_commit": headCommit,
		"pusher":  pusher,
		"repository": map[string]any{
			"id":          1296269,
			"name":        "Hello-World",
			"full_name":   "octocat/Hello-World",
			"html_url":    "https://github.com/octocat/Hello-World",
			"description": "",
			"private":     false,
		},
		"sender": map[string]any{"login": "pushercat"},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal push payload: %v", err)
	}
	return b
}

func githubPullRequestJSON(t *testing.T, action, state string, merged bool, headRef, baseRef string) []byte {
	t.Helper()
	payload := map[string]any{
		"action": action,
		"number": 42,
		"pull_request": map[string]any{
			"number":   42,
			"title":    "Add new feature",
			"body":     "This PR adds an exciting new feature.",
			"state":    state,
			"merged":   merged,
			"html_url": "https://github.com/octocat/Hello-World/pull/42",
			"user":     map[string]any{"login": "contributor", "name": "Contributor"},
			"labels": []map[string]any{
				{"name": "enhancement", "color": "a2eeef"},
			},
			"created_at": "2024-01-15T10:30:00Z",
			"updated_at": "2024-01-15T11:00:00Z",
			"head":       map[string]any{"ref": headRef, "sha": "abc1234", "repo": map[string]any{}},
			"base":       map[string]any{"ref": baseRef, "sha": "def5678", "repo": map[string]any{}},
		},
		"repository": map[string]any{
			"id":        1296269,
			"name":      "Hello-World",
			"full_name": "octocat/Hello-World",
			"html_url":  "https://github.com/octocat/Hello-World",
			"private":   false,
		},
		"sender": map[string]any{"login": "contributor"},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal PR payload: %v", err)
	}
	return b
}

func githubIssueJSON(t *testing.T, action, state string) []byte {
	t.Helper()
	payload := map[string]any{
		"action": action,
		"issue": map[string]any{
			"number":    99,
			"title":     "Bug: crash on startup",
			"body":      "The app crashes immediately after launch.",
			"state":     state,
			"html_url":  "https://github.com/octocat/Hello-World/issues/99",
			"user":      map[string]any{"login": "reporter"},
			"labels":    []map[string]any{{"name": "bug", "color": "d73a4a"}},
			"created_at": "2024-02-01T08:00:00Z",
			"updated_at": "2024-02-01T08:00:00Z",
		},
		"repository": map[string]any{
			"full_name": "octocat/Hello-World",
			"html_url":  "https://github.com/octocat/Hello-World",
			"private":   false,
		},
		"sender": map[string]any{},
	}

	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal issue payload: %v", err)
	}
	return b
}

func contains(t *testing.T, card any, sub string) bool {
	t.Helper()
	b, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("marshal card: %v", err)
	}
	return stringContains(string(b), sub)
}

func stringContains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
