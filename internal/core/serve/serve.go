// Package serve implements vget's library server: a WebDAV handler with
// HTTP Basic Auth and three access policies: a read-only library, a
// read-write library, and a write-only inbox. It knows nothing about
// transports; the caller serves it on whatever net.Listener it likes (a
// tailcat tunnel, plain TCP behind a reverse proxy, a LAN address).
package serve

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/webdav"
)

// Options configures a Handler.
type Options struct {
	// Dir is the library root served at /. Empty means no library.
	Dir string

	// Files, if set, replaces Dir with a virtual read-only root listing
	// exactly these files and directory trees (vget send).
	Files []string

	// Inbox is a directory that accepts uploads and reveals nothing about
	// its contents. It is served at /inbox next to a library, or at / when
	// there is no library (vget recv).
	Inbox string

	// ReadWrite allows clients to modify the library. Without it only
	// OPTIONS, GET, HEAD and PROPFIND reach the library.
	ReadWrite bool

	// Auth, if non-nil, returns the credentials every request must present.
	// It is called per request so configuration changes apply live; ok=false
	// rejects everything. nil disables authentication.
	Auth func() (user, password string, ok bool)

	// Once makes Done fire once every served file has been downloaded in
	// full (vget send --once).
	Once bool

	// OnEvent, if non-nil, receives notable events.
	OnEvent func(Event)
}

// EventKind classifies an Event.
type EventKind int

const (
	// EventSent: a client finished downloading Name (Size bytes).
	EventSent EventKind = iota
	// EventReceived: an upload landed in the inbox as Path.
	EventReceived
	// EventAuthFailed: a request presented missing or wrong credentials.
	EventAuthFailed
	// EventError: the WebDAV layer reported Err for a request.
	EventError
)

// Event is something worth telling the person running the server.
type Event struct {
	Kind   EventKind
	Name   string // path as seen by the client
	Path   string // path on disk (EventReceived)
	Size   int64
	Remote string
	Err    error
}

// Handler is the library server. It implements http.Handler.
type Handler struct {
	opts    Options
	dav     *webdav.Handler
	inboxAt string // "/inbox", "/" (inbox only), or "" (no inbox)
	tracker *sendTracker
	done    chan struct{}
	once    sync.Once

	// Summary lines for the operator, e.g. "1 file (4.7 GB)".
	FileCount int
	TotalSize int64
}

// NewHandler validates opts and builds the handler.
func NewHandler(opts Options) (*Handler, error) {
	h := &Handler{opts: opts, done: make(chan struct{})}
	var lib webdav.FileSystem
	var tracked map[string]int64

	switch {
	case len(opts.Files) > 0:
		sfs, err := newSendFS(opts.Files)
		if err != nil {
			return nil, err
		}
		files, err := sfs.Files()
		if err != nil {
			return nil, err
		}
		tracked = files
		lib = sfs
	case opts.Dir != "":
		abs, err := filepath.Abs(opts.Dir)
		if err != nil {
			return nil, err
		}
		if fi, err := os.Stat(abs); err != nil {
			return nil, err
		} else if !fi.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", opts.Dir)
		}
		lib = webdav.Dir(abs)
	}
	for _, size := range tracked {
		h.FileCount++
		h.TotalSize += size
	}

	var inbox *inboxFS
	if opts.Inbox != "" {
		var err error
		inbox, err = newInboxFS(opts.Inbox, func(name, p string, size int64, remote string) {
			h.emit(Event{Kind: EventReceived, Name: name, Path: p, Size: size, Remote: remote})
		})
		if err != nil {
			return nil, err
		}
	}

	var root webdav.FileSystem
	switch {
	case lib != nil && inbox != nil:
		root = &overlayFS{lib: lib, inbox: inbox}
		h.inboxAt = "/" + inboxName
	case lib != nil:
		root = lib
	case inbox != nil:
		root = inbox
		h.inboxAt = "/"
	default:
		return nil, errors.New("nothing to serve: give a directory, files, or an inbox")
	}
	if opts.Once && len(tracked) == 0 {
		return nil, errors.New("--once needs files to serve")
	}

	h.tracker = newSendTracker(root, tracked, h)
	h.dav = &webdav.Handler{
		FileSystem: root,
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				h.emit(Event{Kind: EventError, Name: r.Method + " " + r.URL.Path, Remote: r.RemoteAddr, Err: err})
			}
		},
	}
	return h, nil
}

// Done is closed once every served file has been downloaded (Options.Once).
func (h *Handler) Done() <-chan struct{} { return h.done }

// HasInbox reports whether uploads are accepted somewhere.
func (h *Handler) HasInbox() bool { return h.inboxAt != "" }

// InboxPath returns where uploads go ("/inbox" or "/"), or "" if nowhere.
func (h *Handler) InboxPath() string { return h.inboxAt }

func (h *Handler) emit(e Event) {
	if h.opts.OnEvent != nil {
		h.opts.OnEvent(e)
	}
}

func (h *Handler) inInbox(p string) bool {
	switch h.inboxAt {
	case "":
		return false
	case "/":
		return true
	default:
		return p == h.inboxAt || strings.HasPrefix(p, h.inboxAt+"/")
	}
}

// ServeHTTP enforces authentication and the access policy, then hands the
// request to the WebDAV layer.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.opts.Auth != nil && !h.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="vget"`)
		http.Error(w, "authentication required", http.StatusUnauthorized)
		h.emit(Event{Kind: EventAuthFailed, Name: r.Method + " " + r.URL.Path, Remote: r.RemoteAddr})
		return
	}

	p := path.Clean("/" + r.URL.Path)
	if !h.allowed(r.Method, p) {
		http.Error(w, "forbidden: this library does not accept "+r.Method+" here", http.StatusForbidden)
		return
	}

	ctx := context.WithValue(r.Context(), ctxContentLength, r.ContentLength)
	ctx = context.WithValue(ctx, ctxRemoteAddr, r.RemoteAddr)
	r = r.WithContext(ctx)

	if r.Method == http.MethodGet && h.tracker != nil {
		tw := h.tracker.wrap(w, p, r.RemoteAddr)
		h.dav.ServeHTTP(tw, r)
		tw.finish()
		return
	}
	h.dav.ServeHTTP(w, r)
}

func (h *Handler) authorized(r *http.Request) bool {
	user, pass, ok := h.opts.Auth()
	if !ok || user == "" {
		return false
	}
	u, p, got := r.BasicAuth()
	if !got {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(u), []byte(user)) == 1 &&
		subtle.ConstantTimeCompare([]byte(p), []byte(pass)) == 1
}

func (h *Handler) allowed(method, p string) bool {
	if h.inInbox(p) {
		switch method {
		case http.MethodOptions, http.MethodPut, http.MethodGet, http.MethodHead, "PROPFIND":
			// GET/HEAD/PROPFIND of inbox contents answer 404 from the
			// filesystem, revealing nothing. LOCK is refused because the
			// WebDAV layer would create an empty placeholder file for it.
			return true
		}
		return false
	}
	if h.opts.ReadWrite {
		return true
	}
	switch method {
	case http.MethodOptions, http.MethodGet, http.MethodHead, "PROPFIND":
		return true
	}
	return false
}

// ---- download tracking ----

// sendTracker records which byte ranges of each file clients have fetched, so
// the operator learns when a file has been sent in full and "send --once"
// knows when it can exit. Ranged requests from the multi-stream downloader
// arrive out of order, hence intervals rather than a byte count.
type sendTracker struct {
	fs       webdav.FileSystem
	h        *Handler
	required map[string]int64 // files that must be sent in full for Done, by path
	mu       sync.Mutex
	files    map[string]*trackedFile
}

type trackedFile struct {
	size   int64
	ranges [][2]int64 // sorted, disjoint [start, end) intervals
	done   bool
}

func newSendTracker(fs webdav.FileSystem, required map[string]int64, h *Handler) *sendTracker {
	return &sendTracker{fs: fs, h: h, required: required, files: map[string]*trackedFile{}}
}

type trackingWriter struct {
	http.ResponseWriter
	t      *sendTracker
	path   string
	remote string
	status int
	start  int64
	n      int64
}

func (t *sendTracker) wrap(w http.ResponseWriter, p, remote string) *trackingWriter {
	return &trackingWriter{ResponseWriter: w, t: t, path: p, remote: remote}
}

func (tw *trackingWriter) WriteHeader(code int) {
	if tw.status == 0 {
		tw.status = code
		if code == http.StatusPartialContent {
			var a, b, total int64
			if _, err := fmt.Sscanf(tw.Header().Get("Content-Range"), "bytes %d-%d/%d", &a, &b, &total); err == nil {
				tw.start = a
			}
		}
	}
	tw.ResponseWriter.WriteHeader(code)
}

func (tw *trackingWriter) Write(p []byte) (int, error) {
	if tw.status == 0 {
		tw.WriteHeader(http.StatusOK)
	}
	n, err := tw.ResponseWriter.Write(p)
	tw.n += int64(n)
	return n, err
}

// Flush keeps streaming responses flowing through the wrapper.
func (tw *trackingWriter) Flush() {
	if f, ok := tw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (tw *trackingWriter) finish() {
	if tw.status != http.StatusOK && tw.status != http.StatusPartialContent {
		return
	}
	tw.t.record(tw.path, tw.start, tw.start+tw.n, tw.remote)
}

func (t *sendTracker) record(p string, start, end int64, remote string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	f, ok := t.files[p]
	if !ok {
		size, known := t.required[p]
		if !known {
			fi, err := t.fs.Stat(context.Background(), p)
			if err != nil || fi.IsDir() {
				return
			}
			size = fi.Size()
		}
		f = &trackedFile{size: size}
		t.files[p] = f
	}
	if f.done {
		return
	}
	if end > start {
		f.ranges = mergeRange(f.ranges, start, end)
	}
	complete := f.size == 0 || (len(f.ranges) == 1 && f.ranges[0][0] == 0 && f.ranges[0][1] >= f.size)
	if !complete {
		return
	}
	f.done = true
	t.h.emit(Event{Kind: EventSent, Name: p, Size: f.size, Remote: remote})
	if !t.h.opts.Once {
		return
	}
	for req := range t.required {
		if tf, ok := t.files[req]; !ok || !tf.done {
			return
		}
	}
	t.h.once.Do(func() { close(t.h.done) })
}

func mergeRange(ranges [][2]int64, start, end int64) [][2]int64 {
	ranges = append(ranges, [2]int64{start, end})
	sort.Slice(ranges, func(i, j int) bool { return ranges[i][0] < ranges[j][0] })
	out := ranges[:1]
	for _, r := range ranges[1:] {
		last := &out[len(out)-1]
		if r[0] <= last[1] {
			if r[1] > last[1] {
				last[1] = r[1]
			}
			continue
		}
		out = append(out, r)
	}
	return out
}
