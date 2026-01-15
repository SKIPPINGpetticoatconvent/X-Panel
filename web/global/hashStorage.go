package global

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sync"
	"time"
)

type HashEntry struct {
	Hash      string
	Value     string
	Timestamp time.Time
}

type HashStorage struct {
	mu         sync.RWMutex
	Data       map[string]HashEntry
	Expiration time.Duration
}

func NewHashStorage(expiration time.Duration) *HashStorage {
	return &HashStorage{
		Data:       make(map[string]HashEntry),
		Expiration: expiration,
	}
}

func (h *HashStorage) SaveHash(query string) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	hash := sha256.Sum256([]byte(query))
	hashString := hex.EncodeToString(hash[:])

	entry := HashEntry{
		Hash:      hashString,
		Value:     query,
		Timestamp: time.Now(),
	}

	h.Data[hashString] = entry

	return hashString
}

func (h *HashStorage) GetValue(hash string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entry, exists := h.Data[hash]

	return entry.Value, exists
}

func (h *HashStorage) IsHash(hash string) bool {
	match, _ := regexp.MatchString("^[a-f0-9]{64}$", hash)
	return match
}

func (h *HashStorage) RemoveExpiredHashes() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()

	for hash, entry := range h.Data {
		if now.Sub(entry.Timestamp) > h.Expiration {
			delete(h.Data, hash)
		}
	}
}

func (h *HashStorage) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.Data = make(map[string]HashEntry)
}
