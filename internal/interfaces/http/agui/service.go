package agui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/huanglei214/agent-demo/internal/agent"
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
			go drainRunCompletion(observer.events, observer.answerStream, outcomeCh)
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
		close(observer.answerStream)
	}()

	var (
		sawTerminal              bool
		snapshotSent             bool
		streamingMessageID       string
		answerStreamStarted      bool
		answerStreamEnded        bool
		suppressAssistantMessage bool
		pendingTerminalEvents    []Event
		streamThreadID           = threadID
		streamRunID              string
		finalOutcome             *runOutcome
	)

	for observer.events != nil || observer.answerStream != nil || outcomeCh != nil {
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
			if shouldSuppressAssistantMessage(event, suppressAssistantMessage) {
				suppressAssistantMessage = false
				mapped = nil
			}
			if shouldDeferTerminalEvent(event, answerStreamStarted, answerStreamEnded) {
				pendingTerminalEvents = append([]Event(nil), mapped...)
				mapped = nil
			}
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
		case streamEvent, ok := <-observer.answerStream:
			if !ok {
				observer.answerStream = nil
				continue
			}
			if streamThreadID == "" {
				streamThreadID = streamEvent.SessionID
			}
			if streamRunID == "" {
				streamRunID = streamEvent.RunID
			}
			switch streamEvent.Type {
			case agent.AnswerStreamEventStart:
				if !answerStreamStarted {
					answerStreamStarted = true
					suppressAssistantMessage = true
					streamingMessageID = streamEvent.MessageID
					if err = writeStreamEvent(writer, Event{
						Type:      "TEXT_MESSAGE_START",
						ThreadID:  streamEvent.SessionID,
						RunID:     streamEvent.RunID,
						MessageID: streamEvent.MessageID,
						Role:      "assistant",
					}); err != nil {
						return err
					}
				}
			case agent.AnswerStreamEventDelta:
				if answerStreamStarted && streamEvent.MessageID == streamingMessageID {
					if err = writeStreamEvent(writer, Event{
						Type:      "TEXT_MESSAGE_CONTENT",
						ThreadID:  streamEvent.SessionID,
						RunID:     streamEvent.RunID,
						MessageID: streamEvent.MessageID,
						Delta:     streamEvent.Delta,
					}); err != nil {
						return err
					}
				}
			case agent.AnswerStreamEventCompleted:
				if answerStreamStarted && !answerStreamEnded && streamEvent.MessageID == streamingMessageID {
					answerStreamEnded = true
					if err = writeStreamEvent(writer, Event{
						Type:      "TEXT_MESSAGE_END",
						ThreadID:  streamEvent.SessionID,
						RunID:     streamEvent.RunID,
						MessageID: streamEvent.MessageID,
					}); err != nil {
						return err
					}
				}
			case agent.AnswerStreamEventFailed:
				// ignore for now
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

		if shouldFlushPendingTerminal(pendingTerminalEvents, answerStreamEnded, observer.answerStream == nil) {
			for _, item := range pendingTerminalEvents {
				if err = writeStreamEvent(writer, item); err != nil {
					return err
				}
			}
			pendingTerminalEvents = nil
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

func shouldSuppressAssistantMessage(event harnessruntime.Event, suppress bool) bool {
	return suppress && event.Type == "assistant.message"
}

func shouldDeferTerminalEvent(event harnessruntime.Event, answerStreamStarted, answerStreamEnded bool) bool {
	if !answerStreamStarted || answerStreamEnded {
		return false
	}
	return event.Type == "run.completed" || event.Type == "run.failed"
}

func shouldFlushPendingTerminal(events []Event, answerStreamEnded, answerStreamClosed bool) bool {
	if len(events) == 0 {
		return false
	}
	return answerStreamEnded || answerStreamClosed
}

func drainRunCompletion(events <-chan harnessruntime.Event, answerStream <-chan agent.AnswerStreamEvent, outcomes <-chan runOutcome) {
	for events != nil || answerStream != nil || outcomes != nil {
		select {
		case _, ok := <-events:
			if !ok {
				events = nil
			}
		case _, ok := <-answerStream:
			if !ok {
				answerStream = nil
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
	events       chan harnessruntime.Event
	answerStream chan agent.AnswerStreamEvent
}

func newChannelObserver() *channelObserver {
	return &channelObserver{
		events:       make(chan harnessruntime.Event, 128),
		answerStream: make(chan agent.AnswerStreamEvent, 128),
	}
}

func (o *channelObserver) OnRuntimeEvent(event harnessruntime.Event) {
	o.events <- event
}

func (o *channelObserver) OnAnswerStreamEvent(event agent.AnswerStreamEvent) {
	o.answerStream <- event
}
