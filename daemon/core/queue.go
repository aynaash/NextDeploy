package core

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"nextdeploy/shared"
)

type CommandQueue struct {
	mu       sync.Mutex
	queue    []shared.AgentMessage
	filePath string
}

func NewCommandQueue(filePath string) *CommandQueue {
	cq := &CommandQueue{
		filePath: filePath,
	}

	// Load existing queue from file
	if data, err := os.ReadFile(filePath); err == nil {
		json.Unmarshal(data, &cq.queue)
	}

	return cq
}

func (cq *CommandQueue) Add(command shared.AgentMessage) error {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	command.Timestamp = time.Now().Unix()
	cq.queue = append(cq.queue, command)

	return cq.save()
}

func (cq *CommandQueue) GetNext() *shared.AgentMessage {
	cq.mu.Lock()
	defer cq.mu.Unlock()

	if len(cq.queue) == 0 {
		return nil
	}

	cmd := cq.queue[0]
	cq.queue = cq.queue[1:]

	if err := cq.save(); err != nil {
		return nil
	}

	return &cmd
}

func (cq *CommandQueue) save() error {
	data, err := json.Marshal(cq.queue)
	if err != nil {
		return err
	}

	return os.WriteFile(cq.filePath, data, 0644)
}

func (cq *CommandQueue) ProcessQueue(processor func(shared.AgentMessage) error) {
	for {
		if cmd := cq.GetNext(); cmd != nil {
			if err := processor(*cmd); err != nil {
				// Requeue if processing fails
				cq.Add(*cmd)
			}
		}
		time.Sleep(5 * time.Second)
	}
}
