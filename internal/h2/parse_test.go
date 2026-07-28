package h2

import (
	"bytes"
	"io"
	"testing"

	"marmota/internal/utils"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestSniffH2TrafficKeepsMalformedEncodedStreamAsMessage(t *testing.T) {
	var wire bytes.Buffer
	wire.WriteString(http2.ClientPreface)

	var headerBlock bytes.Buffer
	encoder := hpack.NewEncoder(&headerBlock)
	for _, field := range []hpack.HeaderField{
		{Name: ":method", Value: "GET"},
		{Name: ":path", Value: "/"},
		{Name: ":authority", Value: "example.test"},
		{Name: "content-encoding", Value: "br"},
	} {
		if err := encoder.WriteField(field); err != nil {
			t.Fatalf("encode header %q: %v", field.Name, err)
		}
	}

	framer := http2.NewFramer(&wire, nil)
	if err := framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      1,
		BlockFragment: headerBlock.Bytes(),
		EndHeaders:    true,
	}); err != nil {
		t.Fatalf("write HEADERS frame: %v", err)
	}
	rawBody := []byte("not a Brotli stream")
	if err := framer.WriteData(1, true, rawBody); err != nil {
		t.Fatalf("write DATA frame: %v", err)
	}

	messages := make(chan HTTP2Message, 1)
	err := SniffH2Traffic(bytes.NewReader(wire.Bytes()), false, messages)
	if err != io.EOF {
		t.Fatalf("sniffer error = %v, want io.EOF", err)
	}

	message := <-messages
	if message.DecodeError == nil {
		t.Fatal("malformed Brotli body did not report a decoding error")
	}
	if message.Body != string(rawBody) {
		t.Fatalf("captured body = %q, want raw body %q", message.Body, rawBody)
	}
}

func TestSniffH2TrafficUsesFirstDuplicateContentTypeAndCombinesEncodings(t *testing.T) {
	var wire bytes.Buffer
	wire.WriteString(http2.ClientPreface)

	var headerBlock bytes.Buffer
	encoder := hpack.NewEncoder(&headerBlock)
	for _, field := range []hpack.HeaderField{
		{Name: ":method", Value: "GET"},
		{Name: ":path", Value: "/"},
		{Name: ":authority", Value: "example.test"},
		{Name: "content-type", Value: "text/html; charset=utf-8"},
		{Name: "content-type", Value: "application/octet-stream"},
		{Name: "content-encoding", Value: "identity"},
		{Name: "content-encoding", Value: "br"},
	} {
		if err := encoder.WriteField(field); err != nil {
			t.Fatalf("encode header %q: %v", field.Name, err)
		}
	}

	framer := http2.NewFramer(&wire, nil)
	if err := framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      1,
		BlockFragment: headerBlock.Bytes(),
		EndHeaders:    true,
		EndStream:     true,
	}); err != nil {
		t.Fatalf("write HEADERS frame: %v", err)
	}

	messages := make(chan HTTP2Message, 1)
	err := SniffH2Traffic(bytes.NewReader(wire.Bytes()), false, messages)
	if err != io.EOF {
		t.Fatalf("sniffer error = %v, want io.EOF", err)
	}

	message := <-messages
	if got := message.Headers["content-type"]; got != "text/html; charset=utf-8" {
		t.Fatalf("content-type = %q, want first field value", got)
	}
	if message.ContentEncoding != "identity,br" {
		t.Fatalf(
			"content encoding = %q, want %q",
			message.ContentEncoding,
			"identity,br",
		)
	}
	if got := message.Headers["content-encoding"]; got != "identity,br" {
		t.Fatalf("serialized content-encoding = %q, want combined value", got)
	}
}

func TestWriteBoundedBodyCapture(t *testing.T) {
	var body bytes.Buffer
	input := bytes.Repeat([]byte("x"), 128*1024)

	for body.Len() < utils.MaxEncodedBodySize+1 {
		written, err := writeBoundedBodyCapture(&body, input)
		if err != nil {
			t.Fatalf("write bounded body: %v", err)
		}
		if written != len(input) {
			t.Fatalf("reported write length = %d, want %d", written, len(input))
		}
	}

	if body.Len() != utils.MaxEncodedBodySize+1 {
		t.Fatalf(
			"captured length = %d, want %d",
			body.Len(),
			utils.MaxEncodedBodySize+1,
		)
	}
}
