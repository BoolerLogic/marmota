package proxy

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"marmota/internal/bridge"
	"marmota/internal/h1x"
	"marmota/internal/h2"
	"marmota/internal/utils"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	xproxy "golang.org/x/net/proxy"
)

const (
	outboundConnectTimeout = 30 * time.Second
	tlsHandshakeTimeout    = 30 * time.Second
	inboundHeaderTimeout   = 15 * time.Second
)

type UpstreamProxyConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     uint16 `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type ConfigProxy struct {
	IP                   string              `json:"ip"`
	Port                 uint16              `json:"port"`
	SkipServerCertVerify bool                `json:"skipServerCertVerify"`
	UpstreamProxy        UpstreamProxyConfig `json:"upstreamProxy"`
}

type handlerConfig struct {
	skipServerCertVerify bool
	outboundDialer       xproxy.Dialer
	httpTransport        *http.Transport
	connections          *connectionRegistry
	lifecycleContext     context.Context
	caCert               *x509.Certificate
	caPrivateKey         *rsa.PrivateKey
}

type runningProxy struct {
	server          *http.Server
	listener        net.Listener
	httpTransport   *http.Transport
	connections     *connectionRegistry
	cancelLifecycle context.CancelFunc
}

type connectionRegistry struct {
	mu          sync.Mutex
	connections map[net.Conn]struct{}
	closed      bool
}

func newConnectionRegistry() *connectionRegistry {
	return &connectionRegistry{
		connections: make(map[net.Conn]struct{}),
	}
}

func (registry *connectionRegistry) add(conn net.Conn) bool {
	if conn == nil {
		return false
	}

	registry.mu.Lock()
	if registry.closed {
		registry.mu.Unlock()
		_ = conn.Close()
		return false
	}
	registry.connections[conn] = struct{}{}
	registry.mu.Unlock()
	return true
}

func (registry *connectionRegistry) remove(conn net.Conn) {
	if conn == nil {
		return
	}

	registry.mu.Lock()
	delete(registry.connections, conn)
	registry.mu.Unlock()
}

func (registry *connectionRegistry) closeAll() error {
	registry.mu.Lock()
	registry.closed = true
	connections := make([]net.Conn, 0, len(registry.connections))
	for conn := range registry.connections {
		connections = append(connections, conn)
		delete(registry.connections, conn)
	}
	registry.mu.Unlock()

	closeErrors := make([]error, 0)
	for _, conn := range connections {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

var globalID atomic.Uint64

func newAtomicID() uint64 {
	// #8 Genera un ID único SI O SI bajo cualquier nivel de concurrencia dentro de la misma instancia de tu aplicación.
	return globalID.Add(1)
}

var lifecycleMu sync.Mutex
var activeProxy *runningProxy

func StartProxy(config ConfigProxy) error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	if activeProxy != nil {
		return errors.New("the proxy is already running")
	}

	// Generamos nuestro CA al arrancar el programa.
	caCert, caPrivateKey, err := GetOrCreateCA()
	if err != nil {
		return err
	}

	configuredOutboundDialer, err := buildOutboundDialer(config.UpstreamProxy)
	if err != nil {
		return err
	}

	connectionTracker := newConnectionRegistry()
	lifecycleContext, cancelLifecycle := context.WithCancel(context.Background())
	httpTransport := newHTTPForwardTransport(
		configuredOutboundDialer,
		connectionTracker,
	)
	runtimeConfig := handlerConfig{
		skipServerCertVerify: config.SkipServerCertVerify,
		outboundDialer:       configuredOutboundDialer,
		httpTransport:        httpTransport,
		connections:          connectionTracker,
		lifecycleContext:     lifecycleContext,
		caCert:               caCert,
		caPrivateKey:         caPrivateKey,
	}

	nextServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			proxyHandler(w, r, runtimeConfig)
		}),
		ReadHeaderTimeout: inboundHeaderTimeout,
		IdleTimeout:       60 * time.Second,
	}

	// #9 net.Listen hace esto:
	// #8 - Crea Socket
	// #8 - Hace bind() -> Asocia el Socket a una direccion Local (IP + Puerto
	// #8 - Hace listen() -> Crea una cola de conexiones pendientes
	listenAddress := net.JoinHostPort(config.IP, strconv.Itoa(int(config.Port)))
	ln, err := net.Listen("tcp", listenAddress)
	if err != nil {
		cancelLifecycle()
		return err
	}

	instance := &runningProxy{
		server:          nextServer,
		listener:        ln,
		httpTransport:   httpTransport,
		connections:     connectionTracker,
		cancelLifecycle: cancelLifecycle,
	}
	activeProxy = instance

	go func(instance *runningProxy) {
		if serveErr := nextServer.Serve(ln); serveErr != nil &&
			!errors.Is(serveErr, http.ErrServerClosed) {
			bridge.EmitError(fmt.Sprintf("MITM proxy listener stopped unexpectedly: %v", serveErr))
			instance.cancelLifecycle()
			instance.httpTransport.CloseIdleConnections()
			_ = instance.connections.closeAll()

			lifecycleMu.Lock()
			wasActiveProxy := false
			if activeProxy == instance {
				activeProxy = nil
				wasActiveProxy = true
			}
			lifecycleMu.Unlock()
			if wasActiveProxy {
				bridge.EmitProxyStopped()
			}
		}
	}(instance)

	log.Printf("🔥 Native MITM proxy listening on %s", ln.Addr())

	return nil
}

func CloseProxy() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()

	if activeProxy == nil {
		return errors.New("cannot close the proxy because it has not been started yet")
	}

	instance := activeProxy
	activeProxy = nil

	instance.cancelLifecycle()
	instance.httpTransport.CloseIdleConnections()
	serverErr := instance.server.Close()
	listenerErr := instance.listener.Close()
	if errors.Is(listenerErr, net.ErrClosed) {
		listenerErr = nil
	}
	connectionsErr := instance.connections.closeAll()

	return errors.Join(serverErr, listenerErr, connectionsErr)
}

func IsProxyActive() bool {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	return activeProxy != nil
}

func ValidateConfig(config ConfigProxy) error {
	if net.ParseIP(strings.TrimSpace(config.IP)) == nil {
		return errors.New("proxy listen IP is invalid")
	}
	if config.Port == 0 {
		return errors.New("proxy listen port must be between 1 and 65535")
	}
	if _, err := buildOutboundDialer(config.UpstreamProxy); err != nil {
		return err
	}
	return nil
}

func buildOutboundDialer(config UpstreamProxyConfig) (xproxy.Dialer, error) {
	directDialer := &net.Dialer{
		Timeout:   outboundConnectTimeout,
		KeepAlive: 30 * time.Second,
	}

	if !config.Enabled {
		return directDialer, nil
	}

	host := normalizeHost(config.Host)
	if host == "" {
		return nil, errors.New("SOCKS5 upstream proxy host is required")
	}
	if strings.ContainsAny(host, "/\\ \t\r\n") {
		return nil, errors.New("SOCKS5 upstream proxy host is invalid")
	}
	if len([]byte(host)) > 255 {
		return nil, errors.New("SOCKS5 upstream proxy host must not exceed 255 bytes")
	}
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return nil, errors.New("SOCKS5 upstream proxy host is invalid")
	}
	if config.Port == 0 {
		return nil, errors.New("SOCKS5 upstream proxy port must be between 1 and 65535")
	}

	username := config.Username
	password := config.Password
	if (username == "") != (password == "") {
		return nil, errors.New("SOCKS5 username and password must either both be set or both be empty")
	}
	if len([]byte(username)) > 255 || len([]byte(password)) > 255 {
		return nil, errors.New("SOCKS5 username and password must not exceed 255 bytes")
	}

	var auth *xproxy.Auth
	if username != "" {
		auth = &xproxy.Auth{
			User:     username,
			Password: password,
		}
	}

	proxyAddress := net.JoinHostPort(host, strconv.Itoa(int(config.Port)))
	dialer, err := xproxy.SOCKS5("tcp", proxyAddress, auth, directDialer)
	if err != nil {
		return nil, fmt.Errorf("could not configure SOCKS5 upstream proxy: %w", err)
	}

	return dialer, nil
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if len(host) >= 2 && strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return host[1 : len(host)-1]
	}
	return host
}

func dialOutbound(ctx context.Context, dialer xproxy.Dialer, address string) (net.Conn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, outboundConnectTimeout)
	defer cancel()

	if contextDialer, ok := dialer.(xproxy.ContextDialer); ok {
		return contextDialer.DialContext(dialCtx, "tcp", address)
	}

	return nil, errors.New("the configured outbound dialer does not support context cancellation")
}

func contextBoundToProxyLifecycle(
	lifecycleContext context.Context,
	operationContext context.Context,
) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(lifecycleContext)
	stopOperationCancellation := context.AfterFunc(operationContext, cancel)

	return ctx, func() {
		stopOperationCancellation()
		cancel()
	}
}

func proxyHandler(w http.ResponseWriter, r *http.Request, config handlerConfig) {
	// fmt.Printf(" > Nueva Petición:\n%v\n", r)

	if isCleartextHTTP2Request(r) {
		writeCleartextHTTP2Unsupported(w)
		return
	}

	if r.Method != http.MethodConnect {
		forwardHTTP1(w, r, config)
		return
	}

	// #8 ESTABLECIMIENTO DE TÚNELES MEDIANTE EL MÉTODO HTTP CONNECT
	// #6 Para que un cliente establezca un túnel hacia un servidor destino a través de un Proxy HTTP y así realizar el Handshake TLS con el, debe enviarle una petición con el método CONNECT.
	// #6 Esta Petición CONNECT se puede enviar a través de HTTP/1.x o HTTP/2

	// #8 1. HTTP/1.x: TÚNEL A NIVEL DE CONEXIÓN TCP (Connection-Level Tunnel)
	// #6 1.- El cliente envía la Request CONNECT sobre una conexión TCP.
	// #6 2.- El Proxy resuelve el destino y abre un nuevo socket TCP contra el servidor final.
	// #6 3.- Tras responder el Proxy con un '200 Connection Established' al cliente, el Proxy deja de analizar el tráfico. Simplemente reenvía bytes crudos de un socket TCP al otro.
	// #6 4.- La conexión TCP (Cliente <-> Proxy) queda tunelizada con la conexión TCP (Proxy <-> Servidor)

	// #8 2. HTTP/2: TÚNEL A NIVEL DE STREAM (Stream-Level Tunnel)
	// #6 1.- HTTP/2 funciona enviando múltiples "Streams" sobre una ÚNICA conexión TCP subyacente.
	// #6 2.- En HTTP/2, cada intercambio request/response se asocia a un Stream identificado por un Stream ID único dentro de la conexión. 1 Stream = 1 Request y 1 Response
	// #6 3.- Tanto la petición como su respuesta se transmiten como una secuencia de frames (HEADERS, DATA, etc.) pertenecientes a ese mismo Stream.
	// #6 4.- El cliente inicia la Request CONNECT abriendo un nuevo Stream dentro de esa conexión TCP compartida.
	// #6 5.- El Proxy abre un nuevo socket TCP contra el servidor final.
	// #6 6.- El Proxy recibe los bytes crudos del servidor final, pero hacia el cliente los ENCAPSULA dentro de frames con el ID del Stream creado.
	// #6 7.- Así el Proxy puede mantener múltiples túneles CONNECT hacia distintos servidores de forma simultánea en la misma conexión TCP.

	// #8 ¿COMO SE CONECTAN A UN PROXY HTTP LOS DISTINTOS DISPOSITIVOS?
	// #6 1.- Dado que el sistema operativo (IOS o Android) o el Navegador del PC no sabe de antemano si el Servidor soporta HTTP/2, opta por conectar con el Proxy por HTTP/1.1 (CONNECT).
	// #6 2.- Luego, una vez tunelizada la conexion Cliente <=> Servidor, estos realizan el Handshake TLS, donde se negociara la versión de HTTP usando ALPN, pudiendo elegir HTTP/1.1 o HTTP/2
	// #6 3.- Es decir, es posible que a pesar de haber conectado el cliente con el Server Proxy con HTTP/1.1, luego negocie Cliente y Servidor HTTP/2 a través de ese Tunel.

	// #8 ¿QUE HACEMOS NOSOTROS?
	// #6 1.- NOSOTROS ESPERAMOS UNA CONEXIÓN CONNECT HTTP/1.1,
	// #6 2.- LUEGO ACEPTAMOS CON "200 Connection Established" Y NOS HACEMOS PASAR POR EL SERVIDOR, REALIZANDO EL HANDSHAKE TLS CON EL CLIENTE Y NEGOCIANDO NOSOTROS MISMO LA VERSIÓN DEL PROTOCOLO.
	// #6 3.- LUEGO ABRIMOS UNA NUEVA CONEXIÓN TCP CON EL SERVIDOR DESTINO Y NOS HACEMOS PASAR POR EL CLIENTE, REALIZANDO EL HANDSHAKE TLS CON EL SERVIDOR Y NEGOCIANDO LA VERSIÓN QUE HEMOS ELEGIDO CON EL CLIENTE
	// #6 4.- LUEGO REENVIAMOS BYTES DE ORIGEN A DESTINO, PERO COMO TERMINAMOS TLS EN CADA SOCKET, CADA VEZ QUE RECIBIMOS DE UN EXTREMO, DESENCRIPTAMOS CON LA CLAVE DE ESE EXTREMO, LEEMOS LOS DATOS, Y LUEGO ENCRIPTAMOS CON LA CLAVE DEL OTRO EXTREMO Y SE LA ENVIAMOS. ESTO ES "MAN IN THE MIDDLE"

	// #8 http.Hijacker es una interfaz de Go (net/http) que permite romper el modelo HTTP gestionado y acceder directamente a la conexión TCP subyacente (net.Conn).
	// #8 Comprobamos si el writer (w) implmenta la interfaz Hijacker, ya que conexiones como HTTP/2 no la implementan, ya que al usar multiplexacion por cada socket TCP, no podemos acceder directamente al socket TCP. De todos modos esto no nos preocupa, ya que las peticiones HTTP/2 que usen nuestro Proxy, se conectaran a nosotros por HTTP/1.1 Connect, no por HTTP/2 Connect. (Como se dice arriba)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking is not supported", http.StatusInternalServerError)
		bridge.EmitError("Hijacking is not supported")
		return
	}

	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		http.Error(w, "Invalid CONNECT target", http.StatusBadRequest)
		bridge.EmitError(fmt.Sprintf(
			"Could not parse CONNECT target %q: %v",
			r.Host,
			err,
		))
		return
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 || strings.TrimSpace(host) == "" {
		http.Error(w, "Invalid CONNECT target", http.StatusBadRequest)
		bridge.EmitError(fmt.Sprintf(
			"Invalid CONNECT target %q",
			r.Host,
		))
		return
	}

	certTLS, err := GenFakeCertSignedByCA(
		host,
		config.caCert,
		config.caPrivateKey,
	)
	if err != nil {
		http.Error(w, "Could not prepare TLS interception", http.StatusInternalServerError)
		bridge.EmitError(fmt.Sprintf(
			"Could not generate certificate for %s:%s: %v",
			host,
			port,
			err,
		))
		return
	}

	outboundDialContext, cancelOutboundDial :=
		contextBoundToProxyLifecycle(config.lifecycleContext, r.Context())
	outboundConn, err := dialOutbound(
		outboundDialContext,
		config.outboundDialer,
		net.JoinHostPort(host, port),
	)
	cancelOutboundDial()
	if err != nil {
		statusCode := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			statusCode = http.StatusGatewayTimeout
		} else if config.lifecycleContext.Err() != nil {
			statusCode = http.StatusServiceUnavailable
		}

		http.Error(w, "Could not establish the configured outbound route", statusCode)
		bridge.EmitError(fmt.Sprintf(
			"Could not establish the outbound route to %s:%s: %v",
			host,
			port,
			err,
		))
		return
	}
	if !config.connections.add(outboundConn) {
		http.Error(w, "The proxy is stopping", http.StatusServiceUnavailable)
		return
	}
	defer config.connections.remove(outboundConn)
	defer outboundConn.Close()

	// #8 Ejecutamos Hijack(), para obtener el Socket a la Conexion TCP
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		bridge.EmitError("Could not obtain the TCP connection")
		return
	}
	if !config.connections.add(clientConn) {
		return
	}
	defer config.connections.remove(clientConn)
	defer clientConn.Close()

	// La ruta de salida ya está establecida. A partir del 200, el cliente
	// negocia TLS con Marmota mientras Marmota negocia TLS con el destino.
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		bridge.EmitError(fmt.Sprintf("Could not acknowledge the CONNECT tunnel: %v", err))
		return
	}

	// #8 tls.Conn simplemente es una estructura que tiene punteros a la conexion TCP original y a la Configuracion de TLS y que además, tiene metodos como Read o Write
	// #8 - Cada vez que llamamos a tlsConn.Read(), este llama a clientConn.Read(), lee los bytes encriptado por TLS, los desencripta y nos entraga los bytes en texto plano
	// #8 - Cada vez que llamamos a tlsConn.Write(), este encripta con TLS los bytes pasados como parametro y los escribe con clientConn.Write() en la conexion TCP

	var serverTLSConn *tls.Conn
	var negotiatedProtocol string

	clientTLSConfig := &tls.Config{
		MinVersion: tls.VersionTLS10,
		// NO definimos NextProtos aquí. Lo haremos dinámicamente.

		// Esta función intercepta el ClientHello del Cliente en el Handshake TLS
		GetConfigForClient: func(clientHello *tls.ClientHelloInfo) (*tls.Config, error) {

			// #8 Configuramos la conexión hacia el servidor usando los protocolos que el cliente pidió
			serverTLSConfig := &tls.Config{
				MinVersion:         tls.VersionTLS10,
				InsecureSkipVerify: config.skipServerCertVerify,
				ServerName:         host,

				// #8 NextProtos debe contener la lista de los protocolos que nosotros soportamos
				// #8 clientHello.SupportedProtos contiene la lista original de protocolos que soporta el cliente (ej: ["h2", "http/1.1"])
				NextProtos: clientHello.SupportedProtos,
			}

			serverTLSConn = tls.Client(outboundConn, serverTLSConfig)
			serverHandshakeBaseContext, cancelServerHandshakeBase :=
				contextBoundToProxyLifecycle(
					config.lifecycleContext,
					clientHello.Context(),
				)
			handshakeCtx, cancelHandshake := context.WithTimeout(
				serverHandshakeBaseContext,
				tlsHandshakeTimeout,
			)
			handshakeErr := serverTLSConn.HandshakeContext(handshakeCtx)
			cancelHandshake()
			cancelServerHandshakeBase()
			if handshakeErr != nil {
				serverTLSConn.Close()
				// Si el servidor falla, devolvemos el error y el handshake del cliente también fallará
				return nil, handshakeErr
			}

			// #8 Protocolo que eligió finalmente el servidor
			negotiatedProtocol = serverTLSConn.ConnectionState().NegotiatedProtocol

			// #8 Creamos una configuración TLS dinámica para responderle al cliente. Solo le vamos a ofrecer el protocolo que el servidor aceptó.
			var nextProtosForClient []string
			if negotiatedProtocol != "" {
				nextProtosForClient = []string{negotiatedProtocol}
			}

			return &tls.Config{
				MinVersion:   tls.VersionTLS10,
				Certificates: []tls.Certificate{certTLS},
				NextProtos:   nextProtosForClient, // Forzamos al cliente a usar lo que el servidor eligió
			}, nil
		},
	}

	clientTLSConn := tls.Server(clientConn, clientTLSConfig)
	clientHandshakeBaseContext, cancelClientHandshakeBase :=
		contextBoundToProxyLifecycle(config.lifecycleContext, r.Context())
	clientHandshakeCtx, cancelClientHandshake := context.WithTimeout(
		clientHandshakeBaseContext,
		tlsHandshakeTimeout,
	)
	err = clientTLSConn.HandshakeContext(clientHandshakeCtx)
	cancelClientHandshake()
	cancelClientHandshakeBase()
	if err != nil {
		bridge.EmitError(fmt.Sprintf("TLS handshake failed with client or server %s:%s: %v", host, port, err))
		return
	}
	if serverTLSConn == nil {
		bridge.EmitError(fmt.Sprintf(
			"TLS interception for %s:%s completed without an outbound TLS connection",
			host,
			port,
		))
		return
	}

	defer clientTLSConn.Close()
	defer serverTLSConn.Close()

	switch negotiatedProtocol {
	case "", "http/1.1":
		http1xHandler(host, port, clientTLSConn, serverTLSConn)
	case "h2":
		http2Handler(host, port, clientTLSConn, serverTLSConn)
	}
}

func http1xHandler(host string, port string, clientConn net.Conn, serverConn net.Conn) {
	for {
		// #6 [DILEMA] ==> http.ReadResquest y http.ReadResponse parsean a la perfeccion el mensaje HTTP/1.x, leyendo justo hasta el final del body si es que hay.
		// #6 El problema es que estas funciones eliminar headers Hop By Hop, reordenar Headers, cambian mayusculas y minusculas...
		// #6 Esto hace que nuestro Proxy MITM no actue como queremos, que es reenviar los que nos llegue de un extremo, tal cual, al otro.
		// #6 Por lo que para la Request y Response, la leemos nosotros con nuestra función auxiliar ReadSanitizedHTTP

		// #8 Leemos Request HTTP quitando los headers que corresponden al Proxy, pero manteniendo orden de headers, versiones, mayusculas y minusculas y todo (Esto no lo podría hacer http.ReadRequest)
		rawRequest, requestStartLine, err := h1x.ReadSanitizedHTTP(clientConn, "")
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Cierre limpio la conexión TCP. No mas peticiones HTTP en esta Conexioón
				return
			}
			if errors.Is(err, syscall.ECONNRESET) {
				// Cierre Abrupto de la conexión TCP. No mas peticiones HTTP en esta Conexioón
				return
			}

			bridge.EmitError(fmt.Sprintf("Could not read and parse the client request in HTTP/1.1: %v", err))
			return
		}

		id := newAtomicID()
		requestReceivedAtMs := time.Now().UnixMilli()

		go func(
			rawRequest []byte,
			requestStartLine h1x.StartLineData,
			id uint64,
			receivedAtMs int64,
		) {
			headBlockStr, bodyStr, err := h1x.ParseRawRequest(rawRequest)
			if err != nil {
				var decodingErr *utils.ContentDecodingError
				if !errors.As(err, &decodingErr) {
					bridge.EmitError(fmt.Sprintf("Could not parse the HTTP/1.1 raw request for the frontend. ID: %d. Error: %v", id, err))
					return
				}
				bridge.EmitError(fmt.Sprintf("Could not decode the HTTP/1.1 request body for inspection. ID: %d. Raw encoded data was preserved. Error: %v", id, err))
			}
			bridge.AddRequestToHistory(&bridge.HTTPRequestDetail{
				ID:           id,
				Host:         host,
				Port:         port,
				Version:      requestStartLine.Version,
				Method:       requestStartLine.Method,
				Path:         requestStartLine.Path,
				Scheme:       "https",
				HeadBlockStr: headBlockStr,
				BodyStr:      bodyStr,
			})
			bridge.EmitHTTPRequestSummary(bridge.HTTPRequestSummary{
				ID:           id,
				Host:         host,
				Port:         port,
				Version:      requestStartLine.Version,
				Method:       requestStartLine.Method,
				Path:         requestStartLine.Path,
				Scheme:       "https",
				ReceivedAtMs: receivedAtMs,
			})
		}(rawRequest, requestStartLine, id, requestReceivedAtMs)

		// #8 Escribimos la Request Cruda en la Conexion del Servidor
		if _, err := serverConn.Write(rawRequest); err != nil {
			bridge.EmitError(fmt.Sprintf("Could not write the raw HTTP/1.1 request to the server: %v", err))
			return
		}

		// #8 Leemos Response HTTP manteniendo orden de headers, versiones, mayusculas y minusculas y todo (Esto no lo podría hacer http.ReadResponse)
		rawResponse, responseStartLine, err := h1x.ReadSanitizedHTTP(
			serverConn,
			requestStartLine.Method,
		)
		if err != nil {
			bridge.EmitError(fmt.Sprintf("Could not read and parse the server response in HTTP/1.1: %v", err))
			return
		}
		responseReceivedAtMs := time.Now().UnixMilli()

		go func(
			rawResponse []byte,
			rawRequest []byte,
			responseStartLine h1x.StartLineData,
			id uint64,
			receivedAtMs int64,
		) {
			headBlockStr, bodyStr, contentEncoding, err :=
				h1x.ParseRawResponseWithContentEncoding(rawResponse, rawRequest)
			contentDecodingFailed := false
			if err != nil {
				var decodingErr *utils.ContentDecodingError
				if !errors.As(err, &decodingErr) {
					bridge.EmitError(fmt.Sprintf("Could not parse the HTTP/1.1 raw response for the frontend. ID: %d. Error: %v", id, err))
					return
				}
				contentDecodingFailed = true
				bridge.EmitError(fmt.Sprintf("Could not decode the HTTP/1.1 response body for inspection. ID: %d. Raw encoded data was preserved. Error: %v", id, err))
			}
			unsupportedContentEncodings :=
				utils.UnsupportedContentEncodings(contentEncoding)
			bridge.AddResponseToHistory(&bridge.HTTPResponseDetail{
				ID:                          id,
				Host:                        host,
				Port:                        port,
				Version:                     responseStartLine.Version,
				StatusCode:                  responseStartLine.StatusCode,
				HeadBlockStr:                headBlockStr,
				BodyStr:                     bodyStr,
				UnsupportedContentEncodings: unsupportedContentEncodings,
				ContentDecodingFailed:       contentDecodingFailed,
			})
			bridge.EmitHTTPResponseSummary(bridge.HTTPResponseSummary{
				ID:                          id,
				Host:                        host,
				Port:                        port,
				Version:                     responseStartLine.Version,
				StatusCode:                  responseStartLine.StatusCode,
				ReceivedAtMs:                receivedAtMs,
				UnsupportedContentEncodings: unsupportedContentEncodings,
				ContentDecodingFailed:       contentDecodingFailed,
			})
		}(
			rawResponse,
			rawRequest,
			responseStartLine,
			id,
			responseReceivedAtMs,
		)

		// #8 Escribimos la Request Cruda en la Conexion del Cliente
		if _, err := clientConn.Write(rawResponse); err != nil {
			bridge.EmitError(fmt.Sprintf("Could not write the raw HTTP/1.1 response to the client: %v", err))
			return
		}
	}
}

func http2Handler(host string, port string, clientConn net.Conn, serverConn net.Conn) {
	// #8 func io.Pipe() (*io.PipeReader, *io.PipeWriter) ==> Crea un canal de datos síncrono en memoria
	// #8 - (*io.PipeReader).Read() se bloquea hasta recibir un Write() o un Close().
	// #8 - (*io.PipeWriter).Write() se bloquea hasta que Read() consuma todos sus datos.
	prClient, pwClient := io.Pipe() // Bytes: Cliente -> Servidor
	prServer, pwServer := io.Pipe() // Bytes: Servidor -> Cliente

	// #8 func io.TeeReader(r io.Reader, w io.Writer) io.Reader ==> Crea un lector que intercepta los datos de un flujo origen y los duplica (los escribe) en un destino secundario de forma síncrona.
	// #8 Los datos solo se extraen del Reader original y se escriben en el Writer cuando el consumidor final invoca el método Read() sobre el Reader devuelto por TeeReader. Si no hay lectura, no hay escritura.
	teeClient := io.TeeReader(clientConn, pwClient)
	teeServer := io.TeeReader(serverConn, pwServer)

	// #8 Copiamos del Socket Cliente al Socket Servidor y al pwClient
	go func() {
		defer pwClient.Close()
		io.Copy(serverConn, teeClient)
	}()

	// #8 Copiamos del Socket Servidor al Socket Cliente y al pwServer
	go func() {
		defer pwServer.Close()
		io.Copy(clientConn, teeServer)
	}()

	msgChan := make(chan h2.HTTP2Message, 100)

	var wg sync.WaitGroup
	wg.Add(2) // +2

	// #8 h2.SniffH2Traffic retorna cuando hay error en el socket o frameso cuando se cierra limpiamente la conexion (io.EOF)

	go func() {
		defer wg.Done()                                    // -1
		err := h2.SniffH2Traffic(prClient, false, msgChan) // Analiza peticiones
		if err != io.EOF {
			bridge.EmitError(fmt.Sprintf("Could not analyze HTTP/2 requests: %v", err))
		}
		// Inspection must never back-pressure or stop the proxied connection.
		// If parsing fails, keep consuming the tee pipe until forwarding ends.
		_, _ = io.Copy(io.Discard, prClient)
	}()

	go func() {
		defer wg.Done()                                   // -1
		err := h2.SniffH2Traffic(prServer, true, msgChan) // Analiza respuestas
		if err != io.EOF {
			bridge.EmitError(fmt.Sprintf("Could not analyze HTTP/2 responses: %v", err))
		}
		_, _ = io.Copy(io.Discard, prServer)
	}()

	go func() {
		wg.Wait() // Bloquea hasta que wg = 0
		close(msgChan)
	}()

	type capturedStream struct {
		id               uint64
		requestCaptured  bool
		responseCaptured bool
	}
	capturedStreams := make(map[uint32]*capturedStream)

	for msg := range msgChan {
		if msg.Error != nil {
			bridge.EmitError(fmt.Sprintf(
				"Could not inspect HTTP/2 stream %d: %v",
				msg.StreamID,
				msg.Error,
			))
			delete(capturedStreams, msg.StreamID)
			continue
		}
		if msg.DecodeError != nil {
			bridge.EmitError(fmt.Sprintf(
				"Could not decode the HTTP/2 body for inspection on stream %d. Raw encoded data was preserved. Error: %v",
				msg.StreamID,
				msg.DecodeError,
			))
		}

		headBlockStr, err := h2.BuildHTTP1HeadBlockStr(&msg)
		if err != nil {
			bridge.EmitError(fmt.Sprintf("Could not build the Head Block string from an HTTP/2 message (isResponse: %t): %v", msg.IsResponse, err))
			continue
		}

		stream := capturedStreams[msg.StreamID]
		if stream == nil {
			stream = &capturedStream{id: newAtomicID()}
			capturedStreams[msg.StreamID] = stream
		}

		if !msg.IsResponse {
			bridge.AddRequestToHistory(&bridge.HTTPRequestDetail{
				ID:           stream.id,
				Host:         host,
				Port:         port,
				Version:      "HTTP/2",
				Method:       msg.Method,
				Path:         msg.Path,
				Scheme:       "https",
				HeadBlockStr: headBlockStr,
				BodyStr:      msg.Body,
			})
			bridge.EmitHTTPRequestSummary(bridge.HTTPRequestSummary{
				ID:           stream.id,
				Host:         host,
				Port:         port,
				Version:      "HTTP/2",
				Method:       msg.Method,
				Path:         msg.Path,
				Scheme:       "https",
				ReceivedAtMs: time.Now().UnixMilli(),
			})
			stream.requestCaptured = true
		} else {
			statusCode, err := strconv.Atoi(msg.Status)
			if err != nil {
				bridge.EmitError(fmt.Sprintf(
					"Could not parse HTTP/2 status %q on stream %d",
					msg.Status,
					msg.StreamID,
				))
				continue
			}

			unsupportedContentEncodings :=
				utils.UnsupportedContentEncodings(msg.ContentEncoding)
			bridge.AddResponseToHistory(&bridge.HTTPResponseDetail{
				ID:                          stream.id,
				Host:                        host,
				Port:                        port,
				Version:                     "HTTP/2",
				StatusCode:                  statusCode,
				HeadBlockStr:                headBlockStr,
				BodyStr:                     msg.Body,
				UnsupportedContentEncodings: unsupportedContentEncodings,
				ContentDecodingFailed:       msg.DecodeError != nil,
			})
			bridge.EmitHTTPResponseSummary(bridge.HTTPResponseSummary{
				ID:                          stream.id,
				Host:                        host,
				Port:                        port,
				Version:                     "HTTP/2",
				StatusCode:                  statusCode,
				ReceivedAtMs:                time.Now().UnixMilli(),
				UnsupportedContentEncodings: unsupportedContentEncodings,
				ContentDecodingFailed:       msg.DecodeError != nil,
			})
			stream.responseCaptured = true
		}

		if stream.requestCaptured && stream.responseCaptured {
			delete(capturedStreams, msg.StreamID)
		}
	}
}
