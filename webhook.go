package gitfs

import (
	"fmt"
	"io/fs"
	"net/http"

	"go.uber.org/zap"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(Handler{})
	httpcaddyfile.RegisterHandlerDirective("gitfs_webhook", parseCaddyfile)
}

func parseCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	h.Next()
	h.Next()
	return &Handler{
		Filesystem: h.Val(),
	}, nil
}

type Handler struct {
	// The filesystem name to use, as defined in `filesystems`
	Filesystem string `json:"filesystem,omitempty"`

	logger *zap.Logger
	ctx    caddy.Context
}

// Validate implements caddy.Validator.
func (h *Handler) Validate() error {
	f, ok := h.ctx.Filesystems().Get(h.Filesystem)
	if !ok {
		return fmt.Errorf("filesystem '%s' not found", h.Filesystem)
	}
	if fs, ok := f.(unwrappableFS); ok {
		f = fs.Unwrap()
		if _, ok := f.(*Repo); !ok {
			return fmt.Errorf("filesystem '%s' is not 'gitfs", h.Filesystem)
		}
		return nil
	}
	return fmt.Errorf("filesystem '%s' cannot be unwrapped; likely not gitfs", h.Filesystem)
}

// CaddyModule implements caddy.Module.
func (h Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "http.handlers.gitfs_webhook",
		New: func() caddy.Module {
			return new(Handler)
		},
	}
}

// Provision implements caddy.Provisioner.
func (h *Handler) Provision(ctx caddy.Context) error {
	if h.Filesystem == "" {
		return fmt.Errorf("filesystem name is required")
	}
	h.logger = ctx.Logger()
	h.ctx = ctx
	return nil
}

// ServeHTTP implements caddyhttp.Handler.
func (h *Handler) ServeHTTP(http.ResponseWriter, *http.Request, caddyhttp.Handler) error {
	h.logger.Info("received webhook request")
	fss := h.ctx.Filesystems()
	fs, ok := fss.Get(h.Filesystem)
	if !ok {
		return fmt.Errorf("unable to find filesystem '%s'", h.Filesystem)
	}
	h.logger.Debug("found filesystem", zap.String("name", h.Filesystem), zap.String("content", fmt.Sprintf("%+v", fs)))
	if fs, ok := fs.(unwrappableFS); ok {
		ufs := fs.Unwrap()
		gitfs, _ := ufs.(*Repo)
		if !ok {
			return fmt.Errorf("Filesystem %s is not a *gitfs.Repo; it is %T", h.Filesystem, gitfs)
		}
		return gitfs.pull()
	}
	return fmt.Errorf("filesystem %s cannot be unwrapped", h.Filesystem)
}

type unwrappableFS interface {
	Unwrap() fs.FS
}

var (
	_ caddy.Module                = (*Handler)(nil)
	_ caddy.Provisioner           = (*Handler)(nil)
	_ caddy.Validator             = (*Handler)(nil)
	_ caddyhttp.MiddlewareHandler = (*Handler)(nil)
)
