package gitfs

import (
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"rsc.io/gitfs"
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

	statFs
	repo *gitfs.Repo
	hash gitfs.Hash
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
	return nil
}

var (
	_ caddy.Module          = (*Repo)(nil)
	_ caddy.Provisioner     = (*Repo)(nil)
	_ fs.StatFS             = (*Repo)(nil)
	_ caddyfile.Unmarshaler = (*Repo)(nil)
)
