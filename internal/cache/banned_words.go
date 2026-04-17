package cache

import (
	"strings"
	"sync"
)

type BannedWord struct {
	Word        string `db:"word"`
	PlatformVK  bool   `db:"platform_vk"`
	PlatformMax bool   `db:"platform_max"`
}

type BannedWords struct {
	db    *DB
	words []BannedWord
	index map[string]BannedWord

	mu sync.RWMutex
}

func NewBannedWordsDB(db *DB) (*BannedWords, error) {
	store := &BannedWords{
		db:    db,
		index: make(map[string]BannedWord),
	}

	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *BannedWords) List() ([]BannedWord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	words := make([]BannedWord, len(s.words))
	copy(words, s.words)
	return words, nil
}

func (s *BannedWords) Add(word string, platformVK bool, platformMax bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil || word == "" {
		return nil
	}
	if _, exists := s.index[word]; exists {
		return nil
	}

	result, err := s.db.Exec(`INSERT OR IGNORE INTO BannedWords (word, platform_vk, platform_max) VALUES (?, ?, ?)`, word, platformVK, platformMax)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return nil
	}

	bw := BannedWord{Word: word, PlatformVK: platformVK, PlatformMax: platformMax}
	s.words = append(s.words, bw)
	s.index[word] = bw
	return nil
}

func (s *BannedWords) Delete(word string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil || word == "" {
		return false, nil
	}
	if _, exists := s.index[word]; !exists {
		return false, nil
	}

	result, err := s.db.Exec(`DELETE FROM BannedWords WHERE word = ?`, word)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected > 0 {
		delete(s.index, word)
		for i, cachedWord := range s.words {
			if cachedWord.Word != word {
				continue
			}
			s.words = append(s.words[:i], s.words[i+1:]...)
			break
		}
	}

	return rowsAffected > 0, nil
}

func (s *BannedWords) Has(word string) (BannedWord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if word == "" {
		return BannedWord{}, false, nil
	}

	bw, exists := s.index[word]
	return bw, exists, nil
}

func (s *BannedWords) Contains(text string) (BannedWord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if text == "" {
		return BannedWord{}, false
	}

	for _, bw := range s.words {
		if strings.Contains(text, bw.Word) {
			return bw, true
		}
	}

	return BannedWord{}, false
}

func (s *BannedWords) load() error {
	if s.db == nil {
		return nil
	}

	var words []BannedWord
	if err := s.db.Select(&words, `SELECT word, platform_vk, platform_max FROM BannedWords ORDER BY row_id ASC`); err != nil {
		return err
	}

	s.words = words
	s.index = make(map[string]BannedWord, len(words))
	for _, bw := range words {
		s.index[bw.Word] = bw
	}

	return nil
}
