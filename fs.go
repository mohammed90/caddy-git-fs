package gitfs

import "io/fs"

type statFs struct {
	fs.FS
}

// Stat implements fs.StatFS.
func (s statFs) Stat(name string) (fs.FileInfo, error) {
	f, err := s.Open(name)
	if err != nil {
		return nil, err
	}
	return f.Stat()
}

var (
	_ fs.StatFS = statFs{}
	_ fs.FS     = statFs{}
)
