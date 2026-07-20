package webhook

import "testing"

func TestParseEventEnvelope(t *testing.T) {
	body := []byte(`{
		"action":"completed",
		"repository":{"full_name":"octo/example","html_url":"https://github.com/octo/example"},
		"sender":{"login":"octocat"},
		"workflow_run":{"name":"CI","run_number":42,"conclusion":"success","pull_requests":[{"number":7}]}
	}`)

	event, err := ParseEventEnvelope(body)
	if err != nil {
		t.Fatalf("ParseEventEnvelope failed: %v", err)
	}
	if event.Action != "completed" || event.Repository.FullName != "octo/example" || event.Sender.Login != "octocat" {
		t.Fatalf("common event fields were not decoded: %#v", event)
	}
	if got := event.String("workflow_run.name"); got != "CI" {
		t.Errorf("String() = %q, want CI", got)
	}
	if got := event.Int("workflow_run.run_number"); got != 42 {
		t.Errorf("Int() = %d, want 42", got)
	}
	if got := event.String("workflow_run.pull_requests.0.number"); got != "7" {
		t.Errorf("nested number String() = %q, want 7", got)
	}
	if event.Value("workflow_run.missing") != nil || event.Value("workflow_run.pull_requests.bad") != nil {
		t.Error("missing or invalid paths must return nil")
	}
}

func TestParseEventEnvelopeRejectsNonObject(t *testing.T) {
	for _, body := range [][]byte{[]byte("not json"), []byte("[]"), []byte("null")} {
		if _, err := ParseEventEnvelope(body); err == nil {
			t.Errorf("ParseEventEnvelope(%s) should fail", body)
		}
	}
}

func TestEventEnvelopeRawSnippet(t *testing.T) {
	event, err := ParseEventEnvelope([]byte(`{"message":"abcdef"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := event.RawSnippet(5); got != `{"mes…` {
		t.Errorf("RawSnippet() = %q, want truncated result", got)
	}
}
