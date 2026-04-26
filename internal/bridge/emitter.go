package bridge

import (
	"context"
	"fmt"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var ctx context.Context

func Init(c context.Context) {
	ctx = c
}

func emit(event string, data any) {
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, event, data)
}

func EmitError(m string) {
	emit("error", m)
	log.Println(m)
}

type HTTPRequestSummary struct {
	ID            uint64               `json:"id"`
	Host          string               `json:"host"`
	Port          string               `json:"port"`
	Version       string               `json:"version"`
	Method        string               `json:"method"`
	Path          string               `json:"path"`
	Scheme        string               `json:"scheme"` // "http" o "https"
	ReceivedAtMs  int64                `json:"receivedAtMs"`
	FilterMatches []HistoryFilterMatch `json:"filterMatches"`
}

type HTTPResponseSummary struct {
	ID            uint64               `json:"id"`
	Host          string               `json:"host"`
	Port          string               `json:"port"`
	Version       string               `json:"version"`
	StatusCode    int                  `json:"statusCode"`
	ReceivedAtMs  int64                `json:"receivedAtMs"`
	FilterMatches []HistoryFilterMatch `json:"filterMatches"`
}

func EmitHTTPRequestSummary(request HTTPRequestSummary) {
	request.FilterMatches = getActiveFilterMatchesForEntryID(request.ID)
	emit("request", request)
}

func EmitHTTPResponseSummary(response HTTPResponseSummary) {
	response.FilterMatches = getActiveFilterMatchesForEntryID(response.ID)
	emit("response", response)
}

func getActiveFilterMatchesForEntryID(entryID uint64) []HistoryFilterMatch {
	historyMu.RLock()
	entry, ok := history[entryID]
	if !ok {
		panic(fmt.Sprintf("Could not find an entry for entryId %d in getActiveFilterMatchesForEntryID(...)", entryID))
	}
	historyMu.RUnlock()

	if entry == nil {
		return []HistoryFilterMatch{}
	}

	activeFilters := snapshotActiveHistoryFilters()
	if len(activeFilters) == 0 {
		return []HistoryFilterMatch{}
	}

	candidateCache := make(map[HistoryFilterTarget]string, 8)
	matches := make([]HistoryFilterMatch, 0, len(activeFilters))

	for _, filter := range activeFilters {
		if entryMatchesCompiledHistoryFilters(entry, filter, candidateCache) {
			matches = append(matches, HistoryFilterMatch{
				FilterID: filter.filterID,
				Version:  filter.version,
			})
		}
	}

	return matches
}
