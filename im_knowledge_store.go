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

type IMKnowledgeEntry struct {
	ID            string   `json:"id"`
	Title         string   `json:"title,omitempty"`
	Keywords      []string `json:"keywords"`
	Answer        string   `json:"answer"`
	ItemRef       string   `json:"item_ref,omitempty"`
	OrderStatuses []string `json:"order_statuses,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Enabled       bool     `json:"enabled"`
	Priority      int      `json:"priority,omitempty"`
	CreatedAt     int64    `json:"created_at"`
	UpdatedAt     int64    `json:"updated_at"`
}

type imKnowledgeStoreFile struct {
	Entries map[string]IMKnowledgeEntry `json:"entries"`
}

type IMKnowledgeStore struct {
	mu      sync.Mutex
	path    string
	entries map[string]IMKnowledgeEntry
}

var (
	imKnowledgeStoreOnce sync.Once
	imKnowledgeStoreInst *IMKnowledgeStore
	imKnowledgeStoreErr  error
)

func getIMKnowledgeStore() (*IMKnowledgeStore, error) {
	imKnowledgeStoreOnce.Do(func() {
		imKnowledgeStoreInst, imKnowledgeStoreErr = newIMKnowledgeStore(dataFilePath("im_knowledge.json"))
	})
	return imKnowledgeStoreInst, imKnowledgeStoreErr
}

func newIMKnowledgeStore(path string) (*IMKnowledgeStore, error) {
	s := &IMKnowledgeStore{
		path:    path,
		entries: map[string]IMKnowledgeEntry{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *IMKnowledgeStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read im knowledge store failed: %w", err)
	}

	var f imKnowledgeStoreFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("unmarshal im knowledge store failed: %w", err)
	}
	if f.Entries != nil {
		s.entries = f.Entries
	}
	return nil
}

func (s *IMKnowledgeStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create data dir failed: %w", err)
	}

	data, err := json.MarshalIndent(imKnowledgeStoreFile{Entries: s.entries}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal im knowledge store failed: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp im knowledge store failed: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename im knowledge store failed: %w", err)
	}
	return nil
}

func normalizeStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := strings.TrimSpace(item)
		if v == "" {
			continue
		}
		lower := strings.ToLower(v)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, v)
	}
	return out
}

func normalizeOrderStatus(status string) string {
	v := strings.TrimSpace(status)
	switch {
	case strings.Contains(v, "未下单"):
		return "未下单"
	case strings.Contains(v, "已拍"), strings.Contains(v, "待发货"):
		return "已拍下"
	case strings.Contains(v, "我已发货"), strings.Contains(v, "待收货"), strings.Contains(v, "已发货"):
		return "我已发货"
	case strings.Contains(v, "已收货"), strings.Contains(v, "交易成功"), strings.Contains(v, "已完成"):
		return "已收货"
	default:
		return ""
	}
}

func normalizeOrderStatuses(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		v := normalizeOrderStatus(item)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func normalizeKBEntry(in IMKnowledgeEntry, now int64) (IMKnowledgeEntry, error) {
	out := in
	out.ID = strings.TrimSpace(out.ID)
	out.Title = strings.TrimSpace(out.Title)
	out.Answer = strings.TrimSpace(out.Answer)
	out.ItemRef = strings.TrimSpace(out.ItemRef)
	out.Keywords = normalizeStringList(out.Keywords)
	out.Tags = normalizeStringList(out.Tags)
	out.OrderStatuses = normalizeOrderStatuses(out.OrderStatuses)
	if len(out.Keywords) == 0 {
		return IMKnowledgeEntry{}, fmt.Errorf("keywords is required")
	}
	if out.Answer == "" {
		return IMKnowledgeEntry{}, fmt.Errorf("answer is required")
	}
	if out.CreatedAt == 0 {
		out.CreatedAt = now
	}
	out.UpdatedAt = now
	return out, nil
}

func newKnowledgeID() string {
	return fmt.Sprintf("kb_%d", time.Now().UnixNano())
}

func (s *IMKnowledgeStore) Upsert(req *UpsertIMKnowledgeRequest) (IMKnowledgeEntry, error) {
	if req == nil {
		return IMKnowledgeEntry{}, fmt.Errorf("request is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	id := strings.TrimSpace(req.ID)
	var base IMKnowledgeEntry
	if id != "" {
		if existing, ok := s.entries[id]; ok {
			base = existing
		}
	} else {
		id = newKnowledgeID()
	}

	enabled := true
	if base.ID != "" {
		enabled = base.Enabled
	}
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	entry := IMKnowledgeEntry{
		ID:            id,
		Title:         req.Title,
		Keywords:      req.Keywords,
		Answer:        req.Answer,
		ItemRef:       req.ItemRef,
		OrderStatuses: req.OrderStatuses,
		Tags:          req.Tags,
		Enabled:       enabled,
		Priority:      req.Priority,
		CreatedAt:     base.CreatedAt,
	}
	normalized, err := normalizeKBEntry(entry, now)
	if err != nil {
		return IMKnowledgeEntry{}, err
	}

	s.entries[normalized.ID] = normalized
	if err := s.saveLocked(); err != nil {
		return IMKnowledgeEntry{}, err
	}
	return normalized, nil
}

func (s *IMKnowledgeStore) Delete(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.TrimSpace(id)
	if key == "" {
		return false, fmt.Errorf("id is required")
	}
	if _, ok := s.entries[key]; !ok {
		return false, nil
	}
	delete(s.entries, key)
	if err := s.saveLocked(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *IMKnowledgeStore) ListAll() []IMKnowledgeEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]IMKnowledgeEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}
