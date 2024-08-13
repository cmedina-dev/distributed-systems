package main

import (
	"encoding/json"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log"
)

type MessageStore struct {
	store []float64
}

func main() {
	n := maelstrom.NewNode()
	messages := MessageStore{}

	n.Handle("broadcast", func(msg maelstrom.Message) error {
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		if err != nil {
			return err
		}
		messageID := body["message"].(float64)
		messages.store = append(messages.store, messageID)
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
