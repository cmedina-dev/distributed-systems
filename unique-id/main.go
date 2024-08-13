package main

import (
	"context"
	"encoding/json"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log"
)

type DataStore struct {
	storage *maelstrom.KV
	ctx     context.Context
}

/*
generateID reads the existing `id` value from the distributed key-value store,
*/
func generateID(store *DataStore) (int, error) {
	id, err := store.storage.ReadInt(store.ctx, "id")
	if err != nil {
		return -1, err
	}
	err = store.storage.CompareAndSwap(store.ctx, "id", id, id+1, false)
	for err != nil {
		id, err = store.storage.ReadInt(store.ctx, "id")
		if err != nil {
			return -1, err
		}
		err = store.storage.CompareAndSwap(store.ctx, "id", id, id+1, false)
	}
	return id, nil
}

func main() {
	n := maelstrom.NewNode()

	n.Handle("generate", func(msg maelstrom.Message) error {
		ctx := context.Background()
		/*
			We use a linearized key-value store for performance reasons.
			We only care that each id itself is unique.
		*/
		store := DataStore{ctx: ctx, storage: maelstrom.NewLinKV(n)}
		_ = store.storage.CompareAndSwap(ctx, "id", 0, 0, true)
		var body map[string]any
		err := json.Unmarshal(msg.Body, &body)
		if err != nil {
			return err
		}
		body["id"], err = generateID(&store)
		if err != nil {
			log.Fatal(err)
		}
		body["type"] = "generate_ok"
		return n.Reply(msg, body)
	})
	err := n.Run()
	if err != nil {
		log.Fatal(err)
	}
}
