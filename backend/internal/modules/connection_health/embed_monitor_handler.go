package connection_health

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"transithub/backend/internal/shared/authctx"
	"transithub/backend/internal/shared/httpjson"
)

func RegisterEmbedMonitorRoutes(mux *http.ServeMux, s *EmbedMonitorService) {
	// All admin routes also perform explicit auth/workspace checks, independent
	// of the outer server middleware. Public routes are read-only capabilities.
	for _, route := range []string{"GET /api/group-quality/board", "GET /api/group-quality/settings", "PUT /api/group-quality/settings", "POST /api/group-quality/rotate-token", "DELETE /api/group-quality/history", "GET /api/group-quality/groups", "POST /api/group-quality/groups/{id}/models", "POST /api/group-quality/monitors", "PUT /api/group-quality/monitors/{id}", "DELETE /api/group-quality/monitors/{id}", "POST /api/group-quality/monitors/{id}/run/{kind}", "GET /api/group-quality/samples/{id}"} {
		mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) { s.serveAdmin(w, r) })
	}
	mux.HandleFunc("GET /api/embed/group-quality", s.servePublic)
	mux.HandleFunc("GET /api/embed/group-quality/samples/{id}", s.servePublic)
}

func embedMonitorError(w http.ResponseWriter, err error) {
	var req requestError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		httpjson.WriteError(w, 404, "检测项或嵌入链接不存在")
	case errors.As(err, &req):
		httpjson.WriteError(w, 400, string(req))
	default:
		httpjson.WriteError(w, 500, "分组检测暂时不可用，请稍后重试")
	}
}

func (s *EmbedMonitorService) serveAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user, ok := authctx.UserID(r.Context())
	if !ok {
		httpjson.WriteError(w, 401, "auth.errors.unauthorized")
		return
	}
	workspace, err := s.health.currentAdminAccountID(r.Context(), user)
	if err != nil {
		writeError(w, err)
		return
	}
	ctx := r.Context()
	var out any
	decode := func(v any) bool {
		r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
		if e := httpjson.Decode(r, v); e != nil {
			httpjson.WriteError(w, 400, "配置格式无效")
			return false
		}
		return true
	}
	switch {
	case r.URL.Path == "/api/group-quality/history":
		var deleted int64
		deleted, err = s.repo.clearHistory(ctx, user, workspace)
		out = map[string]int64{"deleted": deleted}
	case r.URL.Path == "/api/group-quality/board":
		var cfg EmbedBoardSettings
		cfg, err = s.repo.settings(ctx, user, workspace)
		if err == nil {
			out, err = s.repo.board(ctx, user, workspace, cfg.Title, false)
		}
	case r.URL.Path == "/api/group-quality/settings":
		if r.Method == http.MethodGet {
			out, err = s.repo.settings(ctx, user, workspace)
		} else {
			var cfg EmbedBoardSettings
			if !decode(&cfg) {
				return
			}
			out, err = s.saveSettings(ctx, user, workspace, cfg, false)
		}
	case r.URL.Path == "/api/group-quality/rotate-token":
		out, err = s.saveSettings(ctx, user, workspace, EmbedBoardSettings{}, true)
	case r.URL.Path == "/api/group-quality/groups":
		groups, e := s.groups(ctx, user, workspace)
		err = e
		items := []map[string]string{}
		for _, g := range groups {
			items = append(items, map[string]string{"id": g.ID, "name": g.Name, "platform": g.Platform})
		}
		out = items
	case strings.HasSuffix(r.URL.Path, "/models"):
		m := EmbedMonitor{UserID: user, WorkspaceID: workspace, GroupID: r.PathValue("id")}
		cred, _, e := s.credential(ctx, m)
		err = e
		if err == nil {
			out, err = s.health.modelDiscovery.ListModels(ctx, cred.BaseURL, cred.Key)
		}
	case r.PathValue("kind") != "":
		err = s.repo.queue(ctx, user, workspace, r.PathValue("id"), r.PathValue("kind"))
		out = map[string]bool{"queued": true}
	case strings.HasPrefix(r.URL.Path, "/api/group-quality/samples/"):
		out, err = s.repo.detail(ctx, user, workspace, r.PathValue("id"))
	case r.Method == http.MethodDelete:
		var tag interface{ RowsAffected() int64 }
		tag, err = s.repo.db.Exec(ctx, `DELETE FROM embed_group_monitors WHERE id=$1 AND user_id=$2 AND admin_account_id=$3`, r.PathValue("id"), user, workspace)
		if err == nil && tag.RowsAffected() == 0 {
			err = pgx.ErrNoRows
		}
		out = map[string]bool{"deleted": true}
	default:
		var m EmbedMonitor
		if !decode(&m) {
			return
		}
		m.ID = r.PathValue("id")
		out, err = s.save(ctx, user, workspace, m)
	}
	if err != nil {
		embedMonitorError(w, err)
		return
	}
	status := http.StatusOK
	if r.PathValue("kind") != "" {
		status = http.StatusAccepted
	}
	httpjson.Write(w, status, out)
}

func (s *EmbedMonitorService) servePublic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	parts := strings.Fields(r.Header.Get("Authorization"))
	token := ""
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		token = parts[1]
	}
	user, workspace, title, err := s.repo.publicScope(r.Context(), token)
	if err != nil {
		httpjson.WriteError(w, 404, "嵌入链接无效或已关闭")
		return
	}
	var out any
	if r.PathValue("id") != "" {
		var sample EmbedSample
		sample, err = s.repo.publicDetail(r.Context(), user, workspace, r.PathValue("id"))
		// Public reports omit upstream diagnostic text; artwork and verdicts are
		// explicitly published through the enabled board.
		sample.ErrorDetail = ""
		out = sample
	} else {
		out, err = s.repo.board(r.Context(), user, workspace, title, true)
	}
	if err != nil {
		embedMonitorError(w, err)
		return
	}
	httpjson.Write(w, 200, out)
}
