package transformer

import (
	"fmt"
	"strings"

	"gh2lark/lark"
	"gh2lark/webhook"
)

const maxCommits = 5       // max individual commits to show in a push card
const maxMsgLen = 80       // max length of commit first line
const shortSHALen = 7      // number of characters for abbreviated SHA

// Transform converts a GitHub webhook event into a Lark interactive card.
func Transform(eventType string, body []byte) (*lark.InteractiveMessage, error) {
	switch eventType {
	case "push":
		return buildPushCard(body)
	case "pull_request":
		return buildPullRequestCard(body)
	case "issues":
		return buildIssueCard(body)
	default:
		return buildFallbackCard(eventType, body), nil
	}
}

// --- Push ---

func buildPushCard(body []byte) (*lark.InteractiveMessage, error) {
	ev, err := webhook.ParsePush(body)
	if err != nil {
		return nil, fmt.Errorf("parse push: %w", err)
	}

	color := "green"
	actionEmoji := "🔨"
	if ev.Deleted {
		color = "red"
		actionEmoji = "💀"
	} else if ev.Forced {
		color = "orange"
		actionEmoji = "⚠️"
	} else if ev.Created {
		color = "turquoise"
		actionEmoji = "✨"
	}

	card := lark.InteractiveMessage{
		Card: lark.CardConfig{
			Schema: "2.0",
			Header: lark.CardHeader{
				Title:    lark.TextTag{Tag: "plain_text", Content: fmt.Sprintf("%s Push", actionEmoji)},
				Template: color,
			},
			Body: lark.CardBody{
				Direction: "vertical",
				Elements:  []lark.CardElement{},
			},
		},
	}

	addMD(&card, "**Repository:** [%s](%s)", ev.Repository.FullName, ev.Repository.HTMLURL)

	if ev.IsTag() {
		addMD(&card, "**Tag:** 🏷️ `%s`", ev.TagName())
	} else {
		addMD(&card, "**Branch:** `%s`", ev.BranchName())
	}

	pusher := ev.Pusher.Login
	if pusher == "" {
		pusher = ev.Pusher.Name
	}
	if pusher == "" {
		pusher = ev.Sender.Login
	}
	addMD(&card, "**Pusher:** %s", pusher)

	// Commits
	totalCommits := len(ev.Commits)
	if totalCommits > 0 {
		addElement(&card, lark.CardElement{Tag: "hr"})
		limit := min(totalCommits, maxCommits)
		for i := range limit {
			c := ev.Commits[i]
			short := shortSHA(c.ID)
			msg := firstLine(c.Message)
			msg = truncate(msg, maxMsgLen)
			author := c.Author.Username
			if author == "" {
				author = c.Author.Name
			}
			addMD(&card, "[`%s`](%s) %s — *%s*", short, c.URL, msg, author)
		}
		if totalCommits > maxCommits {
			addMD(&card, "*… and %d more commits*", totalCommits-maxCommits)
		}
	}

	if ev.Compare != "" {
		addElement(&card, lark.CardElement{Tag: "hr"})
		addMD(&card, "[Compare changes](%s)", ev.Compare)
	}

	return &card, nil
}

// --- Pull Request ---

func buildPullRequestCard(body []byte) (*lark.InteractiveMessage, error) {
	ev, err := webhook.ParsePullRequest(body)
	if err != nil {
		return nil, fmt.Errorf("parse pull_request: %w", err)
	}
	pr := ev.PullRequest

	color, emoji, actionText := prActionStyle(ev.Action, pr.Merged)

	card := lark.InteractiveMessage{
		Card: lark.CardConfig{
			Schema: "2.0",
			Header: lark.CardHeader{
				Title:    lark.TextTag{Tag: "plain_text", Content: fmt.Sprintf("%s Pull Request %s", emoji, actionText)},
				Template: color,
			},
			Body: lark.CardBody{
				Direction: "vertical",
				Elements:  []lark.CardElement{},
			},
		},
	}

	addMD(&card, "**[#%d %s](%s)**", pr.Number, pr.Title, pr.HTMLURL)
	addMD(&card, "**Repository:** [%s](%s)", ev.Repository.FullName, ev.Repository.HTMLURL)
	addMD(&card, "**Author:** %s", userLogin(pr.User))
	addMD(&card, "**Branch:** `%s` → `%s`", pr.Head.Ref, pr.Base.Ref)
	addMD(&card, "**State:** %s  |  **Merged:** %v", pr.State, pr.Merged)

	if len(pr.Labels) > 0 {
		addMD(&card, "**Labels:** %s", formatLabels(pr.Labels))
	}

	addElement(&card, lark.CardElement{Tag: "hr"})
	addElement(&card, lark.CardElement{
		Tag: "button",
		Text: &lark.TextTag{
			Tag:     "plain_text",
			Content: "View Pull Request",
		},
		Behaviors: []lark.Behavior{{
			Type:       "open_url",
			DefaultURL: pr.HTMLURL,
		}},
	})

	return &card, nil
}

// --- Issues ---

func buildIssueCard(body []byte) (*lark.InteractiveMessage, error) {
	ev, err := webhook.ParseIssues(body)
	if err != nil {
		return nil, fmt.Errorf("parse issues: %w", err)
	}
	issue := ev.Issue

	color, emoji := issueActionStyle(ev.Action)

	card := lark.InteractiveMessage{
		Card: lark.CardConfig{
			Schema: "2.0",
			Header: lark.CardHeader{
				Title:    lark.TextTag{Tag: "plain_text", Content: fmt.Sprintf("%s Issue %s", emoji, ev.Action)},
				Template: color,
			},
			Body: lark.CardBody{
				Direction: "vertical",
				Elements:  []lark.CardElement{},
			},
		},
	}

	addMD(&card, "**[#%d %s](%s)**", issue.Number, issue.Title, issue.HTMLURL)
	addMD(&card, "**Repository:** [%s](%s)", ev.Repository.FullName, ev.Repository.HTMLURL)
	addMD(&card, "**Author:** %s", userLogin(issue.User))
	addMD(&card, "**State:** %s", issue.State)

	if len(issue.Labels) > 0 {
		addMD(&card, "**Labels:** %s", formatLabels(issue.Labels))
	}

	addElement(&card, lark.CardElement{Tag: "hr"})
	addElement(&card, lark.CardElement{
		Tag: "button",
		Text: &lark.TextTag{
			Tag:     "plain_text",
			Content: "View Issue",
		},
		Behaviors: []lark.Behavior{{
			Type:       "open_url",
			DefaultURL: issue.HTMLURL,
		}},
	})

	return &card, nil
}

// --- Fallback ---

func buildFallbackCard(eventType string, body []byte) *lark.InteractiveMessage {
	card := lark.InteractiveMessage{
		Card: lark.CardConfig{
			Schema: "2.0",
			Header: lark.CardHeader{
				Title:    lark.TextTag{Tag: "plain_text", Content: fmt.Sprintf("📡 GitHub %s", eventType)},
				Template: "blue",
			},
			Body: lark.CardBody{
				Direction: "vertical",
				Elements:  []lark.CardElement{},
			},
		},
	}
	addMD(&card, "Received **%s** event (unsupported type — raw payload below).", eventType)
	raw := string(body)
	if len(raw) > 2000 {
		raw = raw[:2000] + "\n… (truncated)"
	}
	addMD(&card, "```\n%s\n```", raw)
	return &card
}

// --- Helpers ---

func addMD(card *lark.InteractiveMessage, format string, args ...any) {
	card.Card.Body.Elements = append(card.Card.Body.Elements, lark.CardElement{
		Tag:     "markdown",
		Content: fmt.Sprintf(format, args...),
	})
}

func addElement(card *lark.InteractiveMessage, el lark.CardElement) {
	card.Card.Body.Elements = append(card.Card.Body.Elements, el)
}

func shortSHA(sha string) string {
	if len(sha) > shortSHALen {
		return sha[:shortSHALen]
	}
	return sha
}

func firstLine(msg string) string {
	if i := strings.IndexAny(msg, "\n\r"); i >= 0 {
		return msg[:i]
	}
	return msg
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

func userLogin(u webhook.User) string {
	if u.Login != "" {
		return u.Login
	}
	if u.Username != "" {
		return u.Username
	}
	return u.Name
}

func formatLabels(labels []webhook.Label) string {
	parts := make([]string, len(labels))
	for i, l := range labels {
		parts[i] = fmt.Sprintf("`%s`", l.Name)
	}
	return strings.Join(parts, " ")
}

// prActionStyle returns (header color, emoji, display action) for a given PR action.
// GitHub uses action="closed" with merged=true/false — there is no action="merged".
func prActionStyle(action string, merged bool) (string, string, string) {
	switch {
	case action == "closed" && merged:
		return "purple", "✅", "merged"
	case action == "closed":
		return "red", "🔒", "closed"
	case action == "opened":
		return "green", "🆕", "opened"
	case action == "reopened":
		return "turquoise", "🔄", "reopened"
	case action == "ready_for_review":
		return "blue", "👀", action
	case action == "review_requested":
		return "blue", "👀", action
	case action == "converted_to_draft":
		return "orange", "📝", action
	case action == "edited":
		return "blue", "✏️", action
	case action == "labeled":
		return "blue", "🏷️", action
	case action == "unlabeled":
		return "blue", "🏷️", action
	case action == "assigned":
		return "blue", "👤", action
	default:
		return "blue", "ℹ️", action
	}
}

// issueActionStyle returns (header color, emoji) for a given issue action.
func issueActionStyle(action string) (string, string) {
	switch action {
	case "opened":
		return "green", "🆕"
	case "closed":
		return "red", "🔒"
	case "reopened":
		return "turquoise", "🔄"
	case "edited":
		return "blue", "✏️"
	case "labeled":
		return "blue", "🏷️"
	case "unlabeled":
		return "blue", "🏷️"
	case "assigned":
		return "blue", "👤"
	default:
		return "blue", "ℹ️"
	}
}
