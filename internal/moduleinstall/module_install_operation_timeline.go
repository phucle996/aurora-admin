package moduleinstall

import (
	keycfg "admin/internal/key"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	moduleInstallOperationRunning   = "running"
	moduleInstallOperationCompleted = "completed"
	moduleInstallOperationFailed    = "failed"
)

type ModuleInstallOperationSummary struct {
	OperationID      string `json:"operation_id"`
	AgentID          string `json:"agent_id"`
	Module           string `json:"module"`
	Version          string `json:"version,omitempty"`
	ServiceName      string `json:"service_name,omitempty"`
	ArtifactChecksum string `json:"artifact_checksum,omitempty"`
	AppHost          string `json:"app_host,omitempty"`
	Endpoint         string `json:"endpoint,omitempty"`
	Status           string `json:"status"`
	Health           string `json:"health,omitempty"`
	LastStage        string `json:"last_stage,omitempty"`
	LastMessage      string `json:"last_message,omitempty"`
	ErrorText        string `json:"error_text,omitempty"`
	StartedAt        string `json:"started_at"`
	UpdatedAt        string `json:"updated_at"`
	CompletedAt      string `json:"completed_at,omitempty"`
}

type ModuleInstallOperationEvent struct {
	OperationID string `json:"operation_id"`
	Sequence    int64  `json:"sequence"`
	Type        string `json:"type"`
	Stage       string `json:"stage,omitempty"`
	Message     string `json:"message,omitempty"`
	ObservedAt  string `json:"observed_at"`
}

type moduleInstallOperationTracker struct {
	service *ModuleInstallService
	summary ModuleInstallOperationSummary
	nextSeq int64
	mu      sync.Mutex
}

func (s *ModuleInstallService) beginInstallOperation(
	ctx context.Context,
	target moduleInstallTarget,
	moduleName string,
	appHost string,
	endpoint string,
) (*moduleInstallOperationTracker, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	summary := ModuleInstallOperationSummary{
		OperationID: newModuleInstallOperationID(moduleName),
		AgentID:     strings.TrimSpace(target.AgentID),
		Module:      canonicalModuleName(moduleName),
		AppHost:     strings.TrimSpace(appHost),
		Endpoint:    strings.TrimSpace(endpoint),
		Status:      moduleInstallOperationRunning,
		Health:      moduleInstallHealthUnknown,
		StartedAt:   now,
		UpdatedAt:   now,
	}
	tracker := &moduleInstallOperationTracker{
		service: s,
		summary: summary,
	}
	if err := tracker.persistSummary(ctx); err != nil {
		return nil, err
	}
	return tracker, nil
}

func (s *ModuleInstallService) GetInstallOperation(
	ctx context.Context,
	operationID string,
) (*ModuleInstallOperationSummary, []ModuleInstallOperationEvent, error) {
	if s == nil || s.runtimeRepo == nil {
		return nil, nil, nil
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return nil, nil, fmt.Errorf("operation_id is required")
	}

	summaryText, found, err := s.runtimeRepo.Get(ctx, keycfg.RuntimeInstallOperationSummaryKey(operationID))
	if err != nil {
		return nil, nil, fmt.Errorf("load install operation summary failed: %w", err)
	}
	if !found {
		return nil, nil, fmt.Errorf("install operation not found")
	}

	var summary ModuleInstallOperationSummary
	if err := json.Unmarshal([]byte(summaryText), &summary); err != nil {
		return nil, nil, fmt.Errorf("decode install operation summary failed: %w", err)
	}
	summary = normalizeModuleInstallOperationSummary(summary)

	kvs, err := s.runtimeRepo.ListByPrefix(ctx, keycfg.RuntimeInstallOperationEventsPrefix(operationID))
	if err != nil {
		return nil, nil, fmt.Errorf("load install operation events failed: %w", err)
	}
	events := make([]ModuleInstallOperationEvent, 0, len(kvs))
	for _, kv := range kvs {
		var event ModuleInstallOperationEvent
		if err := json.Unmarshal([]byte(kv.Value), &event); err != nil {
			continue
		}
		events = append(events, normalizeModuleInstallOperationEvent(event))
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Sequence == events[j].Sequence {
			return events[i].ObservedAt < events[j].ObservedAt
		}
		return events[i].Sequence < events[j].Sequence
	})
	return &summary, events, nil
}

func (t *moduleInstallOperationTracker) RecordEvent(ctx context.Context, stage string, message string) {
	if t == nil || t.service == nil || t.service.runtimeRepo == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	t.nextSeq++
	event := normalizeModuleInstallOperationEvent(ModuleInstallOperationEvent{
		OperationID: t.summary.OperationID,
		Sequence:    t.nextSeq,
		Type:        "log",
		Stage:       strings.TrimSpace(stage),
		Message:     strings.TrimSpace(message),
		ObservedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	})
	_ = t.persistEvent(ctx, event)

	t.summary.LastStage = event.Stage
	t.summary.LastMessage = event.Message
	t.summary.UpdatedAt = event.ObservedAt
	_ = t.persistSummary(ctx)
}

func (t *moduleInstallOperationTracker) SetInstallPlan(
	ctx context.Context,
	agentID string,
	version string,
	artifactChecksum string,
) {
	if t == nil || t.service == nil || t.service.runtimeRepo == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	t.summary.AgentID = firstNonEmpty(strings.TrimSpace(agentID), t.summary.AgentID)
	t.summary.Version = strings.TrimSpace(version)
	t.summary.ArtifactChecksum = strings.TrimSpace(artifactChecksum)
	t.summary.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_ = t.persistSummary(ctx)
}

func (t *moduleInstallOperationTracker) Complete(ctx context.Context, result *ModuleInstallResult) {
	if t == nil || t.service == nil || t.service.runtimeRepo == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	t.summary.Status = moduleInstallOperationCompleted
	if result != nil {
		t.summary.Health = normalizeObservedInstallHealth(result.Health)
		t.summary.AgentID = firstNonEmpty(strings.TrimSpace(result.AgentID), t.summary.AgentID)
		t.summary.Version = strings.TrimSpace(result.Version)
		t.summary.ServiceName = strings.TrimSpace(result.ServiceName)
		t.summary.ArtifactChecksum = strings.TrimSpace(result.ArtifactChecksum)
		t.summary.Endpoint = firstNonEmpty(strings.TrimSpace(result.Endpoint), t.summary.Endpoint)
	}
	t.summary.LastStage = "completed"
	t.summary.LastMessage = "module install completed"
	t.summary.ErrorText = ""
	t.summary.UpdatedAt = now
	t.summary.CompletedAt = now
	_ = t.persistSummary(ctx)

	t.nextSeq++
	_ = t.persistEvent(ctx, normalizeModuleInstallOperationEvent(ModuleInstallOperationEvent{
		OperationID: t.summary.OperationID,
		Sequence:    t.nextSeq,
		Type:        "result",
		Stage:       "completed",
		Message:     "module install completed",
		ObservedAt:  now,
	}))
}

func (t *moduleInstallOperationTracker) Fail(ctx context.Context, err error) {
	if t == nil || t.service == nil || t.service.runtimeRepo == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	errorText := ""
	if err != nil {
		errorText = strings.TrimSpace(err.Error())
	}
	t.summary.Status = moduleInstallOperationFailed
	t.summary.Health = moduleInstallHealthUnknown
	t.summary.LastStage = "failed"
	t.summary.LastMessage = errorText
	t.summary.ErrorText = errorText
	t.summary.UpdatedAt = now
	t.summary.CompletedAt = now
	_ = t.persistSummary(ctx)

	t.nextSeq++
	_ = t.persistEvent(ctx, normalizeModuleInstallOperationEvent(ModuleInstallOperationEvent{
		OperationID: t.summary.OperationID,
		Sequence:    t.nextSeq,
		Type:        "error",
		Stage:       "failed",
		Message:     errorText,
		ObservedAt:  now,
	}))
}

func (t *moduleInstallOperationTracker) persistSummary(ctx context.Context) error {
	body, err := json.Marshal(normalizeModuleInstallOperationSummary(t.summary))
	if err != nil {
		return fmt.Errorf("marshal install operation summary failed: %w", err)
	}
	return t.service.runtimeRepo.Upsert(ctx, keycfg.RuntimeInstallOperationSummaryKey(t.summary.OperationID), strings.TrimSpace(string(body)))
}

func (t *moduleInstallOperationTracker) persistEvent(ctx context.Context, event ModuleInstallOperationEvent) error {
	body, err := json.Marshal(normalizeModuleInstallOperationEvent(event))
	if err != nil {
		return fmt.Errorf("marshal install operation event failed: %w", err)
	}
	sequence := fmt.Sprintf("%020d", event.Sequence)
	return t.service.runtimeRepo.Upsert(ctx, keycfg.RuntimeInstallOperationEventKey(t.summary.OperationID, sequence), strings.TrimSpace(string(body)))
}

func newModuleInstallOperationID(moduleName string) string {
	return fmt.Sprintf("install-%s-%d", canonicalModuleName(moduleName), time.Now().UTC().UnixNano())
}

func normalizeModuleInstallOperationSummary(summary ModuleInstallOperationSummary) ModuleInstallOperationSummary {
	summary.OperationID = strings.TrimSpace(summary.OperationID)
	summary.AgentID = strings.TrimSpace(summary.AgentID)
	summary.Module = canonicalModuleName(summary.Module)
	summary.Version = strings.TrimSpace(summary.Version)
	summary.ServiceName = strings.TrimSpace(summary.ServiceName)
	summary.ArtifactChecksum = strings.TrimSpace(summary.ArtifactChecksum)
	summary.AppHost = strings.TrimSpace(summary.AppHost)
	summary.Endpoint = strings.TrimSpace(summary.Endpoint)
	summary.Status = normalizeModuleInstallOperationStatus(summary.Status)
	summary.Health = normalizeObservedInstallHealth(summary.Health)
	summary.LastStage = strings.TrimSpace(summary.LastStage)
	summary.LastMessage = strings.TrimSpace(summary.LastMessage)
	summary.ErrorText = strings.TrimSpace(summary.ErrorText)
	summary.StartedAt = strings.TrimSpace(summary.StartedAt)
	summary.UpdatedAt = strings.TrimSpace(summary.UpdatedAt)
	summary.CompletedAt = strings.TrimSpace(summary.CompletedAt)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if summary.StartedAt == "" {
		summary.StartedAt = now
	}
	if summary.UpdatedAt == "" {
		summary.UpdatedAt = summary.StartedAt
	}
	return summary
}

func normalizeModuleInstallOperationEvent(event ModuleInstallOperationEvent) ModuleInstallOperationEvent {
	event.OperationID = strings.TrimSpace(event.OperationID)
	event.Type = strings.TrimSpace(event.Type)
	event.Stage = strings.TrimSpace(event.Stage)
	event.Message = strings.TrimSpace(event.Message)
	event.ObservedAt = strings.TrimSpace(event.ObservedAt)
	if event.ObservedAt == "" {
		event.ObservedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return event
}

func normalizeModuleInstallOperationStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", moduleInstallOperationRunning:
		return moduleInstallOperationRunning
	case moduleInstallOperationCompleted:
		return moduleInstallOperationCompleted
	case moduleInstallOperationFailed:
		return moduleInstallOperationFailed
	default:
		return strings.TrimSpace(raw)
	}
}

func backgroundOperationWriteContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func wrapInstallLogFn(base InstallLogFn, tracker *moduleInstallOperationTracker) InstallLogFn {
	if tracker == nil {
		return base
	}
	return func(stage, message string) {
		ctx, cancel := backgroundOperationWriteContext()
		tracker.RecordEvent(ctx, stage, message)
		cancel()
		if base != nil {
			base(stage, message)
		}
	}
}

func operationTrackerID(tracker *moduleInstallOperationTracker) string {
	if tracker == nil {
		return ""
	}
	return strings.TrimSpace(tracker.summary.OperationID)
}
