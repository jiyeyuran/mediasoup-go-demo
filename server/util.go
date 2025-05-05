package main

import (
	"encoding/json"
	"sync"

	"github.com/jiyeyuran/mediasoup-go/v2"
)

type H = mediasoup.H

func Ref(b bool) *bool {
	return &b
}

func Clone(dst, source any) error {
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

type baseNotifier struct {
	mu       sync.RWMutex
	handlers []func()
}

func (n *baseNotifier) OnClose(handler func()) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.handlers = append(n.handlers, handler)
}

func (n *baseNotifier) notifyClosed() {
	n.mu.RLock()
	handlers := n.handlers
	n.mu.RUnlock()

	for _, handler := range handlers {
		handler()
	}
}
