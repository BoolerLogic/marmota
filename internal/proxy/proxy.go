package proxy

import (
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
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type ConfigProxy struct {
	IP                   string `json:"ip"`
	Port                 uint16 `json:"port"`
	SkipServerCertVerify bool   `json:"skipServerCertVerify"`
}

var globalID atomic.Uint64

func newAtomicID() uint64 {
	// #8 Genera un ID único SI O SI bajo cualquier nivel de concurrencia dentro de la misma instancia de tu aplicación.
	return globalID.Add(1)
}

var server *http.Server
var currentCACert *x509.Certificate
var currentCAPrivKey *rsa.PrivateKey
var skipServerCertVerify bool

func StartProxy(config ConfigProxy) error {
	// Generamos nuestro CA al arrancar el programa.
	var err error
	currentCACert, currentCAPrivKey, err = GetOrCreateCA()
	if err != nil {
		return err
	}

	skipServerCertVerify = config.SkipServerCertVerify

	server = &http.Server{
		Handler: http.HandlerFunc(proxyHandler),
	}

	// #9 net.Listen hace esto:
	// #8 - Crea Socket
	// #8 - Hace bind() -> Asocia el Socket a una direccion Local (IP + Puerto
	// #8 - Hace listen() -> Crea una cola de conexiones pendientes
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.IP, config.Port))
	if err != nil {
		return err
	}

	go server.Serve(ln)

	log.Printf("🔥 Native MITM proxy listening on %s:%d", config.IP, config.Port)

	return nil
}

func CloseProxy() error {
	if server == nil {
		return errors.New("cannot close the proxy because it has not been started yet")
	}

	if err := server.Close(); err != nil {
		return err
	}

	server = nil

	return nil
}

func IsProxyActive() bool {
	return server != nil
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// fmt.Printf(" > Nueva Petición:\n%v\n", r)

	if r.Method != http.MethodConnect {
		msg := fmt.Sprintf("Method %s not allowed: only HTTPS (CONNECT) is supported", r.Method)
		http.Error(w, msg, http.StatusMethodNotAllowed)
		bridge.EmitError(msg)
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

	// #8 Ejecutamos Hijack(), para obtener el Socket a la Conexion TCP
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		bridge.EmitError("Could not obtain the TCP connection")
		return
	}
	defer clientConn.Close()

	// #8 Contestamos con 200 OK simulando haber abierto un tunel ya con el servidor destino, pero en realidad el cliente va a negociar el TLS con nosotros
	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	host, port, err := net.SplitHostPort(r.Host) // Host y Port del Servidor Destino
	if err != nil {
		bridge.EmitError(fmt.Sprintf("Could not get the host or port from the client connection: %v", err))
		return
	}

	// #8 Generamos un Certificado de TLS asociado al dominio y firmado por nuestro CA
	certTLS, err := GenFakeCertSignedByCA(host, currentCACert, currentCAPrivKey)
	if err != nil {
		bridge.EmitError(fmt.Sprintf("Could not generate certificate for %s:%s: %v", host, port, err))
		return
	}

	// #8 tls.Conn simplemente es una estructura que tiene punteros a la conexion TCP original y a la Configuracion de TLS y que además, tiene metodos como Read o Write
	// #8 - Cada vez que llamamos a tlsConn.Read(), este llama a clientConn.Read(), lee los bytes encriptado por TLS, los desencripta y nos entraga los bytes en texto plano
	// #8 - Cada vez que llamamos a tlsConn.Write(), este encripta con TLS los bytes pasados como parametro y los escribe con clientConn.Write() en la conexion TCP

	var serverTLSConn *tls.Conn
	var dialErr error
	var negotiatedProtocol string

	clientTLSConfig := &tls.Config{
		MinVersion: tls.VersionTLS10,
		// NO definimos NextProtos aquí. Lo haremos dinámicamente.

		// Esta función intercepta el ClientHello del Cliente en el Handshake TLS
		GetConfigForClient: func(clientHello *tls.ClientHelloInfo) (*tls.Config, error) {

			// #8 Configuramos la conexión hacia el servidor usando los protocolos que el cliente pidió
			serverTLSConfig := &tls.Config{
				MinVersion:         tls.VersionTLS10,
				InsecureSkipVerify: skipServerCertVerify,

				// #8 NextProtos debe contener la lista de los protocolos que nosotros soportamos
				// #8 clientHello.SupportedProtos contiene la lista original de protocolos que soporta el cliente (ej: ["h2", "http/1.1"])
				NextProtos: clientHello.SupportedProtos,
			}

			serverTLSConn, dialErr = tls.Dial("tcp", host+":"+port, serverTLSConfig)
			if dialErr != nil {
				// Si el servidor falla, devolvemos el error y el handshake del cliente también fallará
				return nil, dialErr
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
	if err := clientTLSConn.Handshake(); err != nil {
		// #8 Si el handshake con el cliente falla por lo que sea, pero ya habíamos abierto la conexión con el servidor, debemos cerrarla para no dejar conexiones fantasma.
		if serverTLSConn != nil {
			serverTLSConn.Close()
		}

		bridge.EmitError(fmt.Sprintf("TLS handshake failed with client or server %s:%s: %v", host, port, err))
		return
	}

	defer clientTLSConn.Close()
	defer serverTLSConn.Close()

	switch negotiatedProtocol {
	case "http/1.1":
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
		rawRequest, startLineData, err := h1x.ReadSanitizedHTTP(clientConn, "")
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

		go func() {
			headBlockStr, bodyStr, err := h1x.ParseRawRequest(rawRequest)
			if err != nil {
				bridge.EmitError(fmt.Sprintf("Could not parse the HTTP/1.1 raw request for the frontend. ID: %d. Error: %v", id, err))
				return
			}
			bridge.AddRequestToHistory(&bridge.HTTPRequestDetail{
				ID:           id,
				Host:         host,
				Port:         port,
				Version:      startLineData.Version,
				Method:       startLineData.Method,
				Path:         startLineData.Path,
				Scheme:       "https",
				HeadBlockStr: headBlockStr,
				BodyStr:      bodyStr,
			})
			bridge.EmitHTTPRequestSummary(bridge.HTTPRequestSummary{
				ID:      id,
				Host:    host,
				Port:    port,
				Version: startLineData.Version,
				Method:  startLineData.Method,
				Path:    startLineData.Path,
				Scheme:  "https",
			})
		}()

		// #8 Escribimos la Request Cruda en la Conexion del Servidor
		if _, err := serverConn.Write(rawRequest); err != nil {
			bridge.EmitError(fmt.Sprintf("Could not write the raw HTTP/1.1 request to the server: %v", err))
			return
		}

		// #8 Leemos Response HTTP manteniendo orden de headers, versiones, mayusculas y minusculas y todo (Esto no lo podría hacer http.ReadResponse)
		rawResponse, startLineData, err := h1x.ReadSanitizedHTTP(serverConn, startLineData.Method)
		if err != nil {
			bridge.EmitError(fmt.Sprintf("Could not read and parse the server response in HTTP/1.1: %v", err))
			return
		}

		go func() {
			headBlockStr, bodyStr, err := h1x.ParseRawResponse(rawResponse, rawRequest)
			if err != nil {
				bridge.EmitError(fmt.Sprintf("Could not parse the HTTP/1.1 raw response for the frontend. ID: %d. Error: %v", id, err))
				return
			}
			bridge.AddResponseToHistory(&bridge.HTTPResponseDetail{
				ID:           id,
				Host:         host,
				Port:         port,
				Version:      startLineData.Version,
				StatusCode:   startLineData.StatusCode,
				HeadBlockStr: headBlockStr,
				BodyStr:      bodyStr,
			})
			bridge.EmitHTTPResponseSummary(bridge.HTTPResponseSummary{
				ID:         id,
				Host:       host,
				Port:       port,
				Version:    startLineData.Version,
				StatusCode: startLineData.StatusCode,
			})
		}()

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
			return
		}
	}()

	go func() {
		defer wg.Done()                                   // -1
		err := h2.SniffH2Traffic(prServer, true, msgChan) // Analiza respuestas
		if err != io.EOF {
			bridge.EmitError(fmt.Sprintf("Could not analyze HTTP/2 responses: %v", err))
			return
		}
	}()

	go func() {
		wg.Wait() // Bloquea hasta que wg = 0
		close(msgChan)
	}()

	streamIDToInternalID := map[uint32]uint64{}

	for msg := range msgChan {
		if msg.Error != nil {
			bridge.EmitError(fmt.Sprintf("Could not receive an HTTP/2 message: %v", msg.Error))
			return
		}

		headBlockStr, err := h2.BuildHTTP1HeadBlockStr(&msg)
		if err != nil {
			bridge.EmitError(fmt.Sprintf("Could not build the Head Block string from an HTTP/2 message (isResponse: %t): %v", msg.IsResponse, err))
			return
		}

		if !msg.IsResponse {
			id := newAtomicID()
			streamIDToInternalID[msg.StreamID] = id

			bridge.AddRequestToHistory(&bridge.HTTPRequestDetail{
				ID:           id,
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
				ID:           id,
				Host:         host,
				Port:         port,
				Version:      "HTTP/2",
				Method:       msg.Method,
				Path:         msg.Path,
				Scheme:       "https",
				ReceivedAtMs: time.Now().UnixMilli(),
			})
		} else {
			id, ok := streamIDToInternalID[msg.StreamID]
			if !ok {
				bridge.EmitError(fmt.Sprintf("streamIDToInternalID[%d] was not found while receiving the HTTP/2 response", msg.StreamID))
				return
			}

			statusCode, err := strconv.Atoi(msg.Status)
			if err != nil {
				bridge.EmitError("Could not parse msg.Status as int")
				return
			}

			bridge.AddResponseToHistory(&bridge.HTTPResponseDetail{
				ID:           id,
				Host:         host,
				Port:         port,
				Version:      "HTTP/2",
				StatusCode:   statusCode,
				HeadBlockStr: headBlockStr,
				BodyStr:      msg.Body,
			})
			bridge.EmitHTTPResponseSummary(bridge.HTTPResponseSummary{
				ID:           id,
				Host:         host,
				Port:         port,
				Version:      "HTTP/2",
				StatusCode:   statusCode,
				ReceivedAtMs: time.Now().UnixMilli(),
			})
		}
	}
}
