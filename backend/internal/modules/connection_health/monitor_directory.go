package connection_health

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"transithub/backend/internal/modules/upstream"
)

const monitorDirectoryTTL = 30 * time.Second
const monitorDirectoryMaxAge = 2 * time.Minute

// Only sanitized upstream directory records are cached. Health, permissions,
// policy configuration and probe histories are read from storage on each request.
// Entries are immutable after publication, and concurrent readers share a fetch.
type monitorDirectoryEntry[T any] struct {
	value     T
	valid     bool
	updatedAt time.Time
	retryAt   time.Time
	err       error
	pending   chan struct{}
}

type monitorDirectoryCache[T any] struct {
	mu      sync.Mutex
	entries map[string]*monitorDirectoryEntry[T]
}

// wait=false starts missing reads in the background so the sidebar can render
// before account inventory is ready. Stale values are served for at most 2 min.
func (c *monitorDirectoryCache[T]) get(ctx context.Context, key string, wait bool, fetch func() (T, error)) (value T, loaded bool, refreshing bool, err error) {
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[string]*monitorDirectoryEntry[T])
	}
	now := time.Now()
	entry := c.entries[key]
	if entry == nil {
		// Bound idle cache retention without evicting active requests.
		for oldKey, old := range c.entries {
			if old.pending == nil && now.Sub(old.updatedAt) > 10*time.Minute {
				delete(c.entries, oldKey)
			}
		}
		entry = &monitorDirectoryEntry[T]{}
		c.entries[key] = entry
	}
	if entry.valid && now.Sub(entry.updatedAt) < monitorDirectoryTTL {
		value = entry.value
		c.mu.Unlock()
		return value, true, false, nil
	}
	if entry.pending == nil && !now.Before(entry.retryAt) {
		entry.pending = make(chan struct{})
		go func() {
			next, fetchErr := fetch()
			c.mu.Lock()
			defer c.mu.Unlock()
			if fetchErr == nil {
				entry.value, entry.valid, entry.updatedAt = next, true, time.Now()
			}
			entry.err = fetchErr
			entry.retryAt = time.Now().Add(5 * time.Second)
			close(entry.pending)
			entry.pending = nil
		}()
	}
	if entry.valid && now.Sub(entry.updatedAt) < monitorDirectoryMaxAge {
		value = entry.value
		c.mu.Unlock()
		return value, true, true, nil
	}
	pending := entry.pending
	if !wait || pending == nil {
		err = entry.err
		c.mu.Unlock()
		return value, false, pending != nil, err
	}
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		return value, false, false, ctx.Err()
	case <-pending:
		c.mu.Lock()
		defer c.mu.Unlock()
		if entry.err != nil {
			return value, false, false, entry.err
		}
		return entry.value, entry.valid, false, nil
	}
}

func monitorDirectoryKey(user, workspace string, session upstream.Session) string {
	// A changed upstream URL or login must not reuse an earlier session's cache.
	// Only the digest is stored; credentials never enter the directory records.
	encoded, _ := json.Marshal(session)
	return fmt.Sprintf("%s|%s|%x", user, workspace, sha256.Sum256(encoded))
}

// A manual priority restore should be visible on the next read, even inside the
// directory TTL. In-flight old entries may finish but cannot repopulate this map.
func (s *Service) invalidateMonitorAccount(user, workspace, accountID string) {
	s.monitorAccountCache.mu.Lock()
	defer s.monitorAccountCache.mu.Unlock()
	for key, entry := range s.monitorAccountCache.entries {
		if !strings.HasPrefix(key, user+"|"+workspace+"|") {
			continue
		}
		if !entry.valid {
			delete(s.monitorAccountCache.entries, key)
			continue
		}
		for _, account := range entry.value {
			if account.ID == accountID {
				delete(s.monitorAccountCache.entries, key)
				break
			}
		}
	}
}

type monitorInventoryGroup struct {
	accounts   []upstream.AdminGroupAccountInfo
	err        error
	loaded     bool
	refreshing bool
}

func (s *Service) monitorAccounts(ctx context.Context, key string, session upstream.Session, groups []upstream.AdminGroupInfo, wait bool) []monitorInventoryGroup {
	result := make([]monitorInventoryGroup, len(groups))
	var wg sync.WaitGroup
	s.monitorFetchOnce.Do(func() { s.monitorFetchSlots = make(chan struct{}, 4) })
	for i, group := range groups {
		wg.Add(1)
		go func(i int, group upstream.AdminGroupInfo) {
			defer wg.Done()
			accounts, loaded, refreshing, err := s.monitorAccountCache.get(ctx, key+"|"+group.ID, wait, func() ([]upstream.AdminGroupAccountInfo, error) {
				s.monitorFetchSlots <- struct{}{}
				defer func() { <-s.monitorFetchSlots }()
				return s.platformGroups.ListAdminGroupAccounts(session, group)
			})
			result[i] = monitorInventoryGroup{accounts: accounts, err: err, loaded: loaded, refreshing: refreshing}
		}(i, group)
	}
	wg.Wait()
	return result
}

type adminGroupReadOptions struct {
	summary bool
	groupID string
}

func (s *Service) AdminGroupSummaries(ctx context.Context, userID string) ([]AdminGroupHealth, error) {
	return s.readAdminGroups(ctx, userID, adminGroupReadOptions{summary: true})
}

func (s *Service) AdminGroupDetail(ctx context.Context, userID, groupID string) (*AdminGroupHealth, error) {
	groups, err := s.readAdminGroups(ctx, userID, adminGroupReadOptions{groupID: groupID})
	if err != nil {
		return nil, err
	}
	if len(groups) != 1 {
		return nil, requestError(ErrorNotFound)
	}
	return &groups[0], nil
}
