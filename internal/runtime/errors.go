package runtime

import "fmt"

var (
	ErrUnsupportedProvider = fmt.Errorf("unsupported model provider")
	ErrRunNotFound         = fmt.Errorf("run not found")
	ErrSessionNotFound     = fmt.Errorf("session not found")
)

type UnsupportedProviderError struct {
	Provider string
}

func (e *UnsupportedProviderError) Error() string {
	return fmt.Sprintf("unsupported model provider: %s", e.Provider)
}

func (e *UnsupportedProviderError) Unwrap() error {
	return ErrUnsupportedProvider
}

func NewUnsupportedProviderError(provider string) error {
	return &UnsupportedProviderError{Provider: provider}
}

type RunNotFoundError struct {
	RunID string
	Cause error
}

func (e *RunNotFoundError) Error() string {
	return fmt.Sprintf("run not found: %s", e.RunID)
}

func (e *RunNotFoundError) Unwrap() []error {
	if e.Cause == nil {
		return []error{ErrRunNotFound}
	}
	return []error{ErrRunNotFound, e.Cause}
}

func NewRunNotFoundError(runID string, cause error) error {
	return &RunNotFoundError{RunID: runID, Cause: cause}
}

type SessionNotFoundError struct {
	SessionID string
	Cause     error
}

func (e *SessionNotFoundError) Error() string {
	return fmt.Sprintf("session not found: %s", e.SessionID)
}

func (e *SessionNotFoundError) Unwrap() []error {
	if e.Cause == nil {
		return []error{ErrSessionNotFound}
	}
	return []error{ErrSessionNotFound, e.Cause}
}

func NewSessionNotFoundError(sessionID string, cause error) error {
	return &SessionNotFoundError{SessionID: sessionID, Cause: cause}
}
