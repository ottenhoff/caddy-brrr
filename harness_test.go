package caddybrrr

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/molecule-man/go-brrr"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp/encode"
	caddygzip "github.com/caddyserver/caddy/v2/modules/caddyhttp/encode/gzip"
	caddyzstd "github.com/caddyserver/caddy/v2/modules/caddyhttp/encode/zstd"
)

type benchmarkCorpus struct {
	name        string
	data        []byte
	contentType string
}

type encoderCase struct {
	name        string
	encoder     string
	level       string
	encoding    encode.Encoding
	decompress  func([]byte) ([]byte, error)
	contentType string
}

var (
	benchmarkGzipLevels   = []int{1, 5, 9}
	benchmarkZstdLevels   = []string{"fastest", "default", "best"}
	benchmarkBrotliLevels = []int{0, 1, 2, 4, 6, 9, 11}
)

func benchmarkCorpora(tb testing.TB) []benchmarkCorpus {
	tb.Helper()

	return []benchmarkCorpus{
		{name: "html", data: readBenchmarkPayload(tb, "testdata/caddy_home.html"), contentType: "text/html; charset=utf-8"},
		{name: "json", data: readBenchmarkPayload(tb, "testdata/caddy_config_http_servers.json"), contentType: "application/json"},
		{name: "js", data: readBenchmarkPayload(tb, "testdata/caddy_asciinema_player.js"), contentType: "application/javascript"},
		{name: "css", data: readBenchmarkPayload(tb, "testdata/caddy_asciinema_player.css"), contentType: "text/css"},
	}
}

func readBenchmarkPayload(tb testing.TB, filename string) []byte {
	tb.Helper()

	data, err := os.ReadFile(filename)
	if err != nil {
		tb.Fatalf("reading benchmark payload %s: %v", filename, err)
	}
	return data
}

func conformanceLargeBody() []byte {
	data, err := os.ReadFile("testdata/caddy_home.html")
	if err != nil {
		panic("conformanceLargeBody: " + err.Error())
	}
	return data
}

func conformanceEncoderCases(t testing.TB) []encoderCase {
	t.Helper()
	return []encoderCase{brotliEncoderCase(t, 4)}
}

func benchmarkEncoderCases(t testing.TB) []encoderCase {
	t.Helper()

	cases := provisionEncoderCases(t, benchmarkGzipLevels, benchmarkZstdLevels)
	for _, level := range benchmarkBrotliLevels {
		cases = append(cases, brotliEncoderCase(t, level))
	}
	return cases
}

func provisionEncoderCases(t testing.TB, gzipLevels []int, zstdLevels []string) []encoderCase {
	t.Helper()

	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	t.Cleanup(cancel)

	var cases []encoderCase
	for _, level := range gzipLevels {
		gzipEncoding := &caddygzip.Gzip{Level: level}
		if err := gzipEncoding.Provision(ctx); err != nil {
			t.Fatalf("gzip level %d Provision() error = %v", level, err)
		}
		cases = append(cases, encoderCase{
			name:        fmt.Sprintf("gzip-level-%d", level),
			encoder:     "gzip",
			level:       fmt.Sprintf("%d", level),
			encoding:    gzipEncoding,
			contentType: "text/plain",
		})
	}
	for _, level := range zstdLevels {
		zstdEncoding := &caddyzstd.Zstd{Level: level}
		if err := zstdEncoding.Provision(ctx); err != nil {
			t.Fatalf("zstd level %q Provision() error = %v", level, err)
		}
		cases = append(cases, encoderCase{
			name:        "zstd-level-" + level,
			encoder:     "zstd",
			level:       level,
			encoding:    zstdEncoding,
			contentType: "text/plain",
		})
	}
	return cases
}

func brotliEncoderCase(t testing.TB, level int) encoderCase {
	t.Helper()

	br := &Brotli{Level: intPtr(level)}
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	t.Cleanup(cancel)
	if err := br.Provision(ctx); err != nil {
		t.Fatalf("brotli level %d Provision() error = %v", level, err)
	}
	if err := br.Validate(); err != nil {
		t.Fatalf("brotli level %d Validate() error = %v", level, err)
	}
	return encoderCase{
		name:        fmt.Sprintf("brotli-level-%d", level),
		encoder:     "brotli",
		level:       fmt.Sprintf("%d", level),
		encoding:    br,
		decompress:  decompressBrotli,
		contentType: "text/plain",
	}
}

func intPtr(v int) *int {
	return &v
}

func newEncodeHandler(tb testing.TB, encCase encoderCase, minLength int) *encode.Encode {
	tb.Helper()

	encodingName := encCase.encoding.AcceptEncoding()
	enc := &encode.Encode{
		EncodingsRaw: caddy.ModuleMap{
			encodingName: caddyconfig.JSON(encCase.encoding, nil),
		},
		Prefer:    []string{encodingName},
		MinLength: minLength,
	}
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	tb.Cleanup(cancel)
	if err := enc.Provision(ctx); err != nil {
		tb.Fatalf("Provision() error = %v", err)
	}
	if err := enc.Validate(); err != nil {
		tb.Fatalf("Validate() error = %v", err)
	}
	return enc
}

func assertDecompresses(t *testing.T, encCase encoderCase, compressed, original []byte) {
	t.Helper()

	decompressed, err := encCase.decompress(compressed)
	if err != nil {
		t.Fatalf("decompress %s: %v", encCase.name, err)
	}
	if !bytes.Equal(decompressed, original) {
		t.Fatalf("decompressed len = %d, want len = %d", len(decompressed), len(original))
	}
}

func assertVaryAcceptEncoding(t *testing.T, h map[string][]string) {
	t.Helper()

	for _, vary := range h["Vary"] {
		for val := range strings.SplitSeq(vary, ",") {
			if strings.EqualFold(strings.TrimSpace(val), "Accept-Encoding") {
				return
			}
		}
	}
	t.Fatalf("Vary = %q, want Accept-Encoding", h["Vary"])
}

func encodeAndVerifyRoundTrip(t *testing.T, encCase encoderCase, encoder encode.Encoder, original []byte) {
	t.Helper()

	var compressed bytes.Buffer
	encoder.Reset(&compressed)

	if _, err := encoder.Write(original[:len(original)/2]); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := encoder.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	if compressed.Len() == 0 {
		t.Fatal("Flush() wrote no compressed bytes")
	}
	if _, err := encoder.Write(original[len(original)/2:]); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	assertDecompresses(t, encCase, compressed.Bytes(), original)
}

func decompressBrotli(compressed []byte) ([]byte, error) {
	reader := brrr.NewReader(bytes.NewReader(compressed))
	defer reader.Close()
	return io.ReadAll(reader)
}
