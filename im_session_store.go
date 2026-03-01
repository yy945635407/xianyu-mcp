package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type IMSessionState struct {
	Username      string `json:"username"`
	Mode          string `json:"mode"` // bot|human
	HandoffReason string `json:"handoff_reason,omitempty"`
	LockOwner     string `json:"lock_owner,omitempty"`
	LockUntil     int64  `json:"lock_until,omitempty"`
	LastReadAt    int64  `json:"last_read_at,omitempty"`
	UpdatedAt     int64  `json:"updated_at"`
}

type imSessionStoreFile struct {
	States map[string]IMSessionState `json:"states"`
}

type IMSessionStore struct {
	mu     sync.Mutex
	path   string
	states map[string]IMSessionState
}

var (
	imSessionStoreOnce sync.Once
	imSessionStoreInst *IMSessionStore
	imSessionStoreErr  error
)

func getIMSessionStore() (*IMSessionStore, error) {
	imSessionStoreOnce.Do(func() {
		imSessionStoreInst, imSessionStoreErr = newIMSessionStore(dataFilePath("im_session_states.json"))
	})
	return imSessionStoreInst, imSessionStoreErr
}

func newIMSessionStore(path string) (*IMSessionStore, error) {
	s := &IMSessionStore{
		path:   path,
		states: map[string]IMSessionState{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func normalizeUsername(v string) string {
	return strings.TrimSpace(v)
}

func normalizeMode(mode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "human", "manual", "人工":
		return "human"
	case "bot", "robot", "自动":
		return "bot"
	default:
		return ""
	}
}

func (s *IMSessionStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read im session store failed: %w", err)
	}

	var f imSessionStoreFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("unmarshal im session store failed: %w", err)
	}
	if f.States != nil {
		s.states = f.States
	}
	return nil
}

func (s *IMSessionStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create data dir failed: %w", err)
	}
	payload := imSessionStoreFile{States: s.states}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal im session store failed: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp im session store failed: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename im session store failed: %w", err)
	}
	return nil
}

func (s *IMSessionStore) Get(username string) IMSessionState {
	s.mu.Lock()
	defer s.mu.Unlock()

	uname := normalizeUsername(username)
	st, ok := s.states[uname]
	if !ok {
		st = IMSessionState{Username: uname, Mode: "bot", UpdatedAt: time.Now().UnixMilli()}
	}
	if st.Mode == "" {
		st.Mode = "bot"
	}
	if st.LockUntil > 0 && st.LockUntil < time.Now().UnixMilli() {
		st.LockUntil = 0
		st.LockOwner = ""
	}
	return st
}

func (s *IMSessionStore) List(limit int) []IMSessionState {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 || limit > 1000 {
		limit = 200
	}

	out := make([]IMSessionState, 0, len(s.states))
	now := time.Now().UnixMilli()
	for _, st := range s.states {
		if st.Mode == "" {
			st.Mode = "bot"
		}
		if st.LockUntil > 0 && st.LockUntil < now {
			st.LockUntil = 0
			st.LockOwner = ""
		}
		out = append(out, st)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *IMSessionStore) Upsert(username, mode, handoffReason, lockOwner string, lockSeconds int64, clearLock bool) (IMSessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uname := normalizeUsername(username)
	if uname == "" {
		return IMSessionState{}, fmt.Errorf("username is required")
	}

	now := time.Now().UnixMilli()
	st, ok := s.states[uname]
	if !ok {
		st = IMSessionState{Username: uname, Mode: "bot"}
	}

	if m := normalizeMode(mode); m != "" {
		st.Mode = m
	}
	if strings.TrimSpace(handoffReason) != "" {
		st.HandoffReason = strings.TrimSpace(handoffReason)
	}
	if clearLock {
		st.LockOwner = ""
		st.LockUntil = 0
	}
	if lockSeconds > 0 {
		st.LockOwner = strings.TrimSpace(lockOwner)
		if st.LockOwner == "" {
			st.LockOwner = "system"
		}
		st.LockUntil = now + lockSeconds*1000
	}
	if st.Mode == "" {
		st.Mode = "bot"
	}
	st.UpdatedAt = now

	s.states[uname] = st
	if err := s.saveLocked(); err != nil {
		return IMSessionState{}, err
	}
	return st, nil
}

func (s *IMSessionStore) MarkRead(username string, at int64) (IMSessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uname := normalizeUsername(username)
	if uname == "" {
		return IMSessionState{}, fmt.Errorf("username is required")
	}

	st, ok := s.states[uname]
	if !ok {
		st = IMSessionState{Username: uname, Mode: "bot"}
	}
	if st.Mode == "" {
		st.Mode = "bot"
	}
	st.LastReadAt = at
	st.UpdatedAt = at
	s.states[uname] = st
	if err := s.saveLocked(); err != nil {
		return IMSessionState{}, err
	}
	return st, nil
}

func (s *IMSessionStore) CheckSendPermission(username string, now int64) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	uname := normalizeUsername(username)
	st, ok := s.states[uname]
	if !ok {
		return true, ""
	}
	if st.Mode == "human" {
		return false, "conversation is in human mode"
	}
	if st.LockUntil > now {
		owner := st.LockOwner
		if owner == "" {
			owner = "unknown"
		}
		return false, fmt.Sprintf("conversation is locked by %s until %d", owner, st.LockUntil)
	}
	return true, ""
}
