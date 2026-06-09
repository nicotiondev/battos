package egress

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// logCapture es un slog.Handler minimo que captura los mensajes en memoria.
type logCapture struct {
	mu   sync.Mutex
	msgs []string
	keys map[string][]string // key => []value
}

func newLogCapture() *logCapture {
	return &logCapture{keys: make(map[string][]string)}
}

func (lc *logCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (lc *logCapture) Handle(_ context.Context, r slog.Record) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.msgs = append(lc.msgs, r.Message)
	r.Attrs(func(a slog.Attr) bool {
		lc.keys[a.Key] = append(lc.keys[a.Key], a.Value.String())
		return true
	})
	return nil
}

func (lc *logCapture) WithAttrs(attrs []slog.Attr) slog.Handler { return lc }
func (lc *logCapture) WithGroup(name string) slog.Handler       { return lc }

func (lc *logCapture) hasMsg(msg string) bool {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	for _, m := range lc.msgs {
		if strings.Contains(m, msg) {
			return true
		}
	}
	return false
}

// makeFakeDial crea un DialFunc que siempre conecta a addr (un listener local de tests).
func makeFakeDial(addr string) DialFunc {
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
}

// startEchoServer lanza un servidor TCP minimo que hace echo de cada linea que recibe.
// Retorna la direccion donde escucha y una funcion de cleanup.
func startEchoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo server listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c) // echo
			}(conn)
		}
	}()
	return ln.Addr().String()
}

// startHTTPUpstream lanza un servidor HTTP que responde 200 con body "ok".
func startHTTPUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---------------------------------------------------------------------------
// Tests de CONNECT
// ---------------------------------------------------------------------------

// TestCONNECT_AllowedHost_TunnelsBytesThrough verifica que un host permitido
// recibe el tunel y los bytes se propagan en ambas direcciones.
func TestCONNECT_AllowedHost_TunnelsBytesThrough(t *testing.T) {
	t.Parallel()

	echoAddr := startEchoServer(t)

	lc := newLogCapture()
	p := &Proxy{
		Allowlist: []string{"anthropic.com"},
		Mode:      ModeEnforce,
		Logger:    slog.New(lc),
		Dial:      makeFakeDial(echoAddr),
	}

	srv := httptest.NewServer(p.Handler())
	t.Cleanup(srv.Close)

	// Conectar al proxy como cliente HTTP/1.1 crudo.
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("connect to proxy: %v", err)
	}
	defer conn.Close()

	// Enviar CONNECT para un host permitido.
	fmt.Fprintf(conn, "CONNECT api.anthropic.com:443 HTTP/1.1\r\nHost: api.anthropic.com:443\r\n\r\n")

	// Leer la respuesta 200.
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verificar tunel: enviar datos y recibir echo.
	payload := []byte("hello tunnel\n")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write through tunnel: %v", err)
	}
	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read echo from tunnel: %v", err)
	}
	if !bytes.Equal(buf, payload) {
		t.Errorf("tunnel echo mismatch: got %q, want %q", buf, payload)
	}
}

// TestCONNECT_DisallowedHost_Enforce_Returns403 verifica que en modo enforce
// un host no permitido recibe un 403 y no hay tunel.
func TestCONNECT_DisallowedHost_Enforce_Returns403(t *testing.T) {
	t.Parallel()

	echoAddr := startEchoServer(t)

	lc := newLogCapture()
	p := &Proxy{
		Allowlist: []string{"anthropic.com"},
		Mode:      ModeEnforce,
		Logger:    slog.New(lc),
		Dial:      makeFakeDial(echoAddr),
	}

	srv := httptest.NewServer(p.Handler())
	t.Cleanup(srv.Close)

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("connect to proxy: %v", err)
	}
	defer conn.Close()

	// Host NO permitido.
	fmt.Fprintf(conn, "CONNECT attacker.net:443 HTTP/1.1\r\nHost: attacker.net:443\r\n\r\n")

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// TestCONNECT_DisallowedHost_LogOnly_TunnelsAndLogs verifica que en modo
// log_only un host no permitido IGUAL recibe el tunel, Y se loguea el warning.
func TestCONNECT_DisallowedHost_LogOnly_TunnelsAndLogs(t *testing.T) {
	t.Parallel()

	echoAddr := startEchoServer(t)

	lc := newLogCapture()
	p := &Proxy{
		Allowlist: []string{"anthropic.com"},
		Mode:      ModeLogOnly,
		Logger:    slog.New(lc),
		Dial:      makeFakeDial(echoAddr),
	}

	srv := httptest.NewServer(p.Handler())
	t.Cleanup(srv.Close)

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("connect to proxy: %v", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "CONNECT attacker.net:443 HTTP/1.1\r\nHost: attacker.net:443\r\n\r\n")

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read CONNECT response: %v", err)
	}
	// En log_only debe PERMITIR (200).
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 in log_only, got %d", resp.StatusCode)
	}

	// Bytes deben fluir (echo).
	payload := []byte("data in log only\n")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if !bytes.Equal(buf, payload) {
		t.Errorf("echo mismatch: got %q, want %q", buf, payload)
	}

	// Verificar que se loguo "would block".
	if !lc.hasMsg("egress: would block CONNECT") {
		t.Errorf("expected 'would block CONNECT' log message, got: %v", lc.msgs)
	}
}

// TestCONNECT_EmptyAllowlist_Enforce_Blocks verifica que allowlist vacia
// bloquea todo en modo enforce.
func TestCONNECT_EmptyAllowlist_Enforce_Blocks(t *testing.T) {
	t.Parallel()

	echoAddr := startEchoServer(t)
	p := &Proxy{
		Allowlist: []string{},
		Mode:      ModeEnforce,
		Logger:    slog.New(newLogCapture()),
		Dial:      makeFakeDial(echoAddr),
	}

	srv := httptest.NewServer(p.Handler())
	t.Cleanup(srv.Close)

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	fmt.Fprintf(conn, "CONNECT api.anthropic.com:443 HTTP/1.1\r\nHost: api.anthropic.com:443\r\n\r\n")

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 with empty allowlist, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Tests de HTTP plano
// ---------------------------------------------------------------------------

// TestHTTP_AllowedHost_Forwarded verifica que un host permitido se forwardea.
func TestHTTP_AllowedHost_Forwarded(t *testing.T) {
	t.Parallel()

	upstream := startHTTPUpstream(t)
	upstreamHost := strings.TrimPrefix(upstream.URL, "http://")

	// El fake dial redirige cualquier conexion al upstream real.
	fakeDial := makeFakeDial(upstreamHost)

	lc := newLogCapture()
	p := &Proxy{
		Allowlist: []string{"allowed.example.com"},
		Mode:      ModeEnforce,
		Logger:    slog.New(lc),
		Dial:      fakeDial,
	}

	proxySrv := httptest.NewServer(p.Handler())
	t.Cleanup(proxySrv.Close)

	// Usar un transport que envie al proxy.
	transport := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return url.Parse(proxySrv.URL)
		},
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Get("http://allowed.example.com/path")
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestHTTP_DisallowedHost_Enforce_Returns403 verifica que un host no permitido
// en modo enforce recibe 403.
func TestHTTP_DisallowedHost_Enforce_Returns403(t *testing.T) {
	t.Parallel()

	upstream := startHTTPUpstream(t)
	upstreamHost := strings.TrimPrefix(upstream.URL, "http://")
	fakeDial := makeFakeDial(upstreamHost)

	lc := newLogCapture()
	p := &Proxy{
		Allowlist: []string{"allowed.example.com"},
		Mode:      ModeEnforce,
		Logger:    slog.New(lc),
		Dial:      fakeDial,
	}

	proxySrv := httptest.NewServer(p.Handler())
	t.Cleanup(proxySrv.Close)

	transport := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return url.Parse(proxySrv.URL)
		},
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Get("http://attacker.net/steal")
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

// TestHTTP_DisallowedHost_LogOnly_ForwardsAndLogs verifica log_only en HTTP plano.
func TestHTTP_DisallowedHost_LogOnly_ForwardsAndLogs(t *testing.T) {
	t.Parallel()

	upstream := startHTTPUpstream(t)
	upstreamHost := strings.TrimPrefix(upstream.URL, "http://")
	fakeDial := makeFakeDial(upstreamHost)

	lc := newLogCapture()
	p := &Proxy{
		Allowlist: []string{"allowed.example.com"},
		Mode:      ModeLogOnly,
		Logger:    slog.New(lc),
		Dial:      fakeDial,
	}

	proxySrv := httptest.NewServer(p.Handler())
	t.Cleanup(proxySrv.Close)

	transport := &http.Transport{
		Proxy: func(r *http.Request) (*url.URL, error) {
			return url.Parse(proxySrv.URL)
		},
	}
	client := &http.Client{Transport: transport}

	resp, err := client.Get("http://attacker.net/path")
	if err != nil {
		t.Fatalf("GET through proxy: %v", err)
	}
	defer resp.Body.Close()

	// En log_only debe forwardear (200).
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 in log_only, got %d", resp.StatusCode)
	}

	if !lc.hasMsg("egress: would block HTTP") {
		t.Errorf("expected 'would block HTTP' log, got: %v", lc.msgs)
	}
}

// TestDefaultMode_IsLogOnly verifica que un Proxy con Mode="" usa log_only.
func TestDefaultMode_IsLogOnly(t *testing.T) {
	t.Parallel()
	p := &Proxy{}
	if p.mode() != ModeLogOnly {
		t.Errorf("default mode should be log_only, got %q", p.mode())
	}
}
