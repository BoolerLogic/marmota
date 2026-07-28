package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"marmota/internal/bridge"

	"github.com/andybalholm/brotli"
)

type forwardedRequestObservation struct {
	host               string
	requestURI         string
	body               string
	proxyAuthorization string
	removedHopHeader   string
	endToEndHeader     string
}

type recordingHTTPDialer struct {
	dialer    net.Dialer
	addresses chan string
	once      sync.Once
}

func newRecordingHTTPDialer() *recordingHTTPDialer {
	return &recordingHTTPDialer{
		dialer: net.Dialer{
			Timeout: 3 * time.Second,
		},
		addresses: make(chan string, 1),
	}
}

func (dialer *recordingHTTPDialer) Dial(
	network string,
	address string,
) (net.Conn, error) {
	return dialer.DialContext(context.Background(), network, address)
}

func (dialer *recordingHTTPDialer) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	dialer.once.Do(func() {
		dialer.addresses <- address
	})
	return dialer.dialer.DialContext(ctx, network, address)
}

func TestForwardHTTP1RequestAndCaptureHistory(t *testing.T) {
	bridge.ClearHistoryEntries()
	t.Cleanup(bridge.ClearHistoryEntries)

	observedRequest := make(chan forwardedRequestObservation, 1)
	origin := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read forwarded request body: %v", err)
			return
		}
		observedRequest <- forwardedRequestObservation{
			host:               request.Host,
			requestURI:         request.RequestURI,
			body:               string(body),
			proxyAuthorization: request.Header.Get("Proxy-Authorization"),
			removedHopHeader:   request.Header.Get("X-Remove-Me"),
			endToEndHeader:     request.Header.Get("X-End-To-End"),
		}

		response.Header().Set("Content-Type", "text/plain")
		response.Header().Set("X-Origin", "visible")
		response.Header().Set("Connection", "X-Origin-Hop")
		response.Header().Set("X-Origin-Hop", "must-not-leak")
		response.WriteHeader(http.StatusCreated)
		_, _ = response.Write([]byte("forwarded response"))
	}))
	defer origin.Close()

	originAddress := strings.TrimPrefix(origin.URL, "http://")
	dialer := newRecordingHTTPDialer()
	registry := newConnectionRegistry()
	lifecycleContext, cancelLifecycle := context.WithCancel(context.Background())
	transport := newHTTPForwardTransport(dialer, registry)
	t.Cleanup(func() {
		cancelLifecycle()
		transport.CloseIdleConnections()
		_ = registry.closeAll()
	})

	config := handlerConfig{
		outboundDialer:   dialer,
		httpTransport:    transport,
		connections:      registry,
		lifecycleContext: lifecycleContext,
	}
	marmota := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		proxyHandler(response, request, config)
	}))
	defer marmota.Close()

	id := globalID.Load() + 1
	proxyConnection, err := net.DialTimeout(
		"tcp",
		marmota.Listener.Addr().String(),
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("connect to Marmota test proxy: %v", err)
	}
	defer proxyConnection.Close()

	rawRequest := fmt.Sprintf(
		"POST %s/submit?q=1 HTTP/1.1\r\n"+
			"Host: wrong.example\r\n"+
			"Proxy-Authorization: Basic c2VjcmV0\r\n"+
			"Connection: X-Remove-Me\r\n"+
			"X-Remove-Me: must-not-leak\r\n"+
			"X-End-To-End: preserved\r\n"+
			"Content-Type: text/plain\r\n"+
			"Content-Length: 15\r\n\r\n"+
			"forwarded body!",
		origin.URL,
	)
	if _, err := io.WriteString(proxyConnection, rawRequest); err != nil {
		t.Fatalf("write HTTP proxy request: %v", err)
	}

	response, err := http.ReadResponse(
		bufio.NewReader(proxyConnection),
		&http.Request{Method: http.MethodPost},
	)
	if err != nil {
		t.Fatalf("read HTTP proxy response: %v", err)
	}
	responseBody, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatalf("read HTTP proxy response body: %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("response status = %d, want 201", response.StatusCode)
	}
	if string(responseBody) != "forwarded response" {
		t.Fatalf("response body = %q", responseBody)
	}
	if response.Header.Get("X-Origin") != "visible" {
		t.Fatalf("end-to-end response header was not forwarded")
	}
	if response.Header.Get("X-Origin-Hop") != "" {
		t.Fatalf("hop-by-hop response header leaked to the client")
	}

	select {
	case observation := <-observedRequest:
		if observation.host != originAddress {
			t.Fatalf(
				"origin Host = %q, want absolute target authority %q",
				observation.host,
				originAddress,
			)
		}
		if observation.requestURI != "/submit?q=1" {
			t.Fatalf("origin request URI = %q", observation.requestURI)
		}
		if observation.body != "forwarded body!" {
			t.Fatalf("origin body = %q", observation.body)
		}
		if observation.proxyAuthorization != "" ||
			observation.removedHopHeader != "" {
			t.Fatalf("proxy-only headers leaked to the origin: %#v", observation)
		}
		if observation.endToEndHeader != "preserved" {
			t.Fatalf("end-to-end request header was not preserved")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("origin did not receive the forwarded request")
	}

	select {
	case address := <-dialer.addresses:
		if address != originAddress {
			t.Fatalf("outbound dial target = %q, want %q", address, originAddress)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("configured outbound dialer was not used")
	}

	var detail bridge.HTTPHistoryEntryDetail
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		detail = bridge.GetHistoryEntryDetail(id)
		if detail.Request != nil && detail.Response != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if detail.Request == nil || detail.Response == nil {
		t.Fatalf("HTTP transaction was not fully captured: %#v", detail)
	}
	if detail.Request.Scheme != "http" ||
		detail.Request.Path != "/submit?q=1" ||
		detail.Request.BodyStr != "forwarded body!" {
		t.Fatalf("captured request = %#v", detail.Request)
	}
	if strings.Contains(
		strings.ToLower(detail.Request.HeadBlockStr),
		"proxy-authorization",
	) || strings.Contains(detail.Request.HeadBlockStr, "X-Remove-Me") {
		t.Fatalf(
			"captured request head contains proxy-only headers: %q",
			detail.Request.HeadBlockStr,
		)
	}
	if detail.Response.StatusCode != http.StatusCreated ||
		detail.Response.BodyStr != "forwarded response" {
		t.Fatalf("captured response = %#v", detail.Response)
	}
}

func TestCleartextHTTP2ReturnsExplicitUnsupportedMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		response http.ResponseWriter,
		request *http.Request,
	) {
		proxyHandler(response, request, handlerConfig{})
	}))
	defer server.Close()

	conn, err := net.DialTimeout(
		"tcp",
		server.Listener.Addr().String(),
		3*time.Second,
	)
	if err != nil {
		t.Fatalf("connect to test proxy: %v", err)
	}
	defer conn.Close()

	if _, err := io.WriteString(
		conn,
		"PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n",
	); err != nil {
		t.Fatalf("write HTTP/2 cleartext preface: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	rawResponse, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read unsupported HTTP/2 response: %v", err)
	}
	responseText := string(rawResponse)
	if !strings.Contains(responseText, "505 HTTP Version Not Supported") {
		t.Fatalf("unexpected status response: %q", responseText)
	}
	if !strings.Contains(
		responseText,
		"Cleartext HTTP/2 (h2c) is not currently supported by Marmota.",
	) {
		t.Fatalf("missing explicit h2c message: %q", responseText)
	}
}

func TestHTTPForwardRejectsOriginFormRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/relative", nil)
	response := httptest.NewRecorder()
	lifecycleContext, cancelLifecycle := context.WithCancel(context.Background())
	defer cancelLifecycle()

	dialer := newRecordingHTTPDialer()
	registry := newConnectionRegistry()
	transport := newHTTPForwardTransport(dialer, registry)
	defer transport.CloseIdleConnections()
	defer registry.closeAll()

	proxyHandler(response, request, handlerConfig{
		httpTransport:    transport,
		connections:      registry,
		lifecycleContext: lifecycleContext,
	})

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	if !strings.Contains(
		response.Body.String(),
		"must use an absolute target URL",
	) {
		t.Fatalf("unexpected response body: %q", response.Body.String())
	}
}

func TestForwardHTTPResponsePreservesBrotliAndDecodesHistory(t *testing.T) {
	bridge.ClearHistoryEntries()
	t.Cleanup(bridge.ClearHistoryEntries)

	const decodedBody = "<html><body>Brotli over HTTP</body></html>"
	var encodedBody bytes.Buffer
	encoder := brotli.NewWriter(&encodedBody)
	if _, err := encoder.Write([]byte(decodedBody)); err != nil {
		t.Fatalf("encode Brotli response: %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("close Brotli encoder: %v", err)
	}

	id := newAtomicID()
	upstreamResponse := &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(encodedBody.Bytes())),
		ContentLength: int64(encodedBody.Len()),
	}
	upstreamResponse.Header.Set("Content-Encoding", "br")
	upstreamResponse.Header.Set(
		"Content-Length",
		fmt.Sprintf("%d", encodedBody.Len()),
	)

	response := httptest.NewRecorder()
	forwardHTTPResponse(response, upstreamResponse, id, "example.test", "80")

	if !bytes.Equal(response.Body.Bytes(), encodedBody.Bytes()) {
		t.Fatal("encoded response body was modified while forwarding")
	}
	detail := bridge.GetHistoryEntryDetail(id)
	if detail.Response == nil {
		t.Fatal("forwarded Brotli response was not captured")
	}
	if detail.Response.BodyStr != decodedBody {
		t.Fatalf(
			"decoded history body = %q, want %q",
			detail.Response.BodyStr,
			decodedBody,
		)
	}
	if detail.Response.ContentDecodingFailed ||
		len(detail.Response.UnsupportedContentEncodings) != 0 {
		t.Fatalf("valid Brotli response was marked as unsupported: %#v", detail.Response)
	}
}

func TestForwardHTTPResponseBoundsHistoryWithoutTruncatingClient(t *testing.T) {
	bridge.ClearHistoryEntries()
	t.Cleanup(bridge.ClearHistoryEntries)

	body := bytes.Repeat([]byte("x"), bridge.MAX_BODY_SIZE+64*1024)
	id := newAtomicID()
	upstreamResponse := &http.Response{
		Status:        "200 OK",
		StatusCode:    http.StatusOK,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        make(http.Header),
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}
	upstreamResponse.Header.Set(
		"Content-Length",
		fmt.Sprintf("%d", len(body)),
	)

	response := httptest.NewRecorder()
	forwardHTTPResponse(response, upstreamResponse, id, "example.test", "80")

	if !bytes.Equal(response.Body.Bytes(), body) {
		t.Fatal("large response body was truncated while forwarding")
	}
	detail := bridge.GetHistoryEntryDetail(id)
	if detail.Response == nil {
		t.Fatal("large forwarded response was not captured")
	}
	if !detail.Response.TruncatedBody {
		t.Fatal("large history response was not marked as truncated")
	}
	if len(detail.Response.BodyStr) != bridge.MAX_BODY_SIZE {
		t.Fatalf(
			"captured response length = %d, want %d",
			len(detail.Response.BodyStr),
			bridge.MAX_BODY_SIZE,
		)
	}
}
