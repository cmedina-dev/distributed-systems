package main

import (
	"encoding/json"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log"
)

func main() {
	n := maelstrom.NewNode()
	n.Handle("echo", func(msg maelstrom.Message) error {
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		if err != nil {
			return err
		}
		body["type"] = "echo_ok"
		return n.Reply(msg, body)
	})
	err := n.Run()
	if err != nil {
		log.Fatal(err)
	}
}
