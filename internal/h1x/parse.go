package h1x

import (
	"bufio"
	"bytes"
	"fmt"
	"marmota/internal/utils"
	"net/http"
)

func ParseRawRequest(rawRequest []byte) (string, string, error) {
	// #8 Extracción de rawHeaders preservando el orden y casing de los headers originales
	idx := bytes.Index(rawRequest, []byte("\r\n\r\n"))
	if idx == -1 {
		return "", "", fmt.Errorf("malformed HTTP request: separator not found")
	}

	headBlockStr := string(rawRequest[:idx+4]) // + 4 para incluir el \r\n\r\n final

	// #8 Parseo con http.ReadRequest para obtener el Body de forma sencilla
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(rawRequest)))
	if err != nil {
		return headBlockStr, "", err
	}

	// #8 Extraer el Body en UTF-8 y descomprimirlo si aplica
	bodyStr, err := utils.ExtractAndDecompressBodyHTTP(req.Header.Get("Content-Encoding"), req.Body)
	req.Body.Close()
	return headBlockStr, bodyStr, err
}

func ParseRawResponse(rawResponse []byte, rawRequest []byte) (string, string, error) {
	// #8 Extracción de rawHeaders preservando el orden y casing de los headers originales
	idx := bytes.Index(rawResponse, []byte("\r\n\r\n"))
	if idx == -1 {
		return "", "", fmt.Errorf("malformed HTTP response: separator not found")
	}

	headBlockStr := string(rawResponse[:idx+4]) // + 4 para incluir el \r\n\r\n final

	// #8 Parseo con http.ReadResponse para obtener el Body de forma sencilla
	// #8 Pasamos como parametro "req" a http.ReadResponse, porque a veces lo necesita para peticiones HEAD

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(rawRequest)))
	if err != nil {
		return headBlockStr, "", err
	}

	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(rawResponse)), req)
	if err != nil {
		return headBlockStr, "", err
	}

	// #8 Extraer el Body en UTF-8 y descomprimirlo si aplica
	bodyStr, err := utils.ExtractAndDecompressBodyHTTP(resp.Header.Get("Content-Encoding"), resp.Body)
	req.Body.Close()
	return headBlockStr, bodyStr, err
}
