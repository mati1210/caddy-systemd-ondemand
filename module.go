package caddyondemandsystemd

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

const DIRECTIVE = "systemd_service"

func init() {
	caddy.RegisterModule(ServiceStarter{})
	httpcaddyfile.RegisterHandlerDirective(DIRECTIVE, parseCaddyfile)
	httpcaddyfile.RegisterDirectiveOrder(DIRECTIVE, httpcaddyfile.Before, "reverse_proxy")
}

func (s ServiceStarter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "http.handlers.systemd_service",
		New: func() caddy.Module {
			return new(ServiceStarter)
		},
	}
}

type ServiceStarter struct {
	// the service to start
	Service string `json:"service"`

	/// time the service takes to startup
	StartupSecs int `json:"startup_secs,omitempty"`

	Body       string `json:"body,omitempty"`
	StatusCode string `json:"status_code,omitempty"`

	running bool
	logger  *zap.Logger
}

// ServeHTTP implements [caddyhttp.MiddlewareHandler].
func (s ServiceStarter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if s.ServiceRunning() {
		return next.ServeHTTP(w, r)
	}
	go s.StartService()

	w.Header().Add("Retry-After", fmt.Sprint(s.StartupSecs))
	w.Header().Add("Content-Type", "text/html;charset=utf-8")
	w.WriteHeader(503)

	//TODO: js to automatically restart
	w.Write([]byte("service is starting, please wait"))
	return nil
}

func (s *ServiceStarter) ServiceRunning() bool {
	if s.running {
		return true
	}

	cmd := exec.Command("systemctl", "is-active", "--quiet", s.Service)
	if cmd.Run() == nil {
		s.running = true
		return true
	}
	return false
}

func (s *ServiceStarter) StartService() error {
	//TODO: log errors?
	cmd := exec.Command("systemctl", "start", s.Service)

	err := cmd.Run()
	if err == nil {
		s.running = true
		return nil
	}
	return err
}

// Provision implements [caddy.Provisioner].
func (s *ServiceStarter) Provision(ctx caddy.Context) error {
	s.logger = ctx.Logger()
	return nil
}

// UnmarshalCaddyfile implements [caddyfile.Unmarshaler].
func (s *ServiceStarter) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume directive name

	if !d.Args(&s.Service) {
		return d.ArgErr()
	}

	for nesting := d.Nesting(); d.NextBlock(nesting); {
		switch d.Val() {
		case "body":
			if !d.Args(&s.Body) {
				return d.ArgErr()
			}

		case "startup_seconds":
			secs, err := strconv.Atoi(d.Val())
			if err != nil {
				return d.Errf("startup_seconds not a number!")
			}
			s.StartupSecs = secs

		}

	}
	return nil

}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	var m ServiceStarter
	err := m.UnmarshalCaddyfile(h.Dispenser)
	return m, err
}

// Interface guards
var (
	_ caddy.Provisioner           = (*ServiceStarter)(nil)
	_ caddyhttp.MiddlewareHandler = (*ServiceStarter)(nil)
	_ caddyfile.Unmarshaler       = (*ServiceStarter)(nil)
)
