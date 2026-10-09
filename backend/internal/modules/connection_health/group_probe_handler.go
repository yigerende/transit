package connection_health

import (
	"errors"
	"io"
	"net/http"

	"transithub/backend/internal/shared/authctx"
	"transithub/backend/internal/shared/httpjson"
)

func (h *Handler) groupProbeConfiguration(w http.ResponseWriter, r *http.Request) {
	userID, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	result, err := h.service.GroupProbeConfiguration(r.Context(), userID, r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}

func (h *Handler) saveGroupProbeConfiguration(w http.ResponseWriter, r *http.Request) {
	userID, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	var input GroupProbeConfigInput
	if err := httpjson.Decode(r, &input); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, ErrorRequest)
		return
	}
	result, err := h.service.SaveGroupProbeConfiguration(r.Context(), userID, r.PathValue("id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}

func (h *Handler) prepareGroupProbe(w http.ResponseWriter, r *http.Request) {
	userID, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	var input struct {
		Key *string `json:"key"`
	}
	if err := httpjson.Decode(r, &input); err != nil && !errors.Is(err, io.EOF) {
		httpjson.WriteError(w, http.StatusBadRequest, ErrorRequest)
		return
	}
	result, err := h.service.PrepareGroupProbe(r.Context(), userID, r.PathValue("id"), input.Key)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}

func (h *Handler) probeAdminGroup(w http.ResponseWriter, r *http.Request) {
	userID, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	var input struct {
		Model string `json:"model"`
	}
	if err := httpjson.Decode(r, &input); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, ErrorRequest)
		return
	}
	result, err := h.service.ProbeAdminGroup(r.Context(), userID, r.PathValue("id"), input.Model)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}
