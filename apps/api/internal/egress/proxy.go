package egress

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const (
	// ModeEnforce bloquea conexiones a hosts que no estan en la allowlist.
	ModeEnforce = "enforce"
	// ModeLogOnly permite toda conexion pero loguea un warning cuando el host
	// no esta en la allowlist. Util para descubrir el set real de endpoints
	// antes de pasar a enforce.
	ModeLogOnly = "log_only"
)

// DialFunc es el tipo de funcion para abrir conexiones de red.
// Inyectable para tests sin red real.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// Proxy es un proxy HTTP de egress que filtra conexiones salientes por hostname.
// Soporta CONNECT (HTTPS tunneling) y peticiones HTTP planas con URI absoluta.
type Proxy struct {
	// Addr es la direccion en la que escucha el proxy (e.g. ":8888").
	Addr string

	// Allowlist es el conjunto de hostnames permitidos.
	// Soporta match exacto y por sufijo de subdominio.
	Allowlist []string

	// Mode controla el comportamiento con hosts no permitidos.
	// "enforce" => rechaza; "log_only" => permite pero loguea.
	// Si esta vacio, se asume "log_only" (mas seguro para primer rollout).
	Mode string

	// Logger es el logger estructurado. Si nil, se usa slog.Default().
	Logger *slog.Logger

	// Dial es la funcion para abrir conexiones upstream.
	// Si nil, usa un net.Dialer con timeout de 30s.
	Dial DialFunc
}

// mode retorna el modo efectivo del proxy.
func (p *Proxy) mode() string {
	if p.Mode == ModeEnforce {
		return ModeEnforce
	}
	return ModeLogOnly // default seguro
}

// logger retorna el logger efectivo.
func (p *Proxy) logger() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.Default()
}

// dial retorna la funcion de dial efectiva.
func (p *Proxy) dial() DialFunc {
	if p.Dial != nil {
		return p.Dial
	}
	d := &net.Dialer{Timeout: 30 * time.Second}
	return d.DialContext
}

// Handler retorna un http.Handler que implementa el proxy.
func (p *Proxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			p.handleCONNECT(w, r)
		} else {
			p.handleHTTP(w, r)
		}
	})
}

// ListenAndServe arranca el proxy en p.Addr y bloquea hasta que ctx se cancele.
func (p *Proxy) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:    p.Addr,
		Handler: p.Handler(),
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("egress proxy serve: %w", err)
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			return fmt.Errorf("egress proxy shutdown: %w", err)
		}
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// handleCONNECT procesa peticiones HTTPS tunneling (metodo CONNECT).
// El cliente envia "CONNECT api.anthropic.com:443 HTTP/1.1"; extraemos el
// hostname del target y lo verificamos contra la allowlist.
func (p *Proxy) handleCONNECT(w http.ResponseWriter, r *http.Request) {
	target := r.Host // "host:port"
	host := stripPort(target)

	log := p.logger().With("method", "CONNECT", "target", target, "host", host)

	if allowed(host, p.Allowlist) {
		log.Debug("egress: allow CONNECT")
		p.tunnel(w, r, target)
		return
	}

	// Host no permitido.
	if p.mode() == ModeEnforce {
		log.Warn("egress: block CONNECT", "reason", "not in allowlist")
		// Devolver 403 por HTTP antes de cerrar.
		http.Error(w, "Forbidden: host not in egress allowlist", http.StatusForbidden)
		return
	}

	// log_only: permitir pero avisar.
	log.Warn("egress: would block CONNECT", "reason", "not in allowlist", "action", "allowed_by_log_only")
	p.tunnel(w, r, target)
}

// handleHTTP procesa peticiones HTTP planas con URI absoluta.
// Extrae el host del URL de la peticion y lo verifica contra la allowlist.
func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL == nil || !r.URL.IsAbs() {
		http.Error(w, "Bad Request: expected absolute URI", http.StatusBadRequest)
		return
	}

	host := r.URL.Hostname()
	log := p.logger().With("method", r.Method, "url", r.URL.String(), "host", host)

	if allowed(host, p.Allowlist) {
		log.Debug("egress: allow HTTP")
		p.forwardHTTP(w, r)
		return
	}

	if p.mode() == ModeEnforce {
		log.Warn("egress: block HTTP", "reason", "not in allowlist")
		http.Error(w, "Forbidden: host not in egress allowlist", http.StatusForbidden)
		return
	}

	log.Warn("egress: would block HTTP", "reason", "not in allowlist", "action", "allowed_by_log_only")
	p.forwardHTTP(w, r)
}

// tunnel abre una conexion TCP al target y copia bytes en ambas direcciones.
// Responde primero con "200 Connection established" al cliente.
func (p *Proxy) tunnel(w http.ResponseWriter, r *http.Request, target string) {
	dial := p.dial()
	upstream, err := dial(r.Context(), "tcp", target)
	if err != nil {
		p.logger().Error("egress: dial upstream failed", "target", target, "err", err)
		http.Error(w, "Bad Gateway: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer upstream.Close()

	// Obtener la conexion subyacente del cliente.
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		p.logger().Error("egress: ResponseWriter does not support hijacking")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clientConn, buf, err := hijacker.Hijack()
	if err != nil {
		p.logger().Error("egress: hijack failed", "err", err)
		return
	}
	defer clientConn.Close()

	// Ignorar bytes pendientes en el buffer de lectura (raro en CONNECT).
	_ = buf

	// Confirmar al cliente que el tunel esta establecido.
	_, _ = fmt.Fprint(clientConn, "HTTP/1.1 200 Connection established\r\n\r\n")

	// Copiar en ambas direcciones hasta que alguno cierre.
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, clientConn)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(clientConn, upstream)
		done <- struct{}{}
	}()
	<-done
}

// forwardHTTP reenvía una peticion HTTP plana al destino y copia la respuesta.
func (p *Proxy) forwardHTTP(w http.ResponseWriter, r *http.Request) {
	// Construir el transport que usa nuestro Dial.
	dialFn := p.dial()
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialFn(ctx, network, addr)
		},
	}

	// Forwarding con httputil.ReverseProxy (limpia hop-by-hop headers internamente).
	// Determinar el target del reverse proxy a partir de la URL.
	targetURL := &url.URL{
		Scheme: r.URL.Scheme,
		Host:   r.URL.Host,
	}

	rp := httputil.NewSingleHostReverseProxy(targetURL)
	rp.Transport = transport
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = r.URL.Scheme
		req.URL.Host = r.URL.Host
		req.Host = r.URL.Host
	}

	rp.ServeHTTP(w, r)
}
