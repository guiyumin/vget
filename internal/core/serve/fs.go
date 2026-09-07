package serve

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

// Context keys set by Handler for the filesystems below.
type ctxKey int

const (
	ctxContentLength ctxKey = iota // int64: the request's Content-Length, -1 if unknown
	ctxRemoteAddr                  // string: the client's address
)

// dirInfo is a synthetic directory entry.
type dirInfo struct {
	name    string
	modTime time.Time
}

func (d dirInfo) Name() string       { return d.name }
func (d dirInfo) Size() int64        { return 0 }
func (d dirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o755 }
func (d dirInfo) ModTime() time.Time { return d.modTime }
func (d dirInfo) IsDir() bool        { return true }
func (d dirInfo) Sys() any           { return nil }

// renamedInfo presents an on-disk file under a different name.
type renamedInfo struct {
	fs.FileInfo
	name string
}

func (r renamedInfo) Name() string { return r.name }

// listDir is a synthetic, read-only directory listing.
type listDir struct {
	info    fs.FileInfo
	entries []fs.FileInfo
	pos     int
}

func (d *listDir) Close() error                   { return nil }
func (d *listDir) Read([]byte) (int, error)       { return 0, io.EOF }
func (d *listDir) Write([]byte) (int, error)      { return 0, os.ErrPermission }
func (d *listDir) Seek(int64, int) (int64, error) { return 0, nil }
func (d *listDir) Stat() (fs.FileInfo, error)     { return d.info, nil }

func (d *listDir) Readdir(count int) ([]fs.FileInfo, error) {
	if count <= 0 {
		rest := d.entries[d.pos:]
		d.pos = len(d.entries)
		return append([]fs.FileInfo(nil), rest...), nil
	}
	if d.pos >= len(d.entries) {
		return nil, io.EOF
	}
	end := min(d.pos+count, len(d.entries))
	out := append([]fs.FileInfo(nil), d.entries[d.pos:end]...)
	d.pos = end
	return out, nil
}

func splitFirst(name string) (first, rest string) {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[:i], "/" + name[i+1:]
	}
	return name, "/"
}

func isWriteFlag(flag int) bool {
	return flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0
}

// ---- send mode: a virtual root listing exactly the given files ----

type sendEntry struct {
	name string // name shown at the root
	path string // absolute path on disk
	info fs.FileInfo
}

// sendFS serves the given files (and directory trees) read-only under a
// virtual root. Names at the root are the paths' base names, made unique.
type sendFS struct {
	entries []sendEntry
	byName  map[string]*sendEntry
	created time.Time
}

func newSendFS(paths []string) (*sendFS, error) {
	s := &sendFS{byName: map[string]*sendEntry{}, created: time.Now()}
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s: not a regular file", p)
		}
		name := uniqueName(s.byName, info.Name())
		e := sendEntry{name: name, path: abs, info: renamedInfo{info, name}}
		s.entries = append(s.entries, e)
		s.byName[name] = &s.entries[len(s.entries)-1]
	}
	sort.Slice(s.entries, func(i, j int) bool { return s.entries[i].name < s.entries[j].name })
	s.byName = map[string]*sendEntry{}
	for i := range s.entries {
		s.byName[s.entries[i].name] = &s.entries[i]
	}
	return s, nil
}

// uniqueName returns base, or "base (2)", "base (3)", … if taken.
func uniqueName(taken map[string]*sendEntry, base string) string {
	if _, ok := taken[base]; !ok {
		return base
	}
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s (%d)%s", stem, i, ext)
		if _, ok := taken[cand]; !ok {
			return cand
		}
	}
}

// Files returns the served paths as seen by clients ("/name", "/dir/sub/f")
// with their sizes; directories are walked.
func (s *sendFS) Files() (map[string]int64, error) {
	out := map[string]int64{}
	for _, e := range s.entries {
		if !e.info.IsDir() {
			out["/"+e.name] = e.info.Size()
			continue
		}
		root := e.path
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			out["/"+e.name+"/"+filepath.ToSlash(rel)] = info.Size()
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *sendFS) rootInfo() fs.FileInfo { return dirInfo{name: "vget", modTime: s.created} }

func (s *sendFS) Mkdir(context.Context, string, os.FileMode) error { return os.ErrPermission }
func (s *sendFS) RemoveAll(context.Context, string) error          { return os.ErrPermission }
func (s *sendFS) Rename(context.Context, string, string) error     { return os.ErrPermission }

func (s *sendFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	first, rest := splitFirst(name)
	if first == "" {
		return s.rootInfo(), nil
	}
	e, ok := s.byName[first]
	if !ok {
		return nil, os.ErrNotExist
	}
	if rest == "/" {
		return e.info, nil
	}
	if !e.info.IsDir() {
		return nil, os.ErrNotExist
	}
	return webdav.Dir(e.path).Stat(ctx, rest)
}

func (s *sendFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if isWriteFlag(flag) {
		return nil, os.ErrPermission
	}
	first, rest := splitFirst(name)
	if first == "" {
		entries := make([]fs.FileInfo, 0, len(s.entries))
		for _, e := range s.entries {
			entries = append(entries, e.info)
		}
		return &listDir{info: s.rootInfo(), entries: entries}, nil
	}
	e, ok := s.byName[first]
	if !ok {
		return nil, os.ErrNotExist
	}
	if e.info.IsDir() {
		return webdav.Dir(e.path).OpenFile(ctx, rest, flag, perm)
	}
	if rest != "/" {
		return nil, os.ErrNotExist
	}
	f, err := os.Open(e.path)
	if err != nil {
		return nil, err
	}
	return &renamedFile{File: f, name: e.name}, nil
}

// renamedFile is an *os.File whose Stat reports the served name.
type renamedFile struct {
	*os.File
	name string
}

func (f *renamedFile) Stat() (fs.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return renamedInfo{info, f.name}, nil
}

// ---- inbox: a write-only drop box ----

// inboxFS accepts uploads into dir and reveals nothing about its contents:
// listing is empty, every path stats as missing, reads are refused, and an
// upload whose name is taken is stored under a new name rather than
// overwriting. Uploads are written to a temporary file and only moved into
// place once the whole body arrived.
type inboxFS struct {
	dir        string
	created    time.Time
	onReceived func(name, path string, size int64, remote string)
}

func newInboxFS(dir string, onReceived func(name, path string, size int64, remote string)) (*inboxFS, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	return &inboxFS{dir: abs, created: time.Now(), onReceived: onReceived}, nil
}

func (b *inboxFS) rootInfo() fs.FileInfo { return dirInfo{name: "inbox", modTime: b.created} }

func (b *inboxFS) Mkdir(context.Context, string, os.FileMode) error { return os.ErrPermission }
func (b *inboxFS) RemoveAll(context.Context, string) error          { return os.ErrPermission }
func (b *inboxFS) Rename(context.Context, string, string) error     { return os.ErrPermission }

func (b *inboxFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if first, _ := splitFirst(name); first == "" {
		return b.rootInfo(), nil
	}
	return nil, os.ErrNotExist
}

func (b *inboxFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	first, rest := splitFirst(name)
	if first == "" {
		if isWriteFlag(flag) {
			return nil, os.ErrPermission
		}
		return &listDir{info: b.rootInfo()}, nil
	}
	if !isWriteFlag(flag) {
		return nil, os.ErrNotExist
	}
	if rest != "/" || flag&os.O_CREATE == 0 {
		return nil, os.ErrPermission // no subdirectories, no appending to existing files
	}
	base := first
	if base == "." || base == ".." || strings.ContainsRune(base, 0) {
		return nil, os.ErrPermission
	}
	tmp, err := os.CreateTemp(b.dir, ".vget-upload-*.part")
	if err != nil {
		return nil, err
	}
	expected := int64(-1)
	if v, ok := ctx.Value(ctxContentLength).(int64); ok {
		expected = v
	}
	remote, _ := ctx.Value(ctxRemoteAddr).(string)
	return &inboxFile{File: tmp, fs: b, name: base, expected: expected, remote: remote}, nil
}

// inboxFile is an upload in progress.
type inboxFile struct {
	*os.File
	fs       *inboxFS
	name     string
	expected int64
	written  int64
	remote   string
	closed   bool
}

func (f *inboxFile) Write(p []byte) (int, error) {
	n, err := f.File.Write(p)
	f.written += int64(n)
	return n, err
}

func (f *inboxFile) Read([]byte) (int, error)           { return 0, os.ErrPermission }
func (f *inboxFile) Readdir(int) ([]fs.FileInfo, error) { return nil, os.ErrPermission }

func (f *inboxFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true
	tmp := f.File.Name()
	if err := f.File.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if f.expected >= 0 && f.written != f.expected {
		os.Remove(tmp)
		return fmt.Errorf("incomplete upload: got %d of %d bytes", f.written, f.expected)
	}
	final, err := f.fs.place(tmp, f.name)
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if f.fs.onReceived != nil {
		f.fs.onReceived(f.name, final, f.written, f.remote)
	}
	return nil
}

// place moves a finished upload to its final name, never overwriting: the
// requested name if free, else "<stem>.<timestamp>.<random><ext>".
func (b *inboxFS) place(tmp, name string) (string, error) {
	ext := path.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for attempt := 0; attempt < 10; attempt++ {
		cand := name
		if attempt > 0 {
			var r [3]byte
			rand.Read(r[:])
			cand = stem + "." + time.Now().UTC().Format("20060102-150405") + "." + hex.EncodeToString(r[:]) + ext
		}
		final := filepath.Join(b.dir, cand)
		// Reserve the name exclusively, then move the upload over the placeholder.
		f, err := os.OpenFile(final, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		f.Close()
		if err := os.Rename(tmp, final); err != nil {
			os.Remove(final)
			return "", err
		}
		return final, nil
	}
	return "", errors.New("could not find a free name in the inbox")
}

// ---- library + inbox overlay ----

// overlayFS serves lib at / and an inbox at /inbox.
type overlayFS struct {
	lib   webdav.FileSystem
	inbox *inboxFS
}

const inboxName = "inbox"

func (o *overlayFS) split(name string) (inInbox bool, rest string) {
	first, rest := splitFirst(name)
	return first == inboxName, rest
}

func (o *overlayFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	if in, _ := o.split(name); in {
		return os.ErrPermission
	}
	return o.lib.Mkdir(ctx, name, perm)
}

func (o *overlayFS) RemoveAll(ctx context.Context, name string) error {
	if in, _ := o.split(name); in {
		return os.ErrPermission
	}
	return o.lib.RemoveAll(ctx, name)
}

func (o *overlayFS) Rename(ctx context.Context, oldName, newName string) error {
	if in, _ := o.split(oldName); in {
		return os.ErrPermission
	}
	if in, _ := o.split(newName); in {
		return os.ErrPermission
	}
	return o.lib.Rename(ctx, oldName, newName)
}

func (o *overlayFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if in, rest := o.split(name); in {
		return o.inbox.Stat(ctx, rest)
	}
	return o.lib.Stat(ctx, name)
}

func (o *overlayFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if in, rest := o.split(name); in {
		return o.inbox.OpenFile(ctx, rest, flag, perm)
	}
	f, err := o.lib.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}
	if first, _ := splitFirst(name); first == "" {
		return &overlayRoot{File: f, inbox: o.inbox.rootInfo()}, nil
	}
	return f, nil
}

// overlayRoot lists the library root plus the inbox directory.
type overlayRoot struct {
	webdav.File
	inbox fs.FileInfo
	done  bool
}

func (r *overlayRoot) Readdir(count int) ([]fs.FileInfo, error) {
	entries, err := r.File.Readdir(count)
	if count > 0 {
		return entries, err // paginated callers get the raw listing
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if r.done {
		return entries, nil
	}
	r.done = true
	out := make([]fs.FileInfo, 0, len(entries)+1)
	for _, e := range entries {
		if e.Name() == inboxName {
			continue // the inbox shadows a real entry of the same name
		}
		out = append(out, e)
	}
	return append(out, r.inbox), nil
}
