// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/prometheus/client_golang/prometheus"
)

const maxPrealloc = 4_096

type Mempool[T Item] struct {
	tracer  trace.Tracer
	metrics *metrics

	mu sync.RWMutex

	maxSize      int
	maxPayerSize int // Maximum items allowed by a single payer

	pm *SortedMempool[T] // Price Mempool
	tm *SortedMempool[T] // Time Mempool

	// [Owned] used to remove all items from an account when the balance is
	// insufficient
	owned map[string]set.Set[ids.ID]

	// payers that are exempt from [maxPayerSize]
	exemptPayers set.Set[string]

	// items we are currently executing that we don't want to allow duplicates of
	leasedItems set.Set[ids.ID]
}

// New creates a new [Mempool]. [maxSize] must be > 0 or else the
// implementation may panic.
func New[T Item](
	tracer trace.Tracer,
	maxSize int,
	maxPayerSize int,
	exemptPayers [][]byte,
) (*Mempool[T], *prometheus.Registry, error) {
	m := &Mempool[T]{
		tracer: tracer,

		maxSize:      maxSize,
		maxPayerSize: maxPayerSize,

		pm: NewSortedMempool(
			math.Min(maxSize, maxPrealloc),
			func(item T) uint64 { return item.UnitPrice() },
		),
		tm: NewSortedMempool(
			math.Min(maxSize, maxPrealloc),
			func(item T) uint64 { return uint64(item.Expiry()) },
		),
		owned:        map[string]set.Set[ids.ID]{},
		exemptPayers: set.Set[string]{},
	}
	for _, payer := range exemptPayers {
		m.exemptPayers.Add(string(payer))
	}
	registry, metrics, err := newMetrics()
	if err != nil {
		return nil, nil, err
	}
	m.metrics = metrics
	return m, registry, nil
}

type metrics struct {
	buildOverhead   metric.Averager
	setMinTimestamp metric.Averager
	add             metric.Averager
}

func newMetrics() (*prometheus.Registry, *metrics, error) {
	r := prometheus.NewRegistry()
	buildOverhead, err := metric.NewAverager(
		"mempool",
		"build_overhead",
		"time spent handling mempool build",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	setMinTimestamp, err := metric.NewAverager(
		"mempool",
		"set_min_timestamp",
		"time spent setting min timestamp",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	add, err := metric.NewAverager(
		"mempool",
		"add",
		"time spent adding",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	m := &metrics{
		buildOverhead:   buildOverhead,
		setMinTimestamp: setMinTimestamp,
		add:             add,
		// TODO: add size
	}
	return r, m, nil
}

func (th *Mempool[T]) removeFromOwned(item T) {
	sender := item.Payer()
	acct, ok := th.owned[sender]
	if !ok {
		// May no longer be populated
		return
	}
	acct.Remove(item.ID())
	if len(acct) == 0 {
		delete(th.owned, sender)
	}
}

// Has returns if the pm of [th] contains [itemID]
func (th *Mempool[T]) Has(ctx context.Context, itemID ids.ID) bool {
	_, span := th.tracer.Start(ctx, "Mempool.Has")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()
	return th.pm.Has(itemID)
}

// Add pushes all new items from [items] to th. Does not add a item if
// the item payer is not exempt and their items in the mempool exceed th.maxPayerSize.
// If the size of th exceeds th.maxSize, Add pops the lowest value item
// from th.pm.
func (th *Mempool[T]) Add(ctx context.Context, items []T) {
	_, span := th.tracer.Start(ctx, "Mempool.Add")
	defer span.End()

	start := time.Now()
	defer func() {
		th.metrics.add.Observe(float64(time.Since(start)))
	}()

	th.mu.Lock()
	defer th.mu.Unlock()

	th.add(ctx, items)
}

func (th *Mempool[T]) add(ctx context.Context, items []T) {
	for _, item := range items {
		sender := item.Payer()

		// Ensure no duplicate
		if th.leasedItems != nil && th.leasedItems.Contains(item.ID()) {
			continue
		}
		if th.pm.Has(item.ID()) {
			// Don't drop because already exists
			continue
		}

		// Optimistically add to both mempools
		acct, ok := th.owned[sender]
		if !ok {
			acct = set.Set[ids.ID]{}
			th.owned[sender] = acct
		}
		if !th.exemptPayers.Contains(sender) && acct.Len() == th.maxPayerSize {
			continue // do nothing, wait for items to expire
		}
		th.pm.Add(item)
		th.tm.Add(item)
		acct.Add(item.ID())

		// Remove the lowest paying item if at global max
		if th.pm.Len() > th.maxSize {
			// Remove the lowest paying item
			lowItem, _ := th.pm.PopMin()
			th.tm.Remove(lowItem.ID())
			th.removeFromOwned(lowItem)
		}
	}
}

// PeekMax returns the highest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PeekMax(ctx context.Context) (T, bool) {
	_, span := th.tracer.Start(ctx, "Mempool.PeekMax")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.pm.PeekMax()
}

// PeekMin returns the lowest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PeekMin(ctx context.Context) (T, bool) {
	_, span := th.tracer.Start(ctx, "Mempool.PeekMin")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.pm.PeekMin()
}

// PopMax removes and returns the highest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PopMax(ctx context.Context) (T, bool) { // O(log N)
	_, span := th.tracer.Start(ctx, "Mempool.PopMax")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	return th.popMax()
}

func (th *Mempool[T]) popMax() (T, bool) {
	max, ok := th.pm.PopMax()
	if ok {
		th.tm.Remove(max.ID())
		th.removeFromOwned(max)
	}
	return max, ok
}

// PopMin removes and returns the lowest valued item in th.pm.
// Assumes there is non-zero items in [Mempool]
func (th *Mempool[T]) PopMin(ctx context.Context) (T, bool) { // O(log N)
	_, span := th.tracer.Start(ctx, "Mempool.PopMin")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	min, ok := th.pm.PopMin()
	if ok {
		th.tm.Remove(min.ID())
		th.removeFromOwned(min)
	}
	return min, ok
}

// Remove removes [items] from th.
func (th *Mempool[T]) Remove(ctx context.Context, items []T) {
	_, span := th.tracer.Start(ctx, "Mempool.Remove")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	for _, item := range items {
		th.pm.Remove(item.ID())
		th.tm.Remove(item.ID())
		th.removeFromOwned(item)
		// Remove is called when verifying a block. We should not drop transactions at
		// this time.
	}
}

// Len returns the number of items in th.
func (th *Mempool[T]) Len(ctx context.Context) int {
	_, span := th.tracer.Start(ctx, "Mempool.Len")
	defer span.End()

	th.mu.RLock()
	defer th.mu.RUnlock()

	return th.pm.Len()
}

// RemoveAccount removes all items by [sender] from th.
func (th *Mempool[T]) RemoveAccount(ctx context.Context, sender string) {
	_, span := th.tracer.Start(ctx, "Mempool.RemoveAccount")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	th.removeAccount(sender)
}

func (th *Mempool[T]) removeAccount(sender string) {
	acct, ok := th.owned[sender]
	if !ok {
		return
	}
	for item := range acct {
		th.pm.Remove(item)
		th.tm.Remove(item)
	}
	delete(th.owned, sender)
}

// SetMinTimestamp removes all items with a lower expiry than [t] from th.
// SetMinTimestamp returns the list of removed items.
func (th *Mempool[T]) SetMinTimestamp(ctx context.Context, t int64) []T {
	_, span := th.tracer.Start(ctx, "Mempool.SetMinTimesamp")
	defer span.End()

	start := time.Now()
	defer func() {
		th.metrics.setMinTimestamp.Observe(float64(time.Since(start)))
	}()

	th.mu.Lock()
	defer th.mu.Unlock()

	removed := th.tm.SetMinVal(uint64(t))
	for _, remove := range removed {
		th.pm.Remove(remove.ID())
		th.removeFromOwned(remove)
	}
	return removed
}

// TODO: break build apart into:
// * fetch of X txs (kept in pending map to avoid duplicate entry)
//   - could use MaxUnits to determine and try to build 1 tx block at a time
//
// * pre-fetch state, iterate repeatedly until max time has elapsed our out of
// txs
func (th *Mempool[T]) Build(
	ctx context.Context,
	f func(context.Context, T) (cont bool, restore bool, removeAcct bool, err error),
) error {
	ctx, span := th.tracer.Start(ctx, "Mempool.Build")
	defer span.End()

	start := time.Now()
	var vmTime time.Duration
	defer func() {
		th.metrics.buildOverhead.Observe(float64(time.Since(start) - vmTime))
	}()

	th.mu.Lock()
	defer th.mu.Unlock()

	restorableItems := []T{}
	var err error
	for th.pm.Len() > 0 {
		max, _ := th.pm.PopMax()
		vmStart := time.Now()
		cont, restore, removeAccount, fErr := f(ctx, max)
		vmTime += time.Since(vmStart)
		if restore {
			// Waiting to restore unused transactions ensures that an account will be
			// excluded from future price mempool iterations
			restorableItems = append(restorableItems, max)
		} else {
			th.tm.Remove(max.ID())
			th.removeFromOwned(max)
		}
		if removeAccount {
			// We remove the account typically when the next execution results in an
			// invalid balance
			th.removeAccount(max.Payer())
		}
		if !cont || fErr != nil {
			err = fErr
			break
		}
	}
	//
	// Restore unused items
	for _, item := range restorableItems {
		th.pm.Add(item)
	}
	return err
}

func (th *Mempool[T]) LeaseItems(ctx context.Context, count int) []T {
	ctx, span := th.tracer.Start(ctx, "Mempool.LeaseTxs")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	txs := make([]T, 0, count)
	th.leasedItems = set.NewSet[ids.ID](count)
	for len(txs) < count {
		item, ok := th.popMax()
		if !ok {
			break
		}
		th.leasedItems.Add(item.ID())
		txs = append(txs, item)
	}
	return txs
}

func (th *Mempool[T]) ClearLease(ctx context.Context, restore []T) {
	// We don't handle removed txs here, we just skip
	ctx, span := th.tracer.Start(ctx, "Mempool.ClearLease")
	defer span.End()

	th.mu.Lock()
	defer th.mu.Unlock()

	th.leasedItems = nil
	th.add(ctx, restore)
}
