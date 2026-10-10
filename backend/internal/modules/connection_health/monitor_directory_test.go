package connection_health

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"transithub/backend/internal/modules/upstream"
)

func TestMonitorDirectorySingleFetchAndBackgroundRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var cache monitorDirectoryCache[int]
		var calls atomic.Int32
		gate := make(chan struct{})
		fetch := func() (int, error) { n := int(calls.Add(1)); <-gate; return n, nil }
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				v, ok, _, err := cache.get(context.Background(), "scope", true, fetch)
				if err != nil || !ok || v != 1 {
					t.Errorf("cold read: %d %v %v", v, ok, err)
				}
			}()
		}
		synctest.Wait()
		if calls.Load() != 1 {
			t.Fatalf("duplicate upstream fetches: %d", calls.Load())
		}
		close(gate)
		wg.Wait()
		time.Sleep(31 * time.Second)
		gate = make(chan struct{})
		v, ok, refreshing, err := cache.get(context.Background(), "scope", true, fetch)
		if err != nil || !ok || !refreshing || v != 1 {
			t.Fatalf("stale read blocked or lost data: %d %v %v %v", v, ok, refreshing, err)
		}
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatal("background refresh missing")
		}
		close(gate)
		synctest.Wait()
		v, _, refreshing, err = cache.get(context.Background(), "scope", true, fetch)
		if err != nil || refreshing || v != 2 {
			t.Fatal("new inventory not published")
		}
		// Never serve stale directory indefinitely after an upstream outage.
		time.Sleep(121 * time.Second)
		_, loaded, _, err := cache.get(context.Background(), "scope", true, func() (int, error) { return 0, errors.New("upstream unavailable") })
		if err == nil || loaded {
			t.Fatal("expired directory presented as current after failure")
		}
	})
}

func TestMonitorDirectoryWaitCancellationAndIsolation(t *testing.T) {
	session := upstream.Session{Platform: upstream.PlatformSub2API, BaseURL: "https://example.test", AccessToken: "one"}
	key := monitorDirectoryKey("u", "w", session)
	changed := session
	changed.AccessToken = "two"
	for _, other := range []string{monitorDirectoryKey("other", "w", session), monitorDirectoryKey("u", "other", session), monitorDirectoryKey("u", "w", changed)} {
		if key == other {
			t.Fatal("cache crosses user/workspace/credential scope")
		}
	}
	var cache monitorDirectoryCache[int]
	gate := make(chan struct{})
	_, loaded, refreshing, err := cache.get(context.Background(), key, false, func() (int, error) { <-gate; return 1, nil })
	if err != nil || loaded || !refreshing {
		t.Fatal("sidebar waited for a cold account inventory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, err = cache.get(ctx, key, true, func() (int, error) { t.Error("duplicate fetch"); return 0, nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled waiter did not return")
	}
	close(gate)
	_, _, _, _ = cache.get(context.Background(), key, true, func() (int, error) { return 0, nil })
}

func TestMonitorManualPriorityRestoreInvalidatesOnlyMatchingDirectory(t *testing.T) {
	svc := &Service{}
	session := upstream.Session{Platform: upstream.PlatformSub2API}
	key := monitorDirectoryKey("u", "w", session)
	foreign := monitorDirectoryKey("other", "w", session)
	for _, entry := range []struct{ key, account string }{{key + "|g1", "a"}, {key + "|g2", "a"}, {key + "|g3", "b"}, {foreign + "|g1", "a"}} {
		_, _, _, err := svc.monitorAccountCache.get(context.Background(), entry.key, true, func() ([]upstream.AdminGroupAccountInfo, error) {
			return []upstream.AdminGroupAccountInfo{{ID: entry.account}}, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	svc.invalidateMonitorAccount("u", "w", "a")
	if len(svc.monitorAccountCache.entries) != 2 || svc.monitorAccountCache.entries[key+"|g3"] == nil || svc.monitorAccountCache.entries[foreign+"|g1"] == nil {
		t.Fatal("manual restore invalidated another channel/workspace or kept shared stale records")
	}
}

func TestMonitorDirectoryCapsConcurrentUpstreamReads(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reader := &monitorTestReader{gate: make(chan struct{}), fakePlatformGroupReader: fakePlatformGroupReader{}}
		svc := &Service{platformGroups: reader}
		groups := []upstream.AdminGroupInfo{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"}, {ID: "6"}}
		inventory := svc.monitorAccounts(context.Background(), "scope", upstream.Session{}, groups, false)
		if len(inventory) != 6 {
			t.Fatal("sidebar inventory incomplete")
		}
		synctest.Wait()
		if reader.accountsRead.Load() != 4 {
			t.Fatalf("expected four parallel reads, got %d", reader.accountsRead.Load())
		}
		close(reader.gate)
		synctest.Wait()
		if reader.accountsRead.Load() != 6 || reader.peak.Load() > 4 {
			t.Fatal("concurrency bound or queued reads failed")
		}
	})
}

type monitorTestReader struct {
	fakePlatformGroupReader
	gate         chan struct{}
	groupsRead   atomic.Int32
	accountsRead atomic.Int32
	inflight     atomic.Int32
	peak         atomic.Int32
}

func (r *monitorTestReader) FetchAdminAllGroups(s upstream.Session) ([]upstream.AdminGroupInfo, error) {
	r.groupsRead.Add(1)
	return r.fakePlatformGroupReader.FetchAdminAllGroups(s)
}
func (r *monitorTestReader) ListAdminGroupAccounts(s upstream.Session, g upstream.AdminGroupInfo) ([]upstream.AdminGroupAccountInfo, error) {
	r.accountsRead.Add(1)
	n := r.inflight.Add(1)
	defer r.inflight.Add(-1)
	for old := r.peak.Load(); n > old; old = r.peak.Load() {
		if r.peak.CompareAndSwap(old, n) {
			break
		}
	}
	if r.gate != nil {
		<-r.gate
	}
	return r.fakePlatformGroupReader.ListAdminGroupAccounts(s, g)
}

func TestMonitorSummariesRenderBeforeAccountsAndDetailKeepsSharedPolicy(t *testing.T) {
	repo := newFakeRepository()
	p := probePolicy()
	p.ID = "p1"
	p.ModelTargets[0].ModelName = "model"
	other := p
	other.ID = "p2"
	repo.policies = []Policy{p, other}
	repo.groupAssignments = []GroupPolicyAssignment{{UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g1", PolicyID: "p2"}, {UserID: "user1", AdminAccountID: "ws1", AdminGroupID: "g2", PolicyID: "p1"}}
	reader := &monitorTestReader{gate: make(chan struct{}), fakePlatformGroupReader: fakePlatformGroupReader{
		groups: []upstream.AdminGroupInfo{{ID: "g1"}, {ID: "g2"}}, accountsByGrp: map[string][]upstream.AdminGroupAccountInfo{
			"g1": {{ID: "shared", Models: "model"}}, "g2": {{ID: "shared", Models: "model"}, {ID: "other", Models: "model"}},
		},
	}}
	svc := newAdminGroupsService(reader, fakeMySitesReader{session: upstream.Session{Platform: upstream.PlatformSub2API}}, repo)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	groups, err := svc.AdminGroupSummaries(ctx, "user1")
	if err != nil || len(groups) != 2 || groups[0].AccountsLoaded {
		t.Fatalf("sidebar blocked on accounts: %+v %v", groups, err)
	}
	close(reader.gate)
	target := buildTargetID("sub2api", "ws1", "shared")
	_ = repo.InsertEvent(ctx, ConnectionHealthEvent{ID: "e1", UserID: "user1", AdminAccountID: "ws1", PolicyID: "p1", ConnectionID: target, ModelName: "model", Result: "ok"})
	detail, err := svc.AdminGroupDetail(ctx, "user1", "g1")
	if err != nil || detail == nil || len(detail.Accounts) != 1 || !detail.AccountsLoaded {
		t.Fatalf("detail missing: %+v %v", detail, err)
	}
	if len(detail.Accounts[0].ProbeBudgets) != 1 || detail.Accounts[0].ProbeBudgets[0].PolicyID != "p1" || len(detail.Accounts[0].RecentProbes) != 1 {
		t.Fatalf("shared policy/history lost: %+v", detail.Accounts[0])
	}
	// A warm directory must not cache local probe state or policies.
	repo.policies[1].Enabled = false
	_ = repo.InsertEvent(ctx, ConnectionHealthEvent{ID: "e2", UserID: "user1", AdminAccountID: "ws1", PolicyID: "p1", ConnectionID: target, ModelName: "model", Result: "server_error"})
	detail, err = svc.AdminGroupDetail(ctx, "user1", "g1")
	if err != nil || detail.HasEnabledPolicy || len(detail.Accounts[0].RecentProbes) != 2 {
		t.Fatalf("local state cached: %+v %v", detail, err)
	}
	groups, err = svc.AdminGroupSummaries(ctx, "user1")
	if err != nil || !groups[0].AccountsLoaded || len(groups[0].Accounts[0].RecentProbes) != 0 || len(groups[0].Accounts[0].QualityHistory) != 0 {
		t.Fatal("summary contains heavy channel history")
	}
	if reader.groupsRead.Load() != 1 || reader.accountsRead.Load() != 2 || reader.peak.Load() > 4 {
		t.Fatalf("directory not reused: %d %d", reader.groupsRead.Load(), reader.accountsRead.Load())
	}
	if _, err = svc.AdminGroupDetail(ctx, "user1", "foreign-group"); err == nil {
		t.Fatal("unknown group exposed")
	}
}
