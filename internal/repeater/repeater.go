package repeater

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"marmota/internal/bridge"
	"marmota/internal/h1x"
	"marmota/internal/h2"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

type RepeaterHttp2PseudoHeaders struct {
	Method    string `json:"method"`
	Scheme    string `json:"scheme"`
	Authority string `json:"authority"`
	Path      string `json:"path"`
	Protocol  string `json:"protocol"`
}

type RepeaterSendPayload struct {
	Scheme               string                     `json:"scheme"` // "http" o "https"
	Host                 string                     `json:"host"`
	Port                 string                     `json:"port"`
	Method               string                     `json:"method"`               // metodo de la request line o :method
	Path                 string                     `json:"path"`                 // path de la request line o :path
	HeadBlockStr         string                     `json:"headBlockStr"`         // El frontend lo envian exactamente en "\r\n\r\n"
	BodyStr              string                     `json:"bodyStr"`              // body raw
	SkipServerCertVerify bool                       `json:"skipServerCertVerify"` // true => no validar TLS SERVIDOR
	Version              string                     `json:"version"`              // HTTP/1.0 | HTTP/1.1 | HTTP/2
	PseudoHeaders        RepeaterHttp2PseudoHeaders `json:"pseudoHeaders"`        // siempre va relleno
	Headers              map[string][]string        `json:"headers"`              // headers normales, sin pseudoheaders
}

type RepeaterSendResult struct {
	HeadBlockStr string `json:"headBlockStr"`
	BodyStr      string `json:"bodyStr"`
	Host         string `json:"host,omitempty"`
	Port         string `json:"port,omitempty"`
	Version      string `json:"version,omitempty"`
	StatusCode   *int   `json:"statusCode"`
	DurationMs   *int64 `json:"durationMs"`
}

func SendRepeaterRequest(payload RepeaterSendPayload) (RepeaterSendResult, error) {
	startTime := time.Now()

	rawRequest := []byte(payload.HeadBlockStr + payload.BodyStr)

	var serverConn net.Conn
	var err error
	if payload.Scheme == "http" {
		serverConn, err = net.Dial("tcp", payload.Host+":"+payload.Port)
		if err != nil {
			bridge.EmitError(fmt.Sprintf("Could not open a TCP connection to the server: %v", err))
			return RepeaterSendResult{}, err
		}
	} else {
		var protocolVersion string
		if payload.Version == "HTTP/2" {
			protocolVersion = "h2"
		} else {
			protocolVersion = "http/1.1"
		}

		serverTLSConfig := &tls.Config{
			InsecureSkipVerify: payload.SkipServerCertVerify,
			NextProtos:         []string{protocolVersion},
		}

		serverConn, err = tls.Dial("tcp", payload.Host+":"+payload.Port, serverTLSConfig)
		if err != nil {
			bridge.EmitError(fmt.Sprintf("TLS handshake with the server failed: %v", err))
			return RepeaterSendResult{}, err
		}
	}

	defer serverConn.Close()

	if payload.Version != "HTTP/2" { // #8 HTTP/1.0 y HTTP/1.1
		_, err := serverConn.Write(rawRequest)
		if err != nil {
			return RepeaterSendResult{}, err
		}

		rawResponse, startLineData, err := h1x.ReadSanitizedHTTP(serverConn, payload.Method)
		if err != nil {
			return RepeaterSendResult{}, err
		}

		headBlockStr, bodyStr, err := h1x.ParseRawResponse(rawResponse, rawRequest)
		if err != nil {
			return RepeaterSendResult{}, err
		}

		durationMs := time.Since(startTime).Milliseconds()
		return RepeaterSendResult{
			HeadBlockStr: headBlockStr,
			BodyStr:      bodyStr,
			Host:         payload.Host,
			Port:         payload.Port,
			Version:      payload.Host,
			StatusCode:   &startLineData.StatusCode,
			DurationMs:   &durationMs,
		}, nil
	} else { // #8 HTTP/2
		// Enviar client connection preface
		if _, err := io.WriteString(serverConn, http2.ClientPreface); err != nil {
			return RepeaterSendResult{}, fmt.Errorf("writing preface: %w", err)
		}

		framer := http2.NewFramer(serverConn, serverConn)
		framer.AllowIllegalWrites = true // desactiva las validaciones de frames que violan el RFC

		// Enviar SETTINGS vacío (vacío significa "acepto todos los defaults del RFC")
		if err := framer.WriteSettings(); err != nil {
			return RepeaterSendResult{}, fmt.Errorf("write settings: %w", err)
		}

		// Leer el SETTINGS del servidor y responder con SETTINGS ACK
		for {
			frame, err := framer.ReadFrame()
			if err != nil {
				return RepeaterSendResult{}, err
			}
			if _, ok := frame.(*http2.SettingsFrame); ok {
				break
			}
		}
		if err := framer.WriteSettingsAck(); err != nil {
			return RepeaterSendResult{}, fmt.Errorf("write settings ack: %w", err)
		}

		headers := bytes.Buffer{}
		encoder := hpack.NewEncoder(&headers)

		pseudos := map[string]string{
			":method":    payload.PseudoHeaders.Method,
			":scheme":    payload.PseudoHeaders.Scheme,
			":authority": payload.PseudoHeaders.Authority,
			":path":      payload.PseudoHeaders.Path,
		}

		for name, value := range pseudos {
			if value == "" {
				continue
			}
			if err := encoder.WriteField(hpack.HeaderField{Name: name, Value: value}); err != nil {
				return RepeaterSendResult{}, fmt.Errorf("encode pseudo %s: %w", name, err)
			}
		}

		for name, values := range payload.Headers {
			lower := strings.ToLower(name)
			// Saltar pseudoheaders que hayan llegado mezclados en los headers normales
			if strings.HasPrefix(lower, ":") {
				continue
			}
			// En HTTP/2 no se permite Transfer-Encoding: chunked ni Connection
			if lower == "transfer-encoding" || lower == "connection" || lower == "host" {
				continue
			}
			for _, v := range values {
				if err := encoder.WriteField(hpack.HeaderField{Name: lower, Value: v}); err != nil {
					return RepeaterSendResult{}, fmt.Errorf("encode header %s: %w", lower, err)
				}
			}
		}

		streamID := uint32(1)

		hasBody := len(payload.BodyStr) > 0

		// #8 Enviar HEADERS frame
		err = framer.WriteHeaders(http2.HeadersFrameParam{
			StreamID:      streamID,
			BlockFragment: headers.Bytes(),
			EndHeaders:    true,
			EndStream:     !hasBody, // si no hay body, cerramos el stream aquí
		})
		if err != nil {
			return RepeaterSendResult{}, fmt.Errorf("write headers frame: %w", err)
		}

		if hasBody {
			// #8 Enviar DATA frame
			err = framer.WriteData(streamID, true, []byte(payload.BodyStr))
			if err != nil {
				return RepeaterSendResult{}, fmt.Errorf("write data frame: %w", err)
			}
		}

		ch := make(chan h2.HTTP2Message)

		go func() {
			defer close(ch)
			err := h2.SniffH2Traffic(serverConn, true, ch)
			if err != nil && err != io.EOF {
				ch <- h2.HTTP2Message{
					Error: err,
				}
			}
		}()

		resp := <-ch
		headBlockStr, err := h2.BuildHTTP1HeadBlockStr(&resp)
		if err != nil {
			return RepeaterSendResult{}, fmt.Errorf("build http1 head block str: %w", err)
		}

		statusCode, err := strconv.Atoi(resp.Status)
		if err != nil {
			return RepeaterSendResult{}, fmt.Errorf("converting resp.Status to int: %w", err)
		}

		durationMs := time.Since(startTime).Milliseconds()
		return RepeaterSendResult{
			HeadBlockStr: headBlockStr,
			BodyStr:      resp.Body,
			Host:         payload.Host,
			Port:         payload.Port,
			Version:      payload.Version,
			StatusCode:   &statusCode,
			DurationMs:   &durationMs,
		}, nil
	}
}
