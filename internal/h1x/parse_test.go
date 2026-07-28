package h1x

import "testing"

func TestParseRawRequestCombinesRepeatedContentEncodingHeaders(t *testing.T) {
	rawBody := "raw"
	rawRequest := []byte(
		"POST / HTTP/1.1\r\n" +
			"Host: example.test\r\n" +
			"Content-Encoding: gzip\r\n" +
			"Content-Encoding: x-custom\r\n" +
			"Content-Length: 3\r\n" +
			"\r\n" +
			rawBody,
	)

	_, body, err := ParseRawRequest(rawRequest)
	if err != nil {
		t.Fatalf("parse request with repeated Content-Encoding: %v", err)
	}
	if body != rawBody {
		t.Fatalf("body = %q, want preserved raw body %q", body, rawBody)
	}
}

func TestParseRawResponseAllowsEmptyEncodedHeadResponse(t *testing.T) {
	rawRequest := []byte(
		"HEAD / HTTP/1.1\r\n" +
			"Host: example.test\r\n" +
			"\r\n",
	)
	rawResponse := []byte(
		"HTTP/1.1 200 OK\r\n" +
			"Content-Encoding: br\r\n" +
			"Content-Length: 120\r\n" +
			"\r\n",
	)

	_, body, contentEncoding, err := ParseRawResponseWithContentEncoding(
		rawResponse,
		rawRequest,
	)
	if err != nil {
		t.Fatalf("parse empty encoded HEAD response: %v", err)
	}
	if body != "" {
		t.Fatalf("HEAD response body = %q, want empty", body)
	}
	if contentEncoding != "br" {
		t.Fatalf("Content-Encoding = %q, want br", contentEncoding)
	}
}
