package connection_health

import (
	"context"
	"errors"
	"log"
	"time"

	"transithub/backend/internal/modules/upstream"
)

const (
	schedulerTickInterval      = time.Second
	schedulerInventoryInterval = 30 * time.Second
	channelProbeConcurrency    = 5
	maxJobsPerTick             = 100
	globalProbeConcurrency     = 5
	perSiteProbeConcurrency    = 2
)

// adminProbeJob 是调度器一轮扫描出的、针对一个独立探活目标的到期任务集合。
// 一个目标下可能有多个到期模型，共用一次凭据解析（避免重复命中受保护的 key 接口）。
type adminProbeJob struct {
	userID         string
	adminAccountID string
	session        upstream.Session
	target         AdminProbeTarget
	account        upstream.AdminGroupAccountInfo
	models         []probeModelSpec
	dueSpecs       []probeModelSpec
	groups         []upstream.AdminGroupInfo
}

type probePolicyEventGroup struct {
	resolved       bool
	adminGroupID   string
	adminGroupName string
}

type adminInventoryGroup struct {
	group    upstream.AdminGroupInfo
	accounts []upstream.AdminGroupAccountInfo
	err      error
}

type adminWorkspaceInventory struct {
	session upstream.Session
	groups  []adminInventoryGroup
}

type adminInventoryCacheEntry struct {
	inventory *adminWorkspaceInventory
	err       error
}

type adminInventoryCache map[string]adminInventoryCacheEntry

func (s *Service) loadAdminInventory(ctx context.Context, userID string, adminAccountID string, cache adminInventoryCache) (*adminWorkspaceInventory, error) {
	key := userID + "|" + adminAccountID
	if cached, ok := cache[key]; ok {
		return cached.inventory, cached.err
	}
	session, err := s.mySites.RequireSession(ctx, userID, adminAccountID)
	if err != nil {
		cache[key] = adminInventoryCacheEntry{err: err}
		return nil, err
	}
	groups, err := s.platformGroups.FetchAdminAllGroups(session)
	if err != nil {
		cache[key] = adminInventoryCacheEntry{err: err}
		return nil, err
	}
	inventory := &adminWorkspaceInventory{session: session, groups: make([]adminInventoryGroup, 0, len(groups))}
	for _, group := range groups {
		accounts, accountsErr := s.platformGroups.ListAdminGroupAccounts(session, group)
		inventory.groups = append(inventory.groups, adminInventoryGroup{group: group, accounts: accounts, err: accountsErr})
	}
	cache[key] = adminInventoryCacheEntry{inventory: inventory}
	return inventory, nil
}

// Inventory and priority synchronization run independently of the one-second due
// scan. A slow upstream read or probe must not hold up unrelated due channels.
func (s *Service) StartScheduler(ctx context.Context) {
	go s.runProbeScheduler(ctx)
}

func (s *Service) runProbeScheduler(ctx context.Context) {
	if s.platformGroups == nil {
		return
	}
	ticker := time.NewTicker(schedulerTickInterval)
	defer ticker.Stop()
	inventoryTicker := time.NewTicker(schedulerInventoryInterval)
	defer inventoryTicker.Stop()
	inventories := make(chan adminInventoryCache, 1)
	done := make(chan adminProbeJob, channelProbeConcurrency)
	active := map[string]bool{}
	workspaceActive := map[string]int{}
	var cache adminInventoryCache
	refreshing := false
	refresh := func() {
		if refreshing {
			return
		}
		refreshing = true
		go func() {
			inventory := s.refreshSchedulerInventory(ctx)
			select {
			case inventories <- inventory:
			case <-ctx.Done():
			}
		}()
	}
	dispatch := func() {
		defer func() {
			if recover() != nil {
				log.Printf("[connection-health] due scan panic recovered")
			}
		}()
		if cache == nil || len(active) >= channelProbeConcurrency {
			return
		}
		for _, job := range s.collectCachedProbeJobs(ctx, cache, active) {
			key := job.userID + "|" + job.target.TargetID
			workspace := job.userID + "|" + job.adminAccountID
			if active[key] || workspaceActive[workspace] >= channelProbeConcurrency {
				continue
			}
			if len(active) >= channelProbeConcurrency || ctx.Err() != nil {
				break
			}
			active[key] = true
			workspaceActive[workspace]++
			go func(job adminProbeJob) {
				defer func() { done <- job }()
				s.runAdminProbeJob(ctx, job)
			}(job)
		}
	}
	refresh()
	for {
		select {
		case <-ctx.Done():
			return
		case next := <-inventories:
			refreshing = false
			cache = next
			dispatch()
		case job := <-done:
			delete(active, job.userID+"|"+job.target.TargetID)
			workspaceActive[job.userID+"|"+job.adminAccountID]--
		case <-ticker.C:
			dispatch()
		case <-inventoryTicker.C:
			refresh()
		}
	}
}

// Only upstream membership/session snapshots are cached. Policy switches,
// selections, thresholds, budgets and last-probe timestamps are read each scan.
func (s *Service) collectCachedProbeJobs(ctx context.Context, cache adminInventoryCache, active map[string]bool) []adminProbeJob {
	policies, err := s.repo.ListEnabledPolicies(ctx)
	if err != nil {
		return nil
	}
	assignments, err := s.repo.ListAllPolicyAssignments(ctx)
	if err != nil {
		return nil
	}
	groups, err := s.repo.ListAllGroupPolicyAssignments(ctx)
	if err != nil {
		return nil
	}
	exclusions, err := s.repo.ListAllGroupTargetExclusions(ctx)
	if err != nil {
		return nil
	}
	// A newly added workspace waits for inventory refresh, never a network read
	// on the scheduling goroutine. Copy because loadAdminInventory may cache errors.
	ready := make(adminInventoryCache, len(cache))
	for key, entry := range cache {
		ready[key] = entry
	}
	for _, policy := range policies {
		key := policy.UserID + "|" + policy.AdminAccountID
		if _, ok := ready[key]; !ok {
			ready[key] = adminInventoryCacheEntry{err: errors.New("inventory pending")}
		}
	}
	return s.collectAdminProbeJobsWithGroupsAndCache(ctx, policies, assignments, groups, exclusions, ready, active)
}

func (s *Service) refreshSchedulerInventory(ctx context.Context) (cache adminInventoryCache) {
	defer func() {
		if recover() != nil {
			cache = nil
			log.Printf("[connection-health] inventory refresh panic recovered")
		}
	}()
	policies, err := s.repo.ListEnabledPolicies(ctx)
	if err != nil {
		return nil
	}
	assignments, err := s.repo.ListAllPolicyAssignments(ctx)
	if err != nil {
		return nil
	}
	groups, err := s.repo.ListAllGroupPolicyAssignments(ctx)
	if err != nil {
		return nil
	}
	exclusions, err := s.repo.ListAllGroupTargetExclusions(ctx)
	if err != nil {
		return nil
	}
	priorityStates, err := s.repo.ListAllPrioritySyncStates(ctx)
	if err != nil {
		return nil
	}
	actionStates, err := s.repo.ListAllTargetActionStates(ctx)
	if err != nil {
		return nil
	}
	cache = make(adminInventoryCache)
	assigned := map[string]bool{}
	for _, a := range assignments {
		assigned[a.UserID+"|"+a.AdminAccountID] = true
	}
	for _, a := range groups {
		assigned[a.UserID+"|"+a.AdminAccountID] = true
	}
	release, acquired, err := s.repo.TryAcquireSchedulerLease(ctx)
	maintenance := err == nil && acquired
	if maintenance {
		defer release()
		s.syncMultiplierPrioritiesWithCache(ctx, policies, assignments, groups, exclusions, priorityStates, cache)
	}
	// Fill any remaining probe inventory after priority work, reusing its fresh snapshot.
	for _, policy := range policies {
		if ctx.Err() != nil {
			return nil
		}
		if !assigned[policy.UserID+"|"+policy.AdminAccountID] || !hasEnabledModelTarget([]Policy{policy}) {
			continue
		}
		_, _ = s.loadAdminInventory(ctx, policy.UserID, policy.AdminAccountID, cache)
	}
	if maintenance {
		s.restoreUnmanagedTargetActions(ctx, policies, assignments, groups, exclusions, actionStates, cache)
	}
	return cache
}

// runAdminProbeJob 处理单个目标的到期任务：先解析一次凭据；凭据不可用时对每个到期模型记录
// 一次「不可探活」事件并回填 last_probe_at（不驱动状态机、不计入探活预算），
// 凭据可用时逐个模型执行独立探活。
func (s *Service) runAdminProbeJob(ctx context.Context, j adminProbeJob) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[connection-health] admin probe goroutine panic recovered target_id=%s: %v", j.target.TargetID, r)
		}
	}()
	release, err := s.repo.AcquireTargetLease(ctx, j.target.TargetID)
	if err != nil {
		log.Printf("[connection-health] acquire target lease failed target_id=%s err=%v", j.target.TargetID, err)
		return
	}
	defer release()
	// Another instance or manual probe may have completed while this worker waited.
	// Re-read selection and policy so queued work cannot bypass a recent edit.
	j.models, err = s.currentScheduledModels(ctx, j)
	if err != nil {
		return
	}
	queued := map[string]bool{}
	for _, spec := range j.dueSpecs {
		queued[spec.modelName] = true
	}
	j.dueSpecs = nil
	for _, spec := range j.models {
		if queued[spec.modelName] && s.isDue(ctx, j.target.TargetID, spec.modelName, spec.policy, time.Now()) {
			j.dueSpecs = append(j.dueSpecs, spec)
		}
	}
	if len(j.dueSpecs) == 0 || ctx.Err() != nil {
		return
	}

	var cred upstream.ProbeCredential
	if probeSpecsNeedCredentials(j.dueSpecs) {
		cred, err = s.platformGroups.ResolveProbeCredential(j.session, j.account)
		if err != nil {
			direct, native := []probeModelSpec{}, []probeModelSpec{}
			for _, spec := range j.dueSpecs {
				if normalizeProbeMode(spec.policy.ProbeMode) == ProbeModeSub2API {
					native = append(native, spec)
				} else {
					direct = append(direct, spec)
				}
			}
			s.recordTargetCredentialUnavailable(ctx, j.userID, j.adminAccountID, j.target, direct, upstream.ProbeCredentialReason(err))
			j.dueSpecs = native
		}
	}
	results := make([]targetProbeResult, 0, len(j.dueSpecs))
	for _, spec := range j.dueSpecs {
		result, err := s.probeTargetOnce(ctx, j.userID, j.adminAccountID, j.target, cred, spec, j.session)
		if err != nil {
			log.Printf("[connection-health] scheduled target probe failed target_id=%s model=%s err=%v", j.target.TargetID, spec.modelName, err)
			continue
		}
		if result != nil {
			results = append(results, *result)
		}
	}
	s.refreshProbeActionTarget(&j)
	s.finishTargetProbeBatch(ctx, j.userID, j.adminAccountID, j.session, j.target, j.models, results)
}

// Refresh upstream status before conflict detection or remote actions.
func (s *Service) refreshProbeActionTarget(j *adminProbeJob) {
	if hasRemoteActionModel(j.models) {
		fresh := false
		for _, group := range j.groups {
			accounts, readErr := s.platformGroups.ListAdminGroupAccounts(j.session, group)
			if readErr != nil {
				continue
			}
			for _, account := range accounts {
				if account.ID == j.target.AccountID {
					j.target.AccountStatus, j.target.AccountWeight = account.Status, cloneIntPointer(account.Weight)
					fresh = true
					break
				}
			}
			if fresh {
				break
			}
		}
		if !fresh {
			for i := range j.models {
				j.models[i].policy.AutoRemoteActionEnabled = false
			}
		}
	}
}

// recordTargetCredentialUnavailable 在凭据解析失败时，对每个到期模型回填 last_probe_at（按探活
// 间隔，避免每次扫描都命中受保护的 key/导出接口）并记录一条 unsupported 事件，
// 事件 error_key 为脱敏 reason。不驱动状态机、不计入探活预算。
func (s *Service) recordTargetCredentialUnavailable(ctx context.Context, userID string, adminAccountID string, target AdminProbeTarget, specs []probeModelSpec, reason string) {
	now := time.Now()
	for _, spec := range specs {
		current, err := s.repo.GetState(ctx, target.TargetID, spec.modelName)
		if err != nil {
			log.Printf("[connection-health] get target state failed target_id=%s model=%s err=%v", target.TargetID, spec.modelName, err)
			continue
		}
		var next ConnectionHealthState
		if current == nil {
			next = defaultTargetState(userID, adminAccountID, targetForProbeSpec(target, spec), spec.modelName)
		} else {
			next = *current
		}
		setTargetStateSource(&next, targetForProbeSpec(target, spec))
		next = stateWithoutSuspension(next, spec.policy)
		next.LastProbeAt = &now
		next.LastLatencyMs = nil
		next.LastErrorKey = reason
		next.LastErrorDetail = ""
		// LastRemoteAction 也是旧版本判断「该上游状态是否由健康模块接管」的兼容证据。
		// 凭据暂时不可用只更新探活错误，不能抹掉此前成功执行的远端动作。
		if err := s.repo.UpsertState(ctx, next); err != nil {
			log.Printf("[connection-health] upsert unavailable target state failed target_id=%s model=%s err=%v", target.TargetID, spec.modelName, err)
			continue
		}
		eventTarget := targetForProbeSpec(target, spec)
		s.recordTargetEvent(ctx, userID, adminAccountID, eventTarget, spec.policy.ID, spec.modelName, string(ResultUnsupported), string(next.State), string(next.State), nil, reason, "", "", false, spec.policy.ProbeMode)
	}
}

// collectAdminProbeJobs 按 workspace 生成独立探活目标，并挑出到期的 (target, model) 任务。
// 调度器用 context.Background() 启动，没有请求态「当前 workspace」，必须用策略自带的
// userID + adminAccountID 复合键读取会话与分组，缓存也用复合键，避免多 workspace 串台。
//
// assignments 是全部 workspace 的「target 显式分配策略」关系：只有分配了至少一条已启用策略的
// target 才会被本函数处理；未分配的 target 不解析凭据、不计入 dueSpecs、不生成任何 job。
func (s *Service) collectAdminProbeJobs(ctx context.Context, policies []Policy, assignments []PolicyAssignment) []adminProbeJob {
	return s.collectAdminProbeJobsWithGroups(ctx, policies, assignments, nil, nil)
}

func (s *Service) collectAdminProbeJobsWithGroups(ctx context.Context, policies []Policy, assignments []PolicyAssignment, groupAssignments []GroupPolicyAssignment, exclusions []GroupTargetExclusion) []adminProbeJob {
	return s.collectAdminProbeJobsWithGroupsAndCache(ctx, policies, assignments, groupAssignments, exclusions, make(adminInventoryCache))
}

func (s *Service) collectAdminProbeJobsWithGroupsAndCache(ctx context.Context, policies []Policy, assignments []PolicyAssignment, groupAssignments []GroupPolicyAssignment, exclusions []GroupTargetExclusion, inventoryCache adminInventoryCache, skipTargets ...map[string]bool) []adminProbeJob {
	// 按 workspace 归拢策略。
	type workspace struct {
		userID         string
		adminAccountID string
		policies       []Policy
	}
	order := make([]string, 0)
	byWorkspace := make(map[string]*workspace)
	for _, p := range policies {
		key := p.UserID + "|" + p.AdminAccountID
		ws, ok := byWorkspace[key]
		if !ok {
			ws = &workspace{userID: p.UserID, adminAccountID: p.AdminAccountID}
			byWorkspace[key] = ws
			order = append(order, key)
		}
		ws.policies = append(ws.policies, p)
	}

	// assignedByWorkspace: wsKey -> targetId -> 该 target 已分配且已启用的策略列表。
	// 分配指向的策略如果已被禁用/删除（不在 policies/policyByID 中），对应分配行会被忽略，
	// 相当于该 target 暂时没有生效的分配。
	assignedByWorkspace := assignedEnabledPoliciesByTarget(policies, assignments)
	assignedGroupsByWorkspace := assignedEnabledPoliciesByGroup(policies, groupAssignments)
	excludedByWorkspace := groupTargetExclusionIndex(exclusions)

	jobs := make([]adminProbeJob, 0, maxJobsPerTick)
	modelBudget := maxJobsPerTick
	budgetUsage := make(map[string]int)
	budgetLoaded := make(map[string]bool)
	dayStart := probeBudgetDayStart(time.Now())

	for _, key := range order {
		if modelBudget <= 0 {
			break
		}
		ws := byWorkspace[key]
		// 该 workspace 下没有任何 target 被分配过策略：直接跳过，不建 session、不拉分组/账号，
		// 避免为完全没有分配关系的 workspace 发起任何上游调用。
		assignedTargets := assignedByWorkspace[key]
		assignedGroups := assignedGroupsByWorkspace[key]
		if len(assignedTargets) == 0 && len(assignedGroups) == 0 {
			continue
		}
		// 若该 workspace 的策略没有任何启用的模型目标，直接跳过，避免无谓地拉取分组/账号。
		if !hasEnabledModelTarget(ws.policies) {
			continue
		}
		inventory, err := s.loadAdminInventory(ctx, ws.userID, ws.adminAccountID, inventoryCache)
		if err != nil {
			log.Printf("[connection-health] scheduler load admin inventory failed user_id=%s admin_account_id=%s err=%v", ws.userID, ws.adminAccountID, err)
			continue
		}
		session := inventory.session
		platform := string(session.Platform)

		// 账号/渠道可能同时属于多个 admin 分组。先按稳定 targetId 合并所有来源策略，再生成
		// 一次任务，避免同一目标在一轮中被重复探活。
		type targetCandidate struct {
			target        AdminProbeTarget
			account       upstream.AdminGroupAccountInfo
			policies      []Policy
			policySources map[string]probePolicyEventGroup
			groups        []upstream.AdminGroupInfo
		}
		candidates := make(map[string]*targetCandidate)
		targetOrder := make([]string, 0)
		for _, groupInventory := range inventory.groups {
			if modelBudget <= 0 {
				break
			}
			group := groupInventory.group
			if groupInventory.err != nil {
				log.Printf("[connection-health] scheduler list accounts failed group_id=%s err=%v", group.ID, groupInventory.err)
				continue
			}
			for _, acc := range groupInventory.accounts {
				target := AdminProbeTarget{
					TargetID:       buildTargetID(platform, ws.adminAccountID, acc.ID),
					Platform:       platform,
					AdminGroupID:   group.ID,
					AdminGroupName: group.Name,
					AccountID:      acc.ID,
					AccountName:    acc.Name,
					AccountStatus:  acc.Status,
					AccountWeight:  cloneIntPointer(acc.Weight),
					ProviderFamily: acc.Platform,
					Models:         splitModelList(acc.Models),
				}
				inheritedPolicies := assignedGroups[group.ID]
				if excludedByWorkspace[key][group.ID][target.TargetID] {
					inheritedPolicies = nil
				}
				effectivePolicies := mergePoliciesByID(assignedTargets[target.TargetID], inheritedPolicies)
				if len(effectivePolicies) == 0 {
					continue
				}
				candidate, exists := candidates[target.TargetID]
				if !exists {
					candidate = &targetCandidate{
						target: target, account: acc, policySources: make(map[string]probePolicyEventGroup),
					}
					candidates[target.TargetID] = candidate
					targetOrder = append(targetOrder, target.TargetID)
				}
				candidate.groups = append(candidate.groups, group)
				for _, policy := range assignedTargets[target.TargetID] {
					// An explicit target assignment has no single group owner, even when the
					// target is currently being enumerated through a group membership.
					candidate.policySources[policy.ID] = probePolicyEventGroup{resolved: true}
				}
				for _, policy := range inheritedPolicies {
					if _, alreadyResolved := candidate.policySources[policy.ID]; alreadyResolved {
						continue
					}
					candidate.policySources[policy.ID] = probePolicyEventGroup{
						resolved: true, adminGroupID: group.ID, adminGroupName: group.Name,
					}
				}
				candidate.policies = mergePoliciesByID(candidate.policies, effectivePolicies)
			}
		}

		for _, targetID := range targetOrder {
			if modelBudget <= 0 {
				break
			}
			if len(skipTargets) > 0 && skipTargets[0][ws.userID+"|"+targetID] {
				continue
			}
			candidate := candidates[targetID]
			specs := candidateModelSpecs(candidate.target.Models, candidate.policies)
			for index := range specs {
				if source, exists := candidate.policySources[specs[index].policy.ID]; exists {
					specs[index].eventGroupResolved = source.resolved
					specs[index].eventAdminGroupID = source.adminGroupID
					specs[index].eventAdminGroupName = source.adminGroupName
				}
			}
			available, _ := targetProbeAvailability(platform, candidate.account.BaseURL, len(specs))
			if !available {
				continue
			}
			dueSpecs := make([]probeModelSpec, 0, len(specs))
			for _, spec := range specs {
				if modelBudget <= 0 {
					break
				}
				if !s.isDue(ctx, candidate.target.TargetID, spec.modelName, spec.policy, time.Now()) {
					continue
				}
				budgetKey := ws.userID + "|" + ws.adminAccountID + "|" + spec.policy.ID + "|" + candidate.target.TargetID
				if !budgetLoaded[budgetKey] {
					count, countErr := s.repo.CountProbesToday(ctx, ws.userID, ws.adminAccountID, spec.policy.ID, candidate.target.TargetID, dayStart)
					if countErr != nil {
						log.Printf("[connection-health] count policy probe budget failed policy_id=%s err=%v", spec.policy.ID, countErr)
						continue
					}
					budgetUsage[budgetKey] = count
					budgetLoaded[budgetKey] = true
				}
				if budgetUsage[budgetKey] >= probeBudgetLimit(spec.policy) {
					continue
				}
				dueSpecs = append(dueSpecs, spec)
				budgetUsage[budgetKey]++
				modelBudget--
			}
			if len(dueSpecs) > 0 {
				jobs = append(jobs, adminProbeJob{
					userID: ws.userID, adminAccountID: ws.adminAccountID, session: session,
					target: candidate.target, account: candidate.account, models: specs, dueSpecs: dueSpecs, groups: candidate.groups,
				})
			}
		}
	}
	return jobs
}

// hasEnabledModelTarget 判断一组策略里是否存在至少一个启用策略下的启用模型目标。
func hasEnabledModelTarget(policies []Policy) bool {
	for _, p := range policies {
		if !p.Enabled {
			continue
		}
		for _, t := range p.ModelTargets {
			if t.Enabled {
				return true
			}
		}
	}
	return false
}

// isDue 判断某个 (targetId, model) 组合当前是否到期需要探活。
// 从未探活过立即探活；disabled 状态永不自动探活。
// 成功、失败和暂停都使用策略配置的同一个间隔，不增加退避或冷却等待。
func (s *Service) isDue(ctx context.Context, targetID string, modelName string, policy Policy, now time.Time) bool {
	state, err := s.repo.GetState(ctx, targetID, modelName)
	if err != nil {
		log.Printf("[connection-health] get state failed target_id=%s model=%s err=%v", targetID, modelName, err)
		return false
	}
	if state == nil {
		return true
	}
	if state.State == StateDisabled {
		return false
	}
	// Clear stale suspension even when the budget is exhausted or no probe is due.
	if normalized := stateWithoutSuspension(*state, policy); normalized != *state {
		if err := s.repo.UpsertState(ctx, normalized); err != nil {
			log.Printf("[connection-health] clear suspension failed target_id=%s model=%s err=%v", targetID, modelName, err)
			return false
		}
		state = &normalized
	}
	if state.LastProbeAt == nil {
		return true
	}

	interval := time.Duration(policy.ProbeIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	// Histories keep completion times. Subtract measured request duration to use
	// request start-to-start cadence without changing existing history timestamps.
	started := *state.LastProbeAt
	if state.LastLatencyMs != nil && *state.LastLatencyMs > 0 {
		started = started.Add(-time.Duration(*state.LastLatencyMs) * time.Millisecond)
	}
	return now.Sub(started) >= interval
}

// Re-evaluate current assignments under the target lease. Inventory membership is
// refreshed independently; removing a local selection takes effect immediately.
func (s *Service) currentScheduledModels(ctx context.Context, job adminProbeJob) ([]probeModelSpec, error) {
	policies, err := s.repo.ListPolicies(ctx, job.userID, job.adminAccountID)
	if err != nil {
		return nil, err
	}
	assignments, err := s.repo.ListPolicyAssignmentsByWorkspace(ctx, job.userID, job.adminAccountID)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.ListGroupPolicyAssignmentsByWorkspace(ctx, job.userID, job.adminAccountID)
	if err != nil {
		return nil, err
	}
	exclusions, err := s.repo.ListGroupTargetExclusionsByWorkspace(ctx, job.userID, job.adminAccountID)
	if err != nil {
		return nil, err
	}
	workspace := job.userID + "|" + job.adminAccountID
	targetPolicies := assignedEnabledPoliciesByTarget(policies, assignments)[workspace][job.target.TargetID]
	groupPolicies := assignedEnabledPoliciesByGroup(policies, groups)[workspace]
	excluded := groupTargetExclusionIndex(exclusions)[workspace]
	effective := targetPolicies
	sources := map[string]probePolicyEventGroup{}
	for _, policy := range targetPolicies {
		sources[policy.ID] = probePolicyEventGroup{resolved: true}
	}
	for _, group := range job.groups {
		if excluded[group.ID][job.target.TargetID] {
			continue
		}
		for _, policy := range groupPolicies[group.ID] {
			if _, exists := sources[policy.ID]; !exists {
				sources[policy.ID] = probePolicyEventGroup{resolved: true, adminGroupID: group.ID, adminGroupName: group.Name}
			}
		}
		effective = mergePoliciesByID(effective, groupPolicies[group.ID])
	}
	suspensionEnabled, readErr := s.repo.GetChannelSuspension(ctx, job.userID, job.adminAccountID, job.target.TargetID)
	specs := candidateModelSpecs(job.target.Models, channelSuspensionPolicies(effective, readErr == nil && suspensionEnabled))
	for i := range specs {
		source := sources[specs[i].policy.ID]
		specs[i].eventGroupResolved, specs[i].eventAdminGroupID, specs[i].eventAdminGroupName = source.resolved, source.adminGroupID, source.adminGroupName
	}
	return specs, nil
}
