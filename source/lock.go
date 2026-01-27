package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/beevik/etree"
)

const (
	lockFilePath = "./lock.xml"
)

type Key struct {
	ID    int
	Value string
}

type Keylock struct {
	Keys []Key
}

func GetKeylock() (*Keylock, error) {

	keylock := &Keylock{Keys: []Key{}}

	// lock file doesn't exist
	_, err := os.Stat(lockFilePath)
	if os.IsNotExist(err) {
		return keylock, nil
	}

	// reading the xml content
	lockDocument := etree.NewDocument()
	err = lockDocument.ReadFromFile(lockFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed reading lock file: %w", err)
	}

	lock := lockDocument.SelectElement("lock")

	// parsing keys
	for _, keyElement := range lock.SelectElements("key") {

		keyIDstring := keyElement.SelectAttrValue("id", "")
		keyValue := keyElement.SelectAttrValue("value", "")

		// fmt.Println(keyValue)

		// check numeric value of id
		keyID, err := strconv.Atoi(keyIDstring)
		if err != nil {
			return nil, fmt.Errorf("invalid id '%s' in lock file: %w", keyIDstring, err)
		}

		// adding key to keylock object
		keylock.Keys = append(keylock.Keys, Key{ID: keyID, Value: keyValue})

	}

	return keylock, nil

}

func (keylock *Keylock) Save() error {

	lockDocument := etree.NewDocument()
	lockTag := lockDocument.CreateElement("lock")

	for _, key := range keylock.Keys {

		keyElement := lockTag.CreateElement("key")
		keyElement.CreateAttr("id", strconv.Itoa(key.ID))
		keyElement.CreateAttr("value", key.Value)

	}

	lockDocument.Indent(4)
	return lockDocument.WriteToFile(lockFilePath)

}

func (keylock *Keylock) AssureKey(value string) int {

	for _, key := range keylock.Keys {
		if key.Value == value {
			return key.ID
		}
	}

	newID := len(keylock.Keys) + 1
	keylock.Keys = append(keylock.Keys, Key{ID: newID, Value: value})

	return newID

}
