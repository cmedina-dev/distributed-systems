package main

import (
	"encoding/json"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log"
	"slices"
)

type MessageStore struct {
	store []float64
}

func main() {
	n := maelstrom.NewNode()
	messages := MessageStore{}
	neighbors := make(map[string]interface{})

	// Tell cluster to continue propagating until all values are present in all nodes
	n.Handle("broadcast_ok", func(msg maelstrom.Message) error {
		return nil
	})

	n.Handle("broadcast", func(msg maelstrom.Message) error {
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		if err != nil {
			return err
		}
		messageID := body["message"].(float64)
		if !slices.Contains(messages.store, messageID) {
			messages.store = append(messages.store, messageID)
			curNode := n.ID()
			neighborNodes := neighbors[curNode].([]interface{})
			for _, node := range neighborNodes {
				propagation := make(map[string]any)
				propagation["type"] = "broadcast"
				propagation["message"] = messageID
				err := n.Send(node.(string), propagation)
				if err != nil {
					return err
				}
			}
		}
		res := map[string]any{
			"type": "broadcast_ok",
		}
		return n.Reply(msg, res)
	})

	n.Handle("read", func(msg maelstrom.Message) error {
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		if err != nil {
			return err
		}
		res := map[string]any{
			"type":     "read_ok",
			"messages": messages.store,
		}
		body["messages"] = messages.store
		body["type"] = "read_ok"
		return n.Reply(msg, res)
	})

	n.Handle("topology", func(msg maelstrom.Message) error {
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		if err != nil {
			return err
		}
		topology := body["topology"].(map[string]interface{})
		for k, v := range topology {
			neighbors[k] = v
		}
		res := map[string]any{
			"type": "topology_ok",
		}
		return n.Reply(msg, res)
	})

	err := n.Run()
	if err != nil {
		log.Fatal(err)
	}
}
