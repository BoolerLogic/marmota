package bridge

import "sync"

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
	ID            uint64 `json:"id"`
	Host          string `json:"host"`
	Port          string `json:"port"`
	HeadBlockStr  string `json:"headBlockStr"`
	BodyStr       string `json:"bodyStr"`
	TruncatedBody bool   `json:"truncatedBody"`
	Version       string `json:"version"`
	StatusCode    int    `json:"statusCode"`
}

type HTTPHistoryEntryDetail struct {
	ID       uint64              `json:"id"`
	Request  *HTTPRequestDetail  `json:"request"`
	Response *HTTPResponseDetail `json:"response"`
}

var MAX_BODY_SIZE = 500 * 1024 // 500 KB

var history = make(map[uint64]*HTTPHistoryEntryDetail, 300)
var historyMu sync.RWMutex

func AddRequestToHistory(req *HTTPRequestDetail) {
	historyMu.Lock()
	history[req.ID] = &HTTPHistoryEntryDetail{
		ID:       req.ID,
		Request:  req,
		Response: nil,
	}
	historyMu.Unlock()
}

func AddResponseToHistory(res *HTTPResponseDetail) {
	historyMu.Lock()
	entry, ok := history[res.ID]
	if !ok {
		panic(" > Received a response with an identifier that does not exist")
	}
	entry.Response = res
	historyMu.Unlock()
}

func GetHistoryEntryDetail(id uint64) HTTPHistoryEntryDetail {
	historyMu.RLock()
	entry, ok := history[id]
	if !ok {
		panic(" > Could not find an HTTPHistoryEntryDetail in the history map")
	}
	historyMu.RUnlock()

	if len(entry.Request.BodyStr) > MAX_BODY_SIZE {
		entry.Request.BodyStr = entry.Request.BodyStr[:MAX_BODY_SIZE]
		entry.Request.TruncatedBody = true
	} else {
		entry.Request.TruncatedBody = false
	}

	if len(entry.Response.BodyStr) > MAX_BODY_SIZE {
		entry.Response.BodyStr = entry.Response.BodyStr[:MAX_BODY_SIZE]
		entry.Response.TruncatedBody = true
	} else {
		entry.Response.TruncatedBody = false
	}

	return *entry
}

func RemoveHistoryEntry(id uint64) {
	historyMu.Lock()
	_, exits := history[id]
	if !exits {
		return
	}
	delete(history, id)
	historyMu.Unlock()
}

func ClearHistoryEntries() {
	historyMu.Lock()
	clear(history)
	historyMu.Unlock()
}
