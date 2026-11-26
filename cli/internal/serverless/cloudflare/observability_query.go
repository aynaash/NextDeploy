package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/aynaash/nextdeploy/cli/internal/logs"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/cloudflare/cloudflare-go/v6"
	cfshared "github.com/cloudflare/cloudflare-go/v6/shared"
	"github.com/cloudflare/cloudflare-go/v6/workers"
)

// QueryOptions configures historical Cloudflare Workers Observability queries.
type QueryOptions struct {
	Since       time.Duration
	RouteFilter string
	OwnerFilter logs.Owner
	ErrorsOnly  bool
	MaxEvents   int
	NDJSONOut   io.Writer
}

// QueryLogs fetches recent telemetry events for the Worker via Cloudflare's Workers Observability API,
// merges multi-fragment request logs, classifies fault ownership, and returns an AnalysisReport.
func (p *CloudflareProvider) QueryLogs(ctx context.Context, cfg *config.NextDeployConfig, opts QueryOptions) (*logs.AnalysisReport, error) {
	if opts.Since <= 0 {
		opts.Since = 15 * time.Minute
	}
	maxEvents := opts.MaxEvents
	if maxEvents <= 0 {
		maxEvents = 2000
	}

	workerName := p.getWorkerName(cfg)
	now := time.Now().UTC()
	fromMs := float64(now.Add(-opts.Since).UnixMilli())
	toMs := float64(now.UnixMilli())

	collector := logs.NewStreamCollector(logs.AnalyzeOptions{
		RouteFilter:     opts.RouteFilter,
		OwnerFilter:     opts.OwnerFilter,
		AllLevels:       !opts.ErrorsOnly,
		MergedNDJSONOut: opts.NDJSONOut,
	})

	const pageSize = 500
	offset := ""
	fetched := 0

	for fetched < maxEvents {
		limit := pageSize
		if rem := maxEvents - fetched; rem < limit {
			limit = rem
		}

		filters := []workers.ObservabilityTelemetryQueryParamsParametersFilterUnion{
			workers.ObservabilityTelemetryQueryParamsParametersFiltersWorkersObservabilityFilterLeaf{
				Key:       cloudflare.F("$metadata.service"),
				Kind:      cloudflare.F(workers.ObservabilityTelemetryQueryParamsParametersFiltersWorkersObservabilityFilterLeafKindFilter),
				Operation: cloudflare.F(workers.ObservabilityTelemetryQueryParamsParametersFiltersWorkersObservabilityFilterLeafOperationEq),
				Type:      cloudflare.F(workers.ObservabilityTelemetryQueryParamsParametersFiltersWorkersObservabilityFilterLeafTypeString),
				Value:     cloudflare.F[workers.ObservabilityTelemetryQueryParamsParametersFiltersWorkersObservabilityFilterLeafValueUnion](cfshared.UnionString(workerName)),
			},
		}
		if opts.ErrorsOnly {
			filters = append(filters, workers.ObservabilityTelemetryQueryParamsParametersFiltersWorkersObservabilityFilterLeaf{
				Key:       cloudflare.F("$metadata.error"),
				Kind:      cloudflare.F(workers.ObservabilityTelemetryQueryParamsParametersFiltersWorkersObservabilityFilterLeafKindFilter),
				Operation: cloudflare.F(workers.ObservabilityTelemetryQueryParamsParametersFiltersWorkersObservabilityFilterLeafOperationExists),
				Type:      cloudflare.F(workers.ObservabilityTelemetryQueryParamsParametersFiltersWorkersObservabilityFilterLeafTypeString),
			})
		}

		params := workers.ObservabilityTelemetryQueryParams{
			AccountID: cloudflare.F(p.accountID),
			QueryID:   cloudflare.F(fmt.Sprintf("nextdeploy-logs-%d", now.UnixNano())),
			Timeframe: cloudflare.F(workers.ObservabilityTelemetryQueryParamsTimeframe{
				From: cloudflare.F(fromMs),
				To:   cloudflare.F(toMs),
			}),
			View:  cloudflare.F(workers.ObservabilityTelemetryQueryParamsViewEvents),
			Limit: cloudflare.F(float64(limit)),
			Parameters: cloudflare.F(workers.ObservabilityTelemetryQueryParamsParameters{
				Datasets:          cloudflare.F([]string{"cloudflare-workers"}),
				FilterCombination: cloudflare.F(workers.ObservabilityTelemetryQueryParamsParametersFilterCombinationAnd),
				Filters:           cloudflare.F(filters),
			}),
		}
		if offset != "" {
			params.Offset = cloudflare.F(offset)
		}

		resp, err := p.cf.Workers.Observability.Telemetry.Query(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("cloudflare observability query failed: %w", err)
		}
		if resp == nil || len(resp.Events.Events) == 0 {
			break
		}

		events := resp.Events.Events
		for _, ev := range events {
			collector.Add(telemetryEventToExportEntry(ev))
		}
		fetched += len(events)

		lastID := events[len(events)-1].Metadata.ID
		if len(events) < limit || lastID == "" || lastID == offset {
			break
		}
		offset = lastID
	}

	return collector.Finalize(), nil
}

func telemetryEventToExportEntry(ev workers.ObservabilityTelemetryQueryResponseEventsEvent) logs.CFExportEntry {
	entry := logs.CFExportEntry{
		Timestamp: logs.FlexTimestamp(time.UnixMilli(ev.Timestamp).UTC().Format(time.RFC3339Nano)),
		Metadata: logs.CFExportMetadata{
			ID:        ev.Metadata.ID,
			RequestID: ev.Metadata.RequestID,
			TraceID:   ev.Metadata.TraceID,
			Level:     ev.Metadata.Level,
			Error:     ev.Metadata.Error,
			Message:   ev.Metadata.Message,
			Trigger:   ev.Metadata.Trigger,
			Service:   ev.Metadata.Service,
		},
		Workers: logs.CFExportWorkers{
			ScriptName: ev.Workers.ScriptName,
			Outcome:    ev.Workers.Outcome,
			WallTimeMs: ev.Workers.WallTimeMs,
			CPUTimeMs:  ev.Workers.CPUTimeMs,
		},
	}

	if ev.Source != nil {
		if raw, err := json.Marshal(ev.Source); err == nil {
			var src logs.CFExportSource
			if err := json.Unmarshal(raw, &src); err == nil {
				entry.Source = src
			}
		}
	}
	if entry.Source.Level == "" {
		entry.Source.Level = ev.Metadata.Level
	}
	if entry.Source.Message == nil && ev.Metadata.Message != "" {
		entry.Source.Message = ev.Metadata.Message
	}

	if ev.Workers.Event != nil {
		if raw, err := json.Marshal(ev.Workers.Event); err == nil {
			var wEv logs.CFExportWorkersEvent
			if err := json.Unmarshal(raw, &wEv); err == nil {
				entry.Workers.Event = wEv
				if entry.Metadata.RayID == "" {
					entry.Metadata.RayID = tailHeaderLookup(wEv.Request.Headers, "cf-ray")
				}
			}
		}
	}
	if ev.Workers.ScriptVersion != nil {
		if raw, err := json.Marshal(ev.Workers.ScriptVersion); err == nil {
			var sv struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &sv); err == nil {
				entry.Workers.ScriptVersion = sv
			}
		}
	}

	return entry
}
