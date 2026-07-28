package bridge

import (
	"strings"
	"testing"
	"time"
)

func TestGetHistoryEntryDetailSupportsPendingResponse(t *testing.T) {
	ClearHistoryEntries()
	t.Cleanup(ClearHistoryEntries)

	AddRequestToHistory(&HTTPRequestDetail{
		ID:      1,
		BodyStr: "request",
	})

	detail := GetHistoryEntryDetail(1)
	if detail.Request == nil {
		t.Fatal("request detail is missing")
	}
	if detail.Response != nil {
		t.Fatal("pending request unexpectedly has a response")
	}
}

func TestHistoryMergesResponseThatArrivesBeforeRequest(t *testing.T) {
	ClearHistoryEntries()
	t.Cleanup(ClearHistoryEntries)

	AddResponseToHistory(&HTTPResponseDetail{
		ID:         2,
		StatusCode: 204,
	})
	AddRequestToHistory(&HTTPRequestDetail{
		ID:     2,
		Method: "GET",
	})

	detail := GetHistoryEntryDetail(2)
	if detail.Request == nil || detail.Request.Method != "GET" {
		t.Fatal("request was not merged into the existing history entry")
	}
	if detail.Response == nil || detail.Response.StatusCode != 204 {
		t.Fatal("response was lost when the request arrived later")
	}
}

func TestGetHistoryEntryDetailTruncatesCopyWithoutMutatingHistory(t *testing.T) {
	ClearHistoryEntries()
	t.Cleanup(ClearHistoryEntries)

	originalBody := strings.Repeat("a", MAX_BODY_SIZE+128)
	AddRequestToHistory(&HTTPRequestDetail{
		ID:      3,
		BodyStr: originalBody,
	})

	detail := GetHistoryEntryDetail(3)
	if detail.Request == nil {
		t.Fatal("request detail is missing")
	}
	if !detail.Request.TruncatedBody {
		t.Fatal("large body was not marked as truncated")
	}
	if len(detail.Request.BodyStr) != MAX_BODY_SIZE {
		t.Fatalf(
			"truncated body length = %d, want %d",
			len(detail.Request.BodyStr),
			MAX_BODY_SIZE,
		)
	}

	historyMu.RLock()
	storedBody := history[3].Request.BodyStr
	historyMu.RUnlock()
	if storedBody != originalBody {
		t.Fatal("reading a detail mutated the body stored in history")
	}
}

func TestGetHistoryEntryDetailMissingEntryIsSafe(t *testing.T) {
	ClearHistoryEntries()
	t.Cleanup(ClearHistoryEntries)

	detail := GetHistoryEntryDetail(999)
	if detail.ID != 999 || detail.Request != nil || detail.Response != nil {
		t.Fatalf("unexpected missing-entry detail: %#v", detail)
	}
}

func TestRemoveMissingHistoryEntryReleasesLock(t *testing.T) {
	ClearHistoryEntries()
	t.Cleanup(ClearHistoryEntries)

	done := make(chan struct{})
	go func() {
		RemoveHistoryEntry(404)
		AddRequestToHistory(&HTTPRequestDetail{ID: 5})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("removing a missing history entry left the history lock held")
	}
}

func TestFilterEvaluationIncludesMatchingAndEvaluatedVersions(t *testing.T) {
	ClearHistoryEntries()
	ClearActiveHistoryFilters()
	t.Cleanup(ClearHistoryEntries)
	t.Cleanup(ClearActiveHistoryFilters)

	AddRequestToHistory(&HTTPRequestDetail{
		ID:   6,
		Host: "example.test",
	})
	UpsertActiveHistoryFilter(UpsertActiveHistoryFilterParams{
		FilterID: "host-filter",
		Version:  7,
		Conditions: []HistoryFilterCondition{
			{
				Query:     "example.test",
				Target:    HistoryFilterTargetHost,
				MatchMode: HistoryMatchEquals,
			},
		},
		Operator: HistoryFilterAnd,
	})

	matches, evaluated := getActiveFilterEvaluationForEntryID(6)
	if len(matches) != 1 ||
		matches[0].FilterID != "host-filter" ||
		matches[0].Version != 7 {
		t.Fatalf("unexpected matches: %#v", matches)
	}
	if len(evaluated) != 1 ||
		evaluated[0].FilterID != "host-filter" ||
		evaluated[0].Version != 7 {
		t.Fatalf("unexpected evaluated filters: %#v", evaluated)
	}

	UpsertActiveHistoryFilter(UpsertActiveHistoryFilterParams{
		FilterID: "host-filter",
		Version:  8,
		Conditions: []HistoryFilterCondition{
			{
				Query:     "different.test",
				Target:    HistoryFilterTargetHost,
				MatchMode: HistoryMatchEquals,
			},
		},
		Operator: HistoryFilterAnd,
	})
	matches, evaluated = getActiveFilterEvaluationForEntryID(6)
	if len(matches) != 0 {
		t.Fatalf("non-matching filter unexpectedly matched: %#v", matches)
	}
	if len(evaluated) != 1 || evaluated[0].Version != 8 {
		t.Fatalf("non-matching filter was not reported as evaluated: %#v", evaluated)
	}
}
