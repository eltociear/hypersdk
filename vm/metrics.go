// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/utils/metric"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	unitsVerified           prometheus.Counter
	unitsAccepted           prometheus.Counter
	chunkRequests           prometheus.Counter
	failedChunkRequests     prometheus.Counter
	chunkJobFails           prometheus.Counter
	chunksProcessing        prometheus.Gauge
	discardedBuiltBlocks    prometheus.Counter
	txsSubmitted            prometheus.Counter // includes gossip
	txsVerified             prometheus.Counter
	txsAccepted             prometheus.Counter
	stateChanges            prometheus.Counter
	stateOperations         prometheus.Counter
	decisionsRPCConnections prometheus.Gauge
	blocksRPCConnections    prometheus.Gauge
	chunksFetched           metric.Averager
	rootCalculated          metric.Averager
	waitSignatures          metric.Averager
	waitChunks              metric.Averager
	buildPrefetch           metric.Averager
	verifyPrefetch          metric.Averager
	parseToVerified         metric.Averager
	build                   metric.Averager
	mempoolSize             prometheus.Gauge
}

func newMetrics() (*prometheus.Registry, *Metrics, error) {
	r := prometheus.NewRegistry()

	chunksFetched, err := metric.NewAverager(
		"vm",
		"chunks_fetched",
		"time spent fetching chunks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	rootCalculated, err := metric.NewAverager(
		"chain",
		"root_calculated",
		"time spent calculating the state root in verify",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitSignatures, err := metric.NewAverager(
		"chain",
		"wait_signatures",
		"time spent waiting for signature verification in verify",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	waitChunks, err := metric.NewAverager(
		"chain",
		"wait_chunks",
		"time spent waiting for chunks",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	buildPrefetch, err := metric.NewAverager(
		"chain",
		"build_prefetch",
		"time spent prefetching in build",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	verifyPrefetch, err := metric.NewAverager(
		"chain",
		"verify_prefetch",
		"time spent prefetching in verify",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	parseToVerified, err := metric.NewAverager(
		"chain",
		"parse_to_verified",
		"time from block parse starts to when it is verified",
		r,
	)
	if err != nil {
		return nil, nil, err
	}
	build, err := metric.NewAverager(
		"chain",
		"build",
		"time to build block",
		r,
	)
	if err != nil {
		return nil, nil, err
	}

	m := &Metrics{
		unitsVerified: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "units_verified",
			Help:      "amount of units verified",
		}),
		unitsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "units_accepted",
			Help:      "amount of units accepted",
		}),
		chunkRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "chunk_requests",
			Help:      "number of chunk requests",
		}),
		failedChunkRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "failed_chunk_requests",
			Help:      "number of failed chunk requests",
		}),
		chunkJobFails: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "chunk_job_fails",
			Help:      "number of chunk jobs that failed",
		}),
		chunksProcessing: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "chunks_processing",
			Help:      "number of chunks processing",
		}),
		discardedBuiltBlocks: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "discarded_built_blocks",
			Help:      "number of blocks discarded after being built",
		}),
		txsSubmitted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_submitted",
			Help:      "number of txs submitted to vm",
		}),
		txsVerified: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_verified",
			Help:      "number of txs verified by vm",
		}),
		txsAccepted: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vm",
			Name:      "txs_accepted",
			Help:      "number of txs accepted by vm",
		}),
		stateChanges: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "state_changes",
			Help:      "number of state changes",
		}),
		stateOperations: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "chain",
			Name:      "state_operations",
			Help:      "number of state operations",
		}),
		decisionsRPCConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vm",
			Name:      "decisions_rpc_connections",
			Help:      "number of open decisions connections",
		}),
		blocksRPCConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vm",
			Name:      "blocks_rpc_connections",
			Help:      "number of open blocks connections",
		}),
		chunksFetched:   chunksFetched,
		rootCalculated:  rootCalculated,
		waitSignatures:  waitSignatures,
		waitChunks:      waitChunks,
		buildPrefetch:   buildPrefetch,
		verifyPrefetch:  verifyPrefetch,
		parseToVerified: parseToVerified,
		build:           build,
		mempoolSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "chain",
			Name:      "mempool_size",
			Help:      "number of transactions in the mempool",
		}),
	}
	errs := wrappers.Errs{}
	errs.Add(
		r.Register(m.unitsVerified),
		r.Register(m.unitsAccepted),
		r.Register(m.chunkRequests),
		r.Register(m.failedChunkRequests),
		r.Register(m.chunkJobFails),
		r.Register(m.chunksProcessing),
		r.Register(m.discardedBuiltBlocks),
		r.Register(m.txsSubmitted),
		r.Register(m.txsVerified),
		r.Register(m.txsAccepted),
		r.Register(m.stateChanges),
		r.Register(m.stateOperations),
		r.Register(m.decisionsRPCConnections),
		r.Register(m.blocksRPCConnections),
		r.Register(m.mempoolSize),
	)
	return r, m, errs.Err
}
