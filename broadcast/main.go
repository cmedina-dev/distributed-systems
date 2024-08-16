package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"hash/crc32"
	"log"
	"slices"
)

type MessageStore struct {
	store []float64
}

/*
1. Checksum is broadcast to node
2. Node's existing checksum does not match, sends `syn` message to node
3. Node receives `syn` message, tells sending node to broadcast its message store and responds with `ack`
4. Node receives `ack` message,
*/
func main() {
	n := maelstrom.NewNode()
	messages := MessageStore{}
	neighbors := make(map[string]interface{})

	n.Handle("syn", func(msg maelstrom.Message) error {
		curNode := n.ID()
		neighborNodes := neighbors[curNode].([]interface{})
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		if err != nil {
			return err
		}
		store := body["store"].([]interface{})

		for _, messageID := range store {
			if !slices.Contains(messages.store, messageID.(float64)) {
				messages.store = append(messages.store, messageID.(float64))
			}
		}

		for _, node := range neighborNodes {
			for _, message := range messages.store {
				res := map[string]any{
					"type":    "broadcast",
					"message": message,
				}
				err := n.Send(node.(string), res)
				if err != nil {
					return err
				}
			}
		}
		res := map[string]any{
			"type":  "syn_ok",
			"store": messages.store,
		}
		return n.Reply(msg, res)
	})

	n.Handle("syn_ok", func(msg maelstrom.Message) error {
		return nil
	})

	n.Handle("broadcast_ok", func(msg maelstrom.Message) error {
		return nil
	})

	n.Handle("broadcast", func(msg maelstrom.Message) error {
		ctx := context.Background()
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		if err != nil {
			return err
		}
		messageID := body["message"].(float64)

		// Tell node to propagate new value to neighbors
		if !slices.Contains(messages.store, messageID) {
			messages.store = append(messages.store, messageID)
			curNode := n.ID()
			neighborNodes := neighbors[curNode].([]interface{})
			propagation := make(map[string]any)
			propagation["type"] = "broadcast"
			propagation["message"] = messageID
			// Inform other nodes of the state of the data store in the event of a partition fault
			checksum, _ := getChecksum(messages.store)

			// Tell the originating node to sync with our data store first as we have fallen out of sync

			if body["checksum"] != nil && body["checksum"] != checksum {
				res := map[string]any{
					"type":  "syn",
					"store": messages.store,
				}
				rpc, err := n.SyncRPC(ctx, msg.Src, res)
				if err != nil {
					return err
				}
				var synResp map[string]any
				err = json.Unmarshal(rpc.Body, &synResp)
				if synResp["type"] != "syn_ok" {
					log.Fatal(`syn response not syn_ok`, synResp["type"])
				}
				// Inform originating node of this node's data store
				synStore := synResp["store"].([]interface{})
				for _, message := range synStore {
					if !slices.Contains(messages.store, messageID) {
						messages.store = append(messages.store, message.(float64))
					}
					res = map[string]any{
						"type":    "broadcast",
						"message": message,
					}
					err := n.Reply(msg, res)
					if err != nil {
						return err
					}
				}
			}
			propagation["checksum"] = checksum

			for _, node := range neighborNodes {
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

func getChecksum(store []float64) (string, error) {
	buffer := new(bytes.Buffer)
	for _, message := range store {
		err := binary.Write(buffer, binary.BigEndian, message)
		if err != nil {
			return "", err
		}
	}
	polyTable := crc32.MakeTable(crc32.IEEE)
	byteSlice := buffer.Bytes()
	checksum := crc32.Checksum(byteSlice, polyTable)
	state := fmt.Sprintf("%08x", checksum)
	return state, nil
}
