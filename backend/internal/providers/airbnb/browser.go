// Package airbnb scrapes a single Airbnb listing page for its rendered
// price — Airbnb has no API, and the price is rendered client-side (a
// React app), so a plain HTTP GET + HTML parse would never see it. This
// drives a real (headless) Chromium instead via chromedp.
package airbnb

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// startupTimeout bounds how long NewBrowser waits for Chrome to launch
// and its DevTools websocket to come up, before failing DI outright
// instead of hanging the server's startup indefinitely.
const startupTimeout = 20 * time.Second

// Browser owns one shared, long-lived Chrome process — many short-lived
// tabs are opened off it per fetch, not one process per fetch, which
// would be far too heavy for headless-render fetches running repeatedly
// on a single small VM.
type Browser struct {
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
	ctx         context.Context
	cancelCtx   context.CancelFunc
}

// NewBrowser launches Chrome immediately (rather than lazily on first
// fetch) so a missing/broken Chrome binary fails DI at startup, not on
// the first scheduled scrape. CHROME_PATH, when set, points chromedp at
// a specific binary — needed in the Docker image, where Chromium isn't
// on the default search path chromedp checks; local dev can leave it
// unset and rely on chromedp's own auto-discovery.
func NewBrowser() (*Browser, error) {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	if path := os.Getenv("CHROME_PATH"); path != "" {
		opts = append(opts, chromedp.ExecPath(path))
	}
	// Required to launch at all as a non-root container user without
	// extra capabilities (Chrome's setuid sandbox needs CAP_SYS_ADMIN,
	// which this image's `nonroot` user doesn't have) — without
	// no-sandbox, chromedp.Run silently hangs until fetchTimeout instead
	// of erroring, which is what surfaced this. disable-dev-shm-usage
	// works around Docker's default 64MB /dev/shm, too small for
	// Chrome's shared memory usage and a common cause of renderer
	// crashes; /tmp is used instead. Fine to apply outside Docker too —
	// this only ever visits one trusted domain (Airbnb), so the sandbox
	// isn't defending against anything adversarial here.
	opts = append(opts, chromedp.Flag("no-sandbox", true), chromedp.Flag("disable-dev-shm-usage", true))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	// Bounded only for this initial launch-and-connect check — chromedp.Run
	// blocks until Chrome's DevTools websocket is up, with no deadline of
	// its own, so a Chrome that fails to come up cleanly (stale lock files
	// after an unclean shutdown, resource contention right after a host
	// sleep/wake, ...) would otherwise hang this call forever and, with it,
	// the whole di.New() startup path — the server would never bind its
	// port and every health check would fail with no log line explaining
	// why. ctx itself (used for real tab traffic afterward) stays
	// undecorated; each scrape already gets its own bound via NewTab.
	startCtx, cancelStart := context.WithTimeout(ctx, startupTimeout)
	defer cancelStart()
	if err := chromedp.Run(startCtx); err != nil {
		cancelCtx()
		cancelAlloc()
		return nil, fmt.Errorf("airbnb: launch browser: %w", err)
	}
	return &Browser{allocCtx: allocCtx, cancelAlloc: cancelAlloc, ctx: ctx, cancelCtx: cancelCtx}, nil
}

// NewTab returns a fresh tab context off the shared browser, bounded by
// timeout, and a cancel func the caller must call to close the tab.
func (b *Browser) NewTab(timeout time.Duration) (context.Context, context.CancelFunc) {
	tabCtx, cancelTab := chromedp.NewContext(b.ctx)
	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, timeout)
	return tabCtx, func() {
		cancelTimeout()
		cancelTab()
	}
}

// Close tears down the shared Chrome process — call once, at shutdown.
func (b *Browser) Close() {
	b.cancelCtx()
	b.cancelAlloc()
}
