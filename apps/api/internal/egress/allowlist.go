// Package egress implementa un proxy HTTP de egress con allowlist de
// hostnames para evitar exfiltracion de sesiones OAuth en runs host_session
// (ADR-0022).
package egress

import (
	"net"
	"strings"
)

// allowed reporta si host (con o sin puerto) esta en la allowlist.
// Reglas:
//   - Strip de puerto antes de comparar.
//   - Case-insensitive.
//   - Match exacto: host == entry.
//   - Match por sufijo de subdominio: host termina en "."+entry.
//     (api.anthropic.com matches anthropic.com, pero evilanthropic.com NO)
//   - allowlist vacia => nada es permitido.
func allowed(host string, allowlist []string) bool {
	h := stripPort(host)
	h = strings.ToLower(h)

	for _, entry := range allowlist {
		e := strings.ToLower(strings.TrimSpace(entry))
		if e == "" {
			continue
		}
		if h == e {
			return true
		}
		// Subdomain match: host debe terminar en ".<entry>".
		// Esto evita falsos positivos como "evilanthropic.com" matcheando "anthropic.com"
		// o "anthropic.com.attacker.net" matcheando "anthropic.com".
		if strings.HasSuffix(h, "."+e) {
			return true
		}
	}
	return false
}

// stripPort extrae el hostname puro de un string que puede ser "host:port"
// o solo "host". Si net.SplitHostPort falla (sin puerto), retorna el input.
func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		// No habia puerto; el input ya es el host.
		return hostport
	}
	return host
}
