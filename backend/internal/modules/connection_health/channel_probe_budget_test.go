package connection_health

import (
	"context"
	"errors"
	"testing"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func TestScheduler_ChannelBudgetsAreIndependentAndModelsShareQuota(t *testing.T) {
	svc, repo, reader := cadenceFixture(3)
	repo.policies[0].DailyProbeBudget = 1
	repo.policies[0].ModelTargets = append(repo.policies[0].ModelTargets, ModelTarget{ModelName: "extra-model", Enabled: true})
	reader.accountsByGrp["g1"][0].Models += ",extra-model"
	// Duplicate group membership must not multiply requests or channel quota.
	reader.groups = append(reader.groups, upstream.AdminGroupInfo{ID: "g2"})
	reader.accountsByGrp["g2"] = reader.accountsByGrp["g1"]
	repo.groupAssignments = append(repo.groupAssignments, GroupPolicyAssignment{UserID: "user1", AdminAccountID: "ws1", PolicyID: "p1", AdminGroupID: "g2"})
	jobs := svc.collectAdminProbeJobsWithGroups(context.Background(), repo.policies, nil, repo.groupAssignments, nil)
	if len(jobs) != 3 {
		t.Fatalf("expected each channel to have a job, got %d", len(jobs))
	}
	for _, job := range jobs {
		if len(job.dueSpecs) != 1 {
			t.Fatalf("multiple models exceeded channel quota: %+v", job.dueSpecs)
		}
	}
	_, _ = repo.TryConsumeProbeBudget(context.Background(), "user1", "ws1", "p1", jobs[0].target.TargetID, probeBudgetDayStart(time.Now()), 1)
	remaining := svc.collectAdminProbeJobsWithGroups(context.Background(), repo.policies, nil, repo.groupAssignments, nil)
	if len(remaining) != 2 {
		t.Fatalf("one exhausted channel blocked others: %d remaining", len(remaining))
	}
}

type failedBudgetReadRepo struct{ *fakeRepository }

func (r failedBudgetReadRepo) ListProbeBudgetUsage(context.Context, string, string, time.Time) ([]ProbeBudgetUsage, error) {
	return nil, errors.New("budget unavailable")
}

func TestAdminGroups_ChannelBudgetUsesEffectiveSharedPolicy(t *testing.T) {
	svc, repo, reader := cadenceFixture(2)
	svc.accounts = fakeAdminAccountResolver{id: "ws1"}
	repo.policies[0].DailyProbeBudget = 1
	second := repo.policies[0]
	second.ID, second.DailyProbeBudget = "p2", 10
	repo.policies = append(repo.policies, second)
	reader.groups = append(reader.groups, upstream.AdminGroupInfo{ID: "g2"})
	reader.accountsByGrp["g2"] = reader.accountsByGrp["g1"]
	repo.groupAssignments = append(repo.groupAssignments, GroupPolicyAssignment{UserID: "user1", AdminAccountID: "ws1", PolicyID: "p2", AdminGroupID: "g2"})
	target := buildTargetID("sub2api", "ws1", "0")
	day := probeBudgetDayStart(time.Now())
	_, _ = repo.TryConsumeProbeBudget(context.Background(), "user1", "ws1", "p1", target, day, 1)
	groups, err := svc.AdminGroups(context.Background(), "user1")
	if err != nil || len(groups) != 2 {
		t.Fatalf("groups: %+v %v", groups, err)
	}
	for _, group := range groups {
		for _, account := range group.Accounts {
			if len(account.ProbeBudgets) != 1 {
				t.Fatalf("expected winning policy only: %+v", account.ProbeBudgets)
			}
			budget := account.ProbeBudgets[0]
			if budget.PolicyID != "p1" || budget.Limit != 1 || budget.Exhausted != (account.TargetID == target) || !budget.ResetsAt.Equal(day.Add(24*time.Hour)) {
				t.Fatalf("wrong shared-channel budget: %+v", budget)
			}
		}
	}
	// A failed optional budget read must not fabricate usage or break the page.
	svc.repo = failedBudgetReadRepo{repo.fakeRepository}
	groups, err = svc.AdminGroups(context.Background(), "user1")
	if err != nil || !groups[0].Accounts[0].ProbeBudgetError || len(groups[0].Accounts[0].ProbeBudgets) != 0 {
		t.Fatalf("budget read failure not surfaced: %+v %v", groups, err)
	}
}
