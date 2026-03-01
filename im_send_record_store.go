package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type IMSendRecord struct {
	ClientMsgID string              `json:"client_msg_id"`
	Username    string              `json:"username"`
	Message     string              `json:"message"`
	Sent        bool                `json:"sent"`
	Attempts    int                 `json:"attempts"`
	Error       string              `json:"error,omitempty"`
	UpdatedAt   int64               `json:"updated_at"`
	Response    SendMessageResponse `json:"response,omitempty"`
}

type imSendRecordStoreFile struct {
	Records map[string]IMSendRecord `json:"records"`
}

type IMSendRecordStore struct {
	mu      sync.Mutex
	path    string
	records map[string]IMSendRecord
}

var (
	imSendRecordStoreOnce sync.Once
	imSendRecordStoreInst *IMSendRecordStore
	imSendRecordStoreErr  error
)

func getIMSendRecordStore() (*IMSendRecordStore, error) {
	imSendRecordStoreOnce.Do(func() {
		imSendRecordStoreInst, imSendRecordStoreErr = newIMSendRecordStore(dataFilePath("im_send_records.json"))
	})
	return imSendRecordStoreInst, imSendRecordStoreErr
}

func newIMSendRecordStore(path string) (*IMSendRecordStore, error) {
	s := &IMSendRecordStore{
		path:    path,
		records: map[string]IMSendRecord{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *IMSendRecordStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read im send record store failed: %w", err)
	}

	var f imSendRecordStoreFile
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("unmarshal im send record store failed: %w", err)
	}
	if f.Records != nil {
		s.records = f.Records
	}
	return nil
}

func (s *IMSendRecordStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create data dir failed: %w", err)
	}
	payload := imSendRecordStoreFile{Records: s.records}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal im send record store failed: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp im send record store failed: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename im send record store failed: %w", err)
	}
	return nil
}

func (s *IMSendRecordStore) Get(clientMsgID string) (IMSendRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := strings.TrimSpace(clientMsgID)
	rec, ok := s.records[id]
	return rec, ok
}

func (s *IMSendRecordStore) SaveSuccess(clientMsgID, username, message string, attempts int, resp *SendMessageResponse) error {
	id := strings.TrimSpace(clientMsgID)
	if id == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := IMSendRecord{
		ClientMsgID: id,
		Username:    strings.TrimSpace(username),
		Message:     message,
		Sent:        true,
		Attempts:    attempts,
		UpdatedAt:   time.Now().UnixMilli(),
	}
	if resp != nil {
		rec.Response = *resp
	}
	s.records[id] = rec
	return s.saveLocked()
}

func (s *IMSendRecordStore) SaveFailure(clientMsgID, username, message string, attempts int, errMsg string) error {
	id := strings.TrimSpace(clientMsgID)
	if id == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := IMSendRecord{
		ClientMsgID: id,
		Username:    strings.TrimSpace(username),
		Message:     message,
		Sent:        false,
		Attempts:    attempts,
		Error:       errMsg,
		UpdatedAt:   time.Now().UnixMilli(),
	}
	s.records[id] = rec
	return s.saveLocked()
}
