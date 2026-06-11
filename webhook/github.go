package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// PushEvent represents a simplified GitHub push event payload.
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#push
type PushEvent struct {
	Ref        string   `json:"ref"`
	Before     string   `json:"before"`
	After      string   `json:"after"`
	Created    bool     `json:"created"`
	Deleted    bool     `json:"deleted"`
	Forced     bool     `json:"forced"`
	Compare    string   `json:"compare"`
	Commits    []Commit `json:"commits"`
	HeadCommit *Commit  `json:"head_commit"`
	Pusher     User     `json:"pusher"`
	Repository Repo     `json:"repository"`
	Sender     User     `json:"sender"`
}

// BranchName returns the branch name without the refs/heads/ prefix.
func (p PushEvent) BranchName() string {
	if p.IsTag() {
		return strings.TrimPrefix(p.Ref, "refs/tags/")
	}
	return strings.TrimPrefix(p.Ref, "refs/heads/")
}

// IsTag reports whether this push created/updated a tag.
func (p PushEvent) IsTag() bool {
	return strings.HasPrefix(p.Ref, "refs/tags/")
}

// TagName returns the tag name without the refs/tags/ prefix.
func (p PushEvent) TagName() string {
	return strings.TrimPrefix(p.Ref, "refs/tags/")
}

// PullRequestEvent represents a simplified GitHub pull_request event payload.
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#pull_request
type PullRequestEvent struct {
	Action      string      `json:"action"`
	Number      int         `json:"number"`
	PullRequest PullRequest `json:"pull_request"`
	Repository  Repo        `json:"repository"`
	Sender      User        `json:"sender"`
}

// IssuesEvent represents a simplified GitHub issues event payload.
// https://docs.github.com/en/webhooks/webhook-events-and-payloads#issues
type IssuesEvent struct {
	Action     string `json:"action"`
	Issue      Issue  `json:"issue"`
	Repository Repo   `json:"repository"`
	Sender     User   `json:"sender"`
}

// --- Common types ---

// Commit represents a single commit in a push event.
type Commit struct {
	ID        string   `json:"id"`
	Message   string   `json:"message"`
	URL       string   `json:"url"`
	Author    User     `json:"author"`
	Committer User     `json:"committer"`
	Added     []string `json:"added"`
	Removed   []string `json:"removed"`
	Modified  []string `json:"modified"`
	Distinct  bool     `json:"distinct"`
}

// Repo holds repository metadata included in every webhook payload.
type Repo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
}

// User holds GitHub user metadata present in various payload sub-objects.
type User struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Login    string `json:"login"`
}

// PullRequest holds the core pull_request object.
type PullRequest struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	State     string  `json:"state"`
	Merged    bool    `json:"merged"`
	HTMLURL   string  `json:"html_url"`
	User      User    `json:"user"`
	Labels    []Label `json:"labels"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	Head      Branch  `json:"head"`
	Base      Branch  `json:"base"`
}

// Branch holds the branch ref details for a PR's head/base.
type Branch struct {
	Ref  string `json:"ref"`
	SHA  string `json:"sha"`
	Repo Repo   `json:"repo"`
}

// Issue holds the core issue object.
type Issue struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	State     string  `json:"state"`
	HTMLURL   string  `json:"html_url"`
	User      User    `json:"user"`
	Labels    []Label `json:"labels"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// Label represents a GitHub label.
type Label struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// --- Parse functions ---

// ParsePush unmarshals a push event payload.
func ParsePush(body []byte) (*PushEvent, error) {
	var e PushEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// ParsePullRequest unmarshals a pull_request event payload.
func ParsePullRequest(body []byte) (*PullRequestEvent, error) {
	var e PullRequestEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// ParseIssues unmarshals an issues event payload.
func ParseIssues(body []byte) (*IssuesEvent, error) {
	var e IssuesEvent
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// --- Signature validation ---

// ValidateSignature performs HMAC-SHA256 validation of the request body
// against the provided X-Hub-Signature-256 header value (format: "sha256=<hex>").
// When secret is empty, validation is skipped and true is returned.
func ValidateSignature(secret string, body []byte, signatureHeader string) bool {
	if secret == "" {
		return true
	}
	if signatureHeader == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(signatureHeader, prefix) {
		return false
	}
	expectedHex := strings.TrimPrefix(signatureHeader, prefix)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	actualHex := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(actualHex), []byte(expectedHex))
}
