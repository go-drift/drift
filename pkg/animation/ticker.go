// Package animation provides animation primitives for the Drift framework.
package animation

import (
	"sync"
	"time"
)

var (
	tickerMu      sync.Mutex
	activeTickers = make(map[*Ticker]struct{})
	lastTickTime  time.Time
)

// Ticker calls a callback on each frame.
type Ticker struct {
	callback func(elapsed time.Duration)
	isActive bool
	start    time.Time
}

// NewTicker creates a new ticker with the given callback.
func NewTicker(callback func(elapsed time.Duration)) *Ticker {
	return &Ticker{
		callback: callback,
	}
}

// Start activates the ticker.
func (t *Ticker) Start() {
	if t.isActive {
		return
	}
	t.isActive = true
	t.start = time.Now()
	tickerMu.Lock()
	activeTickers[t] = struct{}{}
	tickerMu.Unlock()
}

// Stop deactivates the ticker.
func (t *Ticker) Stop() {
	if !t.isActive {
		return
	}
	t.isActive = false
	tickerMu.Lock()
	delete(activeTickers, t)
	tickerMu.Unlock()
}

// IsActive returns whether the ticker is currently running.
func (t *Ticker) IsActive() bool {
	return t.isActive
}

// Elapsed returns the time since the ticker started.
func (t *Ticker) Elapsed() time.Duration {
	if !t.isActive {
		return 0
	}
	return time.Since(t.start)
}

// TickerProvider creates tickers.
type TickerProvider interface {
	CreateTicker(callback func(time.Duration)) *Ticker
}

// StepTickers advances all active tickers.
// This should be called once per frame from the engine.
func StepTickers() {
	tickerMu.Lock()
	if len(activeTickers) == 0 {
		tickerMu.Unlock()
		return
	}
	// Make a copy to avoid holding lock during callbacks
	tickers := make([]*Ticker, 0, len(activeTickers))
	for ticker := range activeTickers {
		tickers = append(tickers, ticker)
	}
	tickerMu.Unlock()

	for _, ticker := range tickers {
		if ticker.isActive && ticker.callback != nil {
			elapsed := time.Since(ticker.start)
			ticker.callback(elapsed)
		}
	}
}

// HasActiveTickers returns true if any tickers are active.
func HasActiveTickers() bool {
	tickerMu.Lock()
	defer tickerMu.Unlock()
	return len(activeTickers) > 0
}
