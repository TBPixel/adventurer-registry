package registry

import (
	"fmt"
	"sync"
)

// CharacterStorage is the in-memory pq for user data
// reads and writes. CharacterStorage is ideal for testing
// due to being a volatile, in-memory pq.
type CharacterStorage struct {
	characters map[string]string
	mutex      sync.RWMutex
}

func NewUserStorage() *CharacterStorage {
	return &CharacterStorage{
		characters: make(map[string]string),
		mutex:      sync.RWMutex{},
	}
}

func (c *CharacterStorage) Names() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var names []string
	for name, _ := range c.characters {
		names = append(names, name)
	}

	return names
}

// Find a character by their name, retrieving their content
func (c *CharacterStorage) Find(name string) (string, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	character, exists := c.characters[name]
	if !exists {
		return "", fmt.Errorf("character with name %v does not exist", name)
	}

	return character, nil
}

// Create a character and return their name
func (c *CharacterStorage) Create(name, content string) (string, error) {
	_, err := c.Find(name)
	if err == nil {
		return "", fmt.Errorf("character with the name %s already exists", name)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.characters[name] = content

	return name, nil
}

// Update a character by name and return their name
func (c *CharacterStorage) Update(name, content string) (string, error) {
	_, err := c.Find(name)
	if err != nil {
		return "", fmt.Errorf("character with name %s does not exist", name)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.characters[name] = content

	return name, nil
}

// Delete a character by name
func (c *CharacterStorage) Delete(name string) error {
	_, err := c.Find(name)
	if err != nil {
		return fmt.Errorf("character with name %s does not exist", name)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.characters, name)

	return nil
}