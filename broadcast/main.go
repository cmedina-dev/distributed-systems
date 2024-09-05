package main

import (
	"encoding/json"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type MessageStore struct {
	mu       sync.RWMutex
	store    []float64
	storeSet map[float64]struct{}
	batch    []float64
}

type VectorClock struct {
	mu    sync.RWMutex
	clock map[string]*atomic.Uint64
}

type MaelstromNode struct {
	n           *maelstrom.Node
	messages    MessageStore
	peers       []string
	vectorClock VectorClock
}

func getRandomPeers(peers []string) []string {
	length := len(peers)
	if length == 0 {
		return nil
	}
	selectedPeers := make([]string, 0)
	selectedPeersSet := make(map[int]struct{})
	selectedPeerCount := math.Floor(float64(length) / 2)
	if selectedPeerCount == 0 {
		selectedPeerCount = 1
	}
	for len(selectedPeers) < int(selectedPeerCount) {
		peer := rand.Intn(length)
		if _, exists := selectedPeersSet[peer]; !exists {
			selectedPeers = append(selectedPeers, peers[peer])
			selectedPeersSet[peer] = struct{}{}
		}
	}
	return selectedPeers
}

func (m *MaelstromNode) pull(nodeName string, version uint64) {
	msg := map[string]any{}
	msg["type"] = "pull"
	msg["version"] = version
	_ = m.n.RPC(nodeName, msg, func(res maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(res.Body, &body); err != nil {
			return nil
		}
		store := body["message"].([]interface{})
		m.messages.mu.Lock()
		for _, v := range store {
			messageID := v.(float64)
			if _, exists := m.messages.storeSet[messageID]; !exists {
				m.messages.store = append(m.messages.store, messageID)
				m.messages.storeSet[messageID] = struct{}{}
			}
		}
		m.messages.mu.Unlock()
		return nil
	})
}

func (m *MaelstromNode) pullAll() {
	peers := m.peers
	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			msg := map[string]any{}
			msg["type"] = "pull"
			// Load the peer's vector version and retrieve the peer node's deltas
			msg["version"] = float64(m.vectorClock.clock[peer].Load())
			// Use async RPC to prevent blocking, lower message count, and improve latency
			_ = m.n.RPC(p, msg, func(res maelstrom.Message) error {
				var body map[string]any
				if err := json.Unmarshal(res.Body, &body); err != nil {
					return nil
				}
				store := body["message"].([]interface{})
				m.messages.mu.Lock()
				for _, v := range store {
					messageID := v.(float64)
					if _, exists := m.messages.storeSet[messageID]; !exists {
						m.messages.store = append(m.messages.store, messageID)
						m.messages.storeSet[messageID] = struct{}{}
					}
				}
				m.messages.mu.Unlock()
				return nil
			})
		}(peer)
	}
}

// gossip distributes a given message to a list of the node's peers
func (m *MaelstromNode) gossip(body map[string]any) {
	peers := getRandomPeers(m.peers)
	message := body["message"].(float64)
	heartbeat := body["heartbeat"]
	if heartbeat == nil {
		heartbeat = float64(1)
	}
	if len(peers) == 0 {
		return
	}

	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			msg := map[string]any{}
			msg["type"] = "broadcast"
			msg["message"] = message
			msg["heartbeat"] = heartbeat.(float64)
			clockCopy := make(map[string]float64)
			m.vectorClock.mu.RLock()
			for k, v := range m.vectorClock.clock {
				clockCopy[k] = float64(v.Load())
			}
			m.vectorClock.mu.RUnlock()
			msg["clock"] = clockCopy
			err := m.n.Send(p, msg)
			if err != nil {
				return
			}
		}(peer)
	}
}

func main() {
	node := MaelstromNode{
		n: maelstrom.NewNode(),
		messages: MessageStore{
			store:    make([]float64, 0),
			storeSet: make(map[float64]struct{}),
			batch:    make([]float64, 0),
		},
		peers: make([]string, 0),
		vectorClock: VectorClock{
			clock: make(map[string]*atomic.Uint64),
		},
	}

	// Periodically send out vector clock to confirm deltas with peers
	ticker := time.NewTicker(125 * time.Millisecond)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case _ = <-ticker.C:
				// Batch messaging to limit network overhead
				for _, message := range node.messages.batch {
					body := make(map[string]any)
					clockCopy := make(map[string]float64)
					node.vectorClock.mu.RLock()
					for k, v := range node.vectorClock.clock {
						clockCopy[k] = float64(v.Load())
					}
					node.vectorClock.mu.RUnlock()
					body["clock"] = clockCopy
					body["message"] = message
					go node.gossip(body)
				}
				node.messages.batch = make([]float64, 0)
				// Reconcile differences in versions
				node.pullAll()
			}
		}
	}()

	node.n.Handle("pull", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}
		version := body["version"].(float64)
		node.vectorClock.mu.RLock()
		if _, ok := node.vectorClock.clock[node.n.ID()]; !ok {
			node.vectorClock.mu.RUnlock()
			return nil
		}
		if version == float64(node.vectorClock.clock[node.n.ID()].Load()) {
			node.vectorClock.mu.RUnlock()
			return nil
		}
		node.vectorClock.mu.RUnlock()
		deltas := make([]float64, 0)
		for i := int(version); i < len(node.messages.store); i++ {
			deltas = append(deltas, node.messages.store[i])
		}
		node.messages.mu.RLock()
		body["message"] = deltas
		node.messages.mu.RUnlock()
		return node.n.Reply(msg, body)
	})

	node.n.Handle("broadcast", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}
		if body["heartbeat"] != nil {
			heartbeat := body["heartbeat"].(float64)
			if heartbeat == 0 {
				return nil
			}
			heartbeat -= 1
			body["heartbeat"] = heartbeat
		}
		if body["clock"] != nil {
			clock := body["clock"].(map[string]interface{})
			for clockNodeName, clockNodeValue := range clock {
				node.vectorClock.mu.RLock()
				if _, exists := node.vectorClock.clock[clockNodeName]; exists {
					clockFloat := uint64(clockNodeValue.(float64))
					curNodeClockVal := node.vectorClock.clock[clockNodeName].Load()
					node.vectorClock.mu.RUnlock()
					if clockFloat > curNodeClockVal {
						node.vectorClock.clock[clockNodeName].Swap(clockFloat)
						// Because this is grow-only, we only need the difference in vector version between two nodes
						// N1: { 3, 1, 0 }, N2: { 1, 1, 0 } means N2 needs to pull only the most recent two elements
						if clockNodeName != msg.Src {
							node.pull(clockNodeName, curNodeClockVal)
						}
					}
				} else {
					node.vectorClock.mu.RUnlock()
					node.vectorClock.mu.Lock()
					node.vectorClock.clock[clockNodeName] = new(atomic.Uint64)
					node.vectorClock.clock[clockNodeName].Store(uint64(clockNodeValue.(float64)))
					node.vectorClock.mu.Unlock()
				}
			}
			node.vectorClock.clock[node.n.ID()].Add(1)
		}
		message := body["message"].(float64)
		res := map[string]any{}
		res["type"] = "broadcast_ok"
		node.messages.mu.RLock()
		_, exists := node.messages.storeSet[message]
		node.messages.mu.RUnlock()
		if exists {
			// This lowers the number of message transmissions
			// because nodes should not handle broadcast_ok responses
			if msg.Src[0] == 'c' {
				return node.n.Reply(msg, res)
			}
			return nil
		}

		node.messages.mu.Lock()
		node.messages.batch = append(node.messages.batch, message)
		node.messages.store = append(node.messages.store, message)
		node.messages.storeSet[message] = struct{}{}
		node.messages.mu.Unlock()

		if msg.Src[0] == 'c' {
			return node.n.Reply(msg, res)
		}
		return nil
	})

	node.n.Handle("read", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}
		res := map[string]any{}
		res["type"] = "read_ok"
		node.messages.mu.RLock()
		res["messages"] = node.messages.store
		node.messages.mu.RUnlock()
		return node.n.Reply(msg, res)
	})

	node.n.Handle("topology", func(msg maelstrom.Message) error {
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}
		topology := body["topology"].(map[string]interface{})[node.n.ID()].([]interface{})
		node.peers = make([]string, len(topology))
		for i, peer := range topology {
			node.peers[i] = peer.(string)
			node.vectorClock.clock[peer.(string)] = new(atomic.Uint64)
			node.vectorClock.clock[peer.(string)].Store(0)
		}
		res := map[string]any{}
		res["type"] = "topology_ok"
		return node.n.Reply(msg, res)
	})

	err := node.n.Run()
	if err != nil {
		log.Fatal(err)
	}
}
