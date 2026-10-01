// Package middleware holds the HTTP middlewares of the server: request
// logging, gzip compression and token authentication.
package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// compressibleContentTypes are the response types worth compressing; the
// API answers with nothing else.
var compressibleContentTypes = []string{
	"application/json",
	"text/plain",
}

// gzipResponseWriter is a wrapper around http.ResponseWriter that compresses
// the response body.
type gzipResponseWriter struct {
	w     http.ResponseWriter
	gz    *gzip.Writer
	wrote bool // true once the status line has been sent
}

// newGzipResponseWriter wraps w; whether to compress is decided when the
// status is sent, once the handler has set the content type.
func newGzipResponseWriter(w http.ResponseWriter) *gzipResponseWriter {
	return &gzipResponseWriter{w: w}
}

// Header returns the headers of the wrapped writer.
func (g *gzipResponseWriter) Header() http.Header {
	return g.w.Header()
}

// WriteHeader sends the status and turns compression on for a successful
// response of a compressible type. Only the first call has an effect.
func (g *gzipResponseWriter) WriteHeader(statusCode int) {
	if g.wrote {
		return
	}
	g.wrote = true

	// A 204 must have no body at all, not even an empty gzip stream.
	if statusCode < 300 && statusCode != http.StatusNoContent && shouldCompress(g.w.Header().Get("Content-Type")) {
		g.gz = gzip.NewWriter(g.w)
		g.w.Header().Set("Content-Encoding", "gzip")
		g.w.Header().Add("Vary", "Accept-Encoding")
		g.w.Header().Del("Content-Length")
	}
	g.w.WriteHeader(statusCode)
}

// Write writes p to the body, compressed if compression is on.
func (g *gzipResponseWriter) Write(p []byte) (int, error) {
	// If WriteHeader was never called by the handler, default to 200 OK.
	if !g.wrote {
		g.WriteHeader(http.StatusOK)
	}
	if g.gz != nil {
		return g.gz.Write(p)
	}
	return g.w.Write(p)
}

// Close flushes the compressed stream. It must be called after the handler
// has returned.
func (g *gzipResponseWriter) Close() error {
	if g.gz != nil {
		return g.gz.Close()
	}
	return nil
}

// gzipReader is a wrapper around io.ReadCloser that decompresses the request
// body.
type gzipReader struct {
	r  io.ReadCloser
	zr *gzip.Reader
}

// newGzipReader wraps r; it fails if r does not start with a gzip header.
func newGzipReader(r io.ReadCloser) (*gzipReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &gzipReader{
		r:  r,
		zr: zr,
	}, nil
}

// Read reads decompressed data.
func (g *gzipReader) Read(p []byte) (n int, err error) {
	return g.zr.Read(p)
}

// Close closes both the original body and the decompressor.
func (g *gzipReader) Close() error {
	if err := g.r.Close(); err != nil {
		return err
	}
	return g.zr.Close()
}

// acceptsEncoding reports whether an Accept-Encoding or Content-Encoding
// header lists the encoding.
func acceptsEncoding(header, encoding string) bool {
	for part := range strings.SplitSeq(header, ",") {
		enc := strings.TrimSpace(strings.Split(part, ";")[0])
		if enc == encoding || (encoding == "gzip" && enc == "x-gzip") {
			return true
		}
	}
	return false
}

// shouldCompress reports whether a response of this type is worth compressing.
func shouldCompress(contentType string) bool {
	for _, ct := range compressibleContentTypes {
		if strings.HasPrefix(contentType, ct) {
			return true
		}
	}
	return false
}

// decompressRequest replaces a gzip request body with a decompressing reader.
func decompressRequest(r *http.Request) error {
	if !acceptsEncoding(r.Header.Get("Content-Encoding"), "gzip") {
		return nil
	}

	gz, err := newGzipReader(r.Body)
	if err != nil {
		r.Body.Close()
		return err
	}

	r.Body = gz
	r.Header.Del("Content-Encoding")
	r.Header.Del("Content-Length")
	r.ContentLength = -1
	return nil
}

// Gzip is a middleware that decompresses gzip request bodies and compresses
// the responses for clients that accept gzip.
func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := decompressRequest(r); err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		if acceptsEncoding(r.Header.Get("Accept-Encoding"), "gzip") {
			cw := newGzipResponseWriter(w)
			defer cw.Close()
			next.ServeHTTP(cw, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
