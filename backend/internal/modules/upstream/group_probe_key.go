package upstream

import (
	"fmt"
	"strconv"
	"strings"
)

// ResolveSub2APIGroupProbeKey verifies a supplied key against the selected group.
// Only the upstream key ID is persisted by the caller; later probes resolve that
// same ID again, so revoked or reassigned keys cannot silently change the target.
func (s *PlatformService) ResolveSub2APIGroupProbeKey(session Session, groupID, suppliedKey, keyID string) (ProbeCredential, string, error) {
	id, err := strconv.Atoi(groupID)
	if session.Platform != PlatformSub2API || err != nil || id <= 0 || (suppliedKey == "" && keyID == "") {
		return ProbeCredential{}, "", fmt.Errorf("invalid group key selection")
	}
	for page := 1; page <= 100; page++ {
		response, err := s.httpClient.requestJSON(fmt.Sprintf("%s/api/v1/admin/groups/%d/api-keys?page=%d&page_size=100", session.BaseURL, id, page), adminAuthOptions(session))
		if err != nil {
			return ProbeCredential{}, "", err
		}
		root, _ := response.Payload.(map[string]any)
		data, _ := root["data"].(map[string]any)
		items, ok := data["items"].([]any)
		if !ok {
			return ProbeCredential{}, "", fmt.Errorf("invalid group key list")
		}
		for _, item := range items {
			record, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key := safeString(record, "key")
			keyNumber := firstNumber(record, []string{"id"})
			if keyNumber == nil || *keyNumber <= 0 {
				continue
			}
			recordID := strconv.FormatInt(int64(*keyNumber), 10)
			if (suppliedKey != "" && key != suppliedKey) || (suppliedKey == "" && recordID != keyID) {
				continue
			}
			gid := firstNumber(record, []string{"group_id"})
			if gid == nil || int(*gid) != id || safeString(record, "status") != "active" || strings.TrimSpace(key) == "" || strings.Contains(key, "*") {
				return ProbeCredential{}, "", fmt.Errorf("group key unavailable")
			}
			return ProbeCredential{BaseURL: session.BaseURL, Key: key}, recordID, nil
		}
		total := firstNumber(data, []string{"total"})
		if len(items) == 0 || (total != nil && float64(page*100) >= *total) || (total == nil && len(items) < 100) {
			break
		}
	}
	return ProbeCredential{}, "", fmt.Errorf("key not found in selected group")
}

// EnsureSub2APIGroupProbeKey only reuses a dedicated key with both the expected
// name and group. The caller serializes creation with a workspace/group lease.
// Plaintext keys are fetched when needed and are never persisted locally.
func (s *PlatformService) EnsureSub2APIGroupProbeKey(session Session, groupID, name string) (ProbeCredential, error) {
	if session.Platform != PlatformSub2API || strings.TrimSpace(session.AccessToken) == "" {
		return ProbeCredential{}, fmt.Errorf("group probe key requires Sub2API user login")
	}
	id, err := strconv.Atoi(groupID)
	if err != nil || id <= 0 {
		return ProbeCredential{}, fmt.Errorf("invalid group id")
	}
	for page := 1; page <= 100; page++ {
		response, err := s.httpClient.requestJSON(fmt.Sprintf("%s/api/v1/keys?page=%d&page_size=100", session.BaseURL, page), requestOptions{AccessToken: session.AccessToken, TokenType: session.TokenType})
		if err != nil {
			return ProbeCredential{}, err
		}
		root, ok := response.Payload.(map[string]any)
		if !ok {
			return ProbeCredential{}, fmt.Errorf("invalid key list")
		}
		data, ok := root["data"].(map[string]any)
		if !ok {
			return ProbeCredential{}, fmt.Errorf("invalid key list")
		}
		items, ok := data["items"].([]any)
		if !ok {
			return ProbeCredential{}, fmt.Errorf("invalid key list")
		}
		for _, item := range items {
			record, ok := item.(map[string]any)
			if !ok || safeString(record, "name") != name {
				continue
			}
			gid := firstNumber(record, []string{"group_id"})
			if gid == nil || int(*gid) != id {
				// Never create another key or use a reassigned monitoring key.
				return ProbeCredential{}, fmt.Errorf("monitoring key group changed")
			}
			if status := safeString(record, "status"); status != "active" {
				return ProbeCredential{}, fmt.Errorf("monitoring key is inactive")
			}
			key := firstString(record, []string{"key"})
			if key == nil || strings.TrimSpace(*key) == "" || strings.Contains(*key, "*") {
				return ProbeCredential{}, fmt.Errorf("monitoring key is unavailable")
			}
			return ProbeCredential{BaseURL: session.BaseURL, Key: *key}, nil
		}
		total := firstNumber(data, []string{"total"})
		if len(items) == 0 || (total != nil && float64(page*100) >= *total) || (total == nil && len(items) < 100) {
			_, key, err := s.CreateSub2APIKey(session, name, id)
			if err != nil {
				return ProbeCredential{}, err
			}
			return ProbeCredential{BaseURL: session.BaseURL, Key: key}, nil
		}
	}
	return ProbeCredential{}, fmt.Errorf("key list pagination limit reached")
}
