package connection_health

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"transithub/backend/internal/modules/upstream"
)

func groupPolicyEditFixture() (*Service, *fakeRepository, PolicyInput) {
	repo := newFakeRepository()
	other := probePolicy()
	other.ID, other.Name = "policy-2", "other"
	repo.policies = []Policy{probePolicy(), other}
	repo.groupAssignments = []GroupPolicyAssignment{
		{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", PolicyID: "policy-1"},
		{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", PolicyID: "policy-2"},
		{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g2", PolicyID: "policy-1"},
	}
	repo.groupExclusions = []GroupTargetExclusion{
		{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", TargetID: "newapi:ws1:100"},
		{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", TargetID: "newapi:ws1:absent"},
		{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g2", TargetID: "newapi:ws1:200"},
	}
	reader := fakePlatformGroupReader{
		groups: []upstream.AdminGroupInfo{{ID: "g1", Name: "group 1"}, {ID: "g2", Name: "group 2"}},
		accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{
			"g1": {{ID: "100"}, {ID: "200"}},
			"g2": {{ID: "200"}},
		},
	}
	service := newAdminGroupsService(reader, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformNewAPI}}, repo)
	return service, repo, PolicyInput{
		ID: "policy-1", Name: "edited", Enabled: true, ProbeIntervalSeconds: 120,
		ModelTargets: []ModelTargetInput{{ModelName: "new-model", ProviderFamily: ProviderOpenAI, Enabled: true}},
	}
}

func TestEditGroupPolicyChannelSelection(t *testing.T) {
	ctx := context.Background()
	service, repo, edit := groupPolicyEditFixture()
	otherBefore, _ := service.GetAdminGroupPolicyConfiguration(ctx, "user1", "g2")
	for _, exclusions := range [][]string{{"newapi:ws1:200"}, {"newapi:ws1:100", "newapi:ws1:200"}, {}} {
		config, err := service.SetAdminGroupPolicyConfiguration(ctx, "user1", "g1", AdminGroupPolicyConfigurationInput{
			EditPolicy: &edit, ExcludedTargetIDs: exclusions,
			// The editor must preserve existing bindings even when the client omits them.
			PolicyIDs: []string{},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(config.PolicyIDs) != 2 || !slices.Contains(config.PolicyIDs, "policy-2") {
			t.Fatalf("lost another group policy: %+v", config)
		}
		wantExcluded := append(slices.Clone(exclusions), "newapi:ws1:absent")
		slices.Sort(wantExcluded)
		slices.Sort(config.ExcludedTargetIDs)
		if !slices.Equal(wantExcluded, config.ExcludedTargetIDs) {
			t.Fatalf("selection or absent exclusion lost: %+v", config)
		}
		readBack, err := service.GetAdminGroupPolicyConfiguration(ctx, "user1", "g1")
		if err != nil || !reflect.DeepEqual(readBack, config) {
			t.Fatalf("saved selection did not round trip: %+v %v", readBack, err)
		}
	}
	otherAfter, _ := service.GetAdminGroupPolicyConfiguration(ctx, "user1", "g2")
	if !slices.Equal(otherBefore.PolicyIDs, otherAfter.PolicyIDs) || !slices.Equal(otherBefore.ExcludedTargetIDs, otherAfter.ExcludedTargetIDs) {
		t.Fatal("current group selection affected another group sharing the policy")
	}
	saved, _ := repo.GetPolicy(ctx, "policy-1", "user1", "ws1")
	if saved.Name != "edited" || saved.ProbeIntervalSeconds != 120 || len(saved.ModelTargets) != 1 || saved.ModelTargets[0].ModelName != "new-model" {
		t.Fatalf("policy settings and models were not updated: %+v", saved)
	}
}

func TestEditGroupPolicyRejectsInvalidScopeBeforeWriting(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*fakeRepository, *AdminGroupPolicyConfigurationInput)
		want   string
	}{
		{"foreign policy", func(r *fakeRepository, _ *AdminGroupPolicyConfigurationInput) { r.policies[0].AdminAccountID = "ws2" }, ErrorPolicyNotFound},
		{"foreign user", func(r *fakeRepository, _ *AdminGroupPolicyConfigurationInput) { r.policies[0].UserID = "other" }, ErrorPolicyNotFound},
		{"missing policy", func(_ *fakeRepository, i *AdminGroupPolicyConfigurationInput) { i.EditPolicy.ID = "missing" }, ErrorPolicyNotFound},
		{"foreign channel", func(_ *fakeRepository, i *AdminGroupPolicyConfigurationInput) {
			i.ExcludedTargetIDs = []string{"newapi:ws2:100"}
		}, ErrorProbeTargetNotFound},
		{"outside group", func(_ *fakeRepository, i *AdminGroupPolicyConfigurationInput) {
			i.ExcludedTargetIDs = []string{"newapi:ws1:300"}
		}, ErrorProbeTargetNotFound},
		{"mixed create and edit", func(_ *fakeRepository, i *AdminGroupPolicyConfigurationInput) {
			i.QuickPolicy = &PolicyInput{Name: "new"}
		}, ErrorRequest},
		{"new multiplier mode", func(_ *fakeRepository, i *AdminGroupPolicyConfigurationInput) {
			i.EditPolicy.StrategyMode = StrategyModeMultiplierOnly
		}, ErrorMultiplierRequired},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			service, repo, edit := groupPolicyEditFixture()
			input := AdminGroupPolicyConfigurationInput{EditPolicy: &edit}
			scenario.change(repo, &input)
			beforePolicies := slices.Clone(repo.policies)
			beforeBindings := slices.Clone(repo.groupAssignments)
			beforeExclusions := slices.Clone(repo.groupExclusions)
			_, err := service.SetAdminGroupPolicyConfiguration(context.Background(), "user1", "g1", input)
			if err == nil || err.Error() != scenario.want {
				t.Fatalf("want %s, got %v", scenario.want, err)
			}
			if !reflect.DeepEqual(beforePolicies, repo.policies) || !reflect.DeepEqual(beforeBindings, repo.groupAssignments) || !reflect.DeepEqual(beforeExclusions, repo.groupExclusions) {
				t.Fatal("invalid edit partially persisted")
			}
		})
	}
}
