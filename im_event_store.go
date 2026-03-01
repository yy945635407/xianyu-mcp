package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ylyt_bot/xianyu-mcp/xianyu"
)

type IMEvent struct {
	ID          int64  `json:"id"`
	Timestamp   int64  `json:"timestamp"`
	Username    string `json:"username"`
	LastMessage string `json:"last_message"`
	LastTime    string `json:"last_time,omitempty"`
	OrderStatus string `json:"order_status,omitempty"`
	UnreadCount int    `json:"unread_count,omitempty"`
	RawPreview  string `json:"raw_preview,omitempty"`
	Signature   string `json:"signature,omitempty"`
}

type imEventStoreFile struct {
	NextID         int64             `json:"next_id"`
	LastSignatures map[string]string `json:"last_signatures"`
	Events         []IMEvent         `json:"events"`
}

type IMEventStore struct {
	mu             sync.Mutex
	path           string
	nextID         int64
	lastSignatures map[string]string
	events         []IMEvent
}

var (
	imEventStoreOnce sync.Once
	imEventStoreInst *IMEventStore
	imEventStoreErr  error
)

func getIMEventStore() (*IMEventStore, error) {
	imEventStoreOnce.Do(func() {
		imEventStoreInst, imEventStoreErr = newIMEventStore(dataFilePath("im_events.json"))
	})
	return imEventStoreInst, imEventStoreErr
}

func newIMEventStore(path string) (*IMEventStore, error) {
	s := &IMEventStore{
		path:           path,
		nextID:         1,
		lastSignatures: map[string]string{},
		events:         make([]IMEvent, 0),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *IMEventStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read im event store failed: %w", err)
	}

	var f imEventStoreFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("unmarshal im event store failed: %w", err)
	}

	if f.NextID > 0 {
		s.nextID = f.NextID
	}
	if f.LastSignatures != nil {
		s.lastSignatures = f.LastSignatures
	}
	if f.Events != nil {
		s.events = f.Events
	}
	if s.nextID <= 0 {
		s.nextID = 1
	}
	return nil
}

func (s *IMEventStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create data dir failed: %w", err)
	}

	payload := imEventStoreFile{
		NextID:         s.nextID,
		LastSignatures: s.lastSignatures,
		Events:         s.events,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal im event store failed: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write im event store temp failed: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename im event store failed: %w", err)
	}
	return nil
}

func conversationSignature(c xianyu.ConversationSummary) string {
	parts := []string{
		strings.TrimSpace(c.Username),
		strings.TrimSpace(c.LastMessage),
		strings.TrimSpace(c.LastTime),
		strings.TrimSpace(c.OrderStatus),
		strings.TrimSpace(c.StatusTag),
		fmt.Sprintf("%d", c.UnreadCount),
	}
	return strings.Join(parts, "|")
}

func (s *IMEventStore) CaptureConversations(conversations []xianyu.ConversationSummary) ([]IMEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	generated := make([]IMEvent, 0)

	if s.nextID <= 0 {
		s.nextID = 1
	}
	if s.lastSignatures == nil {
		s.lastSignatures = map[string]string{}
	}

	for _, c := range conversations {
		username := strings.TrimSpace(c.Username)
		if username == "" {
			continue
		}
		sig := conversationSignature(c)
		if prev, ok := s.lastSignatures[username]; ok && prev == sig {
			continue
		}
		e := IMEvent{
			ID:          s.nextID,
			Timestamp:   now,
			Username:    username,
			LastMessage: c.LastMessage,
			LastTime:    c.LastTime,
			OrderStatus: c.OrderStatus,
			UnreadCount: c.UnreadCount,
			RawPreview:  c.RawPreview,
			Signature:   sig,
		}
		s.nextID++
		s.lastSignatures[username] = sig
		s.events = append(s.events, e)
		generated = append(generated, e)
	}

	const maxEvents = 5000
	if len(s.events) > maxEvents {
		s.events = s.events[len(s.events)-maxEvents:]
	}

	if len(generated) > 0 {
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
	}
	return generated, nil
}

func (s *IMEventStore) ListSinceID(sinceID int64, limit int) ([]IMEvent, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 || limit > 500 {
		limit = 100
	}

	out := make([]IMEvent, 0, limit)
	var nextCursor int64 = sinceID
	for _, e := range s.events {
		if e.ID <= sinceID {
			continue
		}
		out = append(out, e)
		nextCursor = e.ID
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 {
		if s.nextID > 1 {
			nextCursor = s.nextID - 1
		}
	}
	return out, nextCursor
}
