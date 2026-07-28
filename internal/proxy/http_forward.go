package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"marmota/internal/bridge"
	"marmota/internal/utils"

	xproxy "golang.org/x/net/proxy"
)

const (
	httpResponseHeaderTimeout = 60 * time.Second
	maxForwardResponseHeaders = 1 << 20
)

var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

type trackedConnection struct {
	net.Conn
	registry *connectionRegistry
	once     sync.Once
	closeErr error
}

func (conn *trackedConnection) Close() error {
	conn.once.Do(func() {
		conn.registry.remove(conn)
		conn.closeErr = conn.Conn.Close()
	})
	return conn.closeErr
}

type boundedCapture struct {
	mu        sync.Mutex
	data      bytes.Buffer
	limit     int
	truncated bool
}

func newBoundedCapture(contentEncoding string) *boundedCapture {
	limit := utils.MaxCapturedBodySize + 1
	if usesSupportedCompression(contentEncoding) {
		limit = utils.MaxEncodedBodySize + 1
	}
	return &boundedCapture{limit: limit}
}

func usesSupportedCompression(contentEncoding string) bool {
	if len(utils.UnsupportedContentEncodings(contentEncoding)) != 0 {
		return false
	}
	for _, value := range strings.Split(contentEncoding, ",") {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "gzip", "deflate", "br", "zstd":
			return true
		}
	}
	return false
}

func (capture *boundedCapture) Write(data []byte) (int, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()

	remaining := capture.limit - capture.data.Len()
	if remaining > 0 {
		if remaining > len(data) {
			remaining = len(data)
		}
		_, _ = capture.data.Write(data[:remaining])
	}
	if remaining < len(data) {
		capture.truncated = true
	}
	return len(data), nil
}

func (capture *boundedCapture) snapshot() ([]byte, bool) {
	capture.mu.Lock()
	defer capture.mu.Unlock()

	return append([]byte(nil), capture.data.Bytes()...), capture.truncated
}

type capturingReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func (body *capturingReadCloser) Read(data []byte) (int, error) {
	return body.reader.Read(data)
}

func (body *capturingReadCloser) Close() error {
	return body.closer.Close()
}

func newHTTPForwardTransport(
	outboundDialer xproxy.Dialer,
	connections *connectionRegistry,
) *http.Transport {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)

	return &http.Transport{
		Proxy: nil,
		DialContext: func(
			ctx context.Context,
			network string,
			address string,
		) (net.Conn, error) {
			conn, err := dialOutbound(ctx, outboundDialer, address)
			if err != nil {
				return nil, err
			}

			tracked := &trackedConnection{
				Conn:     conn,
				registry: connections,
			}
			if !connections.add(tracked) {
				return nil, errors.New("the proxy is stopping")
			}
			return tracked, nil
		},
		DisableCompression:     true,
		MaxIdleConns:           100,
		MaxIdleConnsPerHost:    10,
		IdleConnTimeout:        90 * time.Second,
		ResponseHeaderTimeout:  httpResponseHeaderTimeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: maxForwardResponseHeaders,
		ForceAttemptHTTP2:      false,
		Protocols:              protocols,
		WriteBufferSize:        32 * 1024,
		ReadBufferSize:         32 * 1024,
	}
}

func isCleartextHTTP2Request(request *http.Request) bool {
	if request == nil {
		return false
	}
	return request.ProtoMajor >= 2 ||
		(request.Method == "PRI" &&
			request.RequestURI == "*" &&
			request.Proto == "HTTP/2.0")
}

func writeCleartextHTTP2Unsupported(response http.ResponseWriter) {
	response.Header().Set("Connection", "close")
	http.Error(
		response,
		"Cleartext HTTP/2 (h2c) is not currently supported by Marmota. Use HTTP/1.1 or HTTPS.",
		http.StatusHTTPVersionNotSupported,
	)
}

func forwardHTTP1(
	responseWriter http.ResponseWriter,
	request *http.Request,
	config handlerConfig,
) {
	if config.httpTransport == nil {
		http.Error(
			responseWriter,
			"HTTP forwarding is not available.",
			http.StatusInternalServerError,
		)
		bridge.EmitError("HTTP/1.1 forwarding transport is not configured")
		return
	}

	targetURL, host, port, err := validateHTTPForwardTarget(request)
	if err != nil {
		http.Error(responseWriter, err.Error(), http.StatusBadRequest)
		bridge.EmitError(fmt.Sprintf("Rejected HTTP/1.1 proxy request: %v", err))
		return
	}

	requestContext, cancelRequest := contextBoundToProxyLifecycle(
		config.lifecycleContext,
		request.Context(),
	)
	defer cancelRequest()

	outboundRequest := request.Clone(requestContext)
	outboundRequest.URL = targetURL
	outboundRequest.RequestURI = ""
	outboundRequest.Host = targetURL.Host
	outboundRequest.Close = false
	outboundRequest.Header = request.Header.Clone()
	outboundRequest.Header.Del("Host")

	requestUpgrade := upgradeType(outboundRequest.Header)
	removeHopByHopHeaders(outboundRequest.Header)
	if requestUpgrade != "" {
		outboundRequest.Header.Set("Connection", "Upgrade")
		outboundRequest.Header.Set("Upgrade", requestUpgrade)
	}

	requestContentEncoding := strings.Join(
		outboundRequest.Header.Values("Content-Encoding"),
		",",
	)
	requestCapture := newBoundedCapture(requestContentEncoding)
	if outboundRequest.Body != nil && outboundRequest.Body != http.NoBody {
		outboundRequest.Body = &capturingReadCloser{
			reader: io.TeeReader(outboundRequest.Body, requestCapture),
			closer: outboundRequest.Body,
		}
	}

	id := newAtomicID()
	requestReceivedAtMs := time.Now().UnixMilli()
	requestHead := buildHTTPRequestHead(outboundRequest)
	requestPath := targetURL.RequestURI()
	if requestPath == "" {
		requestPath = "/"
	}
	initialRequestDetail := bridge.HTTPRequestDetail{
		ID:           id,
		Host:         host,
		Port:         port,
		Version:      request.Proto,
		Method:       request.Method,
		Path:         requestPath,
		Scheme:       "http",
		HeadBlockStr: requestHead,
	}

	var finalizeRequestOnce sync.Once
	finalizeRequest := func() {
		finalizeRequestOnce.Do(func() {
			body, _ := requestCapture.snapshot()
			bodyString, decodeErr := utils.ExtractAndDecompressBodyHTTP(
				requestContentEncoding,
				bytes.NewReader(body),
			)
			if decodeErr != nil {
				bridge.EmitError(fmt.Sprintf(
					"Could not decode the HTTP/1.1 request body for inspection. ID: %d. Raw encoded data was preserved. Error: %v",
					id,
					decodeErr,
				))
			}

			finalRequestDetail := initialRequestDetail
			finalRequestDetail.BodyStr = bodyString
			bridge.AddRequestToHistory(&finalRequestDetail)
			bridge.EmitHTTPRequestSummary(bridge.HTTPRequestSummary{
				ID:           id,
				Host:         host,
				Port:         port,
				Version:      request.Proto,
				Method:       request.Method,
				Path:         requestPath,
				Scheme:       "http",
				ReceivedAtMs: requestReceivedAtMs,
			})
		})
	}
	defer finalizeRequest()

	upstreamResponse, err := config.httpTransport.RoundTrip(outboundRequest)
	finalizeRequest()
	if err != nil {
		handleHTTPForwardError(
			responseWriter,
			request,
			config.lifecycleContext,
			targetURL,
			err,
		)
		return
	}
	defer upstreamResponse.Body.Close()

	responseUpgrade := upgradeType(upstreamResponse.Header)
	if upstreamResponse.StatusCode == http.StatusSwitchingProtocols {
		handleHTTPUpgrade(
			responseWriter,
			request,
			upstreamResponse,
			config,
			id,
			host,
			port,
			requestUpgrade,
			responseUpgrade,
		)
		return
	}

	forwardHTTPResponse(
		responseWriter,
		upstreamResponse,
		id,
		host,
		port,
	)
}

func validateHTTPForwardTarget(
	request *http.Request,
) (*url.URL, string, string, error) {
	if request == nil || request.URL == nil {
		return nil, "", "", errors.New("Invalid HTTP proxy request.")
	}
	if request.ProtoMajor != 1 || request.ProtoMinor != 1 {
		return nil, "", "", errors.New("Only HTTP/1.1 is currently supported for cleartext HTTP.")
	}
	if !request.URL.IsAbs() || request.URL.Host == "" {
		return nil, "", "", errors.New(
			"HTTP proxy requests must use an absolute target URL.",
		)
	}
	if !strings.EqualFold(request.URL.Scheme, "http") {
		return nil, "", "", errors.New(
			"Only http:// targets are accepted without CONNECT.",
		)
	}
	if request.URL.User != nil {
		return nil, "", "", errors.New(
			"User information is not allowed in an HTTP proxy target URL.",
		)
	}

	targetURL := *request.URL
	targetURL.Scheme = "http"
	targetURL.Fragment = ""

	host := targetURL.Hostname()
	if host == "" {
		return nil, "", "", errors.New("HTTP proxy target host is invalid.")
	}
	port := targetURL.Port()
	if port == "" {
		port = "80"
	} else {
		portNumber, err := strconv.ParseUint(port, 10, 16)
		if err != nil || portNumber == 0 {
			return nil, "", "", errors.New("HTTP proxy target port is invalid.")
		}
	}

	return &targetURL, host, port, nil
}

func forwardHTTPResponse(
	responseWriter http.ResponseWriter,
	upstreamResponse *http.Response,
	id uint64,
	host string,
	port string,
) {
	responseHeaders := upstreamResponse.Header.Clone()
	removeHopByHopHeaders(responseHeaders)
	copyHeaders(responseWriter.Header(), responseHeaders)
	declareTrailers(responseWriter.Header(), upstreamResponse.Trailer)

	responseHead := buildHTTPResponseHead(upstreamResponse, responseHeaders)
	responseWriter.WriteHeader(upstreamResponse.StatusCode)

	contentEncoding := strings.Join(
		upstreamResponse.Header.Values("Content-Encoding"),
		",",
	)
	responseCapture := newBoundedCapture(contentEncoding)
	_, copyErr := io.Copy(
		responseWriter,
		io.TeeReader(upstreamResponse.Body, responseCapture),
	)
	copyTrailers(responseWriter.Header(), upstreamResponse.Trailer)

	body, _ := responseCapture.snapshot()
	bodyString, decodeErr := utils.ExtractAndDecompressBodyHTTP(
		contentEncoding,
		bytes.NewReader(body),
	)
	contentDecodingFailed := decodeErr != nil
	if decodeErr != nil {
		bridge.EmitError(fmt.Sprintf(
			"Could not decode the HTTP/1.1 response body for inspection. ID: %d. Raw encoded data was preserved. Error: %v",
			id,
			decodeErr,
		))
	}
	if copyErr != nil && !errors.Is(copyErr, context.Canceled) {
		bridge.EmitError(fmt.Sprintf(
			"Could not stream the HTTP/1.1 response to the client. ID: %d. Error: %v",
			id,
			copyErr,
		))
	}

	unsupportedContentEncodings :=
		utils.UnsupportedContentEncodings(contentEncoding)
	responseReceivedAtMs := time.Now().UnixMilli()
	bridge.AddResponseToHistory(&bridge.HTTPResponseDetail{
		ID:                          id,
		Host:                        host,
		Port:                        port,
		Version:                     upstreamResponse.Proto,
		StatusCode:                  upstreamResponse.StatusCode,
		HeadBlockStr:                responseHead,
		BodyStr:                     bodyString,
		UnsupportedContentEncodings: unsupportedContentEncodings,
		ContentDecodingFailed:       contentDecodingFailed,
	})
	bridge.EmitHTTPResponseSummary(bridge.HTTPResponseSummary{
		ID:                          id,
		Host:                        host,
		Port:                        port,
		Version:                     upstreamResponse.Proto,
		StatusCode:                  upstreamResponse.StatusCode,
		ReceivedAtMs:                responseReceivedAtMs,
		UnsupportedContentEncodings: unsupportedContentEncodings,
		ContentDecodingFailed:       contentDecodingFailed,
	})
}

func handleHTTPForwardError(
	responseWriter http.ResponseWriter,
	request *http.Request,
	lifecycleContext context.Context,
	targetURL *url.URL,
	forwardErr error,
) {
	if request.Context().Err() != nil {
		return
	}

	statusCode := http.StatusBadGateway
	message := "Marmota could not reach the HTTP server."
	var networkError net.Error
	if errors.Is(forwardErr, context.DeadlineExceeded) ||
		(errors.As(forwardErr, &networkError) && networkError.Timeout()) {
		statusCode = http.StatusGatewayTimeout
		message = "The HTTP server did not respond in time."
	} else if lifecycleContext.Err() != nil {
		statusCode = http.StatusServiceUnavailable
		message = "The Marmota proxy is stopping."
	}

	http.Error(responseWriter, message, statusCode)
	bridge.EmitError(fmt.Sprintf(
		"Could not forward HTTP/1.1 request to %s: %v",
		targetURL.Redacted(),
		forwardErr,
	))
}

func handleHTTPUpgrade(
	responseWriter http.ResponseWriter,
	request *http.Request,
	upstreamResponse *http.Response,
	config handlerConfig,
	id uint64,
	host string,
	port string,
	requestUpgrade string,
	responseUpgrade string,
) {
	if requestUpgrade == "" ||
		!strings.EqualFold(requestUpgrade, responseUpgrade) {
		http.Error(
			responseWriter,
			"Invalid HTTP/1.1 protocol upgrade response.",
			http.StatusBadGateway,
		)
		bridge.EmitError(fmt.Sprintf(
			"Rejected mismatched HTTP/1.1 upgrade for history ID %d: requested %q, received %q",
			id,
			requestUpgrade,
			responseUpgrade,
		))
		return
	}

	upstreamConnection, ok := upstreamResponse.Body.(io.ReadWriteCloser)
	if !ok {
		http.Error(
			responseWriter,
			"HTTP/1.1 protocol upgrade is unavailable.",
			http.StatusBadGateway,
		)
		bridge.EmitError(fmt.Sprintf(
			"Upstream HTTP/1.1 upgrade body is not bidirectional. ID: %d",
			id,
		))
		return
	}

	hijacker, ok := responseWriter.(http.Hijacker)
	if !ok {
		http.Error(
			responseWriter,
			"HTTP/1.1 protocol upgrade is unavailable.",
			http.StatusInternalServerError,
		)
		return
	}

	clientConnection, clientBuffer, err := hijacker.Hijack()
	if err != nil {
		bridge.EmitError(fmt.Sprintf(
			"Could not hijack the HTTP/1.1 upgrade connection. ID: %d. Error: %v",
			id,
			err,
		))
		return
	}
	if !config.connections.add(clientConnection) {
		return
	}
	defer config.connections.remove(clientConnection)
	defer clientConnection.Close()

	responseHeaders := upstreamResponse.Header.Clone()
	removeHopByHopHeaders(responseHeaders)
	responseHeaders.Set("Connection", "Upgrade")
	responseHeaders.Set("Upgrade", responseUpgrade)
	responseHead := buildHTTPResponseHead(upstreamResponse, responseHeaders)
	if _, err := clientBuffer.WriteString(responseHead); err != nil {
		return
	}
	if err := clientBuffer.Flush(); err != nil {
		return
	}

	responseReceivedAtMs := time.Now().UnixMilli()
	bridge.AddResponseToHistory(&bridge.HTTPResponseDetail{
		ID:           id,
		Host:         host,
		Port:         port,
		Version:      upstreamResponse.Proto,
		StatusCode:   upstreamResponse.StatusCode,
		HeadBlockStr: responseHead,
	})
	bridge.EmitHTTPResponseSummary(bridge.HTTPResponseSummary{
		ID:           id,
		Host:         host,
		Port:         port,
		Version:      upstreamResponse.Proto,
		StatusCode:   upstreamResponse.StatusCode,
		ReceivedAtMs: responseReceivedAtMs,
	})

	copyDone := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstreamConnection, clientBuffer)
		copyDone <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(clientConnection, upstreamConnection)
		copyDone <- struct{}{}
	}()

	<-copyDone
	_ = upstreamConnection.Close()
	_ = clientConnection.Close()
	<-copyDone
}

func removeHopByHopHeaders(headers http.Header) {
	for _, connectionValue := range headers.Values("Connection") {
		for _, token := range strings.Split(connectionValue, ",") {
			if token = strings.TrimSpace(token); token != "" {
				headers.Del(token)
			}
		}
	}
	for _, header := range hopByHopHeaders {
		headers.Del(header)
	}
}

func upgradeType(headers http.Header) string {
	return strings.TrimSpace(headers.Get("Upgrade"))
}

func copyHeaders(destination http.Header, source http.Header) {
	for name, values := range source {
		for _, value := range values {
			destination.Add(name, value)
		}
	}
}

func declareTrailers(headers http.Header, trailers http.Header) {
	for name := range trailers {
		headers.Add("Trailer", name)
	}
}

func copyTrailers(headers http.Header, trailers http.Header) {
	for name, values := range trailers {
		headers[http.TrailerPrefix+name] = append([]string(nil), values...)
	}
}

func buildHTTPRequestHead(request *http.Request) string {
	var output bytes.Buffer
	path := request.URL.RequestURI()
	if path == "" {
		path = "/"
	}
	headers := request.Header.Clone()
	headers.Del("Host")
	if request.Body != nil &&
		request.Body != http.NoBody &&
		request.ContentLength >= 0 &&
		headers.Get("Content-Length") == "" {
		headers.Set("Content-Length", strconv.FormatInt(request.ContentLength, 10))
	}
	if len(request.TransferEncoding) > 0 {
		headers.Set("Transfer-Encoding", strings.Join(request.TransferEncoding, ", "))
	}
	fmt.Fprintf(
		&output,
		"%s %s %s\r\nHost: %s\r\n",
		request.Method,
		path,
		request.Proto,
		request.Host,
	)
	_ = headers.Write(&output)
	output.WriteString("\r\n")
	return output.String()
}

func buildHTTPResponseHead(
	response *http.Response,
	headers http.Header,
) string {
	var output bytes.Buffer
	displayHeaders := headers.Clone()
	if len(response.TransferEncoding) > 0 {
		displayHeaders.Set(
			"Transfer-Encoding",
			strings.Join(response.TransferEncoding, ", "),
		)
	}
	statusText := strings.TrimSpace(strings.TrimPrefix(
		response.Status,
		strconv.Itoa(response.StatusCode),
	))
	if statusText == "" {
		statusText = http.StatusText(response.StatusCode)
	}
	fmt.Fprintf(
		&output,
		"%s %d %s\r\n",
		response.Proto,
		response.StatusCode,
		statusText,
	)
	_ = displayHeaders.Write(&output)
	output.WriteString("\r\n")
	return output.String()
}
