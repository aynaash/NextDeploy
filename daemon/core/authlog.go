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
		corelogs.Error("Failed to create audit log directory", "path", path, "error", err)
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
		corelogs.Error("Failed to open audit log file path %s:%s", al.path, err)
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
			corelogs.Info("No audit log file found path:%s", al.path)
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
			corelogs.Warn("Failed to decode audit log entry error:%s", err)
			continue
		}
		if entry.Timestamp.After(since) {
			entries = append(entries, entry)
		}
	}

	return entries, nil

}
