package utils

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

const (
	// MaxCapturedBodySize is the maximum body size exposed by the inspector.
	// Readers keep one extra byte so the history layer can mark the body as
	// truncated without buffering an unbounded decompressed response.
	MaxCapturedBodySize = 500 * 1024

	// MaxEncodedBodySize bounds intermediate compressed/stacked
	// representations, which may be larger than the final preview. It leaves
	// ample room for valid, poorly-compressible payloads.
	MaxEncodedBodySize = 8 * 1024 * 1024

	// maxZstdDecoderMemory bounds the decoder window and allocations when
	// inspecting potentially hostile response bodies.
	maxZstdDecoderMemory = 64 * 1024 * 1024
)

// ContentDecodingError reports a malformed supported Content-Encoding. The
// returned body remains the original encoded bytes (bounded for inspection),
// allowing callers to record the response without disrupting proxy traffic.
type ContentDecodingError struct {
	Encoding string
	Err      error
}

func (err *ContentDecodingError) Error() string {
	return fmt.Sprintf("decode Content-Encoding %q: %v", err.Encoding, err.Err)
}

func (err *ContentDecodingError) Unwrap() error {
	return err.Err
}

// ExtractAndDecompressBodyHTTP reads an HTTP body and decodes every supported
// Content-Encoding in the reverse order in which the encodings were applied.
//
// Supported encodings are gzip, deflate (both the RFC-compliant zlib wrapper
// and the commonly encountered raw DEFLATE variant), br, zstd, and identity.
//
// If any encoding in the header is unsupported, a bounded copy of the original
// body is returned unchanged. This avoids presenting a partially decoded body
// and lets callers surface the condition to the user.
func ExtractAndDecompressBodyHTTP(contentEncoding string, body io.Reader) (string, error) {
	if body == nil {
		return "", nil
	}

	encodings := parseContentEncodings(contentEncoding)
	unsupportedEncodings := UnsupportedContentEncodings(contentEncoding)

	readLimit := int64(MaxCapturedBodySize + 1)
	if len(unsupportedEncodings) == 0 && containsCompressedEncoding(encodings) {
		readLimit = int64(MaxEncodedBodySize + 1)
	}

	data, err := readLimited(body, readLimit)
	if err != nil {
		return "", fmt.Errorf("read HTTP body: %w", err)
	}
	rawBody := truncateBytes(data, MaxCapturedBodySize+1)

	if len(unsupportedEncodings) != 0 {
		return string(rawBody), nil
	}

	// Empty bodies are valid for HEAD and 1xx/204/304 responses even when
	// Content-Encoding describes the selected representation.
	if len(data) == 0 {
		return "", nil
	}

	finalDecoderIndex := firstCompressedEncodingIndex(encodings)
	for i := len(encodings) - 1; i >= 0; i-- {
		encoding := encodings[i]
		if encoding == "identity" {
			continue
		}

		decodeLimit := int64(MaxEncodedBodySize + 1)
		if i == finalDecoderIndex {
			decodeLimit = int64(MaxCapturedBodySize + 1)
		}

		data, err = decodeContentEncoding(encoding, data, decodeLimit)
		if err != nil {
			return string(rawBody), &ContentDecodingError{
				Encoding: encoding,
				Err:      err,
			}
		}
	}

	return string(truncateBytes(data, MaxCapturedBodySize+1)), nil
}

// UnsupportedContentEncodings returns the unsupported encoding tokens found in
// a Content-Encoding header. Tokens are normalized to lower case, trimmed, and
// de-duplicated while preserving their first-seen order.
//
// Empty tokens are ignored. gzip, deflate, br, zstd, and identity are supported.
func UnsupportedContentEncodings(contentEncoding string) []string {
	encodings := parseContentEncodings(contentEncoding)
	unsupported := make([]string, 0)
	seen := make(map[string]struct{})

	for _, encoding := range encodings {
		if isSupportedContentEncoding(encoding) {
			continue
		}
		if _, exists := seen[encoding]; exists {
			continue
		}

		seen[encoding] = struct{}{}
		unsupported = append(unsupported, encoding)
	}

	return unsupported
}

func parseContentEncodings(contentEncoding string) []string {
	if strings.TrimSpace(contentEncoding) == "" {
		return nil
	}

	parts := strings.Split(contentEncoding, ",")
	encodings := make([]string, 0, len(parts))
	for _, part := range parts {
		encoding := strings.ToLower(strings.TrimSpace(part))
		if encoding != "" {
			encodings = append(encodings, encoding)
		}
	}

	return encodings
}

func isSupportedContentEncoding(encoding string) bool {
	switch encoding {
	case "gzip", "deflate", "br", "zstd", "identity":
		return true
	default:
		return false
	}
}

func containsCompressedEncoding(encodings []string) bool {
	return firstCompressedEncodingIndex(encodings) >= 0
}

func firstCompressedEncodingIndex(encodings []string) int {
	for index, encoding := range encodings {
		if encoding != "identity" {
			return index
		}
	}
	return -1
}

func truncateBytes(data []byte, limit int) []byte {
	if len(data) <= limit {
		return data
	}
	return data[:limit]
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, limit))
}

func decodeContentEncoding(encoding string, data []byte, limit int64) ([]byte, error) {
	switch encoding {
	case "gzip":
		return decodeGzip(data, limit)
	case "deflate":
		return decodeDeflate(data, limit)
	case "br":
		return decodeBrotli(data, limit)
	case "zstd":
		return decodeZstd(data, limit)
	default:
		// All encodings are validated before decoding starts. Keep this guard so
		// future changes cannot accidentally turn an unknown encoding into a no-op.
		return nil, fmt.Errorf("unsupported encoding")
	}
}

func decodeGzip(data []byte, limit int64) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("initialize gzip reader: %w", err)
	}

	decoded, err := readAndCloseDecoder(reader, limit)
	if err != nil {
		return nil, fmt.Errorf("read gzip stream: %w", err)
	}
	return decoded, nil
}

func decodeDeflate(data []byte, limit int64) ([]byte, error) {
	zlibReader, zlibErr := zlib.NewReader(bytes.NewReader(data))
	if zlibErr == nil {
		decoded, err := readAndCloseDecoder(zlibReader, limit)
		if err != nil {
			return nil, fmt.Errorf("read zlib-wrapped DEFLATE stream: %w", err)
		}
		return decoded, nil
	}

	rawReader := flate.NewReader(bytes.NewReader(data))
	decoded, rawErr := readAndCloseDecoder(rawReader, limit)
	if rawErr != nil {
		return nil, fmt.Errorf(
			"invalid zlib-wrapped or raw DEFLATE stream (zlib: %v; raw: %w)",
			zlibErr,
			rawErr,
		)
	}
	return decoded, nil
}

func decodeBrotli(data []byte, limit int64) ([]byte, error) {
	decoded, err := readLimited(brotli.NewReader(bytes.NewReader(data)), limit)
	if err != nil {
		return nil, fmt.Errorf("read Brotli stream: %w", err)
	}
	return decoded, nil
}

func decodeZstd(data []byte, limit int64) ([]byte, error) {
	reader, err := zstd.NewReader(
		bytes.NewReader(data),
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderLowmem(true),
		zstd.WithDecoderMaxMemory(maxZstdDecoderMemory),
		zstd.WithDecoderMaxWindow(maxZstdDecoderMemory),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize Zstandard reader: %w", err)
	}
	defer reader.Close()

	decoded, err := readLimited(reader, limit)
	if err != nil {
		return nil, fmt.Errorf("read Zstandard stream: %w", err)
	}
	return decoded, nil
}

func readAndCloseDecoder(reader io.ReadCloser, limit int64) ([]byte, error) {
	decoded, readErr := readLimited(reader, limit)
	closeErr := reader.Close()

	switch {
	case readErr != nil:
		return nil, readErr
	case closeErr != nil:
		return nil, closeErr
	default:
		return decoded, nil
	}
}
