package h2

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"marmota/internal/utils"
	"math"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

// HTTP2Message ahora incluye campos explícitos para los metadatos de enrutamiento (Punto 3)
type HTTP2Message struct {
	StreamID   uint32
	IsResponse bool
	Method     string // Extraído de :method
	Path       string // Extraído de :path
	Authority  string // Extraído de :authority
	Status     string // Extraído de :status
	Headers    map[string]string
	Body       string
	Error      error // Para notificar si el stream se interrumpió abruptamente
}

type streamState struct {
	Method          string
	Path            string
	Authority       string
	Status          string
	Headers         map[string]string
	Body            *bytes.Buffer
	EndStreamSeen   bool
	Error           error
	ContentEncoding string
}

func SniffH2Traffic(r io.Reader, isResponse bool, outChan chan<- HTTP2Message) error {
	if !isResponse {
		preface := make([]byte, 24)
		if _, err := io.ReadFull(r, preface); err != nil {
			return err
		}
		if !bytes.Equal(preface, []byte(http2.ClientPreface)) {
			return errors.New("invalid HTTP/2 preface: connection is not HTTP/2 or is encrypted")
		}
	}

	// #8 http2.NewFramer es el Parser de Frames HTTP/2. Requiere un Writer donde escribir lo leido, pero como nosotros solo queremos escuchar, los mandamos descartamos con io.Discard
	framer := http2.NewFramer(io.Discard, r)

	// #8 hpack.NewDecoder crea el decodificador de Cabeceras de HTTP/2 (Ya que estan vienen codificadas con el algoritmo HPACK)
	hpackDec := hpack.NewDecoder(4096, nil) // 4096 es el limite maximo inicial indicado por la especifciación RFC

	// #8 A veces, en las cabeceras nos indican que aumentemos el tamaño de la tabla para que quepan mas headers.
	// #8 Con esto modificamos el limite maximo de la tabla, asi no tendremos errores si en los HEADERS nos indican que aumentemos el tamaño de la tabla.
	hpackDec.SetAllowedMaxDynamicTableSize(math.MaxUint32)

	streams := make(map[uint32]*streamState)
	var currentStreamID uint32

	// #8 Si la conexión muere dejando streams abiertos, vaciamos la memoria y notificamos los streams incompletos.
	defer func() {

		for id, state := range streams {
			body, _ := utils.ExtractAndDecompressBodyHTTP(state.ContentEncoding, state.Body)

			outChan <- HTTP2Message{
				StreamID:   id,
				IsResponse: isResponse,
				Method:     state.Method,
				Path:       state.Path,
				Authority:  state.Authority,
				Status:     state.Status,
				Headers:    state.Headers,
				Body:       body,
				Error:      io.ErrUnexpectedEOF,
			}
		}
	}()

	// #8 Cuando hacemos hpackDec.Write(frame) se ejecuta la función que pasemos a hpackDec.SetEmitFunc
	// #8 Ojo, hpackDec.Write(frame) es bloqueante. No se libera hasta que termina la función que pasemos a hpackDec.SetEmitFunc. Por lo que currentStreamID es imposible que cambie
	hpackDec.SetEmitFunc(func(f hpack.HeaderField) {
		s, ok := streams[currentStreamID]
		if !ok {
			return
		}

		if strings.HasPrefix(f.Name, ":") {
			switch f.Name {
			case ":method":
				s.Method = f.Value
			case ":path":
				s.Path = f.Value
			case ":authority":
				s.Authority = f.Value
			case ":status":
				s.Status = f.Value
			}
		} else {
			// Cabeceras regulares se añaden al map manteniendo casing
			if strings.EqualFold(f.Name, "Content-Encoding") {
				s.ContentEncoding = f.Value
			}
			s.Headers[f.Name] = f.Value
		}
	})

	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			// #8 Aquí entra cuando se cierra limpiamente (io.EOF) o abruptamente (io.ErrUnexpectedEOF). También por errores de conexion o de frame invalidos de HTTP/2
			// #8 Se dispara el defer y los streams que no habían terminado se les asigna el error io.ErrUnexpectedEOF
			// fmt.Printf("Error en framer.ReadFrame() en HTTP/2: %v\n", err)
			return err
		}

		streamID := frame.Header().StreamID
		if streamID == 0 {
			// #8 El Stream ID 0 está reservado exclusivamente para la gestión y control de la conexión general. Nunca transporta peticiones o respuestas HTTP
			continue
		}

		if _, ok := streams[streamID]; !ok {
			// #8 Si no existe el Stream todavía lo creamos
			if frame.Header().Type == http2.FrameHeaders {
				streams[streamID] = &streamState{
					Headers: make(map[string]string),
					Body:    &bytes.Buffer{},
				}
			} else if frame.Header().Type == http2.FrameContinuation || frame.Header().Type == http2.FramePushPromise {
				// #8 Si entra aquí, es que nos ha llegado un CONTINUATION_FRAME o PUSH_PROMISE de un ID que no conocemos. Como SI tiene datos HPACK, por lo que aunque ignoremos sus datos, tenemos que decodificar las cabeceras para sincronizar la tabla HPACK
			} else {
				// #8 Si entra aquí, es que nos ha llegado un WINDOW_UPDATE, DATA o RST_STREAM de un ID que no conocemos. Como NO tiene datos HPACK, es basura
				continue
			}
		}

		state := streams[streamID]
		readyToEmit := false
		currentStreamID = streamID

		// #8 El Flag END_STREAM se comprueba con f.StreamEnded()
		// #8 Cuando un extremo envia un Frame, pone en "1" este Flag cuando ya es el ultimo Frame que va a enviar
		switch f := frame.(type) {
		case *http2.HeadersFrame:
			// #8 Bloquea hasta que la funcion que hemos pasado a hpackDec.SetEmitFunc(func) terminé
			if _, err := hpackDec.Write(f.HeaderBlockFragment()); err != nil {
				return fmt.Errorf("hpack decoding error on stream %d: %w", streamID, err)
			}
			if f.HeadersEnded() {
				hpackDec.Close() // Indica que el bloque de cabeceras que estábamos construyendo ya ha terminado
			}
			if f.StreamEnded() { // Sin body
				state.EndStreamSeen = true
			}
			if f.HeadersEnded() && state.EndStreamSeen {
				readyToEmit = true
			}
		case *http2.ContinuationFrame:
			// #8 Lo mismo que en *http2.HeadersFrame. Si el Flag END_STREAM no esta en ContinuationFrame, si no en HeadersFrame o DataFrame.
			// #8 ContinuationFrame siempre es una continuacion de HeadersFrame, no de DataFrame. Ya que DataFrame pueden enviarse varios
			if _, err := hpackDec.Write(f.HeaderBlockFragment()); err != nil {
				return fmt.Errorf("hpack decoding error on stream %d: %w", streamID, err)
			}
			if f.HeadersEnded() {
				hpackDec.Close() // <--- ¡AQUÍ TAMBIÉN!
				if state.EndStreamSeen {
					readyToEmit = true
				}
			}

		case *http2.DataFrame:
			_, err := state.Body.Write(f.Data())
			if err != nil {
				return fmt.Errorf("write body error on stream %d: %w", streamID, err)
			}
			if f.StreamEnded() {
				readyToEmit = true
			}
		case *http2.PushPromiseFrame:
			// #8 PUSH_PROMISE: El servidor nos avisa de que va a enviarnos un recurso no solicitado (Server Push).
			// #8 Aunque lo ignoremos, es obligatorio decodificar sus cabeceras para no corromper la tabla de estado (HPACK).
			if _, err := hpackDec.Write(f.HeaderBlockFragment()); err != nil {
				return fmt.Errorf("hpack decoding error on PUSH_PROMISE: %w", err)
			}
			if f.HeadersEnded() {
				hpackDec.Close() // <--- ¡Y AQUÍ!
			}
		case *http2.RSTStreamFrame:
			// #8 El frame RST_STREAM se envía cuando un extremo decide abortar la transacción de forma inmediata. Esto ocurre por errores (ej. "Internal Server Error"), violaciones de protocolo
			state.Error = fmt.Errorf("stream reset (RST_STREAM) with code: %v", f.ErrCode)
			readyToEmit = true
		}

		if readyToEmit {
			body, err := utils.ExtractAndDecompressBodyHTTP(state.ContentEncoding, state.Body)
			if err != nil {
				return fmt.Errorf("error extracting and decompressing the body: %w", err)
			}

			outChan <- HTTP2Message{
				StreamID:   streamID,
				IsResponse: isResponse,
				Method:     state.Method,
				Path:       state.Path,
				Authority:  state.Authority,
				Status:     state.Status,
				Headers:    state.Headers,
				Body:       body,
				Error:      state.Error,
			}
			delete(streams, streamID) // Borramos este stream del Map una vez emitido por el canal
		}
	}
}

func BuildHTTP1HeadBlockStr(msg *HTTP2Message) (string, error) {
	var buf bytes.Buffer

	if msg.IsResponse {
		// HTTP/1.x Response Start-Line: HTTP/1.1 <status_code> <reason_phrase>\r\n
		statusCode, err := strconv.Atoi(msg.Status)
		if err != nil {
			return "", err
		}

		reason := http.StatusText(statusCode)

		_, err = fmt.Fprintf(&buf, "HTTP/2 %s %s\r\n", msg.Status, reason)
		if err != nil {
			return "", err
		}
	} else {
		// HTTP/1.x Request Start-Line: <method> <path> HTTP/1.1\r\n
		method := msg.Method
		if method == "" {
			method = "GET" // Fallback por seguridad
		}
		path := msg.Path
		if path == "" {
			path = "/"
		}

		_, err := fmt.Fprintf(&buf, "%s %s HTTP/2\r\n", method, path)
		if err != nil {
			return "", err
		}

		// #8 En HTTP/1.x, el pseudo-header :authority de HTTP/2 se mapea al header Host.
		// #8 Solo lo añadimos si no viene ya explícitamente en el mapa de Headers.
		hasHost := false
		for k := range msg.Headers {
			if strings.EqualFold(k, "host") {
				hasHost = true
				break
			}
		}
		if !hasHost && msg.Authority != "" {
			_, err := fmt.Fprintf(&buf, "Host: %s\r\n", msg.Authority)
			if err != nil {
				return "", err
			}
		}
	}

	// #8 Inyectar el resto de cabeceras
	for k, v := range msg.Headers {
		_, err := fmt.Fprintf(&buf, "%s: %s\r\n", k, v)
		if err != nil {
			return "", err
		}
	}

	_, err := buf.WriteString("\r\n")
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
