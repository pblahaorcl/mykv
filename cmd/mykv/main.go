package main

import (
	"flag"
	"fmt"
	"log"

	"mykv"
)

const path = "data.db"

var collection, key, value, operation string

func main() {
	flag.StringVar(&operation, "o", "", "Operation to perform, e.g. put, get, delete")
	flag.StringVar(&collection, "c", "col", "Name of collection")
	flag.StringVar(&key, "k", "key", "Key to insert")
	flag.StringVar(&value, "v", "value", "Value to insert")
	flag.Parse()

	db, err := mykv.Open(path, mykv.DefaultOptions)
	defer db.Close()
	errAndDie(err)

	switch operation {
	case "put":
		tx := db.WriteTx()
		coll, err := tx.GetOrCreateCollection([]byte(collection))
		errAndDie(err)
		err = coll.Put([]byte(key), []byte(value))
		errAndDie(err)
		err = tx.Commit()
		errAndDie(err)
	case "get":
		tx := db.ReadTx()
		coll, err := tx.GetCollection([]byte(collection))
		errAndDie(err)
		if coll == nil {
			fmt.Printf("Collection '%s' not found\n", collection)
			errAndDie(tx.Commit())
			return
		}
		item, err := coll.Find([]byte(key))
		errAndDie(err)
		if item != nil {
			fmt.Printf("Found key '%s' with value '%s'\n", key, item.Value())
		} else {
			fmt.Printf("Key '%s' not found in collection %s\n", key, collection)
		}
		errAndDie(tx.Commit())
	case "delete":
		tx := db.WriteTx()
		coll, err := tx.GetCollection([]byte(collection))
		errAndDie(err)
		if coll == nil {
			errAndDie(tx.Commit())
			return
		}
		err = coll.Remove([]byte(key))
		errAndDie(err)
		err = tx.Commit()
		errAndDie(err)
	default:
		errAndDie(fmt.Errorf("unknown operation: %s", operation))
	}
}

func errAndDie(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
