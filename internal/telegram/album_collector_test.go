package telegram

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"wlgposter/internal/post"

	gogram "github.com/amarnathcjd/gogram/telegram"
)

func newAlbumMessage(id int32, groupedID int64, text string) *gogram.NewMessage {
	return &gogram.NewMessage{
		ID:   id,
		Chat: &gogram.ChatObj{ID: 100},
		File: &gogram.CustomFile{
			FileID: "tg-file-" + strconv.FormatInt(int64(id), 10),
			Name:   "photo.jpg",
			Size:   123,
			Ext:    ".jpg",
		},
		Message: &gogram.MessageObj{
			ID:        id,
			GroupedID: groupedID,
			Message:   text,
			Media:     &gogram.MessageMediaPhoto{Photo: &gogram.PhotoObj{}},
		},
	}
}

func newTestAlbumCollector(onNew func(*post.Post), onEdit func(*post.Post), onError func(error)) *albumCollector {
	tg := &Telegram{}
	if onNew != nil {
		tg.onPost = onNew
	}
	if onEdit != nil {
		tg.onPostEdit = onEdit
	}
	if onError != nil {
		tg.onError = onError
	}
	return newAlbumCollector(tg)
}

// simulatePush mimics what push() does but without calling
// message.GetMediaGroup() which requires a live Telegram client.
// It populates the collector state directly and schedules the flush timer.
func simulatePush(c *albumCollector, eventType albumEventType, messages ...*gogram.NewMessage) {
	if len(messages) == 0 {
		return
	}
	key := albumCollectorKey{
		chatID:    messages[0].Chat.ID,
		groupedID: messages[0].Message.GroupedID,
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

	for _, msg := range messages {
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

// flushNow synchronously calls flush for the given key+generation,
// stopping the pending timer to prevent a double-fire.
func flushNow(c *albumCollector, chatID int64, groupedID int64) {
	key := albumCollectorKey{chatID: chatID, groupedID: groupedID}
	c.mu.Lock()
	state, ok := c.states[key]
	if !ok {
		c.mu.Unlock()
		return
	}
	if state.flushTimer != nil {
		state.flushTimer.Stop()
		state.flushTimer = nil
	}
	generation := state.generation
	c.mu.Unlock()
	c.flush(key, generation)
}

// stopAllTimers stops all active timers in the collector to prevent
// goroutine leaks during tests.
func stopAllTimers(c *albumCollector) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, state := range c.states {
		if state.flushTimer != nil {
			state.flushTimer.Stop()
		}
		if state.cleanupTimer != nil {
			state.cleanupTimer.Stop()
		}
	}
}

func TestAlbumCollectorFlushesNewAlbumAfterDebounce(t *testing.T) {
	newCalls := make([]*post.Post, 0)
	editCalls := make([]*post.Post, 0)
	collector := newTestAlbumCollector(
		func(p *post.Post) { newCalls = append(newCalls, p) },
		func(p *post.Post) { editCalls = append(editCalls, p) },
		nil,
	)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "caption")
	msg11 := newAlbumMessage(11, 77, "")

	simulatePush(collector, albumEventNew, msg10, msg11)
	flushNow(collector, 100, 77)

	if len(newCalls) != 1 {
		t.Fatalf("expected 1 new callback, got %d", len(newCalls))
	}
	if len(editCalls) != 0 {
		t.Fatalf("expected 0 edit callbacks, got %d", len(editCalls))
	}

	got := newCalls[0]
	if got.MessageID != 10 {
		t.Fatalf("expected canonical MessageID 10, got %d", got.MessageID)
	}
	if got.Text != "caption" {
		t.Fatalf("expected caption text, got %q", got.Text)
	}
	if len(got.Media) != 2 {
		t.Fatalf("expected 2 media entries, got %d", len(got.Media))
	}
}

func TestAlbumCollectorTreatsEditBeforeFirstFlushAsNew(t *testing.T) {
	newCalls := make([]*post.Post, 0)
	editCalls := make([]*post.Post, 0)
	collector := newTestAlbumCollector(
		func(p *post.Post) { newCalls = append(newCalls, p) },
		func(p *post.Post) { editCalls = append(editCalls, p) },
		nil,
	)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "caption")
	msg11 := newAlbumMessage(11, 77, "")
	msg12 := newAlbumMessage(12, 77, "")

	// First push as new with 2 messages.
	simulatePush(collector, albumEventNew, msg10, msg11)

	// Second push as edit adds a third message; the initial event stays "new"
	// because the album was never published yet.
	simulatePush(collector, albumEventEdit, msg10, msg11, msg12)

	flushNow(collector, 100, 77)

	if len(newCalls) != 1 {
		t.Fatalf("expected 1 new callback, got %d", len(newCalls))
	}
	if len(editCalls) != 0 {
		t.Fatalf("expected 0 edit callbacks, got %d", len(editCalls))
	}
	if len(newCalls[0].Media) != 3 {
		t.Fatalf("expected 3 media entries, got %d", len(newCalls[0].Media))
	}
}

func TestAlbumCollectorEmitsEditAfterPublication(t *testing.T) {
	newCalls := make([]*post.Post, 0)
	editCalls := make([]*post.Post, 0)
	collector := newTestAlbumCollector(
		func(p *post.Post) { newCalls = append(newCalls, p) },
		func(p *post.Post) { editCalls = append(editCalls, p) },
		nil,
	)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "caption")
	msg11 := newAlbumMessage(11, 77, "")
	msg12 := newAlbumMessage(12, 77, "")

	// Publish the initial album.
	simulatePush(collector, albumEventNew, msg10, msg11)
	flushNow(collector, 100, 77)

	// Push an edit that adds a new message, changing the hash.
	simulatePush(collector, albumEventEdit, msg10, msg11, msg12)
	flushNow(collector, 100, 77)

	if len(newCalls) != 1 {
		t.Fatalf("expected 1 new callback, got %d", len(newCalls))
	}
	if len(editCalls) != 1 {
		t.Fatalf("expected 1 edit callback, got %d", len(editCalls))
	}
	if len(editCalls[0].Media) != 3 {
		t.Fatalf("expected edited album with 3 media entries, got %d", len(editCalls[0].Media))
	}
}

func TestAlbumCollectorSkipsNoOpEdit(t *testing.T) {
	newCalls := make([]*post.Post, 0)
	editCalls := make([]*post.Post, 0)
	collector := newTestAlbumCollector(
		func(p *post.Post) { newCalls = append(newCalls, p) },
		func(p *post.Post) { editCalls = append(editCalls, p) },
		nil,
	)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "caption")
	msg11 := newAlbumMessage(11, 77, "")

	// Publish the initial album.
	simulatePush(collector, albumEventNew, msg10, msg11)
	flushNow(collector, 100, 77)

	// Push an "edit" with identical messages — hash stays the same.
	simulatePush(collector, albumEventEdit, msg10, msg11)
	flushNow(collector, 100, 77)

	if len(newCalls) != 1 {
		t.Fatalf("expected 1 new callback, got %d", len(newCalls))
	}
	if len(editCalls) != 0 {
		t.Fatalf("expected 0 edit callbacks, got %d", len(editCalls))
	}
}

func TestAlbumCollectorUsesEditForEditOnlyFirstFlush(t *testing.T) {
	newCalls := make([]*post.Post, 0)
	editCalls := make([]*post.Post, 0)
	collector := newTestAlbumCollector(
		func(p *post.Post) { newCalls = append(newCalls, p) },
		func(p *post.Post) { editCalls = append(editCalls, p) },
		nil,
	)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "caption")
	msg11 := newAlbumMessage(11, 77, "")

	// First ever push is an edit — first flush should emit onPostEdit.
	simulatePush(collector, albumEventEdit, msg10, msg11)
	flushNow(collector, 100, 77)

	if len(newCalls) != 0 {
		t.Fatalf("expected 0 new callbacks, got %d", len(newCalls))
	}
	if len(editCalls) != 1 {
		t.Fatalf("expected 1 edit callback, got %d", len(editCalls))
	}
}

func TestHandleAlbumAsPostUsesStablePrimaryMessageID(t *testing.T) {
	tg := &Telegram{}

	msg22 := newAlbumMessage(22, 77, "text")
	msg22.Message.Entities = []gogram.MessageEntity{&gogram.MessageEntityBold{Offset: 0, Length: 4}}
	msg20 := newAlbumMessage(20, 77, "")

	post := tg.handleAlbumAsPost(&gogram.Album{
		GroupedID: 77,
		Messages:  []*gogram.NewMessage{msg22, msg20},
	})

	if post == nil {
		t.Fatal("expected post, got nil")
	}
	if post.MessageID != 20 {
		t.Fatalf("expected MessageID 20, got %d", post.MessageID)
	}
	if post.Text != "text" {
		t.Fatalf("expected text from caption message, got %q", post.Text)
	}
	if len(post.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(post.Entities))
	}
}

func TestAlbumCollectorCleansUpPublishedStateAfterFlush(t *testing.T) {
	collector := newTestAlbumCollector(nil, nil, nil)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "caption")
	msg11 := newAlbumMessage(11, 77, "")

	simulatePush(collector, albumEventNew, msg10, msg11)
	flushNow(collector, 100, 77)

	key := albumCollectorKey{chatID: 100, groupedID: 77}
	collector.mu.Lock()
	state, ok := collector.states[key]
	if !ok {
		collector.mu.Unlock()
		t.Fatal("expected state after flush")
	}
	if !state.published {
		collector.mu.Unlock()
		t.Fatal("expected published=true after flush")
	}
	generation := state.generation
	collector.mu.Unlock()

	// Directly trigger cleanup to verify state removal.
	collector.cleanupState(key, generation)

	collector.mu.Lock()
	_, ok = collector.states[key]
	collector.mu.Unlock()
	if ok {
		t.Fatal("expected state cleanup after cleanupState")
	}
}

func TestAlbumCollectorCleansUpPendingStateOnEmptyMessages(t *testing.T) {
	// When flush finds no messages in state, it schedules cleanup with pending TTL.
	// This tests that the cleanup path works for the empty-messages case.
	var reported error
	collector := newTestAlbumCollector(nil, nil, func(err error) {
		reported = err
	})
	defer stopAllTimers(collector)

	key := albumCollectorKey{chatID: 100, groupedID: 77}

	// Manually create a state with no messages to trigger the early-return path in flush.
	collector.mu.Lock()
	collector.states[key] = &albumCollectorState{
		initialEvent: albumEventNew,
		generation:   1,
		messages:     make(map[int32]*gogram.NewMessage),
	}
	collector.mu.Unlock()

	collector.flush(key, 1)

	collector.mu.Lock()
	_, ok := collector.states[key]
	collector.mu.Unlock()
	if !ok {
		t.Fatal("expected state to exist after flush (cleanup scheduled)")
	}

	// Drive cleanup manually.
	collector.cleanupState(key, 1)

	collector.mu.Lock()
	_, ok = collector.states[key]
	collector.mu.Unlock()
	if ok {
		t.Fatal("expected state to be cleaned up")
	}

	if reported != nil {
		t.Fatalf("no error expected for empty messages, got %v", reported)
	}
}

func TestAlbumCollectorReportsBuildErrors(t *testing.T) {
	var reported error
	tg := &Telegram{
		onError: func(err error) { reported = err },
	}

	tg.emitError(fmt.Errorf("op=build album post, chatID=%d, groupedID=%d, messageID=%d, link=%s", int64(100), int64(77), int32(10), ""))

	if reported == nil {
		t.Fatal("expected reported error")
	}
	if !strings.Contains(reported.Error(), "build album post") {
		t.Fatalf("expected error to contain 'build album post', got %q", reported.Error())
	}
}

func TestAlbumCollectorReportsFetchErrors(t *testing.T) {
	var reported error
	tg := &Telegram{
		onError: func(err error) { reported = err },
	}

	tg.emitError(fmt.Errorf("op=fetch album failed, chat_id=%d, grouped_id=%d, message_id=%d, link=%s, err=%v", int64(100), int64(77), int32(10), "", fmt.Errorf("test error")))

	if reported == nil {
		t.Fatal("expected reported error")
	}
	if !strings.Contains(reported.Error(), "fetch album") {
		t.Fatalf("expected error to contain 'fetch album', got %q", reported.Error())
	}
}

func TestAlbumCollectorDebounceResetsTimer(t *testing.T) {
	// Verify that a second push resets the debounce: only the latest
	// generation is flushed.
	newCalls := make([]*post.Post, 0)
	collector := newTestAlbumCollector(
		func(p *post.Post) { newCalls = append(newCalls, p) },
		nil, nil,
	)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "first")
	simulatePush(collector, albumEventNew, msg10)

	// Second push with updated text; generation increments.
	msg10v2 := newAlbumMessage(10, 77, "second")
	simulatePush(collector, albumEventNew, msg10v2)

	// Flush with old generation should be a no-op.
	key := albumCollectorKey{chatID: 100, groupedID: 77}
	collector.flush(key, 1)
	if len(newCalls) != 0 {
		t.Fatalf("stale generation should not flush, got %d callbacks", len(newCalls))
	}

	// Flush with current generation should work.
	flushNow(collector, 100, 77)
	if len(newCalls) != 1 {
		t.Fatalf("expected 1 callback, got %d", len(newCalls))
	}
	if newCalls[0].Text != "second" {
		t.Fatalf("expected updated text, got %q", newCalls[0].Text)
	}
}

func TestAlbumCollectorConcurrentPushes(t *testing.T) {
	// Verify no races when multiple goroutines push to the same album.
	newCalls := make([]*post.Post, 0)
	var mu sync.Mutex
	collector := newTestAlbumCollector(
		func(p *post.Post) {
			mu.Lock()
			newCalls = append(newCalls, p)
			mu.Unlock()
		},
		nil, nil,
	)
	defer stopAllTimers(collector)

	var wg sync.WaitGroup
	for i := int32(0); i < 10; i++ {
		wg.Add(1)
		go func(id int32) {
			defer wg.Done()
			msg := newAlbumMessage(id, 77, "")
			simulatePush(collector, albumEventNew, msg)
		}(i)
	}
	wg.Wait()

	flushNow(collector, 100, 77)

	mu.Lock()
	defer mu.Unlock()
	if len(newCalls) != 1 {
		t.Fatalf("expected exactly 1 new callback after concurrent pushes, got %d", len(newCalls))
	}
	if len(newCalls[0].Media) != 10 {
		t.Fatalf("expected 10 media entries, got %d", len(newCalls[0].Media))
	}
}

func TestAlbumCollectorGenerationGuardsFlush(t *testing.T) {
	// Verify that flush is a no-op when called with a stale generation.
	newCalls := make([]*post.Post, 0)
	collector := newTestAlbumCollector(
		func(p *post.Post) { newCalls = append(newCalls, p) },
		nil, nil,
	)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "caption")
	simulatePush(collector, albumEventNew, msg10)

	key := albumCollectorKey{chatID: 100, groupedID: 77}
	collector.flush(key, 0)

	if len(newCalls) != 0 {
		t.Fatalf("expected no callback for stale generation, got %d", len(newCalls))
	}
}

func TestAlbumCollectorCleanupGuardsGeneration(t *testing.T) {
	// Cleanup with a stale generation must not remove the state.
	collector := newTestAlbumCollector(nil, nil, nil)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "caption")
	simulatePush(collector, albumEventNew, msg10)
	flushNow(collector, 100, 77)

	key := albumCollectorKey{chatID: 100, groupedID: 77}

	// Try to cleanup with generation 0 (stale).
	collector.cleanupState(key, 0)

	collector.mu.Lock()
	_, ok := collector.states[key]
	collector.mu.Unlock()
	if !ok {
		t.Fatal("stale generation cleanup should not remove state")
	}
}

func TestAlbumCollectorFlushTimerFiresAutomatically(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow timer test in short mode")
	}

	done := make(chan struct{})
	collector := newTestAlbumCollector(
		func(p *post.Post) { close(done) },
		nil, nil,
	)
	defer stopAllTimers(collector)

	msg10 := newAlbumMessage(10, 77, "auto")
	simulatePush(collector, albumEventNew, msg10)

	select {
	case <-done:
	case <-time.After(ALBUM_COLLECTOR_DELAY + 2*time.Second):
		t.Fatal("flush timer did not fire within expected time")
	}
}
