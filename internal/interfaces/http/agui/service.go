package agui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	harnessruntime "github.com/huanglei214/agent-demo/internal/runtime"
	"github.com/huanglei214/agent-demo/internal/service"
)

var ErrStreamUnwritable = errors.New("agui stream unwritable")

type Service struct {
	services service.Services
}

func NewService(services service.Services) Service {
	return Service{services: services}
}

func (s Service) StreamChat(_ context.Context, req ChatRequest, writer *SSEWriter) (err error) {
	message, err := lastUserMessage(req.Messages)
	if err != nil {
		return err
	}

	workspace := strings.TrimSpace(req.State.Workspace)
	if workspace == "" {
		workspace = s.services.Config.Workspace
	}
	provider := strings.TrimSpace(req.State.Provider)
	if provider == "" {
		provider = s.services.Config.Model.Provider
	}
	model := strings.TrimSpace(req.State.Model)
	if model == "" {
		model = s.services.Config.Model.Model
	}
	maxTurns := req.State.MaxTurns
	if maxTurns <= 0 {
		maxTurns = 20
	}
	planMode := strings.TrimSpace(req.State.PlanMode)

	threadID := strings.TrimSpace(req.ThreadID)
	if threadID == "" {
		if raw, ok := req.Context["sessionId"]; ok && raw != nil {
			threadID = strings.TrimSpace(fmt.Sprint(raw))
		}
	}

	observer := newChannelObserver()
	outcomeCh := make(chan runOutcome, 1)
	runCtx := context.Background()
	deferOnExit := true
	defer func() {
		if deferOnExit {
			go drainRunCompletion(observer.events, outcomeCh)
		}
	}()
	go func() {
		response, err := s.services.StartRunStream(runCtx, service.RunRequest{
			Instruction: message.Content,
			Workspace:   workspace,
			Provider:    provider,
			Model:       model,
			MaxTurns:    maxTurns,
			SessionID:   threadID,
			PlanMode:    harnessruntime.PlanMode(planMode),
		}, observer)
		outcomeCh <- runOutcome{response: response, err: err}
		close(outcomeCh)
		close(observer.events)
	}()

	var (
		sawTerminal    bool
		snapshotSent   bool
		streamThreadID = threadID
		streamRunID    string
		finalOutcome   *runOutcome
	)

	for observer.events != nil || outcomeCh != nil {
		select {
		case event, ok := <-observer.events:
			if !ok {
				observer.events = nil
				continue
			}
			if streamThreadID == "" {
				streamThreadID = event.SessionID
			}
			if streamRunID == "" {
				streamRunID = event.RunID
			}

			mapped := MapRuntimeEvent(event)
			for _, item := range mapped {
				if err = writeStreamEvent(writer, item); err != nil {
					return err
				}
			}
			if event.Type == "run.started" && !snapshotSent {
				snapshots, err := s.initialSnapshots(event.RunID, event.SessionID)
				if err != nil {
					return err
				}
				for _, item := range snapshots {
					if err = writeStreamEvent(writer, item); err != nil {
						return err
					}
				}
				snapshotSent = true
			}
			if event.Type == "run.failed" {
				log.Printf(
					"agui runtime failure run_id=%q session_id=%q payload=%v",
					event.RunID,
					event.SessionID,
					event.Payload,
				)
			}
			if event.Type == "run.completed" || event.Type == "run.failed" {
				sawTerminal = true
			}
		case item, ok := <-outcomeCh:
			if !ok {
				outcomeCh = nil
				continue
			}
			current := item
			finalOutcome = &current
			outcomeCh = nil
		}
	}
	deferOnExit = false

	if finalOutcome != nil && finalOutcome.err != nil && !sawTerminal {
		return writeStreamEvent(writer, Event{
			Type:     "RUN_ERROR",
			ThreadID: streamThreadID,
			RunID:    streamRunID,
			Error:    finalOutcome.err.Error(),
		})
	}
	if finalOutcome != nil && finalOutcome.err == nil && !sawTerminal && finalOutcome.response.Run.ID != "" {
		return writeStreamEvent(writer, Event{
			Type:     "RUN_FINISHED",
			ThreadID: finalOutcome.response.Run.SessionID,
			RunID:    finalOutcome.response.Run.ID,
		})
	}

	return nil
}

func writeStreamEvent(writer *SSEWriter, event Event) error {
	if err := writer.Write(event); err != nil {
		return fmt.Errorf("%w: %w", ErrStreamUnwritable, err)
	}
	return nil
}

func drainRunCompletion(events <-chan harnessruntime.Event, outcomes <-chan runOutcome) {
	for events != nil || outcomes != nil {
		select {
		case _, ok := <-events:
			if !ok {
				events = nil
			}
		case _, ok := <-outcomes:
			if !ok {
				outcomes = nil
			}
		}
	}
}

func (s Service) initialSnapshots(runID, sessionID string) ([]Event, error) {
	messages, err := s.services.LoadRecentSessionMessages(sessionID, 20)
	if err != nil {
		return nil, err
	}

	run, state, err := s.services.LoadRunState(runID)
	if err != nil {
		return nil, err
	}

	return []Event{
		{
			Type:     "MESSAGES_SNAPSHOT",
			ThreadID: sessionID,
			RunID:    runID,
			Messages: toMessages(messages),
		},
		{
			Type:     "STATE_SNAPSHOT",
			ThreadID: sessionID,
			RunID:    runID,
			Snapshot: map[string]any{
				"runId":         run.ID,
				"sessionId":     run.SessionID,
				"status":        run.Status,
				"currentStepId": state.CurrentStepID,
				"turnCount":     state.TurnCount,
				"provider":      run.Provider,
				"model":         run.Model,
				"planMode":      run.PlanMode,
				"todos":         state.Todos,
			},
		},
	}, nil
}

func toMessages(messages []harnessruntime.SessionMessage) []Message {
	result := make([]Message, 0, len(messages))
	for _, message := range messages {
		result = append(result, Message{
			ID:      message.ID,
			Role:    string(message.Role),
			Content: message.Content,
		})
	}
	return result
}

func lastUserMessage(messages []InputMessage) (InputMessage, error) {
	for i := len(messages) - 1; i >= 0; i-- {
		if strings.EqualFold(strings.TrimSpace(messages[i].Role), "user") && strings.TrimSpace(messages[i].Content) != "" {
			return messages[i], nil
		}
	}
	return InputMessage{}, errors.New("request must include at least one non-empty user message")
}

type runOutcome struct {
	response service.RunResponse
	err      error
}

type channelObserver struct {
	events chan harnessruntime.Event
}

func newChannelObserver() *channelObserver {
	return &channelObserver{
		events: make(chan harnessruntime.Event, 128),
	}
}

func (o *channelObserver) OnRuntimeEvent(event harnessruntime.Event) {
	o.events <- event
}
