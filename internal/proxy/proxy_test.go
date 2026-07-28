package proxy

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	xproxy "golang.org/x/net/proxy"
)

func TestBuildOutboundDialerDirect(t *testing.T) {
	dialer, err := buildOutboundDialer(UpstreamProxyConfig{})
	if err != nil {
		t.Fatalf("build direct outbound dialer: %v", err)
	}
	if _, ok := dialer.(xproxy.ContextDialer); !ok {
		t.Fatal("direct outbound dialer must support context cancellation")
	}
}

func TestBuildOutboundDialerSOCKS5(t *testing.T) {
	tests := []struct {
		name   string
		config UpstreamProxyConfig
	}{
		{
			name: "without authentication",
			config: UpstreamProxyConfig{
				Enabled: true,
				Host:    "proxy.example",
				Port:    1080,
			},
		},
		{
			name: "with authentication",
			config: UpstreamProxyConfig{
				Enabled:  true,
				Host:     "127.0.0.1",
				Port:     1080,
				Username: "user",
				Password: "password",
			},
		},
		{
			name: "with bracketed IPv6 host",
			config: UpstreamProxyConfig{
				Enabled: true,
				Host:    "[2001:db8::1]",
				Port:    1080,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dialer, err := buildOutboundDialer(test.config)
			if err != nil {
				t.Fatalf("build SOCKS5 outbound dialer: %v", err)
			}
			if _, ok := dialer.(xproxy.ContextDialer); !ok {
				t.Fatal("SOCKS5 outbound dialer must support context cancellation")
			}
		})
	}
}

func TestBuildOutboundDialerRejectsInvalidSOCKS5Config(t *testing.T) {
	tooLongCredential := strings.Repeat("a", 256)
	tests := []struct {
		name   string
		config UpstreamProxyConfig
	}{
		{
			name: "missing host",
			config: UpstreamProxyConfig{
				Enabled: true,
				Port:    1080,
			},
		},
		{
			name: "invalid host",
			config: UpstreamProxyConfig{
				Enabled: true,
				Host:    "https://proxy.example",
				Port:    1080,
			},
		},
		{
			name: "host includes a port",
			config: UpstreamProxyConfig{
				Enabled: true,
				Host:    "proxy.example:1080",
				Port:    1080,
			},
		},
		{
			name: "host exceeds SOCKS5 address limit",
			config: UpstreamProxyConfig{
				Enabled: true,
				Host:    strings.Repeat("a", 256),
				Port:    1080,
			},
		},
		{
			name: "missing port",
			config: UpstreamProxyConfig{
				Enabled: true,
				Host:    "proxy.example",
			},
		},
		{
			name: "username without password",
			config: UpstreamProxyConfig{
				Enabled:  true,
				Host:     "proxy.example",
				Port:     1080,
				Username: "user",
			},
		},
		{
			name: "password without username",
			config: UpstreamProxyConfig{
				Enabled:  true,
				Host:     "proxy.example",
				Port:     1080,
				Password: "password",
			},
		},
		{
			name: "credential exceeds RFC 1929 limit",
			config: UpstreamProxyConfig{
				Enabled:  true,
				Host:     "proxy.example",
				Port:     1080,
				Username: tooLongCredential,
				Password: "password",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := buildOutboundDialer(test.config); err == nil {
				t.Fatal("expected invalid SOCKS5 configuration to fail")
			}
		})
	}
}

func TestNormalizeHost(t *testing.T) {
	tests := map[string]string{
		" proxy.example ": "proxy.example",
		"[2001:db8::1]":   "2001:db8::1",
		"2001:db8::1":     "2001:db8::1",
	}

	for input, expected := range tests {
		if actual := normalizeHost(input); actual != expected {
			t.Fatalf("normalizeHost(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestDialOutboundThroughSOCKS5(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "without authentication"},
		{
			name:     "with username and password",
			username: "bright-user",
			password: "bright-password",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("listen for fake SOCKS5 server: %v", err)
			}
			defer listener.Close()

			resultChan := make(chan fakeSOCKS5Result, 1)
			go serveFakeSOCKS5(
				listener,
				test.username != "",
				resultChan,
			)

			proxyPort := listener.Addr().(*net.TCPAddr).Port
			dialer, err := buildOutboundDialer(UpstreamProxyConfig{
				Enabled:  true,
				Host:     "127.0.0.1",
				Port:     uint16(proxyPort),
				Username: test.username,
				Password: test.password,
			})
			if err != nil {
				t.Fatalf("build SOCKS5 dialer: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			conn, err := dialOutbound(ctx, dialer, "origin.example:443")
			if err != nil {
				t.Fatalf("dial through fake SOCKS5 server: %v", err)
			}
			conn.Close()

			select {
			case result := <-resultChan:
				if result.err != nil {
					t.Fatalf("fake SOCKS5 handshake: %v", result.err)
				}
				if result.targetHost != "origin.example" || result.targetPort != 443 {
					t.Fatalf(
						"SOCKS5 target = %s:%d, want origin.example:443",
						result.targetHost,
						result.targetPort,
					)
				}
				if result.username != test.username || result.password != test.password {
					t.Fatalf(
						"SOCKS5 credentials = %q/%q, want %q/%q",
						result.username,
						result.password,
						test.username,
						test.password,
					)
				}
			case <-ctx.Done():
				t.Fatal("timed out waiting for fake SOCKS5 handshake")
			}
		})
	}
}

func TestConnectionRegistryClosesTrackedAndLateConnections(t *testing.T) {
	t.Parallel()

	registry := newConnectionRegistry()
	trackedConn, trackedPeer := net.Pipe()
	defer trackedPeer.Close()

	if !registry.add(trackedConn) {
		t.Fatal("new registry rejected a connection")
	}
	if err := registry.closeAll(); err != nil {
		t.Fatalf("close tracked connections: %v", err)
	}
	assertConnectionClosed(t, trackedPeer)

	lateConn, latePeer := net.Pipe()
	defer latePeer.Close()
	if registry.add(lateConn) {
		t.Fatal("closed registry accepted a late connection")
	}
	assertConnectionClosed(t, latePeer)

	if err := registry.closeAll(); err != nil {
		t.Fatalf("close registry a second time: %v", err)
	}
}

func TestProxyHandlerReportsOutboundFailureBeforeHijacking(t *testing.T) {
	caCert, caKey, err := genRootCA()
	if err != nil {
		t.Fatalf("generate test CA: %v", err)
	}

	routeErr := errors.New("SOCKS5 authentication rejected")
	lifecycleContext, cancelLifecycle := context.WithCancel(context.Background())
	defer cancelLifecycle()

	response := &hijackTrackingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}
	request := httptest.NewRequest(http.MethodConnect, "http://marmota.invalid", nil)
	request.Host = "origin.example:443"

	proxyHandler(response, request, handlerConfig{
		outboundDialer:   failingContextDialer{err: routeErr},
		connections:      newConnectionRegistry(),
		lifecycleContext: lifecycleContext,
		caCert:           caCert,
		caPrivateKey:     caKey,
	})

	if response.Code != http.StatusBadGateway {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusBadGateway,
		)
	}
	if response.hijackCalled {
		t.Fatal("CONNECT was hijacked before the outbound route succeeded")
	}
}

func TestProxyLifecycleCancelsPendingOutboundDial(t *testing.T) {
	caCert, caKey, err := genRootCA()
	if err != nil {
		t.Fatalf("generate test CA: %v", err)
	}

	dialer := &blockingContextDialer{
		started: make(chan struct{}),
	}
	lifecycleContext, cancelLifecycle := context.WithCancel(context.Background())
	response := &hijackTrackingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}
	request := httptest.NewRequest(http.MethodConnect, "http://marmota.invalid", nil)
	request.Host = "origin.example:443"

	handlerDone := make(chan struct{})
	go func() {
		defer close(handlerDone)
		proxyHandler(response, request, handlerConfig{
			outboundDialer:   dialer,
			connections:      newConnectionRegistry(),
			lifecycleContext: lifecycleContext,
			caCert:           caCert,
			caPrivateKey:     caKey,
		})
	}()

	select {
	case <-dialer.started:
	case <-time.After(3 * time.Second):
		t.Fatal("outbound dial did not start")
	}

	cancelLifecycle()

	select {
	case <-handlerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("pending outbound dial was not canceled with the proxy lifecycle")
	}

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"status = %d, want %d",
			response.Code,
			http.StatusServiceUnavailable,
		)
	}
	if response.hijackCalled {
		t.Fatal("a canceled outbound route still acknowledged CONNECT")
	}
}

type hijackTrackingRecorder struct {
	*httptest.ResponseRecorder
	hijackCalled bool
}

func (recorder *hijackTrackingRecorder) Hijack() (
	net.Conn,
	*bufio.ReadWriter,
	error,
) {
	recorder.hijackCalled = true
	return nil, nil, errors.New("test recorder must not be hijacked")
}

type failingContextDialer struct {
	err error
}

func (dialer failingContextDialer) Dial(string, string) (net.Conn, error) {
	return nil, dialer.err
}

func (dialer failingContextDialer) DialContext(
	context.Context,
	string,
	string,
) (net.Conn, error) {
	return nil, dialer.err
}

type blockingContextDialer struct {
	started chan struct{}
	once    sync.Once
}

func (dialer *blockingContextDialer) Dial(string, string) (net.Conn, error) {
	return nil, errors.New("blocking dialer requires DialContext")
}

func (dialer *blockingContextDialer) DialContext(
	ctx context.Context,
	network string,
	address string,
) (net.Conn, error) {
	dialer.once.Do(func() {
		close(dialer.started)
	})
	<-ctx.Done()
	return nil, ctx.Err()
}

func assertConnectionClosed(t *testing.T, conn net.Conn) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		// net.Pipe may report the peer close immediately while setting the
		// deadline, which already proves the tracked side was closed.
		return
	}
	buffer := make([]byte, 1)
	if _, err := conn.Read(buffer); err == nil {
		t.Fatal("peer remained readable after its tracked connection closed")
	}
}

type fakeSOCKS5Result struct {
	targetHost string
	targetPort uint16
	username   string
	password   string
	err        error
}

func serveFakeSOCKS5(
	listener net.Listener,
	requireAuthentication bool,
	resultChan chan<- fakeSOCKS5Result,
) {
	result := fakeSOCKS5Result{}
	defer func() {
		resultChan <- result
	}()

	conn, err := listener.Accept()
	if err != nil {
		result.err = err
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	greeting := make([]byte, 2)
	if _, err := io.ReadFull(conn, greeting); err != nil {
		result.err = fmt.Errorf("read greeting: %w", err)
		return
	}
	if greeting[0] != 5 {
		result.err = fmt.Errorf("unexpected SOCKS version %d", greeting[0])
		return
	}

	methods := make([]byte, int(greeting[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		result.err = fmt.Errorf("read authentication methods: %w", err)
		return
	}

	selectedMethod := byte(0)
	if requireAuthentication {
		selectedMethod = 2
	}
	if !containsByte(methods, selectedMethod) {
		result.err = fmt.Errorf("client did not offer authentication method %d", selectedMethod)
		return
	}
	if _, err := conn.Write([]byte{5, selectedMethod}); err != nil {
		result.err = fmt.Errorf("write selected authentication method: %w", err)
		return
	}

	if requireAuthentication {
		result.username, result.password, err = readSOCKS5Credentials(conn)
		if err != nil {
			result.err = err
			return
		}
		if _, err := conn.Write([]byte{1, 0}); err != nil {
			result.err = fmt.Errorf("write authentication result: %w", err)
			return
		}
	}

	result.targetHost, result.targetPort, err = readSOCKS5ConnectRequest(conn)
	if err != nil {
		result.err = err
		return
	}

	_, result.err = conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
}

func readSOCKS5Credentials(conn net.Conn) (string, string, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", "", fmt.Errorf("read authentication header: %w", err)
	}
	if header[0] != 1 {
		return "", "", fmt.Errorf("unexpected authentication version %d", header[0])
	}

	username := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, username); err != nil {
		return "", "", fmt.Errorf("read username: %w", err)
	}

	passwordLength := make([]byte, 1)
	if _, err := io.ReadFull(conn, passwordLength); err != nil {
		return "", "", fmt.Errorf("read password length: %w", err)
	}
	password := make([]byte, int(passwordLength[0]))
	if _, err := io.ReadFull(conn, password); err != nil {
		return "", "", fmt.Errorf("read password: %w", err)
	}

	return string(username), string(password), nil
}

func readSOCKS5ConnectRequest(conn net.Conn) (string, uint16, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", 0, fmt.Errorf("read CONNECT header: %w", err)
	}
	if header[0] != 5 || header[1] != 1 {
		return "", 0, fmt.Errorf(
			"unexpected CONNECT version/command %d/%d",
			header[0],
			header[1],
		)
	}

	var host string
	switch header[3] {
	case 1:
		address := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(conn, address); err != nil {
			return "", 0, fmt.Errorf("read IPv4 target: %w", err)
		}
		host = net.IP(address).String()
	case 3:
		length := make([]byte, 1)
		if _, err := io.ReadFull(conn, length); err != nil {
			return "", 0, fmt.Errorf("read domain length: %w", err)
		}
		address := make([]byte, int(length[0]))
		if _, err := io.ReadFull(conn, address); err != nil {
			return "", 0, fmt.Errorf("read domain target: %w", err)
		}
		host = string(address)
	case 4:
		address := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(conn, address); err != nil {
			return "", 0, fmt.Errorf("read IPv6 target: %w", err)
		}
		host = net.IP(address).String()
	default:
		return "", 0, fmt.Errorf("unsupported target address type %d", header[3])
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return "", 0, fmt.Errorf("read target port: %w", err)
	}

	return host, binary.BigEndian.Uint16(portBytes), nil
}

func containsByte(values []byte, wanted byte) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
