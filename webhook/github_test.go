package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestParsePush(t *testing.T) {
	body := []byte(`{
		"ref": "refs/heads/main",
		"before": "abc123",
		"after": "def456",
		"created": false,
		"deleted": false,
		"forced": false,
		"compare": "https://github.com/octocat/Hello-World/compare/abc123...def456",
		"commits": [
			{
				"id": "def4567890123456789012345678901234567890",
				"message": "fix: resolve nil pointer in handler\n\nCloses #42",
				"url": "https://api.github.com/repos/octocat/Hello-World/commits/def456",
				"author": { "name": "Octo Cat", "email": "octo@github.com", "username": "octocat" },
				"committer": { "name": "Octo Cat", "email": "octo@github.com", "username": "octocat" },
				"added": ["new.go"],
				"removed": [],
				"modified": ["old.go"],
				"distinct": true
			}
		],
		"head_commit": {
			"id": "def4567890123456789012345678901234567890",
			"message": "fix: resolve nil pointer in handler\n\nCloses #42",
			"url": "https://api.github.com/repos/octocat/Hello-World/commits/def456",
			"author": { "name": "Octo Cat", "email": "octo@github.com", "username": "octocat" }
		},
		"pusher": { "name": "Octo Cat", "email": "octo@github.com", "login": "octocat" },
		"repository": {
			"id": 1296269,
			"name": "Hello-World",
			"full_name": "octocat/Hello-World",
			"html_url": "https://github.com/octocat/Hello-World",
			"description": "My first repo",
			"private": false
		},
		"sender": { "login": "octocat" }
	}`)

	ev, err := ParsePush(body)
	if err != nil {
		t.Fatalf("ParsePush failed: %v", err)
	}

	if ev.Ref != "refs/heads/main" {
		t.Errorf("Ref = %s, want refs/heads/main", ev.Ref)
	}
	if ev.BranchName() != "main" {
		t.Errorf("BranchName = %s, want main", ev.BranchName())
	}
	if ev.IsTag() {
		t.Error("IsTag should be false")
	}
	if len(ev.Commits) != 1 {
		t.Errorf("len(Commits) = %d, want 1", len(ev.Commits))
	}
	if ev.Repository.FullName != "octocat/Hello-World" {
		t.Errorf("Repository.FullName = %s, want octocat/Hello-World", ev.Repository.FullName)
	}
	if ev.HeadCommit.ID != "def4567890123456789012345678901234567890" {
		t.Errorf("HeadCommit.ID mismatch")
	}
}

func TestParsePushTag(t *testing.T) {
	body := []byte(`{
		"ref": "refs/tags/v1.2.3",
		"before": "0000000000000000000000000000000000000000",
		"after": "abc1234567890123456789012345678901234567",
		"created": true,
		"deleted": false,
		"forced": false,
		"compare": "https://github.com/octocat/Hello-World/compare/v1.2.3",
		"commits": [],
		"head_commit": null,
		"pusher": { "name": "Octo Cat" },
		"repository": { "full_name": "octocat/Hello-World", "html_url": "https://github.com/octocat/Hello-World", "private": false },
		"sender": {}
}`)

	ev, err := ParsePush(body)
	if err != nil {
		t.Fatalf("ParsePush failed: %v", err)
	}
	if !ev.IsTag() {
		t.Error("IsTag should be true for refs/tags/...")
	}
	if ev.TagName() != "v1.2.3" {
		t.Errorf("TagName = %s, want v1.2.3", ev.TagName())
	}
	if !ev.Created {
		t.Error("Created should be true")
	}
}

func TestParsePullRequest(t *testing.T) {
	body := []byte(`{
		"action": "opened",
		"number": 42,
		"pull_request": {
			"number": 42,
			"title": "Add new feature",
			"body": "This PR adds an exciting new feature.",
			"state": "open",
			"merged": false,
			"html_url": "https://github.com/octocat/Hello-World/pull/42",
			"user": { "login": "contributor", "name": "Contributor" },
			"labels": [
				{ "name": "enhancement", "color": "a2eeef" },
				{ "name": "needs-review", "color": "d93f0b" }
			],
			"created_at": "2024-01-15T10:30:00Z",
			"updated_at": "2024-01-15T11:00:00Z",
			"head": { "ref": "feature-branch", "sha": "abc123", "repo": {} },
			"base": { "ref": "main", "sha": "def456", "repo": {} }
		},
		"repository": {
			"id": 1296269,
			"name": "Hello-World",
			"full_name": "octocat/Hello-World",
			"html_url": "https://github.com/octocat/Hello-World",
			"description": "",
			"private": false
		},
		"sender": { "login": "contributor" }
	}`)

	ev, err := ParsePullRequest(body)
	if err != nil {
		t.Fatalf("ParsePullRequest failed: %v", err)
	}

	if ev.Action != "opened" {
		t.Errorf("Action = %s, want opened", ev.Action)
	}
	if ev.PullRequest.Number != 42 {
		t.Errorf("PullRequest.Number = %d, want 42", ev.PullRequest.Number)
	}
	if ev.PullRequest.Title != "Add new feature" {
		t.Errorf("PullRequest.Title = %s", ev.PullRequest.Title)
	}
	if len(ev.PullRequest.Labels) != 2 {
		t.Errorf("len(Labels) = %d, want 2", len(ev.PullRequest.Labels))
	}
	if ev.PullRequest.Labels[0].Name != "enhancement" {
		t.Errorf("Labels[0].Name = %s, want enhancement", ev.PullRequest.Labels[0].Name)
	}
	if ev.PullRequest.Head.Ref != "feature-branch" {
		t.Errorf("Head.Ref = %s, want feature-branch", ev.PullRequest.Head.Ref)
	}
}

func TestParseIssues(t *testing.T) {
	body := []byte(`{
		"action": "opened",
		"issue": {
			"number": 99,
			"title": "Bug: crash on startup",
			"body": "The app crashes immediately after launch.",
			"state": "open",
			"html_url": "https://github.com/octocat/Hello-World/issues/99",
			"user": { "login": "reporter" },
			"labels": [{ "name": "bug", "color": "d73a4a" }],
			"created_at": "2024-02-01T08:00:00Z",
			"updated_at": "2024-02-01T08:00:00Z"
		},
		"repository": {
			"full_name": "octocat/Hello-World",
			"html_url": "https://github.com/octocat/Hello-World",
			"private": false
		},
		"sender": {}
	}`)

	ev, err := ParseIssues(body)
	if err != nil {
		t.Fatalf("ParseIssues failed: %v", err)
	}

	if ev.Action != "opened" {
		t.Errorf("Action = %s, want opened", ev.Action)
	}
	if ev.Issue.Number != 99 {
		t.Errorf("Issue.Number = %d, want 99", ev.Issue.Number)
	}
	if ev.Issue.Labels[0].Name != "bug" {
		t.Errorf("Labels[0].Name = %s, want bug", ev.Issue.Labels[0].Name)
	}
}

func TestParseReview(t *testing.T) {
	body := []byte(`{
		"action": "submitted",
		"review": {
			"id": 456,
			"body": "LGTM, just a minor nit on the error handling.",
			"state": "approved",
			"html_url": "https://github.com/octocat/Hello-World/pull/42#pullrequestreview-456",
			"user": { "login": "reviewer1", "name": "Reviewer One" },
			"submitted_at": "2026-06-11T14:00:00Z",
			"commit_id": "abc1234567890123456789012345678901234567"
		},
		"pull_request": {
			"number": 42,
			"title": "Add OAuth2 login support",
			"state": "open",
			"html_url": "https://github.com/octocat/Hello-World/pull/42",
			"user": { "login": "author" },
			"head": { "ref": "feat/oauth", "sha": "abc" },
			"base": { "ref": "main", "sha": "def" }
		},
		"repository": {
			"full_name": "octocat/Hello-World",
			"html_url": "https://github.com/octocat/Hello-World",
			"private": false
		},
		"sender": { "login": "reviewer1" }
	}`)

	ev, err := ParseReview(body)
	if err != nil {
		t.Fatalf("ParseReview failed: %v", err)
	}
	if ev.Action != "submitted" {
		t.Errorf("Action = %s, want submitted", ev.Action)
	}
	if ev.Review.State != "approved" {
		t.Errorf("Review.State = %s, want approved", ev.Review.State)
	}
	if ev.Review.User.Login != "reviewer1" {
		t.Errorf("Review.User.Login = %s, want reviewer1", ev.Review.User.Login)
	}
	if ev.PullRequest.Number != 42 {
		t.Errorf("PR Number = %d, want 42", ev.PullRequest.Number)
	}
}

func TestParseReviewThread(t *testing.T) {
	body := []byte(`{
		"action": "resolved",
		"thread": {
			"id": 789,
			"node_id": "PRR_kwDOA",
			"is_resolved": true,
			"comments": [
				{
					"id": 1001,
					"body": "Should we use a different approach here?",
					"html_url": "https://github.com/octocat/Hello-World/pull/42#discussion_r1001",
					"user": { "login": "contributor" },
					"created_at": "2026-06-11T13:00:00Z",
					"updated_at": "2026-06-11T13:00:00Z"
				},
				{
					"id": 1002,
					"body": "Good point, I'll refactor this part.",
					"html_url": "https://github.com/octocat/Hello-World/pull/42#discussion_r1002",
					"user": { "login": "author" },
					"created_at": "2026-06-11T13:30:00Z",
					"updated_at": "2026-06-11T13:30:00Z"
				}
			]
		},
		"pull_request": {
			"number": 42,
			"title": "Add OAuth2 login support",
			"state": "open",
			"html_url": "https://github.com/octocat/Hello-World/pull/42",
			"user": { "login": "author" },
			"head": { "ref": "feat/oauth", "sha": "abc" },
			"base": { "ref": "main", "sha": "def" }
		},
		"repository": {
			"full_name": "octocat/Hello-World",
			"html_url": "https://github.com/octocat/Hello-World",
			"private": false
		},
		"sender": { "login": "author" }
	}`)

	ev, err := ParseReviewThread(body)
	if err != nil {
		t.Fatalf("ParseReviewThread failed: %v", err)
	}
	if ev.Action != "resolved" {
		t.Errorf("Action = %s, want resolved", ev.Action)
	}
	if !ev.Thread.IsResolved {
		t.Error("Thread.IsResolved should be true")
	}
	if len(ev.Thread.Comments) != 2 {
		t.Errorf("len(Comments) = %d, want 2", len(ev.Thread.Comments))
	}
	if ev.Thread.Comments[1].Body != "Good point, I'll refactor this part." {
		t.Errorf("Last comment body mismatch")
	}
	if ev.Thread.Comments[0].User.Login != "contributor" {
		t.Errorf("First comment user mismatch")
	}
}

func TestValidateSignature(t *testing.T) {
	secret := "my-secret-token"
	body := []byte(`{"test": "payload"}`)

	// Compute correct signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	correctHex := hex.EncodeToString(mac.Sum(nil))
	correctHeader := "sha256=" + correctHex

	t.Run("valid signature", func(t *testing.T) {
		if !ValidateSignature(secret, body, correctHeader) {
			t.Error("expected valid signature to pass")
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		if ValidateSignature("wrong-secret", body, correctHeader) {
			t.Error("expected wrong secret to fail")
		}
	})

	t.Run("empty secret skips validation", func(t *testing.T) {
		if !ValidateSignature("", body, "garbage-header") {
			t.Error("expected empty secret to skip validation")
		}
	})

	t.Run("empty header fails", func(t *testing.T) {
		if ValidateSignature(secret, body, "") {
			t.Error("expected empty header to fail")
		}
	})

	t.Run("malformed header", func(t *testing.T) {
		if ValidateSignature(secret, body, "not-a-sha256-header") {
			t.Error("expected malformed header to fail")
		}
	})

	t.Run("tampered body", func(t *testing.T) {
		if ValidateSignature(secret, []byte("tampered"), correctHeader) {
			t.Error("expected tampered body to fail")
		}
	})
}

func TestValidateSignatureConstantTime(t *testing.T) {
	// Verify that correct vs very-close-but-wrong hex are both rejected.
	secret := "test"
	body := []byte("data")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	correctHex := hex.EncodeToString(mac.Sum(nil))

	// Flip last byte
	b := []byte(correctHex)
	if b[len(b)-1] == 'a' {
		b[len(b)-1] = 'b'
	} else {
		b[len(b)-1] = 'a'
	}
	wrongHex := fmt.Sprintf("sha256=%s", string(b))

	if ValidateSignature(secret, body, wrongHex) {
		t.Error("off-by-one hex should fail")
	}
}
