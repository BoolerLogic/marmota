package bridge

import (
	"marmota/internal/utils"
	"sync"
)

type HTTPRequestDetail struct {
	ID            uint64 `json:"id"`
	Host          string `json:"host"`
	Port          string `json:"port"`
	HeadBlockStr  string `json:"headBlockStr"`
	BodyStr       string `json:"bodyStr"`
	TruncatedBody bool   `json:"truncatedBody"`
	Version       string `json:"version"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	Scheme        string `json:"scheme"`
}

type HTTPResponseDetail struct {
	ID                          uint64   `json:"id"`
	Host                        string   `json:"host"`
	Port                        string   `json:"port"`
	HeadBlockStr                string   `json:"headBlockStr"`
	BodyStr                     string   `json:"bodyStr"`
	TruncatedBody               bool     `json:"truncatedBody"`
	Version                     string   `json:"version"`
	StatusCode                  int      `json:"statusCode"`
	UnsupportedContentEncodings []string `json:"unsupportedContentEncodings"`
	ContentDecodingFailed       bool     `json:"contentDecodingFailed"`
}

type HTTPHistoryEntryDetail struct {
	ID       uint64              `json:"id"`
	Request  *HTTPRequestDetail  `json:"request"`
	Response *HTTPResponseDetail `json:"response"`
}

const MAX_BODY_SIZE = utils.MaxCapturedBodySize

var history = make(map[uint64]*HTTPHistoryEntryDetail, 300)
var historyMu sync.RWMutex

func AddRequestToHistory(req *HTTPRequestDetail) {
	historyMu.Lock()
	entry, exists := history[req.ID]
	if !exists {
		entry = &HTTPHistoryEntryDetail{ID: req.ID}
		history[req.ID] = entry
	}
	entry.Request = req
	historyMu.Unlock()
}

func AddResponseToHistory(res *HTTPResponseDetail) {
	historyMu.Lock()
	entry, exists := history[res.ID]
	if !exists {
		entry = &HTTPHistoryEntryDetail{ID: res.ID}
		history[res.ID] = entry
	}
	entry.Response = res
	historyMu.Unlock()
}

func GetHistoryEntryDetail(id uint64) HTTPHistoryEntryDetail {
	historyMu.RLock()
	entry, exists := history[id]
	if !exists || entry == nil {
		historyMu.RUnlock()
		return HTTPHistoryEntryDetail{ID: id}
	}

	detail := HTTPHistoryEntryDetail{ID: entry.ID}
	if entry.Request != nil {
		requestCopy := *entry.Request
		detail.Request = &requestCopy
	}
	if entry.Response != nil {
		responseCopy := *entry.Response
		responseCopy.UnsupportedContentEncodings = append(
			[]string(nil),
			entry.Response.UnsupportedContentEncodings...,
		)
		detail.Response = &responseCopy
	}
	historyMu.RUnlock()

	if detail.Request != nil {
		detail.Request.BodyStr, detail.Request.TruncatedBody =
			truncateHistoryBody(detail.Request.BodyStr)
	}
	if detail.Response != nil {
		detail.Response.BodyStr, detail.Response.TruncatedBody =
			truncateHistoryBody(detail.Response.BodyStr)
	}

	return detail
}

func truncateHistoryBody(body string) (string, bool) {
	if len(body) <= MAX_BODY_SIZE {
		return body, false
	}
	return body[:MAX_BODY_SIZE], true
}

func RemoveHistoryEntry(id uint64) {
	historyMu.Lock()
	defer historyMu.Unlock()

	if _, exists := history[id]; !exists {
		return
	}
	delete(history, id)
}

func ClearHistoryEntries() {
	historyMu.Lock()
	clear(history)
	historyMu.Unlock()
}
