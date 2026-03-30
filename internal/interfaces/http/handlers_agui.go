package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/huanglei214/agent-demo/internal/interfaces/http/agui"
)

func (s Server) handleAGUIChat(w http.ResponseWriter, r *http.Request) {
	var req agui.ChatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	log.Printf(
		"agui chat start thread_id=%q provider=%q model=%q workspace=%q message_count=%d",
		req.ThreadID,
		req.State.Provider,
		req.State.Model,
		req.State.Workspace,
		len(req.Messages),
	)

	writer, err := agui.NewSSEWriter(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "stream_unsupported", err.Error())
		return
	}
	writer.Open()

	service := agui.NewService(s.services)
	if err := service.StreamChat(r.Context(), req, writer); err != nil {
		log.Printf(
			"agui chat failed thread_id=%q provider=%q model=%q workspace=%q error=%v",
			req.ThreadID,
			req.State.Provider,
			req.State.Model,
			req.State.Workspace,
			err,
		)
		if !errors.Is(err, agui.ErrStreamUnwritable) {
			_ = writer.Write(agui.Event{
				Type:  "RUN_ERROR",
				Error: err.Error(),
			})
		}
		return
	}

	log.Printf(
		"agui chat finished thread_id=%q provider=%q model=%q workspace=%q",
		req.ThreadID,
		req.State.Provider,
		req.State.Model,
		req.State.Workspace,
	)
}
