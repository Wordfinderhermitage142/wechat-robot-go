package wechat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ContextTokenData holds the persisted context token for a user conversation.
type ContextTokenData struct {
	Token     string `json:"token"`
	UpdatedAt string `json:"updated_at"`
}

// ContextTokenStore is the interface for persisting context tokens.
// Context tokens are required for sending messages and must be persisted
// to support outbound messages even after gateway restarts.
type ContextTokenStore interface {
	// Save saves a context token for a specific user.
	Save(userID string, token string) error
	// Load loads a context token for a specific user.
	// Returns empty string and nil error if no token exists.
	Load(userID string) (string, error)
	// Clear removes a context token for a specific user.
	Clear(userID string) error
	// ClearAll removes all stored context tokens.
	ClearAll() error
}

// FileContextTokenStore implements ContextTokenStore by persisting tokens to JSON files.
// Files are stored in a directory structure: {basePath}/{userID}.json
type FileContextTokenStore struct {
	basePath string
	mu       sync.RWMutex
	tokens   map[string]*ContextTokenData // in-memory cache
}

// NewFileContextTokenStore creates a new FileContextTokenStore.
// The directory will be created if it doesn't exist.
func NewFileContextTokenStore(basePath string) (*FileContextTokenStore, error) {
	// Ensure directory exists
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("create context token directory: %w", err)
	}

	store := &FileContextTokenStore{
		basePath: basePath,
		tokens:   make(map[string]*ContextTokenData),
	}

	// Load existing tokens into memory cache
	if err := store.loadAll(); err != nil {
		// Log but don't fail - tokens will be loaded on demand
		fmt.Printf("warning: failed to load existing tokens: %v\n", err)
	}

	return store, nil
}

// resolvePath returns the file path for a user ID.
func (f *FileContextTokenStore) resolvePath(userID string) string {
	return filepath.Join(f.basePath, userID+".json")
}

// loadAll loads all existing tokens from disk into memory.
func (f *FileContextTokenStore) loadAll() error {
	entries, err := os.ReadDir(f.basePath)
	if err != nil {
		return fmt.Errorf("read token directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		userID := entry.Name()[:len(entry.Name())-5] // Remove .json
		token, err := f.loadOne(userID)
		if err != nil {
			continue // Skip corrupted files
		}
		f.tokens[userID] = token
	}

	return nil
}

// loadOne loads a single token from disk.
func (f *FileContextTokenStore) loadOne(userID string) (*ContextTokenData, error) {
	path := f.resolvePath(userID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read token file: %w", err)
	}

	var token ContextTokenData
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}

	return &token, nil
}

// Save persists a context token for a user.
func (f *FileContextTokenStore) Save(userID string, token string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	data := &ContextTokenData{
		Token:     token,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	path := f.resolvePath(userID)
	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}

	// Update in-memory cache
	f.tokens[userID] = data
	return nil
}

// Load retrieves a context token for a user.
// Returns empty string and nil error if no token exists.
func (f *FileContextTokenStore) Load(userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("userID cannot be empty")
	}

	f.mu.RLock()
	if token, ok := f.tokens[userID]; ok && token != nil {
		f.mu.RUnlock()
		return token.Token, nil
	}
	f.mu.RUnlock()

	// Not in cache, try to load from disk
	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check after acquiring write lock
	if token, ok := f.tokens[userID]; ok && token != nil {
		return token.Token, nil
	}

	token, err := f.loadOne(userID)
	if err != nil {
		return "", err
	}
	if token == nil {
		return "", nil
	}

	f.tokens[userID] = token
	return token.Token, nil
}

// Clear removes a context token for a user.
func (f *FileContextTokenStore) Clear(userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Remove from cache
	delete(f.tokens, userID)

	// Remove from disk
	path := f.resolvePath(userID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove token file: %w", err)
	}

	return nil
}

// ClearAll removes all stored context tokens.
func (f *FileContextTokenStore) ClearAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for userID := range f.tokens {
		path := f.resolvePath(userID)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove token file %s: %w", path, err)
		}
	}

	f.tokens = make(map[string]*ContextTokenData)
	return nil
}

// Count returns the number of tokens currently stored in cache.
func (f *FileContextTokenStore) Count() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.tokens)
}

// CleanExpired removes tokens older than maxAge from both memory and disk.
// Returns the number of tokens removed.
func (f *FileContextTokenStore) CleanExpired(maxAge time.Duration) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now().UTC()
	removed := 0
	for userID, data := range f.tokens {
		updatedAt, err := time.Parse(time.RFC3339, data.UpdatedAt)
		if err != nil {
			// Cannot parse time, remove as stale
			path := f.resolvePath(userID)
			_ = os.Remove(path)
			delete(f.tokens, userID)
			removed++
			continue
		}
		if now.Sub(updatedAt) > maxAge {
			path := f.resolvePath(userID)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return removed, fmt.Errorf("remove expired token file %s: %w", path, err)
			}
			delete(f.tokens, userID)
			removed++
		}
	}
	return removed, nil
}

// MemoryContextTokenStore implements ContextTokenStore with in-memory storage only.
// This is useful for testing or when persistence is not required.
type MemoryContextTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*ContextTokenData
}

// NewMemoryContextTokenStore creates a new in-memory context token store.
func NewMemoryContextTokenStore() *MemoryContextTokenStore {
	return &MemoryContextTokenStore{
		tokens: make(map[string]*ContextTokenData),
	}
}

// Save persists a context token in memory.
func (m *MemoryContextTokenStore) Save(userID string, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tokens[userID] = &ContextTokenData{
		Token:     token,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return nil
}

// Load retrieves a context token from memory.
func (m *MemoryContextTokenStore) Load(userID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if token, ok := m.tokens[userID]; ok && token != nil {
		return token.Token, nil
	}
	return "", nil
}

// Clear removes a context token from memory.
func (m *MemoryContextTokenStore) Clear(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tokens, userID)
	return nil
}

// ClearAll removes all context tokens from memory.
func (m *MemoryContextTokenStore) ClearAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tokens = make(map[string]*ContextTokenData)
	return nil
}

// Count returns the number of tokens currently stored.
func (m *MemoryContextTokenStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tokens)
}

// CleanExpired removes tokens older than maxAge.
// Returns the number of tokens removed.
func (m *MemoryContextTokenStore) CleanExpired(maxAge time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	removed := 0
	for userID, data := range m.tokens {
		updatedAt, err := time.Parse(time.RFC3339, data.UpdatedAt)
		if err != nil {
			// Cannot parse time, remove as stale
			delete(m.tokens, userID)
			removed++
			continue
		}
		if now.Sub(updatedAt) > maxAge {
			delete(m.tokens, userID)
			removed++
		}
	}
	return removed, nil
}
