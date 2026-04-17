package telegram

import (
	"fmt"
	"sort"
	"sync"
	"time"

	gogram "github.com/amarnathcjd/gogram/telegram"
)

const (
	ALBUM_COLLECTOR_DELAY       = 5000 * time.Millisecond
	ALBUM_COLLECTOR_CLEANUP_TTL = 15 * time.Minute
)

type albumEventType uint8

const (
	albumEventNew albumEventType = iota + 1
	albumEventEdit
)

type albumCollectorTimer interface {
	Stop() bool
}

type albumCollectorKey struct {
	chatID    int64
	groupedID int64
}

type albumCollectorState struct {
	initialEvent albumEventType
	published    bool
	lastHash     string
	generation   uint64
	flushTimer   *time.Timer
	cleanupTimer *time.Timer
	messages     map[int32]*gogram.NewMessage
}

type albumCollector struct {
	tg *Telegram
	mu sync.Mutex

	states map[albumCollectorKey]*albumCollectorState
}

func newAlbumCollector(tg *Telegram) *albumCollector {
	c := &albumCollector{
		tg:     tg,
		states: make(map[albumCollectorKey]*albumCollectorState),
	}

	return c
}

func (c *albumCollector) push(message *gogram.NewMessage, eventType albumEventType) {
	if c == nil || message == nil || message.Message == nil || message.Message.GroupedID == 0 {
		return
	}

	groupMessages := []*gogram.NewMessage{message}
	group, err := message.GetMediaGroup()
	if err != nil {
		c.tg.emitError(fmt.Errorf("op=fetch album failed, chat_id=%d, grouped_id=%d, message_id=%d, link=%s, err=%w", message.Chat.ID, message.Message.GroupedID, message.ID, message.Link(), err))
	} else if len(group) > 0 {
		groupMessages = make([]*gogram.NewMessage, 0, len(group))
		for i := range group {
			msg := group[i]
			groupMessages = append(groupMessages, &msg)
		}
	}

	key := albumCollectorKey{
		chatID:    message.Chat.ID,
		groupedID: message.Message.GroupedID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	state, ok := c.states[key]
	if !ok {
		state = &albumCollectorState{
			initialEvent: eventType,
			messages:     make(map[int32]*gogram.NewMessage),
		}
		c.states[key] = state
	}

	if !state.published && eventType == albumEventNew {
		state.initialEvent = albumEventNew
	}

	for _, msg := range groupMessages {
		if msg == nil {
			continue
		}
		state.messages[msg.ID] = msg
	}

	if state.cleanupTimer != nil {
		state.cleanupTimer.Stop()
	}
	if state.flushTimer != nil {
		state.flushTimer.Stop()
	}

	state.generation++
	generation := state.generation

	state.flushTimer = time.AfterFunc(ALBUM_COLLECTOR_DELAY, func() {
		c.flush(key, generation)
	})

}

func (c *albumCollector) flush(key albumCollectorKey, generation uint64) {
	if c == nil {
		return
	}

	c.mu.Lock()
	state, ok := c.states[key]
	if !ok || state.generation != generation {
		c.mu.Unlock()
		return
	}

	state.flushTimer = nil
	messages := sortedAlbumMessages(state.messages)
	c.mu.Unlock()

	if len(messages) == 0 {
		c.scheduleCleanup(key, generation)
		return
	}

	builtPost := c.tg.handleAlbumAsPost(&gogram.Album{
		GroupedID: key.groupedID,
		Messages:  messages,
	})
	if builtPost == nil {
		c.tg.emitError(fmt.Errorf("op=build album post, chatID=%d, groupedID=%d, messageID=%d, link=%s", key.chatID, key.groupedID, messages[0].ID, messages[0].Link()))
		c.scheduleCleanup(key, generation)
		return
	}

	newHash := builtPost.SourceHash()
	var emitNew bool
	var emitEdit bool

	c.mu.Lock()
	state, ok = c.states[key]
	if !ok || state.generation != generation {
		c.mu.Unlock()
		return
	}

	switch {
	case !state.published:
		state.published = true
		state.lastHash = newHash
		if state.initialEvent == albumEventEdit {
			emitEdit = true
		} else {
			emitNew = true
		}
	case state.lastHash != newHash:
		state.lastHash = newHash
		emitEdit = true
	}

	c.mu.Unlock()

	c.scheduleCleanup(key, generation)

	switch {
	case emitNew && c.tg.onPost != nil:
		c.tg.onPost(builtPost)
	case emitEdit && c.tg.onPostEdit != nil:
		c.tg.onPostEdit(builtPost)
	}
}

func (c *albumCollector) scheduleCleanup(key albumCollectorKey, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, ok := c.states[key]
	if !ok || state.generation != generation {
		return
	}

	if state.cleanupTimer != nil {
		state.cleanupTimer.Stop()
	}

	if ALBUM_COLLECTOR_CLEANUP_TTL <= 0 {
		delete(c.states, key)
		return
	}

	state.cleanupTimer = time.AfterFunc(ALBUM_COLLECTOR_CLEANUP_TTL, func() {
		c.cleanupState(key, generation)
	})
}

func (c *albumCollector) cleanupState(key albumCollectorKey, generation uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, ok := c.states[key]
	if !ok || state.generation != generation {
		return
	}

	if state.flushTimer != nil {
		state.flushTimer.Stop()
	}
	if state.cleanupTimer != nil {
		state.cleanupTimer.Stop()
	}

	delete(c.states, key)
}

func sortedAlbumMessages(messages map[int32]*gogram.NewMessage) []*gogram.NewMessage {
	result := make([]*gogram.NewMessage, 0, len(messages))
	for _, message := range messages {
		result = append(result, message)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}
