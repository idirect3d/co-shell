// Author: L.Shuang
// Created: 2026-07-06
// Last Modified: 2026-07-07
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"sort"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

const (
	VaultAlgoAES         = "aes"
	VaultAlgoSM4         = "sm4"
	DefaultVaultTimeout  = 300
	DefaultPBKDF2IterAES = 600000
	DefaultPBKDF2IterSM4 = 10000
	VaultSaltLen         = 16
	VaultNonceLen        = 12
	VaultMetaBucket      = "vault_meta"
	VaultDataBucket      = "vault_data"
)

// VaultEntry stores tagged credentials for one entity.
// Tags map: key is the tag name (e.g., "user", "pwd", "token", "key", "email", "ip_addr"),
// value is encrypted. Tag names are NOT encrypted — only values are encrypted.
type VaultEntry struct {
	Name      string            `json:"name"`      // entry name, LLM references via @Tag:Name@
	Tags      map[string]string `json:"tags"`      // tag name → encrypted value
	Notes     string            `json:"notes"`     // encrypted notes
	Algorithm string            `json:"algorithm"` // "aes" or "sm4"
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type vaultMeta struct {
	Salt         []byte    `json:"salt"`
	Algorithm    string    `json:"algorithm"`
	PBKDF2Iter   int       `json:"pbkdf2_iter"`
	PasswordHash []byte    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

type VaultStore struct {
	db        *bbolt.DB
	meta      *vaultMeta
	masterKey []byte
	unlocked  bool
}

func NewVaultStore(db *bbolt.DB) *VaultStore {
	_ = db.Update(func(tx *bbolt.Tx) error {
		_, _ = tx.CreateBucketIfNotExists([]byte(VaultMetaBucket))
		_, _ = tx.CreateBucketIfNotExists([]byte(VaultDataBucket))
		return nil
	})
	return &VaultStore{db: db}
}

func (vs *VaultStore) IsInitialized() bool {
	if vs.meta != nil {
		return true
	}
	var found bool
	_ = vs.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(VaultMetaBucket))
		data := b.Get([]byte("config"))
		if data != nil {
			var m vaultMeta
			if json.Unmarshal(data, &m) == nil {
				vs.meta = &m
				found = true
			}
		}
		return nil
	})
	return found
}

func (vs *VaultStore) IsUnlocked() bool {
	return vs.unlocked && vs.masterKey != nil
}

func (vs *VaultStore) Init(masterPassword string, algorithm string) error {
	if vs.IsInitialized() {
		return fmt.Errorf("vault is already initialized")
	}
	if algorithm != VaultAlgoAES && algorithm != VaultAlgoSM4 {
		algorithm = VaultAlgoAES
	}
	salt := make([]byte, VaultSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("cannot generate salt: %w", err)
	}
	iter := DefaultPBKDF2IterAES
	if algorithm == VaultAlgoSM4 {
		iter = DefaultPBKDF2IterSM4
	}
	key := deriveKey(masterPassword, salt, iter, algorithm)
	pwHash := hashPassword(masterPassword, algorithm)
	meta := &vaultMeta{Salt: salt, Algorithm: algorithm, PBKDF2Iter: iter, PasswordHash: pwHash, CreatedAt: time.Now()}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("cannot marshal vault meta: %w", err)
	}
	err = vs.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(VaultMetaBucket)).Put([]byte("config"), data)
	})
	if err != nil {
		return fmt.Errorf("cannot save vault meta: %w", err)
	}
	vs.meta = meta
	vs.masterKey = key
	vs.unlocked = true
	return nil
}

func (vs *VaultStore) Unlock(masterPassword string) error {
	if !vs.IsInitialized() {
		return fmt.Errorf("vault is not initialized")
	}
	if vs.meta == nil {
		return fmt.Errorf("vault meta not loaded")
	}
	expectedHash := hashPassword(masterPassword, vs.meta.Algorithm)
	if !hmac.Equal(expectedHash, vs.meta.PasswordHash) {
		return fmt.Errorf("incorrect master password")
	}
	key := deriveKey(masterPassword, vs.meta.Salt, vs.meta.PBKDF2Iter, vs.meta.Algorithm)
	vs.masterKey = key
	vs.unlocked = true
	return nil
}

func (vs *VaultStore) Lock() {
	vs.masterKey = nil
	vs.unlocked = false
}

func (vs *VaultStore) GetAlgorithm() string {
	if vs.meta != nil {
		return vs.meta.Algorithm
	}
	return VaultAlgoAES
}

// Put stores an encrypted vault entry. Each tag value and Notes are encrypted independently.
func (vs *VaultStore) Put(entry *VaultEntry) error {
	if !vs.IsUnlocked() {
		return fmt.Errorf("vault is locked")
	}
	algo := entry.Algorithm
	if algo == "" {
		algo = vs.meta.Algorithm
	}
	entry.Algorithm = algo

	// Encrypt each tag value
	encTags := make(map[string]string, len(entry.Tags))
	for tagName, tagValue := range entry.Tags {
		encVal, err := encryptField(vs.masterKey, []byte(tagValue), algo)
		if err != nil {
			return fmt.Errorf("cannot encrypt tag %q: %w", tagName, err)
		}
		encTags[tagName] = bytesToString(encVal)
	}
	entry.Tags = encTags

	// Encrypt notes
	if entry.Notes != "" {
		encNotes, err := encryptField(vs.masterKey, []byte(entry.Notes), algo)
		if err != nil {
			return fmt.Errorf("cannot encrypt notes: %w", err)
		}
		entry.Notes = bytesToString(encNotes)
	}

	now := time.Now()
	entry.CreatedAt = now
	entry.UpdatedAt = now

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("cannot marshal entry: %w", err)
	}
	return vs.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(VaultDataBucket)).Put([]byte(entry.Name), data)
	})
}

// Get retrieves and decrypts a vault entry.
func (vs *VaultStore) Get(name string) (*VaultEntry, error) {
	if !vs.IsUnlocked() {
		return nil, fmt.Errorf("vault is locked")
	}
	var data []byte
	err := vs.db.View(func(tx *bbolt.Tx) error {
		d := tx.Bucket([]byte(VaultDataBucket)).Get([]byte(name))
		if d == nil {
			return fmt.Errorf("entry %q not found", name)
		}
		data = make([]byte, len(d))
		copy(data, d)
		return nil
	})
	if err != nil {
		return nil, err
	}

	var entry VaultEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("cannot unmarshal entry: %w", err)
	}
	algo := entry.Algorithm
	if algo == "" {
		algo = VaultAlgoAES
	}

	// Decrypt each tag value
	decTags := make(map[string]string, len(entry.Tags))
	for tagName, encVal := range entry.Tags {
		decVal, err := decryptField(vs.masterKey, stringToBytes(encVal), algo)
		if err != nil {
			return nil, fmt.Errorf("cannot decrypt tag %q: %w", tagName, err)
		}
		decTags[tagName] = string(decVal)
	}
	entry.Tags = decTags

	// Decrypt notes
	if entry.Notes != "" {
		decNotes, err := decryptField(vs.masterKey, stringToBytes(entry.Notes), algo)
		if err != nil {
			return nil, fmt.Errorf("cannot decrypt notes: %w", err)
		}
		entry.Notes = string(decNotes)
	}

	return &entry, nil
}

func (vs *VaultStore) List() ([]string, error) {
	var names []string
	err := vs.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(VaultDataBucket))
		return b.ForEach(func(k, _ []byte) error {
			names = append(names, string(k))
			return nil
		})
	})
	sort.Strings(names)
	return names, err
}

func (vs *VaultStore) Delete(name string) error {
	return vs.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(VaultDataBucket)).Delete([]byte(name))
	})
}

func (vs *VaultStore) ReEncrypt(masterPassword string, newAlgorithm string) error {
	if !vs.IsUnlocked() {
		return fmt.Errorf("vault is locked")
	}
	if newAlgorithm != VaultAlgoAES && newAlgorithm != VaultAlgoSM4 {
		return fmt.Errorf("unsupported algorithm: %s", newAlgorithm)
	}
	if newAlgorithm == vs.meta.Algorithm {
		return nil
	}
	names, err := vs.List()
	if err != nil {
		return err
	}
	for _, name := range names {
		entry, err := vs.Get(name)
		if err != nil {
			return fmt.Errorf("cannot get entry %q: %w", name, err)
		}
		entry.Algorithm = newAlgorithm
		// Re-encrypt each tag
		for tagName, tagValue := range entry.Tags {
			encVal, err := encryptField(vs.masterKey, []byte(tagValue), newAlgorithm)
			if err != nil {
				return fmt.Errorf("cannot encrypt tag %q for %q: %w", tagName, name, err)
			}
			entry.Tags[tagName] = bytesToString(encVal)
		}
		if entry.Notes != "" {
			encNotes, err := encryptField(vs.masterKey, []byte(entry.Notes), newAlgorithm)
			if err != nil {
				return fmt.Errorf("cannot encrypt notes for %q: %w", name, err)
			}
			entry.Notes = bytesToString(encNotes)
		}
		entry.UpdatedAt = time.Now()
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("cannot marshal entry %q: %w", name, err)
		}
		err = vs.db.Update(func(tx *bbolt.Tx) error {
			return tx.Bucket([]byte(VaultDataBucket)).Put([]byte(entry.Name), data)
		})
		if err != nil {
			return fmt.Errorf("cannot save entry %q: %w", name, err)
		}
	}
	newIter := DefaultPBKDF2IterAES
	if newAlgorithm == VaultAlgoSM4 {
		newIter = DefaultPBKDF2IterSM4
	}
	newKey := deriveKey(masterPassword, vs.meta.Salt, newIter, newAlgorithm)
	vs.meta.Algorithm = newAlgorithm
	vs.meta.PBKDF2Iter = newIter
	vs.masterKey = newKey
	metaData, err := json.Marshal(vs.meta)
	if err != nil {
		return fmt.Errorf("cannot marshal meta: %w", err)
	}
	return vs.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte(VaultMetaBucket)).Put([]byte("config"), metaData)
	})
}

func (vs *VaultStore) Info() string {
	initialized := vs.IsInitialized()
	if !initialized {
		return "vault: not initialized"
	}
	unlocked := vs.IsUnlocked()
	algo := vs.GetAlgorithm()
	count := 0
	_ = vs.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(VaultDataBucket))
		stats := b.Stats()
		count = stats.KeyN
		return nil
	})
	lockStatus := "locked"
	if unlocked {
		lockStatus = "unlocked"
	}
	return fmt.Sprintf("vault: %s, %d entries, algorithm: %s", lockStatus, count, algo)
}

// bytesToString and stringToBytes convert between []byte and string for encrypted data storage.
func bytesToString(b []byte) string { return string(b) }
func stringToBytes(s string) []byte { return []byte(s) }

// --- Encryption helpers ---

func encryptField(key, plaintext []byte, algorithm string) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	block, err := newCipherBlock(key, algorithm)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cannot create GCM: %w", err)
	}
	nonce := make([]byte, VaultNonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("cannot generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

func decryptField(key, data []byte, algorithm string) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) < VaultNonceLen+16 {
		return nil, fmt.Errorf("ciphertext too short")
	}
	block, err := newCipherBlock(key, algorithm)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cannot create GCM: %w", err)
	}
	nonce := data[:VaultNonceLen]
	ciphertext := data[VaultNonceLen:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}
	return plaintext, nil
}

func newCipherBlock(key []byte, algorithm string) (cipher.Block, error) {
	switch algorithm {
	case VaultAlgoAES:
		return aes.NewCipher(key)
	case VaultAlgoSM4:
		return NewSM4Cipher(key)
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}

func deriveKey(password string, salt []byte, iter int, algorithm string) []byte {
	var h func() hash.Hash
	var keyLen int
	switch algorithm {
	case VaultAlgoAES:
		h = sha256.New
		keyLen = 32
	case VaultAlgoSM4:
		h = NewSM3
		keyLen = 16
	default:
		h = sha256.New
		keyLen = 32
	}
	return pbkdf2([]byte(password), salt, iter, keyLen, h)
}

func hashPassword(password string, algorithm string) []byte {
	var h hash.Hash
	switch algorithm {
	case VaultAlgoAES:
		h = sha256.New()
	case VaultAlgoSM4:
		h = NewSM3()
	default:
		h = sha256.New()
	}
	h.Write([]byte(password))
	return h.Sum(nil)
}

// --- PBKDF2 ---

func pbkdf2(password, salt []byte, iter, keyLen int, h func() hash.Hash) []byte {
	hashLen := h().Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	dk := make([]byte, 0, numBlocks*hashLen)
	buf := make([]byte, 4)
	for block := 1; block <= numBlocks; block++ {
		binary.BigEndian.PutUint32(buf, uint32(block))
		mac := hmac.New(h, password)
		mac.Write(salt)
		mac.Write(buf)
		u := mac.Sum(nil)
		t := make([]byte, len(u))
		copy(t, u)
		for i := 2; i <= iter; i++ {
			mac.Reset()
			mac.Write(u)
			u = mac.Sum(nil)
			xorBytes(t, u)
		}
		dk = append(dk, t...)
	}
	return dk[:keyLen]
}

func xorBytes(dst, src []byte) {
	for i := range dst {
		if i < len(src) {
			dst[i] ^= src[i]
		}
	}
}

// VaultResolveResult holds the result of placeholder resolution.
type VaultResolveResult struct {
	MissingEntries map[string][]string // entry name → missing tags
}

// ResolveVaultPlaceholders replaces @Tag:EntryName@ placeholders in all string values
// of a map (recursive). Tag values are decrypted and injected. Missing entries/tags
// are reported in the result.
func (vs *VaultStore) ResolveVaultPlaceholders(args map[string]interface{}) (*VaultResolveResult, error) {
	result := &VaultResolveResult{MissingEntries: make(map[string][]string)}
	if !vs.IsUnlocked() {
		return result, nil
	}
	resolvePlaceholdersRecursive(args, vs, result)
	return result, nil
}

func resolvePlaceholdersRecursive(v interface{}, vs *VaultStore, result *VaultResolveResult) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, sub := range val {
			resolvePlaceholdersRecursive(sub, vs, result)
			if s, ok := sub.(string); ok {
				resolved, missing := resolveStringPlaceholders(s, vs)
				if len(missing) > 0 {
					for entryName, tags := range missing {
						if _, exists := result.MissingEntries[entryName]; !exists {
							result.MissingEntries[entryName] = tags
						} else {
							result.MissingEntries[entryName] = append(result.MissingEntries[entryName], tags...)
						}
					}
				}
				val[k] = resolved
			}
		}
	case []interface{}:
		for i, sub := range val {
			resolvePlaceholdersRecursive(sub, vs, result)
			if s, ok := sub.(string); ok {
				resolved, missing := resolveStringPlaceholders(s, vs)
				if len(missing) > 0 {
					for entryName, tags := range missing {
						if _, exists := result.MissingEntries[entryName]; !exists {
							result.MissingEntries[entryName] = tags
						} else {
							result.MissingEntries[entryName] = append(result.MissingEntries[entryName], tags...)
						}
					}
				}
				val[i] = resolved
			}
		}
	}
}

// resolveStringPlaceholders scans a string for @Tag:EntryName@ patterns.
// Returns the resolved string and a map of missing entries to their missing tags.
func resolveStringPlaceholders(s string, vs *VaultStore) (string, map[string][]string) {
	if !strings.Contains(s, "@") {
		return s, nil
	}
	result := s
	missing := make(map[string][]string)

	// Match @Tag:EntryName@ pattern
	for {
		atIdx := strings.Index(result, "@")
		if atIdx < 0 {
			break
		}
		// Check next character after @ is not a delimiter (skip plain @ in code)
		if atIdx+1 >= len(result) {
			break
		}
		// Find the ':' that separates tag from entry name
		colonIdx := strings.Index(result[atIdx+1:], ":")
		if colonIdx < 0 {
			// No colon — skip this @ and continue
			result = result[atIdx+1:]
			continue
		}
		colonIdx += atIdx + 1

		// Find the closing @ (must exist after colon)
		endAt := strings.Index(result[colonIdx+1:], "@")
		if endAt < 0 {
			// No closing @ — skip this @
			result = result[atIdx+1:]
			continue
		}
		endAt += colonIdx + 1

		tagName := result[atIdx+1 : colonIdx]
		entryName := result[colonIdx+1 : endAt]

		if tagName == "" || entryName == "" {
			result = result[endAt+1:]
			continue
		}

		// Look up the entry
		entry, err := vs.Get(entryName)
		if err != nil {
			// Entry not found — record as missing
			if _, exists := missing[entryName]; !exists {
				missing[entryName] = []string{tagName}
			} else {
				missing[entryName] = append(missing[entryName], tagName)
			}
			result = result[:atIdx] + "@" + tagName + ":" + entryName + "@" + result[endAt+1:]
			continue
		}

		// Look up the tag in the entry's Tags map
		val, exists := entry.Tags[tagName]
		if !exists {
			// Tag not found in this entry — record as missing tag
			if _, exists := missing[entryName]; !exists {
				missing[entryName] = []string{tagName}
			} else {
				missing[entryName] = append(missing[entryName], tagName)
			}
			result = result[:atIdx] + "@" + tagName + ":" + entryName + "@" + result[endAt+1:]
			continue
		}

		// Replace placeholder with the decrypted tag value
		result = result[:atIdx] + val + result[endAt+1:]
	}

	return result, missing
}

// HasPlaceholders checks if any string value contains vault placeholders (@Tag:EntryName@).
func HasPlaceholders(args map[string]interface{}) bool {
	return hasPlaceholdersRecursive(args)
}

func hasPlaceholdersRecursive(v interface{}) bool {
	switch val := v.(type) {
	case map[string]interface{}:
		for _, sub := range val {
			if hasPlaceholdersRecursive(sub) {
				return true
			}
		}
	case []interface{}:
		for _, sub := range val {
			if hasPlaceholdersRecursive(sub) {
				return true
			}
		}
	case string:
		// Look for @Tag:EntryName@ pattern
		return hasVaultPlaceholder(val)
	}
	return false
}

func hasVaultPlaceholder(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '@' && i+1 < len(s) {
			// Look for : after @
			for j := i + 1; j < len(s); j++ {
				if s[j] == ':' && j+1 < len(s) {
					// Look for closing @ after :
					for k := j + 1; k < len(s); k++ {
						if s[k] == '@' {
							return true
						}
						// Stop if we hit space/non-alnum (entry name chars)
						if !isValidEntryChar(s[k]) {
							break
						}
					}
				}
				// Stop if tag name is too long or has invalid chars
				if !isValidEntryChar(s[j]) {
					break
				}
			}
		}
	}
	return false
}

func isValidEntryChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
}

// MaskPlaceholders replaces @Tag:EntryName@ placeholders with **** for confirmation display.
func MaskPlaceholders(args map[string]interface{}) {
	maskPlaceholdersRecursive(args)
}

func maskPlaceholdersRecursive(v interface{}) {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, sub := range val {
			if s, ok := sub.(string); ok {
				val[k] = maskPlaceholdersInString(s)
			} else {
				maskPlaceholdersRecursive(sub)
			}
		}
	case []interface{}:
		for i, sub := range val {
			if s, ok := sub.(string); ok {
				val[i] = maskPlaceholdersInString(s)
			} else {
				maskPlaceholdersRecursive(sub)
			}
		}
	}
}

func maskPlaceholdersInString(s string) string {
	if !strings.Contains(s, "@") {
		return s
	}
	result := s
	for {
		atIdx := strings.Index(result, "@")
		if atIdx < 0 {
			break
		}
		colonIdx := strings.Index(result[atIdx+1:], ":")
		if colonIdx < 0 {
			break
		}
		colonIdx += atIdx + 1
		endAt := strings.Index(result[colonIdx+1:], "@")
		if endAt < 0 {
			break
		}
		endAt += colonIdx + 1
		result = result[:atIdx] + "****" + result[endAt+1:]
	}
	return result
}
