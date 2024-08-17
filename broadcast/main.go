package main

import (
	"bytes"
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
3. Node receives `syn` message, tells sending node to broadcast its message store
*/
func main() {
	n := maelstrom.NewNode()
	messages := MessageStore{}
	neighbors := make(map[string]interface{})

	/*
		1. Broadcast received by node
		2. Receiving node checks CRC hash of sender
		3. If hash exists and does not match, tell sender to send data store (`syn`) and receiver sends own data store
		4. Sender sends back full data store to receiver, sender passes receiver's data store to neighbors
		5. Neighbors check own store for equivalency. If not equal, repeat step 4.
	*/

	n.Handle("syn", func(msg maelstrom.Message) error {
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		if err != nil {
			return err
		}

		for _, message := range body["store"].([]interface{}) {
			if !slices.Contains(messages.store, message.(float64)) {
				messages.store = append(messages.store, message.(float64))
			}
		}

		res := map[string]any{
			"type":  "refresh",
			"store": messages.store,
		}
		err = n.Send(msg.Src, res)
		if err != nil {
			return err
		}
		return nil
	})

	n.Handle("refresh", func(msg maelstrom.Message) error {
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		curNode := n.ID()
		neighborNodes := neighbors[curNode].([]interface{})
		if err != nil {
			return err
		}

		for _, message := range body["store"].([]interface{}) {
			if !slices.Contains(messages.store, message.(float64)) {
				messages.store = append(messages.store, message.(float64))
			}
		}

		refreshBody := map[string]any{
			"type":  "refresh",
			"store": messages.store,
		}
		for _, node := range neighborNodes {
			if node != msg.Src {
				err := n.Send(node.(string), refreshBody)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})

	n.Handle("broadcast", func(msg maelstrom.Message) error {
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		curNode := n.ID()
		neighborNodes := neighbors[curNode].([]interface{})
		if err != nil {
			return err
		}
		messageID := body["message"].(float64)
		checksum, _ := getChecksum(messages.store)

		// Tell node to propagate new value to neighbors
		if !slices.Contains(messages.store, messageID) {
			messages.store = append(messages.store, messageID)
			checksum, _ = getChecksum(messages.store)
			broadcast := make(map[string]any)
			broadcast["type"] = "broadcast"
			broadcast["message"] = messageID
			broadcast["checksum"] = checksum

			for _, node := range neighborNodes {
				err := n.Send(node.(string), broadcast)
				if err != nil {
					return err
				}
			}
		}

		if body["checksum"] != nil && body["checksum"] != checksum {
			res := map[string]any{
				"type":  "syn",
				"store": messages.store,
			}
			err := n.Send(msg.Src, res)
			if err != nil {
				return err
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

	n.Handle("broadcast_ok", func(msg maelstrom.Message) error {
		return nil
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
