package utils

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func TestUnsupportedContentEncodings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		want   []string
	}{
		{
			name:   "empty header",
			header: "",
			want:   []string{},
		},
		{
			name:   "supported encodings and whitespace",
			header: " GZip, deflate,\tBR , identity ",
			want:   []string{},
		},
		{
			name:   "normalized and de-duplicated",
			header: " ZSTD, gzip, x-CUSTOM, zstd, X-Custom, compress ",
			want:   []string{"x-custom", "compress"},
		},
		{
			name:   "empty list members ignored",
			header: " , x-custom, , X-CUSTOM, ",
			want:   []string{"x-custom"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := UnsupportedContentEncodings(test.header)
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("UnsupportedContentEncodings(%q) = %#v, want %#v", test.header, got, test.want)
			}
		})
	}
}

func TestExtractAndDecompressBodyHTTPSupportedEncodings(t *testing.T) {
	t.Parallel()

	original := []byte("Marmota: texto UTF-8 — and binary\x00content")
	tests := []struct {
		name       string
		header     string
		encodings  []string
		rawDeflate bool
	}{
		{
			name:   "no content encoding",
			header: "",
		},
		{
			name:      "identity",
			header:    " identity ",
			encodings: []string{"identity"},
		},
		{
			name:      "gzip is case insensitive",
			header:    " GZiP ",
			encodings: []string{"gzip"},
		},
		{
			name:      "zlib wrapped deflate",
			header:    "deflate",
			encodings: []string{"deflate"},
		},
		{
			name:       "raw deflate compatibility",
			header:     " DEFLATE ",
			encodings:  []string{"deflate"},
			rawDeflate: true,
		},
		{
			name:      "brotli",
			header:    "br",
			encodings: []string{"br"},
		},
		{
			name:      "zstandard",
			header:    "zstd",
			encodings: []string{"zstd"},
		},
		{
			name:      "multiple encodings decoded in reverse order",
			header:    " gzip, BR, identity, zstd, deflate ",
			encodings: []string{"gzip", "br", "identity", "zstd", "deflate"},
		},
		{
			name:      "repeated encoding",
			header:    "gzip, gzip",
			encodings: []string{"gzip", "gzip"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			encoded := append([]byte(nil), original...)
			for _, encoding := range test.encodings {
				if test.rawDeflate && encoding == "deflate" {
					encoded = encodeRawDeflate(t, encoded)
					continue
				}
				encoded = encodeContent(t, encoding, encoded)
			}

			got, err := ExtractAndDecompressBodyHTTP(test.header, bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("ExtractAndDecompressBodyHTTP() error = %v", err)
			}
			if !bytes.Equal([]byte(got), original) {
				t.Fatalf("decoded body = %q, want %q", []byte(got), original)
			}
		})
	}
}

func TestExtractAndDecompressBodyHTTPConcatenatedGzipStreams(t *testing.T) {
	t.Parallel()

	first := encodeContent(t, "gzip", []byte("first "))
	second := encodeContent(t, "gzip", []byte("second"))
	encoded := append(first, second...)

	got, err := ExtractAndDecompressBodyHTTP("gzip", bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("ExtractAndDecompressBodyHTTP() error = %v", err)
	}
	if got != "first second" {
		t.Fatalf("decoded body = %q, want %q", got, "first second")
	}
}

func TestExtractAndDecompressBodyHTTPUnsupportedEncodingReturnsRawBody(t *testing.T) {
	t.Parallel()

	compressed := encodeContent(t, "gzip", []byte("must remain compressed"))
	headers := []string{
		"x-custom",
		"x-custom, gzip",
		"gzip, x-custom",
	}

	for _, header := range headers {
		header := header
		t.Run(header, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractAndDecompressBodyHTTP(header, bytes.NewReader(compressed))
			if err != nil {
				t.Fatalf("ExtractAndDecompressBodyHTTP() error = %v", err)
			}
			if !bytes.Equal([]byte(got), compressed) {
				t.Fatal("body was changed despite the Content-Encoding chain containing an unsupported encoding")
			}
		})
	}
}

func TestExtractAndDecompressBodyHTTPCorruptSupportedEncoding(t *testing.T) {
	t.Parallel()

	validGzip := encodeContent(t, "gzip", []byte("gzip payload"))
	validZlib := encodeContent(t, "deflate", []byte("zlib payload"))
	validRawDeflate := encodeRawDeflate(t, []byte("raw deflate payload"))
	validBrotli := encodeContent(t, "br", []byte(strings.Repeat("brotli payload ", 8)))
	validZstd := encodeContent(t, "zstd", []byte(strings.Repeat("zstd payload ", 32)))

	tests := []struct {
		name       string
		header     string
		body       []byte
		wantErrSub string
	}{
		{
			name:       "invalid gzip header",
			header:     "gzip",
			body:       []byte("not gzip"),
			wantErrSub: `decode Content-Encoding "gzip"`,
		},
		{
			name:       "truncated gzip",
			header:     "gzip",
			body:       validGzip[:len(validGzip)-4],
			wantErrSub: "read gzip stream",
		},
		{
			name:       "corrupt zlib checksum",
			header:     "deflate",
			body:       corruptLastByte(validZlib),
			wantErrSub: "read zlib-wrapped DEFLATE stream",
		},
		{
			name:       "truncated raw deflate",
			header:     "deflate",
			body:       validRawDeflate[:len(validRawDeflate)-1],
			wantErrSub: "invalid zlib-wrapped or raw DEFLATE stream",
		},
		{
			name:       "truncated brotli",
			header:     "br",
			body:       validBrotli[:len(validBrotli)/2],
			wantErrSub: "read Brotli stream",
		},
		{
			name:       "truncated zstandard",
			header:     "zstd",
			body:       validZstd[:len(validZstd)/2],
			wantErrSub: "read Zstandard stream",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractAndDecompressBodyHTTP(test.header, bytes.NewReader(test.body))
			if err == nil {
				t.Fatalf("ExtractAndDecompressBodyHTTP() = %q, want an error", got)
			}
			var decodingErr *ContentDecodingError
			if !errors.As(err, &decodingErr) {
				t.Fatalf("error type = %T, want *ContentDecodingError", err)
			}
			if !strings.Contains(err.Error(), test.wantErrSub) {
				t.Fatalf("error = %q, want it to contain %q", err, test.wantErrSub)
			}
			if !bytes.Equal([]byte(got), test.body) {
				t.Fatal("a corrupt supported stream did not preserve its raw encoded body")
			}
		})
	}
}

func TestExtractAndDecompressBodyHTTPAllowsEmptyEncodedBodies(t *testing.T) {
	t.Parallel()

	for _, encoding := range []string{"gzip", "deflate", "br", "zstd"} {
		encoding := encoding
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			got, err := ExtractAndDecompressBodyHTTP(
				encoding,
				bytes.NewReader(nil),
			)
			if err != nil {
				t.Fatalf("empty %s body returned an error: %v", encoding, err)
			}
			if got != "" {
				t.Fatalf("empty %s body = %q, want empty", encoding, got)
			}
		})
	}
}

func TestExtractAndDecompressBodyHTTPBoundsDecodedBodies(t *testing.T) {
	t.Parallel()

	original := []byte(strings.Repeat("bounded capture payload ", 40_000))
	for _, encoding := range []string{"gzip", "deflate", "br", "zstd"} {
		encoding := encoding
		t.Run(encoding, func(t *testing.T) {
			t.Parallel()

			encoded := encodeContent(t, encoding, original)
			got, err := ExtractAndDecompressBodyHTTP(
				encoding,
				bytes.NewReader(encoded),
			)
			if err != nil {
				t.Fatalf("decode bounded %s body: %v", encoding, err)
			}
			if len(got) != MaxCapturedBodySize+1 {
				t.Fatalf(
					"decoded length = %d, want %d",
					len(got),
					MaxCapturedBodySize+1,
				)
			}
			if !bytes.Equal([]byte(got), original[:MaxCapturedBodySize+1]) {
				t.Fatal("bounded decoded body is not the expected prefix")
			}
		})
	}
}

func TestExtractAndDecompressBodyHTTPBoundsRawBodies(t *testing.T) {
	t.Parallel()

	rawBody := []byte(strings.Repeat("x", MaxCapturedBodySize+4_096))
	for _, encoding := range []string{"", "x-custom"} {
		got, err := ExtractAndDecompressBodyHTTP(
			encoding,
			bytes.NewReader(rawBody),
		)
		if err != nil {
			t.Fatalf("read bounded raw body for %q: %v", encoding, err)
		}
		if len(got) != MaxCapturedBodySize+1 {
			t.Fatalf(
				"raw length for %q = %d, want %d",
				encoding,
				len(got),
				MaxCapturedBodySize+1,
			)
		}
	}
}

func TestExtractAndDecompressBodyHTTPReaderError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("reader failed")
	got, err := ExtractAndDecompressBodyHTTP("", &failingReader{err: wantErr})
	if got != "" {
		t.Fatalf("body = %q, want empty body", got)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestExtractAndDecompressBodyHTTPNilBody(t *testing.T) {
	t.Parallel()

	got, err := ExtractAndDecompressBodyHTTP("br", nil)
	if err != nil {
		t.Fatalf("ExtractAndDecompressBodyHTTP() error = %v", err)
	}
	if got != "" {
		t.Fatalf("body = %q, want empty body", got)
	}
}

type failingReader struct {
	err error
}

func (reader *failingReader) Read([]byte) (int, error) {
	return 0, reader.err
}

func encodeContent(t *testing.T, encoding string, data []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	var writer io.WriteCloser

	switch encoding {
	case "identity":
		return append([]byte(nil), data...)
	case "gzip":
		writer = gzip.NewWriter(&buffer)
	case "deflate":
		writer = zlib.NewWriter(&buffer)
	case "br":
		writer = brotli.NewWriter(&buffer)
	case "zstd":
		zstdWriter, err := zstd.NewWriter(
			&buffer,
			zstd.WithEncoderConcurrency(1),
		)
		if err != nil {
			t.Fatalf("initialize zstd writer: %v", err)
		}
		writer = zstdWriter
	default:
		t.Fatalf("test helper does not support encoding %q", encoding)
	}

	if _, err := writer.Write(data); err != nil {
		t.Fatalf("encode %s payload: %v", encoding, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close %s writer: %v", encoding, err)
	}

	return buffer.Bytes()
}

func encodeRawDeflate(t *testing.T, data []byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer, err := flate.NewWriter(&buffer, flate.DefaultCompression)
	if err != nil {
		t.Fatalf("initialize raw DEFLATE writer: %v", err)
	}
	if _, err := writer.Write(data); err != nil {
		t.Fatalf("encode raw DEFLATE payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close raw DEFLATE writer: %v", err)
	}
	return buffer.Bytes()
}

func corruptLastByte(data []byte) []byte {
	corrupt := append([]byte(nil), data...)
	corrupt[len(corrupt)-1] ^= 0xff
	return corrupt
}
