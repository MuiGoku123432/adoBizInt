package ado

import (
	"encoding/json"
	"errors"
	"sort"
	"testing"
	"time"
)

// This file pins the JSON field contract for every serializable type in this
// package (OUT-02, D-04). From Phase 3's --json onward these key names are a
// public contract consumed by agents and scripts, so a rename is a one-way
// door. Each subtest asserts an EXACT key set rather than the presence of
// individual keys: an exact-set assertion is what catches a *missing* tag,
// which is the failure mode a per-field assertion silently passes over.

// keysOf marshals v and returns the top-level object's key names, sorted.
func keysOf(t *testing.T, v any) []string {
	t.Helper()

	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal %T into map: %v (raw: %s)", v, err, raw)
	}

	keys := make([]string, 0, len(generic))
	for k := range generic {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func assertKeySet(t *testing.T, v any, want []string) {
	t.Helper()

	got := keysOf(t, v)
	sorted := append([]string(nil), want...)
	sort.Strings(sorted)

	if len(got) != len(sorted) {
		t.Fatalf("%T key count = %d, want %d\n got: %v\nwant: %v", v, len(got), len(sorted), got, sorted)
	}
	for i := range got {
		if got[i] != sorted[i] {
			t.Fatalf("%T key set mismatch at %d: got %q, want %q\n got: %v\nwant: %v",
				v, i, got[i], sorted[i], got, sorted)
		}
	}
}

// TestJSONFieldNames marshals a ZERO value of each tagged struct and asserts the
// exact key set. A zero value is used deliberately: every tagged field must
// appear regardless of data, so a consumer sees a stable key set on every
// record.
func TestJSONFieldNames(t *testing.T) {
	t.Run("Branch", func(t *testing.T) {
		assertKeySet(t, Branch{}, []string{
			"name", "project", "repository_id", "repository_name",
			"default_branch", "is_default", "ahead_count", "behind_count",
			"last_commit_date", "last_commit_by", "commit_id",
		})
	})

	t.Run("DashboardCounts", func(t *testing.T) {
		assertKeySet(t, DashboardCounts{}, []string{
			"work_items", "prs", "pipelines",
		})
	})

	t.Run("PipelineRun", func(t *testing.T) {
		assertKeySet(t, PipelineRun{}, []string{
			"id", "build_number", "pipeline_name", "pipeline_id", "project",
			"status", "result", "source_branch", "source_version",
			"requested_by", "requested_for", "start_time", "finish_time",
			"queue_time", "duration_ns", "repository_id",
		})
	})

	t.Run("PipelineDefinition", func(t *testing.T) {
		assertKeySet(t, PipelineDefinition{}, []string{
			"id", "name", "path", "project", "repository_id",
			"default_branch", "queue_status",
		})
	})

	t.Run("PullRequest", func(t *testing.T) {
		assertKeySet(t, PullRequest{}, []string{
			"id", "title", "description", "project", "repository_id",
			"repository_name", "source_branch", "target_branch", "created_by",
			"created_by_id", "creation_date", "status", "is_draft",
			"reviewer_count", "approval_status",
		})
	})

	t.Run("Release", func(t *testing.T) {
		assertKeySet(t, Release{}, []string{
			"id", "name", "definition_id", "definition_name", "project",
			"status", "created_on", "modified_on", "environments",
			"pending_approvals",
		})
	})

	t.Run("ReleaseEnvironment", func(t *testing.T) {
		assertKeySet(t, ReleaseEnvironment{}, []string{
			"id", "name", "status", "deployment_status", "release_id",
			"release_name", "definition_id", "definition_name", "project",
		})
	})

	t.Run("ReleaseApproval", func(t *testing.T) {
		assertKeySet(t, ReleaseApproval{}, []string{
			"id", "release_id", "release_name", "environment_id",
			"environment_name", "project", "status", "approval_type",
			"approver", "approver_id", "created_on", "comments",
		})
	})

	t.Run("WorkItem", func(t *testing.T) {
		assertKeySet(t, WorkItem{}, []string{
			"id", "title", "summary", "story_points", "project", "type",
			"state", "assigned_to", "assigned_to_email",
		})
	})

	// WorkItemDetail's key set is the union of its own fields and the embedded
	// WorkItem's, because the embedding is untagged and therefore inlined.
	t.Run("WorkItemDetail", func(t *testing.T) {
		assertKeySet(t, WorkItemDetail{}, []string{
			// inlined from the embedded WorkItem
			"id", "title", "summary", "story_points", "project", "type",
			"state", "assigned_to", "assigned_to_email",
			// WorkItemDetail's own fields
			"description", "acceptance_criteria", "repro_steps", "attachments",
			"created_date", "modified_date", "created_by", "modified_by",
			"iteration_path", "area_path",
			// the three scheduling effort fields the write path diffs against
			"original_estimate", "remaining_work", "completed_work",
		})
	})

	t.Run("Attachment", func(t *testing.T) {
		assertKeySet(t, Attachment{}, []string{"name", "url", "size"})
	})

	// The envelope and the failure record are tagged in the same convention so
	// Phase 3 serialises the whole read response under one contract (D-01).
	t.Run("Result", func(t *testing.T) {
		assertKeySet(t, Result[WorkItem]{}, []string{"items", "failures", "complete"})
	})

	// Result must present the same three keys for every payload type, including
	// the dashboard's single-element Items slice from 02-03.
	t.Run("Result_uniform_across_reads", func(t *testing.T) {
		assertKeySet(t, Result[Branch]{}, []string{"items", "failures", "complete"})
		assertKeySet(t, Result[PullRequest]{}, []string{"items", "failures", "complete"})
		assertKeySet(t, Result[PipelineRun]{}, []string{"items", "failures", "complete"})
		assertKeySet(t, Result[Release]{}, []string{"items", "failures", "complete"})
		assertKeySet(t, Result[ReleaseApproval]{}, []string{"items", "failures", "complete"})
		assertKeySet(t, Result[WorkItemDetail]{}, []string{"items", "failures", "complete"})
		assertKeySet(t, Result[DashboardCounts]{}, []string{"items", "failures", "complete"})
	})

	// Threat T-02-01: exactly four keys. If a field is ever added to Failure
	// this assertion fails, which is the entire point — the failure record is
	// the struct most likely to accidentally gain a URL, a header, or a token.
	t.Run("Failure", func(t *testing.T) {
		assertKeySet(t, Failure{}, []string{"project", "operation", "kind", "err"})
	})
}

// TestJSONFieldNames_WorkItemDetailEmbedsFlat pins the inlining decision: the
// embedded WorkItem must NOT appear as a nested object under its own key.
func TestJSONFieldNames_WorkItemDetailEmbedsFlat(t *testing.T) {
	detail := WorkItemDetail{
		WorkItem:    WorkItem{ID: 42, Title: "embedded title"},
		Description: "detail description",
	}

	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal WorkItemDetail: %v", err)
	}

	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal into map: %v", err)
	}

	if _, nested := generic["WorkItem"]; nested {
		t.Errorf("embedded WorkItem nested under its own key; want inlined. raw: %s", raw)
	}
	if _, nested := generic["work_item"]; nested {
		t.Errorf("embedded WorkItem nested under work_item; want inlined. raw: %s", raw)
	}

	var flat struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &flat); err != nil {
		t.Fatalf("unmarshal into flat struct: %v", err)
	}
	if flat.ID != 42 || flat.Title != "embedded title" {
		t.Errorf("embedded fields not inlined: got id=%d title=%q", flat.ID, flat.Title)
	}
	if flat.Description != "detail description" {
		t.Errorf("own field lost: got description=%q", flat.Description)
	}
}

// TestJSONFieldNames_InitialismsAreSingleWords guards the two field names that
// mechanical camel-to-snake conversion would mangle into letter-split garbage.
func TestJSONFieldNames_InitialismsAreSingleWords(t *testing.T) {
	attachmentKeys := keysOf(t, Attachment{})
	if !contains(attachmentKeys, "url") {
		t.Errorf("Attachment.URL should marshal as %q, got keys %v", "url", attachmentKeys)
	}
	for _, bad := range []string{"u_r_l", "ur_l", "URL", "Url"} {
		if contains(attachmentKeys, bad) {
			t.Errorf("Attachment.URL marshalled as %q; initialism was split", bad)
		}
	}

	runKeys := keysOf(t, PipelineRun{})
	if !contains(runKeys, "pipeline_id") {
		t.Errorf("PipelineRun.PipelineID should marshal as %q, got keys %v", "pipeline_id", runKeys)
	}
	for _, bad := range []string{"pipeline_i_d", "pipelineid"} {
		if contains(runKeys, bad) {
			t.Errorf("PipelineRun.PipelineID marshalled as %q; identifier suffix was split", bad)
		}
	}

	countKeys := keysOf(t, DashboardCounts{})
	if !contains(countKeys, "prs") {
		t.Errorf("DashboardCounts.PRs should marshal as %q, got keys %v", "prs", countKeys)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// TestJSONDurationIsIntegerNanoseconds records the Task 1 contract decision:
// PipelineRun.Duration marshals as encoding/json's default for time.Duration,
// an integer nanosecond count. This is a locked one-way door — switching to a
// duration string or to seconds after Phase 3 ships --json would fail SILENTLY
// for any consumer doing arithmetic on the value.
func TestJSONDurationIsIntegerNanoseconds(t *testing.T) {
	run := PipelineRun{Duration: time.Hour}

	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal PipelineRun: %v", err)
	}

	var decoded struct {
		Duration json.RawMessage `json:"duration_ns"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	const wantNanos = "3600000000000"
	if string(decoded.Duration) != wantNanos {
		t.Errorf("duration_ns = %s, want integer nanoseconds %s (a quoted string here means "+
			"the contract changed to a duration string — that is a breaking change)",
			decoded.Duration, wantNanos)
	}
}

// TestJSONZeroTimestampIsSentinelDate confirms RESEARCH assumption A3, which was
// recorded as standard-library behavior that had not been re-verified this
// session. It is recorded as its own named test so the confirmation is a
// finding rather than an incidental pass.
//
// Confirmed: a zero time.Time VALUE field marshals to the year-one RFC 3339
// sentinel and is NOT omitted. ,omitempty cannot suppress it, because a struct
// is never "empty" to encoding/json. Omission would require a pointer field or
// a custom marshaller; Task 1 deliberately chose neither, so unfinished
// pipeline runs emit the sentinel date.
func TestJSONZeroTimestampIsSentinelDate(t *testing.T) {
	raw, err := json.Marshal(PipelineRun{})
	if err != nil {
		t.Fatalf("marshal PipelineRun: %v", err)
	}

	var decoded struct {
		FinishTime *string `json:"finish_time"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.FinishTime == nil {
		t.Fatal("assumption A3 REFUTED: zero timestamp was omitted or null; " +
			"the recorded contract says it marshals as the year-one sentinel")
	}
	const wantSentinel = "0001-01-01T00:00:00Z"
	if *decoded.FinishTime != wantSentinel {
		t.Errorf("assumption A3 REFUTED: zero finish_time = %q, want sentinel %q",
			*decoded.FinishTime, wantSentinel)
	}
}

// TestJSONFailureErrIsFlattenedString is the T-02-01 leak guard. Marshalling the
// Err interface directly would serialise the dynamic error value's exported
// fields, which for an SDK error can carry server-controlled response detail.
// Failure.MarshalJSON must emit only the message string.
func TestJSONFailureErrIsFlattenedString(t *testing.T) {
	t.Run("nil error marshals to null", func(t *testing.T) {
		raw, err := json.Marshal(Failure{Project: "p", Operation: "op", Kind: FailureOther})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var decoded struct {
			Err *string `json:"err"`
		}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded.Err != nil {
			t.Errorf("nil Err marshalled as %q, want null", *decoded.Err)
		}
	})

	t.Run("structured error flattens to its message only", func(t *testing.T) {
		// leakyError stands in for an SDK error carrying request context. If
		// encoding/json marshalled the interface structurally, these exported
		// fields would reach the output.
		f := Failure{
			Project:   "proj",
			Operation: "get pull requests",
			Kind:      FailureAuth,
			Err:       leakyError{Message: "unauthorized", RequestURL: "https://dev.azure.com/o/_apis?token=SECRET"},
		}

		raw, err := json.Marshal(f)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}

		var decoded struct {
			Err *string `json:"err"`
		}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded.Err == nil {
			t.Fatal("Err marshalled to null, want the message string")
		}
		if *decoded.Err != "unauthorized" {
			t.Errorf("err = %q, want the untruncated message %q", *decoded.Err, "unauthorized")
		}

		// The structural leak assertion: no request URL, no token, anywhere.
		for _, forbidden := range []string{"RequestURL", "request_url", "SECRET", "dev.azure.com"} {
			if bytesContain(raw, forbidden) {
				t.Errorf("T-02-01 VIOLATION: %q reached the marshalled failure record: %s", forbidden, raw)
			}
		}
	})

	// The message is emitted untruncated on purpose. Terminal rendering
	// truncates for display (02-01); a machine-readable record should not.
	t.Run("message is not truncated", func(t *testing.T) {
		long := ""
		for i := 0; i < 500; i++ {
			long += "x"
		}
		raw, err := json.Marshal(Failure{Kind: FailureOther, Err: errors.New(long)})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var decoded struct {
			Err *string `json:"err"`
		}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded.Err == nil || len(*decoded.Err) != len(long) {
			t.Errorf("failure message was truncated; JSON fidelity is intentional")
		}
	})
}

type leakyError struct {
	Message    string
	RequestURL string
}

func (e leakyError) Error() string { return e.Message }

func bytesContain(haystack []byte, needle string) bool {
	h, n := string(haystack), needle
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}

// TestNewResultEmitsEmptyArraysNotNull pins the wire shape of an empty read.
//
// Regression: newResult passed the caller's possibly-nil slices straight
// through, so "failures":null was the normal successful-read output and an
// agent evaluating `.failures | length` errored on the happy path. Both keys
// must now be [] regardless of what the caller passed.
func TestNewResultEmitsEmptyArraysNotNull(t *testing.T) {
	result := newResult[PullRequest](nil, nil)

	if result.Items == nil {
		t.Errorf("newResult(nil, nil).Items is nil, want an empty slice")
	}
	if result.Failures == nil {
		t.Errorf("newResult(nil, nil).Failures is nil, want an empty slice")
	}
	if !result.Complete {
		t.Errorf("newResult(nil, nil).Complete = false, want true")
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal Result[PullRequest]: %v", err)
	}

	var envelope struct {
		Items    json.RawMessage `json:"items"`
		Failures json.RawMessage `json:"failures"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	if string(envelope.Items) != "[]" {
		t.Errorf("items = %s, want [] (raw: %s)", envelope.Items, raw)
	}
	if string(envelope.Failures) != "[]" {
		t.Errorf("failures = %s, want [] (raw: %s)", envelope.Failures, raw)
	}
}
