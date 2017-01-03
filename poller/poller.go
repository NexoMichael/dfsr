package poller

import (
	"context"
	"sync"
	"time"
)

// Source is a polling source.
type Source interface {
	Poll(ctx context.Context)
	Close() // TODO: Consider removing this and doing a runtime type check for io.Closer
}

// Poller executes a polling function on an interval.
type Poller struct {
	mutex    sync.Mutex
	interval time.Duration
	source   Source
	updating bool
	pulse    chan struct{} // Signals update. nil indicates closed.
	stop     chan struct{} // Signals stop. nil indicates stopped.
	idle     *sync.Cond
	closed   bool
}

// New returns a new poller for the given source.
func New(source Source, interval time.Duration) *Poller {
	p := &Poller{
		source:   source,
		interval: interval,
		pulse:    make(chan struct{}),
		stop:     make(chan struct{}),
	}
	p.idle = sync.NewCond(&p.mutex)
	go p.run()
	return p
}

// Close causes the poller to stop polling and release any resources consumed
// by the poller. It will implicitly call the close function on the polling
// source.
func (p *Poller) Close() {
	p.mutex.Lock()
	// Don't defer p.mutex.Unlock() here because that would mess up sync.Cond.Wait
	if p.closed {
		p.mutex.Unlock()
		return
	}

	p.closed = true

	close(p.stop)
	close(p.pulse)

	// If there's an update goroutine still running, wait until it's done before
	// closing the source.
	for p.updating {
		p.idle.Wait()
	}

	p.source.Close() // TODO: Consider doing a runtime interface type check here
	p.mutex.Unlock()
}

// Poll causes the poller to immediately poll the polling source. It does
// not wait for the polling action to complete.
func (p *Poller) Poll() {
	p.mutex.Lock()
	if !p.closed {
		p.pulse <- struct{}{}
	}
	p.mutex.Unlock()
}

func (p *Poller) run() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	for {
		ctx, cancel = context.WithCancel(context.Background())
		select {
		case <-p.stop:
			cancel()
			return
		case <-p.pulse:
		case <-ticker.C:
		}

		go p.update(ctx, cancel)
	}
}

func (p *Poller) update(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	if !p.startUpdate() {
		// There is an update goroutine already running, so we're skipping this
		// tick so that we don't spawn doubles
		return
	}

	p.source.Poll(ctx)

	p.finishUpdate()
}

func (p *Poller) startUpdate() (acquired bool) {
	p.mutex.Lock()
	if !p.closed && !p.updating {
		p.updating = true
		acquired = true
	}
	p.mutex.Unlock()
	return
}

func (p *Poller) finishUpdate() {
	p.mutex.Lock()
	p.updating = false
	p.mutex.Unlock()
	p.idle.Broadcast()
}
