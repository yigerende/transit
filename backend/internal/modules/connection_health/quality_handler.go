package connection_health

import (
	"net/http"
	"transithub/backend/internal/shared/authctx"
	"transithub/backend/internal/shared/httpjson"
)

func (h *Handler) qualityHistory(w http.ResponseWriter, r *http.Request) {
	user, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.service.QualityHistory(r.Context(), user, r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}

func (h *Handler) qualityHistoryDetail(w http.ResponseWriter, r *http.Request) {
	user, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	result, err := h.service.QualityHistoryDetail(r.Context(), user, r.PathValue("id"), r.PathValue("sampleId"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}

func (h *Handler) probeChannelQuality(w http.ResponseWriter, r *http.Request) {
	user, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var input struct {
		GroupID string `json:"groupId"`
		Method  string `json:"method"`
	}
	if httpjson.Decode(r, &input) != nil {
		httpjson.WriteError(w, http.StatusBadRequest, ErrorRequest)
		return
	}
	result, err := h.service.ProbeChannelQuality(r.Context(), user, r.PathValue("id"), input.GroupID, input.Method)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}

func (h *Handler) qualitySettings(w http.ResponseWriter, r *http.Request) {
	user, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	result, err := h.service.QualityConfiguration(r.Context(), user)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}
func (h *Handler) saveQualitySettings(w http.ResponseWriter, r *http.Request) {
	user, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var q QualitySettings
	if httpjson.Decode(r, &q) != nil {
		httpjson.WriteError(w, http.StatusBadRequest, qualityPrefix+"invalidConfig")
		return
	}
	result, err := h.service.SaveQualityConfiguration(r.Context(), user, q)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}
func (h *Handler) setGroupQuality(w http.ResponseWriter, r *http.Request) {
	user, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	var input struct {
		Enabled bool `json:"enabled"`
	}
	if httpjson.Decode(r, &input) != nil {
		httpjson.WriteError(w, http.StatusBadRequest, ErrorRequest)
		return
	}
	result, err := h.service.SetGroupQuality(r.Context(), user, r.PathValue("id"), input.Enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}

func (h *Handler) setChannelQuality(w http.ResponseWriter, r *http.Request) {
	user, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, http.StatusUnauthorized, "auth.errors.unauthorized")
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if httpjson.Decode(r, &input) != nil || input.Enabled == nil {
		httpjson.WriteError(w, http.StatusBadRequest, ErrorRequest)
		return
	}
	result, err := h.service.SetChannelQuality(r.Context(), user, r.PathValue("id"), *input.Enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, result)
}
