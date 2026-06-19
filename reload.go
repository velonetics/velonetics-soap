package soap

import (
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pucora/lura/v2/logging"
)

type templateHolder struct {
	mu       sync.RWMutex
	tmpl     *template.Template
	path     string
	raw      string
	inline   bool
	watch    bool
	interval time.Duration
	logger   logging.Logger
	prefix   string
	stopCh   chan struct{}
}

func newTemplateHolder(logger logging.Logger, prefix string) *templateHolder {
	return &templateHolder{
		logger: logger,
		prefix: prefix,
		stopCh: make(chan struct{}),
	}
}

func (h *templateHolder) loadFromRaw(raw string) error {
	tmpl, err := template.New("soap").Parse(raw)
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.raw = raw
	h.tmpl = tmpl
	h.mu.Unlock()
	return nil
}

func (h *templateHolder) loadFromPath(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	h.path = path
	return h.loadFromRaw(string(b))
}

func (h *templateHolder) setInline(raw string) error {
	h.inline = true
	return h.loadFromRaw(raw)
}

func (h *templateHolder) startBackground(watch bool, interval time.Duration) {
	if h.inline || h.path == "" {
		return
	}
	h.watch = watch
	h.interval = interval
	if watch {
		go h.watchFile()
	}
	if interval > 0 {
		go h.pollFile()
	}
}

func (h *templateHolder) close() {
	close(h.stopCh)
}

func (h *templateHolder) get() *template.Template {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.tmpl
}

func (h *templateHolder) watchFile() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		h.logger.Error(h.prefix, "template watch:", err)
		return
	}
	defer watcher.Close()
	if err := watcher.Add(h.path); err != nil {
		h.logger.Error(h.prefix, "template watch add:", err)
		return
	}
	for {
		select {
		case <-h.stopCh:
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				h.reloadPath()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			h.logger.Error(h.prefix, "template watch:", err)
		}
	}
}

func (h *templateHolder) pollFile() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.reloadPath()
		}
	}
}

func (h *templateHolder) reloadPath() {
	if h.path == "" {
		return
	}
	if err := h.loadFromPath(h.path); err != nil {
		h.logger.Error(h.prefix, "template reload failed, keeping previous:", err)
		return
	}
	h.logger.Debug(h.prefix, "template reloaded from", h.path)
}
