package caddyondemandsystemd

import (
	_ "embed"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

const DIRECTIVE = "systemd_service"

type Status int

const (
	Unknown = iota
	Starting
	Started
)

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
	StartupTime time.Duration `json:"startup_time,omitempty"`

	Body       string `json:"body,omitempty"`
	StatusCode string `json:"status_code,omitempty"`

	running Status
	started time.Time
	logger  *zap.Logger
}

//go:embed default_starting.html
var body string

// ServeHTTP implements [caddyhttp.MiddlewareHandler].
func (s ServiceStarter) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if s.ServiceRunning() {
		return next.ServeHTTP(w, r)
	}

	w.Header().Add("Retry-After", s.started.Add(s.StartupTime).UTC().Format(http.TimeFormat))
	w.Header().Add("Refresh", fmt.Sprint(int(s.StartupTime.Seconds())))

	w.Header().Add("Content-Type", "text/html;charset=utf-8")
	w.WriteHeader(503)

	if s.Body == "" {
		s.Body = body
	}
	w.Write([]byte(s.Body))
	return nil
}

func (s *ServiceStarter) ServiceRunning() bool {
	switch s.running {
	case Unknown:
		cmd := exec.Command("systemctl", "is-active", "--quiet", s.Service)
		if cmd.Run() == nil {
			s.running = Started
			return true
		}
		go s.StartService()

	case Started:
		return true

	case Starting:
		if time.Since(s.started) > s.StartupTime {
			s.running = Started
			return true
		}
	}
	return false
}

func (s *ServiceStarter) StartService() error {
	//TODO: log errors?
	cmd := exec.Command("systemctl", "start", s.Service)

	err := cmd.Run()
	if err == nil {
		s.running = Starting
		s.started = time.Now()
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

		case "startup_time":
			d.Next()
			dur, err := time.ParseDuration(d.Val())
			if err != nil {
				return d.Errf("startup_time not a valid duration! %s", err.Error())
			}
			s.StartupTime = dur

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
