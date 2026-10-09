package connection_health

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"sort"
	"strings"
	"time"
	"transithub/backend/internal/modules/upstream"
)

func (s *Service) QualityConfiguration(ctx context.Context, user string) (QualitySettings, error) {
	workspace, err := s.currentAdminAccountID(ctx, user)
	if err != nil {
		return QualitySettings{}, err
	}
	return s.qualityRepo.GetQualitySettings(ctx, user, workspace)
}
func (s *Service) SaveQualityConfiguration(ctx context.Context, user string, q QualitySettings) (QualitySettings, error) {
	if err := q.validate(); err != nil {
		return QualitySettings{}, err
	}
	workspace, err := s.currentAdminAccountID(ctx, user)
	if err != nil {
		return QualitySettings{}, err
	}
	// Identical saves preserve the revision, question position and streaks.
	q.Revision = ""
	raw, _ := json.Marshal(q)
	previous, err := s.qualityRepo.GetQualitySettings(ctx, user, workspace)
	if err != nil {
		return QualitySettings{}, err
	}
	revision := previous.Revision
	previous.Revision = ""
	old, _ := json.Marshal(previous)
	if revision != "" && bytes.Equal(raw, old) {
		q.Revision = revision
	} else {
		q.Revision, err = newID()
		if err != nil {
			return QualitySettings{}, err
		}
	}
	return q, s.qualityRepo.SaveQualitySettings(ctx, user, workspace, q)
}
func (s *Service) SetGroupQuality(ctx context.Context, user, groupID string, enabled bool) (QualityGroup, error) {
	workspace, err := s.currentAdminAccountID(ctx, user)
	if err != nil {
		return QualityGroup{}, err
	}
	q, err := s.qualityRepo.GetQualitySettings(ctx, user, workspace)
	if err != nil {
		return QualityGroup{}, err
	}
	if enabled {
		if !q.Enabled || q.Revision == "" || len(q.activeQuestions()) == 0 {
			return QualityGroup{}, requestError(qualityPrefix + "configureFirst")
		}
		session, err := s.mySites.RequireSession(ctx, user, workspace)
		if err != nil {
			return QualityGroup{}, requestError(ErrorRequest)
		}
		groups, err := s.platformGroups.FetchAdminAllGroups(session)
		if err != nil {
			return QualityGroup{}, requestError(ErrorRequest)
		}
		found := false
		for _, g := range groups {
			if g.ID == groupID {
				found = true
				break
			}
		}
		if !found {
			return QualityGroup{}, requestError(ErrorNotFound)
		}
	}
	return QualityGroup{GroupID: groupID, Enabled: enabled, GlobalEnabled: q.Enabled}, s.qualityRepo.SetQualityGroup(ctx, user, workspace, groupID, enabled)
}

func (s *Service) attachQuality(ctx context.Context, user, workspace string, groups []AdminGroupHealth) {
	if s.qualityRepo == nil {
		return
	}
	q, err := s.qualityRepo.GetQualitySettings(ctx, user, workspace)
	if err != nil {
		s.qualityUnavailable(groups)
		return
	}
	switches, err := s.qualityRepo.ListQualityGroups(ctx, user, workspace)
	if err != nil {
		s.qualityUnavailable(groups)
		return
	}
	enabled := map[string]bool{}
	for _, g := range switches {
		enabled[g.GroupID] = g.Enabled
	}
	states, err := s.qualityRepo.ListQualityStates(ctx, user, workspace)
	if err != nil {
		s.qualityUnavailable(groups)
		return
	}
	byTarget := map[string]QualityState{}
	for _, state := range states {
		if state.Revision == q.Revision {
			byTarget[state.TargetID] = state
		}
	}
	seen := map[string]bool{}
	targets := []string{}
	for _, g := range groups {
		for _, a := range g.Accounts {
			if !seen[a.TargetID] {
				seen[a.TargetID] = true
				targets = append(targets, a.TargetID)
			}
		}
	}
	history, err := s.qualityRepo.ListQualityHistory(ctx, user, workspace, targets, min(q.HistoryLimit, 100))
	if err != nil {
		s.qualityUnavailable(groups)
		return
	}
	byHistory := map[string][]QualitySample{}
	for _, sample := range history {
		byHistory[sample.TargetID] = append(byHistory[sample.TargetID], sample)
	}
	for i := range groups {
		g := &groups[i]
		g.Quality = &QualityGroup{GroupID: g.ID, Enabled: enabled[g.ID], GlobalEnabled: q.Enabled}
		for j := range g.Accounts {
			a := &g.Accounts[j]
			a.QualityHistory = append([]QualitySample{}, byHistory[a.TargetID]...)
			if state, ok := byTarget[a.TargetID]; ok {
				a.QualityState = &state
			}
		}
	}
}
func (s *Service) qualityUnavailable(groups []AdminGroupHealth) {
	for i := range groups {
		groups[i].Quality = &QualityGroup{GroupID: groups[i].ID, ErrorKey: qualityPrefix + "historyUnavailable"}
	}
}

type qualityCandidate struct {
	account  upstream.AdminGroupAccountInfo
	groups   []string
	targetID string
	due      time.Time
}

func (s *Service) StartQualityScheduler(ctx context.Context) {
	if s.qualityRepo == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		active := map[string]bool{}
		lastStarted := map[string]time.Time{}
		done := make(chan string, 4)
		tokens := make(chan struct{}, 32)
		start := func() {
			scopes, err := s.qualityRepo.ListQualityScopes(ctx)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("[quality] loading scopes failed")
				}
				return
			}
			sort.SliceStable(scopes, func(i, j int) bool {
				return lastStarted[scopes[i].UserID+"|"+scopes[i].WorkspaceID].Before(lastStarted[scopes[j].UserID+"|"+scopes[j].WorkspaceID])
			})
			for _, scope := range scopes {
				key := scope.UserID + "|" + scope.WorkspaceID
				if active[key] || len(active) >= 4 {
					continue
				}
				active[key] = true
				lastStarted[key] = time.Now()
				go func(scope QualityScope, key string) {
					defer func() {
						if recover() != nil {
							log.Printf("[quality] worker panic recovered")
						}
						done <- key
					}()
					s.runQualityScope(ctx, scope, tokens)
				}(scope, key)
			}
		}
		start()
		for {
			select {
			case <-ctx.Done():
				return
			case key := <-done:
				delete(active, key)
			case <-ticker.C:
				start()
			}
		}
	}()
}

func (s *Service) runQualityScope(ctx context.Context, scope QualityScope, tokens chan struct{}) {
	// A shared workspace lease prevents two server instances from running the
	// same channel concurrently. It is separate from normal health policies.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	release, err := s.repo.AcquireTargetLease(ctx, "quality:"+scope.UserID+":"+scope.WorkspaceID)
	if err != nil {
		return
	}
	defer release()
	q, err := s.qualityRepo.GetQualitySettings(ctx, scope.UserID, scope.WorkspaceID)
	if err != nil || !q.Enabled || q.validate() != nil {
		return
	}
	switches, err := s.qualityRepo.ListQualityGroups(ctx, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return
	}
	enabled := map[string]bool{}
	for _, g := range switches {
		if g.Enabled {
			enabled[g.GroupID] = true
		}
	}
	if len(enabled) == 0 {
		return
	}
	session, err := s.mySites.RequireSession(ctx, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return
	}
	groups, err := s.platformGroups.FetchAdminAllGroups(session)
	if err != nil {
		return
	}
	states, err := s.qualityRepo.ListQualityStates(ctx, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return
	}
	byState := map[string]QualityState{}
	for _, st := range states {
		if st.Revision == q.Revision {
			byState[st.TargetID] = st
		}
	}
	byTarget := map[string]*qualityCandidate{}
	for _, group := range groups {
		if !enabled[group.ID] || ctx.Err() != nil {
			continue
		}
		accounts, err := s.platformGroups.ListAdminGroupAccounts(session, group)
		if err != nil {
			continue
		}
		for _, a := range accounts {
			target := buildTargetID(string(session.Platform), scope.WorkspaceID, a.ID)
			if c := byTarget[target]; c != nil {
				c.groups = append(c.groups, group.ID)
			} else {
				byTarget[target] = &qualityCandidate{account: a, groups: []string{group.ID}, targetID: target, due: byState[target].NextProbeAt}
			}
		}
	}
	candidates := []qualityCandidate{}
	now := time.Now()
	for _, c := range byTarget {
		if !c.due.After(now) {
			candidates = append(candidates, *c)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].due.Equal(candidates[j].due) {
			return candidates[i].targetID < candidates[j].targetID
		}
		return candidates[i].due.Before(candidates[j].due)
	})
	// Bound a batch, preserving oldest due order for the following scan.
	if len(candidates) > max(32, q.Concurrency) {
		candidates = candidates[:max(32, q.Concurrency)]
	}
	jobs := make(chan qualityCandidate)
	finished := make(chan struct{}, q.Concurrency)
	for i := 0; i < q.Concurrency; i++ {
		go func() {
			defer func() {
				if recover() != nil {
					log.Printf("[quality] probe panic recovered")
				}
				finished <- struct{}{}
			}()
			for c := range jobs {
				select {
				case tokens <- struct{}{}:
				case <-ctx.Done():
					return
				}
				func() {
					defer func() {
						<-tokens
						if recover() != nil {
							log.Printf("[quality] probe panic recovered")
						}
					}()
					s.runQualityCandidate(ctx, scope, session, q, c, byState[c.targetID])
				}()
			}
		}()
	}
	for _, c := range candidates {
		select {
		case jobs <- c:
		case <-ctx.Done():
		}
	}
	close(jobs)
	for i := 0; i < q.Concurrency; i++ {
		<-finished
	}
}

func (s *Service) runQualityCandidate(ctx context.Context, scope QualityScope, session upstream.Session, q QualitySettings, c qualityCandidate, state QualityState) {
	current, err := s.qualityRepo.GetQualitySettings(ctx, scope.UserID, scope.WorkspaceID)
	if err != nil || !current.Enabled || current.Revision != q.Revision {
		return
	}
	switches, err := s.qualityRepo.ListQualityGroups(ctx, scope.UserID, scope.WorkspaceID)
	if err != nil {
		return
	}
	allowed := map[string]bool{}
	for _, g := range switches {
		if g.Enabled {
			allowed[g.GroupID] = true
		}
	}
	// Refresh the group membership before each real request, including queued jobs.
	matched := false
	for _, id := range c.groups {
		if !allowed[id] {
			continue
		}
		accounts, err := s.platformGroups.ListAdminGroupAccounts(session, upstream.AdminGroupInfo{ID: id, Name: id})
		if err != nil {
			continue
		}
		for _, a := range accounts {
			if a.ID == c.account.ID {
				c.account = a
				c.groups = []string{id}
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}
	if !matched || ctx.Err() != nil {
		return
	}
	active := q.activeQuestions()
	if len(active) == 0 {
		return
	}
	question := active[0]
	for _, v := range active {
		if v.ID == state.NextQuestionID {
			question = v
			break
		}
	}
	id, err := newID()
	if err != nil {
		return
	}
	sample := QualitySample{ID: id, TargetID: c.targetID, Model: q.Model, QuestionID: question.ID, QuestionName: question.Name, ExpectedAnswer: question.Answer, MatchMode: question.MatchMode, MaxDurationMS: question.MaxDurationMS}
	cred, err := s.platformGroups.ResolveProbeCredential(session, c.account)
	if err != nil {
		sample.ErrorKey = reasonToErrorKey(upstream.ProbeCredentialReason(err))
	} else {
		outcome := s.qualityRunner.ProbeQuality(ctx, cred, c.account.Platform, q, question)
		sample.Answer = outcome.Answer
		sample.DurationMS = outcome.DurationMS
		sample.ErrorKey = outcome.ErrorKey
	}
	if ctx.Err() != nil {
		return
	}
	sample.CreatedAt = time.Now()
	state = applyQualitySample(state, q, question, sample)
	state.Latest.Answer = truncate(strings.TrimSpace(state.Latest.Answer), 4000)
	if _, err := s.qualityRepo.SaveQualityResult(ctx, scope.UserID, scope.WorkspaceID, c.groups, q, state); err != nil {
		log.Printf("[quality] saving result failed")
	}
}
