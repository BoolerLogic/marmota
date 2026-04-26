package utils

import (
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
)

// #8 Extrae el cuerpo de la respuesta HTTP y lo descomprime si es necesario
func ExtractAndDecompressBodyHTTP(contentEncoding string, body io.Reader) (string, error) {
	if body == nil {
		return "", nil
	}

	var reader io.ReadCloser
	var err error

	// 1. Configuración del reader según el Content-Encoding
	switch contentEncoding {
	case "gzip":
		reader, err = gzip.NewReader(body)
		if err != nil {
			return "", fmt.Errorf("error initializing gzip: %w", err)
		}
	case "deflate":
		reader, err = zlib.NewReader(body)
		if err != nil {
			return "", fmt.Errorf("error initializing zlib: %w", err)
		}
	default:
		// Envolvemos el io.Reader original para que cumpla la interfaz io.ReadCloser
		reader = io.NopCloser(body)
	}

	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("error reading the body: %w", err)
	}

	// #8 Si los datos son puramente binarios como iamgenes, archivos o simplemente tiene una codificacion distinta de UTF-8, mostrara texto no legible, pero no dara error.
	return string(data), nil
}
