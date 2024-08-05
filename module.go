package gitfs

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"rsc.io/gitfs"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func init() {
	caddy.RegisterModule(Repo{})
}

// The `git` filesystem module uses a git repository as the
// virtual filesystem.
type Repo struct {
	// The URL of the git repository
	URL string `json:"url,omitempty"`

	// The reference to clone the repository at.
	// An empty value means HEAD.
	Ref string `json:"ref,omitempty"`

	// The period between ref refreshes
	RefreshPeriod caddy.Duration `json:"refresh_period,omitempty"`

	statFs statFs
	mu     *sync.RWMutex
	repo   *gitfs.Repo
	hash   gitfs.Hash
	ctx    context.Context
	cancel context.CancelFunc

	logger *zap.Logger
}

// This method indicates that the type is a Caddy
// module. The returned ModuleInfo must have both
// a name and a constructor function. This method
// must not have any side-effects.
func (Repo) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "caddy.fs.git",
		New: func() caddy.Module {
			return new(Repo)
		},
	}
}

// Provision implements caddy.Provisioner.
func (r *Repo) Provision(ctx caddy.Context) (err error) {
	r.ctx, r.cancel = context.WithCancel(ctx)
	r.logger = ctx.Logger()
	if r.URL == "" {
		return fmt.Errorf("'url' is empty")
	}
	r.repo, err = gitfs.NewRepo(r.URL)
	if err != nil {
		return err
	}
	if r.Ref == "" {
		r.Ref = "HEAD"
	}
	h, fs, err := r.repo.Clone(r.Ref)
	if err != nil {
		return err
	}
	r.hash = h
	r.statFs = statFs{fs}
	r.mu = &sync.RWMutex{}
	if r.RefreshPeriod != 0 {
		r.logger.Info("starting `ref` hash refresh",
			zap.String("ref", r.Ref),
			zap.String("hash", r.hash.String()),
			zap.Duration("period", time.Duration(r.RefreshPeriod)),
		)
		go r.refresh()
	}
	return nil
}

func (r *Repo) Open(name string) (fs.File, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.statFs.Open(name)
}

func (r *Repo) Stat(name string) (fs.FileInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, err := r.statFs.Open(name)
	if err != nil {
		return nil, err
	}
	return f.Stat()
}

func (r *Repo) pull() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logger.Info("pulling gitfs repo")
	h, fs, err := r.repo.Clone(r.Ref)
	if err != nil {
		return err
	}
	r.hash = h
	r.statFs = statFs{fs}
	return nil
}

func (r *Repo) refresh() {
	t := time.NewTicker(time.Duration(r.RefreshPeriod))
	for {
		select {
		case <-r.ctx.Done():
			r.logger.Info("stopping `ref` hash refresh")
			t.Stop()
			return
		case <-t.C:
			r.logger.Debug("checking `ref` hash",
				zap.String("ref", r.Ref),
				zap.String("hash", r.hash.String()),
			)
			h, err := r.repo.Resolve(r.Ref)
			if err != nil {
				r.logger.Error("error resolving new hash of the `ref`", zap.Error(err))
				continue
			}
			if h == r.hash {
				r.logger.Debug("no change in `ref` hash")
				continue
			}
			r.logger.Info(
				"`ref` hash changed; cloning",
				zap.String("ref", r.Ref),
				zap.String("old", r.hash.String()),
				zap.String("new", h.String()),
			)
			hash, f, err := r.repo.Clone(r.Ref)
			if err != nil {
				r.logger.Error("error cloning `ref`", zap.Error(err))
				continue
			}
			r.mu.Lock()
			r.hash = hash
			r.statFs = statFs{f}
			r.mu.Unlock()
		}
	}
}

// Cleanup implements caddy.CleanerUpper.
func (r *Repo) Cleanup() error {
	r.logger.Debug("cleaning up")
	r.cancel()
	return nil
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler.
func (r *Repo) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	// consume the directive
	d.Next()
	var arg string
	if !d.Args(&arg) {
		return d.Err("missing URL")
	}
	u, err := url.Parse(arg)
	if err != nil {
		return err
	}
	parts := strings.Split(u.Path, `@`)
	switch len(parts) {
	case 0:
		// should be impossible
		return d.Errf("the path consists of 0 parts, should have more: %v", parts)
	case 1:
		r.URL = arg
		r.Ref = "HEAD"
	case 2:
		u.Path = strings.TrimSuffix(u.Path, `@`+parts[1])
		r.URL = u.String()
	}
	for nesting := d.Nesting(); d.NextBlock(nesting); {
		switch d.Val() {
		case "ref":
			if !d.Args(&r.Ref) {
				return d.ArgErr()
			}
		case "refresh_period":
			var dur string
			if !d.Args(&dur) {
				return d.ArgErr()
			}
			d, err := caddy.ParseDuration(dur)
			if err != nil {
				return err
			}
			r.RefreshPeriod = caddy.Duration(d)
		default:
			return d.Errf("unrecognized subdirective %s", d.Val())
		}
	}
	return nil
}

var (
	_ caddy.Module          = (*Repo)(nil)
	_ caddy.Provisioner     = (*Repo)(nil)
	_ caddy.CleanerUpper    = (*Repo)(nil)
	_ fs.StatFS             = (*Repo)(nil)
	_ caddyfile.Unmarshaler = (*Repo)(nil)
)
