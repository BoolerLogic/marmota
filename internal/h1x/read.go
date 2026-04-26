package h1x

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// #8 Límites razonables para la parte textual de la request
const (
	maxRequestHeadBytes = 1 << 20  // 1 MiB
	maxLineBytes        = 64 << 10 // 64 KiB
)

// #8 Headers de proxy que sí vamos a eliminar
var proxyHeadersToStrip = map[string]struct{}{
	"proxy-connection":    {},
	"proxy-authorization": {},
	"proxy-authenticate":  {},
}

type rawHeader struct {
	nameLower string
	rawLines  [][]byte // Plural para soportar headers multilínea (antiguo)
}

// #8 Lee una Request/Response HTTP/1.x completa desde "conn", elimina solo los 3 headers de proxy (solo Request) y retorna rawBytes de la peticion
func ReadSanitizedHTTP(conn net.Conn, reqMethod string) ([]byte, StartLineData, error) {
	rawHead, err := readUntilDoubleCRLF(conn, maxRequestHeadBytes)
	if err != nil {
		return nil, StartLineData{}, fmt.Errorf("error reading until double CRLF: %w", err)
	}

	startLine, headers, err := parseRawHead(rawHead)
	if err != nil {
		return nil, StartLineData{}, fmt.Errorf("error parsing the Head Block: %w", err)
	}

	startLineData, err := parseStartLine(startLine)
	if err != nil {
		return nil, StartLineData{}, fmt.Errorf("error parsing the Start Line: %w", err)
	}

	isChunked, err := hasChunkedTransferEncoding(headers)
	if err != nil {
		return nil, startLineData, fmt.Errorf("error parsing the Transfer-Encoding header: %w", err)
	}

	contentLength, hasContentLength, err := parseContentLength(headers)
	if err != nil {
		return nil, startLineData, fmt.Errorf("error parsing the Content-Length header: %w", err)
	}

	hasBody := isChunked || (hasContentLength && contentLength > 0)

	// #8 Hay casos donde el cliente quiere enviar muchos bytes y antes de enviarlo en el body, inserta un header: Expect: 100-continue
	// #8 Si el servidor le indica: HTTP/1.1 100 Continue, entonces el cliente envia el Body (SIN CABECERAS)
	// #8 En este caso, enviamos nosotros manualmente esto haciendonos pasar por el servidor y seguimos con la funcion y esperamos a que envie el Body el CLiente
	// #8 *** Si detectamos este Header, sabemos con exactitud que se ha llamado a esta funcion con la conexion del cliente y NO del servidor
	/*
		POST /upload HTTP/1.1
		Host: example.com
		Content-Length: 1000000
		Expect: 100-continue
	*/
	if !startLineData.IsResponse && hasBody && hasTokenInHeaders(headers, "expect", "100-continue") {
		if _, err := conn.Write([]byte("HTTP/1.1 100 Continue\r\n\r\n")); err != nil {
			return nil, startLineData, fmt.Errorf("could not send 100 Continue: %w", err)
		}
	}

	var out bytes.Buffer

	out.Write(startLine)
	out.WriteString("\r\n")

	for _, h := range headers {
		if _, skip := proxyHeadersToStrip[h.nameLower]; skip {
			fmt.Printf("Removing header ==> %s", string(h.rawLines[0]))
			continue
		}

		for _, line := range h.rawLines {
			out.Write(line)
			out.WriteString("\r\n")
		}
	}

	out.WriteString("\r\n")

	switch {
	case (startLineData.IsResponse && reqMethod == "HEAD") || // #8 El metodo solo aparece en la solicitud, por lo que lo inyectamos
		(startLineData.StatusCode >= 100 && startLineData.StatusCode <= 199) ||
		startLineData.StatusCode == 204 ||
		startLineData.StatusCode == 304:

		// #8 1. El estándar HTTP (RFC 9112) dicta que ciertas respuestas (códigos 1xx, 204, 304, o peticiones HEAD) tienen prohibido incluir un cuerpo.

	case isChunked:
		//#8  2. Prioridad 1 según RFC: Transfer-Encoding anula Content-Length
		body, err := readRawChunkedBody(conn)
		if err != nil {
			return nil, startLineData, fmt.Errorf("error reading the body with Transfer-Encoding: chunked: %w", err)
		}
		out.Write(body)

	case hasContentLength:
		// #8 3. Prioridad 2: Content-Length explícito
		if contentLength > 0 {
			if _, err := io.CopyN(&out, conn, contentLength); err != nil { // Bloqueante hasta que llegan los N bytes o hay error, no se desbloquea por menos bytes
				return nil, startLineData, fmt.Errorf("error reading the body with Content-Length %d: %w", contentLength, err)
			}
		}

	default:
		// #8 En Request HTTP/1.x sin chunked y sin Content-Length, indica que no hay Body
		// #8 En Response HTTP/1.x, si no hay chunked ni Content-Length, el RFC dice todo byte posterior (si es que hay), pertenece al Body, hasta que el servidor cierre conexion (EOF)
		if startLineData.IsResponse {
			body, err := io.ReadAll(conn) // Lee hasta EOF
			if err != nil {
				return nil, startLineData, fmt.Errorf("error reading the response until EOF: %w", err)
			}
			out.Write(body) // Puede que body sea = [], si en la respuesta no hay Body
		}
	}

	return out.Bytes(), startLineData, nil
}

func parseRawHead(rawHead []byte) ([]byte, []rawHeader, error) {
	if len(rawHead) < 4 || !bytes.HasSuffix(rawHead, []byte("\r\n\r\n")) {
		return nil, nil, errors.New("invalid HTTP header: missing trailing CRLF CRLF")
	}

	block := rawHead[:len(rawHead)-4] // Eliminamos el final ("\r\n\r\n")
	lines := bytes.Split(block, []byte("\r\n"))
	if len(lines) == 0 || len(lines[0]) == 0 {
		return nil, nil, errors.New("empty request line")
	}

	startLine := append([]byte(nil), lines[0]...)

	headers := make([]rawHeader, 0, len(lines)-1) // - 1 porque la primera linea no es header
	var current *rawHeader

	for i := 1; i < len(lines); i++ {
		line := append([]byte(nil), lines[i]...)
		if len(line) == 0 {
			continue
		}

		// #8 Soporte para headers multilínea. Se pueden separar los valores por '\n\t' o '\n ', e.g: "Header: valor1\n\tvalor2\n valor3"
		if line[0] == ' ' || line[0] == '\t' {
			if current == nil {
				return nil, nil, errors.New("multiline headers without a previous header")
			}
			current.rawLines = append(current.rawLines, line)
			continue
		}

		colon := bytes.IndexByte(line, ':') // Casteo automatico de rune (uint32) a byte (uint8)
		if colon <= 0 {
			return nil, nil, fmt.Errorf("invalid header: %q", string(line))
		}

		h := rawHeader{
			nameLower: strings.ToLower(strings.TrimSpace(string(line[:colon]))),
			rawLines:  [][]byte{line},
		}

		headers = append(headers, h)
		current = &headers[len(headers)-1]
	}

	return startLine, headers, nil
}

func collectHeaderValues(headers []rawHeader, nameLower string) []string {
	// #8 Devuelve una lista de los valores del nameHeader que se pide por parametro. Puede que un header se repita, por eso devuelve una lista y no un solo string

	values := make([]string, 0, 2) // cap inicial = 2, aunque puede repetirse headers mas de 2 veces

	for _, h := range headers {
		if h.nameLower != nameLower {
			continue
		}

		if len(h.rawLines) == 0 {
			continue
		}

		first := h.rawLines[0]
		colon := bytes.IndexByte(first, ':')
		if colon < 0 {
			continue
		}

		var b strings.Builder
		b.WriteString(strings.TrimSpace(string(first[colon+1:])))

		// #8 Compacta los headers multilinea sustituyendo '\n\t' o '\n ' por ' '
		for i := 1; i < len(h.rawLines); i++ {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strings.TrimSpace(string(h.rawLines[i])))
		}

		values = append(values, b.String())
	}

	return values
}

func hasChunkedTransferEncoding(headers []rawHeader) (bool, error) {
	// #8 Puede que el body se envie chunkeado, en ese caso sabemos que habría el Header ==> Transfer-Encoding: chunked

	values := collectHeaderValues(headers, "transfer-encoding")
	if len(values) == 0 {
		return false, nil
	}

	// #8 Obtiene los valores del header separando por ','
	var codings []string
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			token := strings.ToLower(strings.TrimSpace(part))
			if token != "" {
				codings = append(codings, token)
			}
		}
	}

	if len(codings) == 0 {
		return false, nil
	}

	// #8 En HTTP/1.1, si hay Transfer-Encodin para delimitar body de request, debe terminar en chunked
	if codings[len(codings)-1] != "chunked" {
		return false, fmt.Errorf("unsupported Transfer-Encoding for request: %q", strings.Join(codings, ", "))
	}

	return true, nil
}

func parseContentLength(headers []rawHeader) (int64, bool, error) {
	// #8 Parsea Content-Length aceptando repeticiones solo si todas coinciden

	values := collectHeaderValues(headers, "content-length")
	if len(values) == 0 {
		return 0, false, nil
	}

	var seen *int64

	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			n, err := strconv.ParseInt(part, 10, 64)
			if err != nil || n < 0 {
				return 0, false, fmt.Errorf("invalid Content-Length: %q", part)
			}

			if seen == nil {
				tmp := n
				seen = &tmp
				continue
			}

			if *seen != n {
				return 0, false, fmt.Errorf("inconsistent Content-Length: %d vs %d", *seen, n)
			}
		}
	}

	if seen == nil {
		return 0, false, nil
	}

	return *seen, true, nil
}

func hasTokenInHeaders(headers []rawHeader, headerNameLower string, wantedTokenLower string) bool {
	// #8 Devuelve true si wantedTokenLower se encuentra en alguno de los valores del Header headerNameLower

	values := collectHeaderValues(headers, headerNameLower)
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), wantedTokenLower) {
				return true
			}
		}
	}
	return false
}

func readRawChunkedBody(conn net.Conn) ([]byte, error) {
	var out bytes.Buffer

	for {
		sizeLine, err := readUntilCRLF(conn, maxLineBytes)
		if err != nil {
			return nil, fmt.Errorf("error reading the chunk-size line: %w", err)
		}
		out.Write(sizeLine)

		chunkSize, err := parseChunkSizeLine(sizeLine)
		if err != nil {
			return nil, err
		}

		if chunkSize > 0 {
			if _, err := io.CopyN(&out, conn, chunkSize); err != nil {
				return nil, fmt.Errorf("error reading chunk data (%d bytes): %w", chunkSize, err)
			}

			// #8 Al final de cada Chunk hay '\r\n'
			crlf := make([]byte, 2)
			if _, err := io.ReadFull(conn, crlf); err != nil {
				return nil, fmt.Errorf("error reading the CRLF after the chunk: %w", err)
			}
			if crlf[0] != '\r' || crlf[1] != '\n' {
				return nil, errors.New("invalid chunk: missing CRLF after the data")
			}
			out.Write(crlf)
			continue
		}

		// #8 Chunk final 0: después vienen trailers y línea en blanco final:
		/*
			0\r\n
			<trailers opcionales>\r\n
			\r\n
		*/
		for {
			line, err := readUntilCRLF(conn, maxLineBytes)
			if err != nil {
				return nil, fmt.Errorf("error reading chunked trailers: %w", err)
			}
			out.Write(line)

			if bytes.Equal(line, []byte("\r\n")) {
				// #8 Cuando la linea sea exactamente '\r\n'
				return out.Bytes(), nil
			}
		}
	}
}

func parseChunkSizeLine(sizeLine []byte) (int64, error) {
	// #8 Chunk -> Primera linea contiene el numero de bytes a leer en hexadecimal y codificado en ASCII
	// #8 Chunk -> Lo siguiente ya son los bytes a leer
	// #8 Ejemplos de como podría ser el Chunk (A => 10 en decimal):
	/*
		A\r\n
		0123456789\r\n
		4;foo=bar\r\n
		Wiki\r\n
	*/
	// #8 o:
	/*
		A;foo=bar\r\n
		0123456789\r\n
		4\r\n
		Wiki\r\n
	*/

	if !bytes.HasSuffix(sizeLine, []byte("\r\n")) {
		return 0, errors.New("invalid chunk-size line: missing trailing CRLF")
	}

	line := strings.TrimSuffix(string(sizeLine), "\r\n")
	if idx := strings.IndexByte(line, ';'); idx >= 0 {
		// Eliminamos extensiones como ;foo=bar si es que existen
		line = line[:idx]
	}
	line = strings.TrimSpace(line)

	if line == "" { // #8 Si la linea esta vacia, posiblemente el final de todos los Chunks
		return 0, errors.New("empty chunk-size")
	}

	n, err := strconv.ParseInt(line, 16, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid chunk-size: %q", line)
	}

	// #8 Devuelve el numero de bytes que ocupa => Numero de Bytes + '\r\n' + Bytes a Leer; OJO, no suma al contador los 2 ultimos bytes '\r\n'
	return n, nil
}

func readUntilCRLF(conn net.Conn, maxBytes int) ([]byte, error) {
	// #8 Lee hasta "\r\n"
	var out bytes.Buffer
	one := []byte{0}
	prevCR := false

	for {
		if out.Len() >= maxBytes {
			return nil, fmt.Errorf("HTTP line too long (>%d bytes)", maxBytes)
		}

		if _, err := io.ReadFull(conn, one); err != nil {
			return nil, err
		}

		b := one[0]
		out.WriteByte(b)

		if prevCR && b == '\n' {
			return out.Bytes(), nil
		}

		prevCR = (b == '\r')
	}
}

func readUntilDoubleCRLF(conn net.Conn, maxBytes int) ([]byte, error) {
	// #8 Lee todos los Headers Completos de la peticion HTTP. Es decir, lee hasta "\r\n\r\n"
	// #8 Un bucle infinito en el que cada iteracion:
	// #8 1.- Lee un byte del buffer y lo introduce en una lista de 4 bytes en la posicion 3. Si ya había bytes en las posiciones 0, 1, 2 y/o 3, los desplaza una posicion abajo, descartando evidentemente el de la posicion 0.
	// #8 2.- Comprueba que esa lista coincida con los bytes equivalentes al codificar en ASCII "\r\n\r\n". Si coincide retorna todo el buffer leido hasta el momento

	var out bytes.Buffer
	var tail [4]byte
	tailLen := 0
	one := []byte{0}

	for {
		if out.Len() >= maxBytes {
			return nil, fmt.Errorf("HTTP header too large (>%d bytes)", maxBytes)
		}

		// #8 io.ReadFull(conn, one) lee len(one) bytes de "conn" con .Read. Si lee menos de len(one), bloquea hasta leer len(one) o error. Aunque aquí da igual, ya que solo leemos 1 byte
		if _, err := io.ReadFull(conn, one); err != nil {
			return nil, err
		}

		b := one[0]
		out.WriteByte(b)

		if tailLen < 4 {
			// #8 Llenamos la lista con los primeros 4 bytes
			tail[tailLen] = b
			tailLen++
		} else {
			// #8 Descartamos el primer byte de la lista e insertamos el siguiente byte leido
			tail[0], tail[1], tail[2], tail[3] = tail[1], tail[2], tail[3], b
		}

		// #8 Si los 4 ultimos bytes leidos coinciden con "\r\n\r\n" retornamos todo el buffer en bytes
		// #8 `` indica rune, no string ("")
		// #8 Las runas son uint32, pero pueden comparar con byte (uint8) ya que estos caracteres \r y \n son caracteres ASCII
		if tailLen == 4 &&
			tail[0] == '\r' &&
			tail[1] == '\n' &&
			tail[2] == '\r' &&
			tail[3] == '\n' {
			return out.Bytes(), nil
		}
	}
}

type StartLineData struct {
	IsResponse   bool
	Method       string // Solo Peticiones (ej. "GET", "POST")
	Path         string // Solo Peticiones (URI o Path)
	Version      string // Versión completa (ej. "HTTP/1.1")
	StatusCode   int    // Solo Respuestas (ej. 200)
	ReasonPhrase string // Solo Respuestas (ej. "OK" o "Not Found")
}

func parseStartLine(startLine []byte) (StartLineData, error) {

	var data StartLineData

	idx1 := bytes.IndexByte(startLine, ' ')
	if idx1 == -1 {
		return data, errors.New("invalid format: missing the first SP delimiter")
	}

	part1 := startLine[:idx1]
	rest := startLine[idx1+1:]

	var part2, part3 []byte
	idx2 := bytes.IndexByte(rest, ' ')
	if idx2 == -1 {
		// Tolerancia para HTTP/0.9 o respuestas anómalas sin Reason-Phrase
		part2 = rest
	} else {
		part2 = rest[:idx2]
		part3 = rest[idx2+1:] // El tercer bloque puede contener múltiples espacios (ej. "Not Found")
	}

	// #8 Determinar el Tipo de Mensaje (Request vs Response)
	if bytes.HasPrefix(part1, []byte("HTTP/")) {
		// #8 Es una RESPUESTA (Status Line: HTTP-Version Status-Code Reason-Phrase)
		data.IsResponse = true
		data.Version = string(part1)
		data.ReasonPhrase = string(part3)
		data.StatusCode = func(b []byte) int {
			if len(b) < 3 {
				return 0
			}
			// Multiplicamos posicionalmente: centenas, decenas y unidades.
			return int(b[0]-'0')*100 + int(b[1]-'0')*10 + int(b[2]-'0')
		}(part2)
	} else {
		// #8 Es una PETICIÓN (Request Line: Method Request-Target HTTP-Version)
		data.IsResponse = false
		data.Method = string(part1)
		data.Path = string(part2)
		data.Version = string(part3)
	}

	return data, nil
}
