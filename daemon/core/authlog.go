package core

import (
	"encoding/json"
	"nextdeploy/shared"
	"os"
	"sync"
	"time"
)

type AuditLog struct {
	path string
	mu   sync.Mutex
}

func NewAuditLog(path string) (*AuditLog, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}

	return &AuditLog{
		path: path,
	}, nil
}

func (al *AuditLog) AddEntry(entry shared.AuditLogEntry) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	file, err := os.OpenFile(al.path+"/audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.Encode(entry)

	return nil

}

func (al *AuditLog) Query(since time.Time) ([]shared.AuditLogEntry, error) {
	al.mu.Lock()
	defer al.mu.Unlock()

	file, err := os.Open(al.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []shared.AuditLogEntry{}, nil // No log file, return empty
		}
		return nil, err // Other errors
	}
	defer file.Close()

	var entries []shared.AuditLogEntry
	decoder := json.NewDecoder(file)

	for decoder.More() {
		var entry shared.AuditLogEntry
		if err := decoder.Decode((&entry)); err != nil {
			continue
		}
		if entry.Timestamp.After(since) {
			entries = append(entries, entry)
		}
	}

	return entries, nil

}
