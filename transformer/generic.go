package transformer

import (
	"fmt"
	"strings"

	"gh2lark/lark"
	"gh2lark/webhook"
)

const genericSummaryLimit = 300

// eventSpec names a GitHub webhook family and declares the payload locations
// that contain its primary object, state, URL, and detail fields.
type eventSpec struct {
	name    string
	object  []string
	url     []string
	states  []string
	details []detailSpec
}

type detailSpec struct {
	label string
	paths []string
}

var genericEventSpecs = map[string]eventSpec{
	"create":                                  {"Branch or tag", []string{"ref"}, []string{"repository.html_url"}, []string{"ref_type"}, []detailSpec{{"Ref type", []string{"ref_type"}}, {"SHA", []string{"master_branch", "repository.default_branch"}}}},
	"delete":                                  {"Branch or tag", []string{"ref"}, []string{"repository.html_url"}, []string{"ref_type"}, []detailSpec{{"Ref type", []string{"ref_type"}}}},
	"branch_protection_configuration":         {"Branch protection", []string{"repository.name"}, []string{"repository.html_url"}, []string{"action"}, []detailSpec{{"Rule", []string{"rule.name", "branch"}}}},
	"branch_protection_rule":                  {"Branch protection rule", []string{"rule.name", "rule.pattern"}, []string{"rule.html_url", "repository.html_url"}, []string{"action"}, []detailSpec{{"Pattern", []string{"rule.pattern"}}, {"Rule ID", []string{"rule.id"}}}},
	"bypass_request_push_ruleset":             securitySpec("Push ruleset bypass request", []string{"bypass_request.reason", "bypass_request.comment", "bypass_request.status"}),
	"bypass_request_secret_scanning":          securitySpec("Secret scanning bypass request", []string{"bypass_request.reason", "bypass_request.comment", "bypass_request.status", "bypass_request.secret_type"}),
	"push_ruleset":                            securitySpec("Push ruleset bypass request", []string{"bypass_request.reason", "bypass_request.comment", "bypass_request.status", "bypass_request.secret_type"}),
	"secret_scanning_push_protection":         securitySpec("Secret scanning push protection bypass request", []string{"bypass_request.reason", "bypass_request.comment", "bypass_request.status", "bypass_request.secret_type"}),
	"check_run":                               ciSpec("Check run", []string{"check_run.name"}, []string{"check_run.html_url", "check_run.details_url"}, []string{"check_run.conclusion", "check_run.status"}, []string{"check_run.output.title", "check_run.output.summary", "check_run.head_sha"}),
	"check_suite":                             ciSpec("Check suite", []string{"check_suite.head_branch", "check_suite.head_sha"}, []string{"check_suite.url", "repository.html_url"}, []string{"check_suite.conclusion", "check_suite.status"}, []string{"check_suite.head_sha", "check_suite.head_branch"}),
	"code_scanning_alert":                     securitySpec("Code scanning alert", []string{"alert.rule.id", "alert.rule.description", "alert.most_recent_instance.location.path", "alert.most_recent_instance.location.start_line", "alert.state", "alert.rule.security_severity_level"}),
	"member":                                  {"Collaborator", []string{"member.login", "membership.user.login"}, []string{"member.html_url", "repository.html_url"}, []string{"action"}, []detailSpec{{"Permission", []string{"membership.role_name", "membership.permissions"}}}},
	"commit_comment":                          commentSpec("Commit comment", []string{"comment.commit_id", "comment.path", "comment.line"}),
	"custom_property_values":                  {"Custom property values", []string{"repository.full_name"}, []string{"repository.html_url"}, []string{"action"}, []detailSpec{{"Properties", []string{"property_values.0.property_name", "properties.0.property_name"}}}},
	"dependabot_alert":                        securitySpec("Dependabot alert", []string{"alert.dependency.package.name", "alert.security_advisory.ghsa_id", "alert.security_advisory.severity", "alert.state", "alert.dependency.manifest_path"}),
	"deploy_key":                              {"Deploy key", []string{"key.title", "key.id"}, []string{"repository.html_url"}, []string{"action"}, []detailSpec{{"Read only", []string{"key.read_only"}}, {"Key", []string{"key.key"}}}},
	"deployment_status":                       ciSpec("Deployment status", []string{"deployment.environment", "deployment.ref"}, []string{"deployment_status.target_url", "deployment.url", "repository.html_url"}, []string{"deployment_status.state"}, []string{"deployment_status.description", "deployment.sha", "deployment_status.environment_url"}),
	"deployment":                              ciSpec("Deployment", []string{"deployment.environment", "deployment.ref"}, []string{"deployment.url", "repository.html_url"}, []string{"action"}, []string{"deployment.sha", "deployment.task", "deployment.description"}),
	"discussion_comment":                      commentSpec("Discussion comment", []string{"discussion.title", "discussion.number"}),
	"discussion":                              {"Discussion", []string{"discussion.title", "discussion.number"}, []string{"discussion.html_url", "repository.html_url"}, []string{"action", "discussion.state"}, []detailSpec{{"Category", []string{"discussion.category.name"}}, {"Answer", []string{"discussion.answer_html_url"}}, {"Body", []string{"discussion.body"}}}},
	"dismissal_request_dependabot":            securitySpec("Dependabot dismissal request", []string{"dismissal_request.reason", "dismissal_request.comment", "dismissal_request.status", "dependabot_alert.dependency.package.name"}),
	"dismissal_request_code_scanning":         securitySpec("Code scanning dismissal request", []string{"dismissal_request.reason", "dismissal_request.comment", "dismissal_request.status", "code_scanning_alert.rule.id"}),
	"dismissal_request_secret_scanning":       securitySpec("Secret scanning dismissal request", []string{"dismissal_request.reason", "dismissal_request.comment", "dismissal_request.status", "secret_scanning_alert.secret_type_display_name"}),
	"dependabot_alert_dismissal_request":      securitySpec("Dependabot dismissal request", []string{"dismissal_request.reason", "dismissal_request.comment", "dismissal_request.status", "dependabot_alert.dependency.package.name"}),
	"code_scanning_alert_dismissal_request":   securitySpec("Code scanning dismissal request", []string{"dismissal_request.reason", "dismissal_request.comment", "dismissal_request.status", "code_scanning_alert.rule.id"}),
	"secret_scanning_alert_dismissal_request": securitySpec("Secret scanning dismissal request", []string{"dismissal_request.reason", "dismissal_request.comment", "dismissal_request.status", "secret_scanning_alert.secret_type_display_name"}),
	"fork":                           {"Repository fork", []string{"forkee.full_name"}, []string{"forkee.html_url", "repository.html_url"}, []string{"action"}, []detailSpec{{"Forked from", []string{"repository.full_name"}}, {"Visibility", []string{"forkee.visibility"}}}},
	"issue_comment":                  commentSpec("Issue comment", []string{"issue.title", "issue.number"}),
	"issue_dependencies":             {"Issue dependency", []string{"issue.title", "issue.number"}, []string{"issue.html_url", "repository.html_url"}, []string{"action"}, []detailSpec{{"Blocked issue", []string{"blocked_issue.title", "blocked_issue.html_url"}}, {"Blocking issue", []string{"blocking_issue.title", "blocking_issue.html_url"}}}},
	"label":                          {"Label", []string{"label.name"}, []string{"repository.html_url"}, []string{"action"}, []detailSpec{{"Color", []string{"label.color"}}, {"Description", []string{"label.description"}}}},
	"merge_group":                    ciSpec("Merge group", []string{"merge_group.head_ref", "merge_group.head_sha"}, []string{"merge_group.html_url", "repository.html_url"}, []string{"action"}, []string{"merge_group.base_ref", "merge_group.head_sha"}),
	"meta":                           {"Webhook", nil, nil, []string{"action"}, nil},
	"milestone":                      {"Milestone", []string{"milestone.title", "milestone.number"}, []string{"milestone.html_url", "repository.html_url"}, []string{"action", "milestone.state"}, []detailSpec{{"Due", []string{"milestone.due_on"}}, {"Open issues", []string{"milestone.open_issues"}}, {"Closed issues", []string{"milestone.closed_issues"}}}},
	"package":                        packageSpec("Package"),
	"page_build":                     ciSpec("Pages build", []string{"build.commit"}, []string{"build.commit_url", "repository.html_url"}, []string{"build.status", "action"}, []string{"build.error.message", "build.pusher.login", "build.duration"}),
	"pull_request_review_comment":    commentSpec("Pull request review comment", []string{"pull_request.title", "pull_request.number", "comment.path", "comment.line"}),
	"registry_package":               packageSpec("Registry package"),
	"release":                        {"Release", []string{"release.name", "release.tag_name"}, []string{"release.html_url", "repository.html_url"}, []string{"action", "release.prerelease", "release.draft"}, []detailSpec{{"Tag", []string{"release.tag_name"}}, {"Target", []string{"release.target_commitish"}}, {"Body", []string{"release.body"}}}},
	"repository":                     {"Repository", []string{"repository.full_name"}, []string{"repository.html_url"}, []string{"action"}, []detailSpec{{"Visibility", []string{"repository.visibility", "repository.private"}}, {"Description", []string{"repository.description"}}, {"Default branch", []string{"repository.default_branch"}}}},
	"repository_advisory":            securitySpec("Repository advisory", []string{"repository_advisory.ghsa_id", "repository_advisory.cve_id", "repository_advisory.severity", "repository_advisory.summary", "repository_advisory.state"}),
	"repository_import":              {"Repository import", []string{"repository.full_name"}, []string{"repository.html_url"}, []string{"action", "status"}, []detailSpec{{"Status", []string{"status"}}, {"Message", []string{"message"}}}},
	"repository_ruleset":             {"Repository ruleset", []string{"repository_ruleset.name", "repository_ruleset.id", "ruleset.name"}, []string{"repository_ruleset.html_url", "ruleset.html_url", "repository.html_url"}, []string{"action", "repository_ruleset.enforcement"}, []detailSpec{{"Enforcement", []string{"repository_ruleset.enforcement", "ruleset.enforcement"}}, {"Target", []string{"repository_ruleset.target", "ruleset.target"}}}},
	"repository_vulnerability_alert": securitySpec("Repository vulnerability alert", []string{"action", "repository.full_name"}),
	"secret_scanning_alert_location": securitySpec("Secret scanning alert location", []string{"location.path", "location.start_line", "alert.secret_type_display_name", "alert.state"}),
	"secret_scanning_alert":          securitySpec("Secret scanning alert", []string{"alert.secret_type_display_name", "alert.secret_type", "alert.state", "alert.resolution", "alert.resolution_comment", "alert.push_protection_bypassed"}),
	"secret_scanning_scan":           securitySpec("Secret scanning scan", []string{"type", "source", "started_at", "completed_at", "secret_types", "custom_pattern_name", "custom_pattern_scope"}),
	"security_and_analysis":          securitySpec("Security and analysis", []string{"security_and_analysis.advanced_security.status", "security_and_analysis.secret_scanning.status", "security_and_analysis.secret_scanning_push_protection.status"}),
	"star":                           {"Star", []string{"repository.full_name"}, []string{"repository.html_url"}, []string{"action"}, nil},
	"status":                         ciSpec("Commit status", []string{"context", "name"}, []string{"target_url", "commit.html_url", "repository.html_url"}, []string{"state"}, []string{"description", "sha", "branches.0.name"}),
	"sub_issues":                     {"Sub-issue", []string{"issue.title", "issue.number"}, []string{"issue.html_url", "repository.html_url"}, []string{"action"}, []detailSpec{{"Sub-issue", []string{"sub_issue.title", "sub_issue.html_url"}}, {"Parent issue", []string{"parent_issue.title", "parent_issue.html_url"}}}},
	"team_add":                       {"Team access", []string{"team.name", "team.slug"}, []string{"team.html_url", "repository.html_url"}, []string{"action"}, []detailSpec{{"Permission", []string{"repository.permissions.admin", "repository.permissions.push"}}}},
	"public":                         {"Repository visibility", []string{"repository.full_name"}, []string{"repository.html_url"}, []string{"action"}, []detailSpec{{"Visibility", []string{"repository.visibility"}}}},
	"watch":                          {"Watch", []string{"repository.full_name"}, []string{"repository.html_url"}, []string{"action"}, nil},
	"gollum":                         {"Wiki", []string{"pages.0.title", "pages.0.page_name"}, []string{"pages.0.html_url", "repository.html_url"}, []string{"pages.0.action"}, []detailSpec{{"Page", []string{"pages.0.page_name"}}, {"Summary", []string{"pages.0.summary"}}}},
	"workflow_job":                   ciSpec("Workflow job", []string{"workflow_job.name"}, []string{"workflow_job.html_url", "workflow_job.url", "repository.html_url"}, []string{"workflow_job.conclusion", "workflow_job.status"}, []string{"workflow_job.head_sha", "workflow_job.head_branch", "workflow_job.started_at"}),
	"workflow_run":                   ciSpec("Workflow run", []string{"workflow_run.name", "workflow_run.display_title", "workflow_run.run_number"}, []string{"workflow_run.html_url", "workflow_run.jobs_url", "repository.html_url"}, []string{"workflow_run.conclusion", "workflow_run.status"}, []string{"workflow_run.head_branch", "workflow_run.head_sha", "workflow_run.event"}),
}

func ciSpec(name string, object, url, states, details []string) eventSpec {
	return eventSpec{name: name, object: object, url: url, states: states, details: namedDetails(details)}
}

func securitySpec(name string, details []string) eventSpec {
	return eventSpec{name: name, object: []string{"alert.number", "alert.html_url", "repository.full_name"}, url: []string{"alert.html_url", "repository_advisory.html_url", "repository.html_url"}, states: []string{"action", "alert.state"}, details: namedDetails(details)}
}

func commentSpec(name string, object []string) eventSpec {
	return eventSpec{name: name, object: object, url: []string{"comment.html_url", "issue.html_url", "pull_request.html_url", "discussion.html_url", "repository.html_url"}, states: []string{"action"}, details: []detailSpec{{"Author", []string{"comment.user.login"}}, {"Comment", []string{"comment.body"}}, {"Path", []string{"comment.path"}}, {"Line", []string{"comment.line", "comment.original_line"}}}}
}

func packageSpec(name string) eventSpec {
	return eventSpec{name: name, object: []string{"package.name", "registry_package.name", "package_version.name"}, url: []string{"package.html_url", "registry_package.html_url", "package_version.html_url", "repository.html_url"}, states: []string{"action"}, details: []detailSpec{{"Package type", []string{"package.package_type", "registry_package.package_type"}}, {"Version", []string{"package_version.name", "package_version.version"}}, {"Description", []string{"package.description", "registry_package.description"}}}}
}

func namedDetails(paths []string) []detailSpec {
	items := make([]detailSpec, 0, len(paths))
	for _, path := range paths {
		label := strings.ReplaceAll(path, ".", " · ")
		items = append(items, detailSpec{label: label, paths: []string{path}})
	}
	return items
}

func buildGenericEventCard(eventType string, body []byte) (*lark.InteractiveMessage, error) {
	event, err := webhook.ParseEventEnvelope(body)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", eventType, err)
	}

	spec, known := genericEventSpecs[eventType]
	if !known {
		return buildFallbackEventCard(eventType, event), nil
	}

	action := event.Action
	if action == "" {
		action = "received"
	}
	color, emoji := eventActionStyle(action, event.String(spec.states...))
	if strings.Contains(eventType, "bypass_request") || eventType == "push_ruleset" || eventType == "secret_scanning_push_protection" {
		color, emoji = "orange", "⚠️"
	}
	card := newCard(fmt.Sprintf("%s %s %s", emoji, spec.name, action), color)
	addRepositoryAndActor(card, event)

	if eventType == "create" || eventType == "delete" {
		addMD(card, "**Event:** %s", eventType)
	}
	if object := event.String(spec.object...); object != "" {
		addMD(card, "**Object:** %s", truncate(object, genericSummaryLimit))
	}
	if state := event.String(spec.states...); state != "" && state != action {
		addMD(card, "**Status:** %s", truncate(state, genericSummaryLimit))
	}
	for _, detail := range spec.details {
		if value := event.String(detail.paths...); value != "" {
			addMD(card, "**%s:** %s", detail.label, truncate(value, genericSummaryLimit))
			continue
		}
		if values := event.Strings(detail.paths[0]); len(values) > 0 {
			addMD(card, "**%s:** %s", detail.label, truncate(strings.Join(values, ", "), genericSummaryLimit))
		}
	}
	if !hasGenericContext(event, spec) {
		if raw := event.RawSnippet(1000); raw != "" {
			addMD(card, "**Payload summary:** `%s`", raw)
		}
	}

	if url := event.String(spec.url...); url != "" {
		addViewButton(card, "View on GitHub", url)
	}
	return card, nil
}

func hasGenericContext(event *webhook.EventEnvelope, spec eventSpec) bool {
	if event.Action != "" || event.Repository.FullName != "" || userLogin(event.Sender) != "" || event.String(spec.object...) != "" || event.String(spec.states...) != "" {
		return true
	}
	for _, detail := range spec.details {
		if event.String(detail.paths...) != "" || len(detail.paths) > 0 && len(event.Strings(detail.paths[0])) > 0 {
			return true
		}
	}
	return false
}

func buildFallbackEventCard(eventType string, event *webhook.EventEnvelope) *lark.InteractiveMessage {
	card := newCard(fmt.Sprintf("📡 GitHub %s", eventType), "blue")
	addRepositoryAndActor(card, event)
	if event.Action != "" {
		addMD(card, "**Action:** %s", event.Action)
	}
	if raw := event.RawSnippet(1000); raw != "" {
		addMD(card, "**Payload summary:** `%s`", raw)
	}
	if event.Repository.HTMLURL != "" {
		addViewButton(card, "View repository", event.Repository.HTMLURL)
	}
	return card
}

func newCard(title, color string) *lark.InteractiveMessage {
	return &lark.InteractiveMessage{
		Card: lark.CardConfig{
			Schema: "2.0",
			Header: lark.CardHeader{
				Title:    lark.TextTag{Tag: "plain_text", Content: title},
				Template: color,
			},
			Body: lark.CardBody{
				Direction: "vertical",
				Elements:  []lark.CardElement{},
			},
		},
	}
}

func addRepositoryAndActor(card *lark.InteractiveMessage, event *webhook.EventEnvelope) {
	if event.Repository.FullName != "" {
		if event.Repository.HTMLURL != "" {
			addMD(card, "**Repository:** [%s](%s)", event.Repository.FullName, event.Repository.HTMLURL)
		} else {
			addMD(card, "**Repository:** %s", event.Repository.FullName)
		}
	}
	if actor := userLogin(event.Sender); actor != "" {
		addMD(card, "**Actor:** %s", actor)
	}
}

func addViewButton(card *lark.InteractiveMessage, label, url string) {
	addElement(card, lark.CardElement{Tag: "hr"})
	addElement(card, lark.CardElement{Tag: "button", Text: &lark.TextTag{Tag: "plain_text", Content: label}, Behaviors: []lark.Behavior{{Type: "open_url", DefaultURL: url}}})
}

func eventActionStyle(action, state string) (string, string) {
	action = strings.ToLower(action)
	state = strings.ToLower(state)
	if containsAny(action, "bypass") || containsAny(state, "bypass", "warning", "waiting", "in_progress", "queued", "requested", "rerequested", "reopened", "unresolved", "pending") {
		return "orange", "⚠️"
	}
	if containsAny(state, "failure", "failed", "error", "deleted", "disabled", "dismissed", "closed", "cancelled", "rejected", "destroyed") {
		return "red", "⛔"
	}
	if containsAny(state, "success", "completed", "approved", "published", "created", "opened", "enabled", "resolved", "fixed", "merged") {
		return "green", "✅"
	}
	if containsAny(action, "edited", "updated", "changed", "assigned", "labeled", "pinned", "transferred") {
		return "blue", "✏️"
	}
	return "blue", "ℹ️"
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}
