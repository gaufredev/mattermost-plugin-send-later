package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"
)

// Key-value layout:
//
//	msg_<id>      one scheduled message
//	user_<userID> the IDs of that person's scheduled messages (pending and failed)
//	due_index     pending messages sorted by send time, read by the job every few seconds
//
// Functions that write expect p.lock to be held. Reads (getMsg, userMsgs) may run without it:
// at worst they see a list a moment old.
const (
	keyMsgPrefix  = "msg_"
	keyUserPrefix = "user_"
	keyIndex      = "due_index"
)

const (
	statusPending = "pending"
	statusSending = "sending"
	statusFailed  = "failed"
	statusSent    = "sent" // only seen if removing a sent message failed; reconcile cleans it up
)

type Scheduled struct {
	ID          string   `json:"id"`
	UserID      string   `json:"user_id"`
	ChannelID   string   `json:"channel_id"`
	RootID      string   `json:"root_id,omitempty"`
	Message     string   `json:"message"`
	FileIDs     []string `json:"file_ids,omitempty"`
	FileNames   []string `json:"file_names,omitempty"`
	SendAt      int64    `json:"send_at"`
	CreateAt    int64    `json:"create_at"`
	UpdateAt    int64    `json:"update_at"`
	Status      string   `json:"status"`
	Error       string   `json:"error,omitempty"`
	ErrorDetail string   `json:"error_detail,omitempty"`

	// Attempts counts temporary failures; RetryAt is when the next try may happen (ms).
	Attempts int   `json:"attempts,omitempty"`
	RetryAt  int64 `json:"retry_at,omitempty"`

	// Confirm is the "scheduled for …" note shown when it was scheduled, so it can be updated.
	Confirm *listRef `json:"confirm,omitempty"`
}

// sentEntry remembers a message this plugin sent, so the person can find, edit or delete it.
type sentEntry struct {
	PostID    string `json:"post_id"`
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id,omitempty"`
	SentAt    int64  `json:"sent_at"`
}

const (
	keySentPrefix = "sent_"
	maxSent       = 50
	sentKeep      = 30 * 24 * time.Hour
)

// errCorrupt marks a stored value that can no longer be read. Such values are skipped and
// logged; reconcile rebuilds the lists without them.
var errCorrupt = errors.New("unreadable stored value")

type indexEntry struct {
	ID     string `json:"id"`
	SendAt int64  `json:"at"`
}

func (p *Plugin) kvGet(key string, v any) (bool, error) {
	data, appErr := p.API.KVGet(key)
	if appErr != nil {
		return false, fmt.Errorf("kv get %s: %s", key, appErr.Error())
	}
	if data == nil {
		return false, nil
	}
	if err := json.Unmarshal(data, v); err != nil {
		return false, fmt.Errorf("%w: %s: %v", errCorrupt, key, err)
	}
	return true, nil
}

func (p *Plugin) kvSet(key string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if appErr := p.API.KVSet(key, data); appErr != nil {
		return fmt.Errorf("kv set %s: %s", key, appErr.Error())
	}
	return nil
}

func (p *Plugin) kvDelete(key string) error {
	if appErr := p.API.KVDelete(key); appErr != nil {
		return fmt.Errorf("kv delete %s: %s", key, appErr.Error())
	}
	return nil
}

func (p *Plugin) getMsg(id string) (*Scheduled, error) {
	var m Scheduled
	found, err := p.kvGet(keyMsgPrefix+id, &m)
	if err != nil || !found {
		return nil, err
	}
	return &m, nil
}

// putMsg saves m and keeps the due index and the person's list in step with its status.
// The index is written first: it is what gets a message sent. reconcile repairs any gap,
// and a failed write asks for that repair on the next job run.
func (p *Plugin) putMsg(m *Scheduled) (err error) {
	defer p.repairOnError(&err)
	if err := p.kvSet(keyMsgPrefix+m.ID, m); err != nil {
		return err
	}

	if m.Status == statusPending || m.Status == statusSending {
		err = p.indexUpsert(m.ID, m.SendAt)
	} else {
		err = p.indexRemove(m.ID)
	}
	if err != nil {
		return err
	}

	ids, err := p.userIDs(m.UserID)
	if err != nil {
		return err
	}
	if !slices.Contains(ids, m.ID) {
		return p.kvSet(keyUserPrefix+m.UserID, append(ids, m.ID))
	}
	return nil
}

func (p *Plugin) deleteMsg(m *Scheduled) (err error) {
	defer p.repairOnError(&err)
	if err := p.kvDelete(keyMsgPrefix + m.ID); err != nil {
		return err
	}

	ids, err := p.userIDs(m.UserID)
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(ids), func(v string) bool { return v == m.ID })
	if len(kept) != len(ids) {
		if len(kept) == 0 {
			err = p.kvDelete(keyUserPrefix + m.UserID)
		} else {
			err = p.kvSet(keyUserPrefix+m.UserID, kept)
		}
		if err != nil {
			return err
		}
	}

	return p.indexRemove(m.ID)
}

// userIDs returns the IDs in the person's list. An unreadable list counts as empty and asks
// the job to rebuild the lists on its next run.
func (p *Plugin) repairOnError(err *error) {
	if *err != nil {
		p.needReconcile.Store(true)
	}
}

func (p *Plugin) userIDs(userID string) ([]string, error) {
	var ids []string
	if _, err := p.kvGet(keyUserPrefix+userID, &ids); err != nil {
		if !errors.Is(err, errCorrupt) {
			return nil, err
		}
		p.corrupted(err)
		return nil, nil
	}
	return ids, nil
}

// userMsgs returns the person's scheduled messages, sorted by send time. Unreadable ones are skipped.
func (p *Plugin) userMsgs(userID string) ([]*Scheduled, error) {
	ids, err := p.userIDs(userID)
	if err != nil {
		return nil, err
	}
	out := make([]*Scheduled, 0, len(ids))
	for _, id := range ids {
		m, err := p.getMsg(id)
		if err != nil {
			if errors.Is(err, errCorrupt) {
				p.corrupted(err)
				continue
			}
			return nil, err
		}
		if m != nil && m.Status != statusSent {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SendAt < out[j].SendAt })
	return out, nil
}

// readIndex returns the due index. An unreadable index counts as empty and asks the job to
// rebuild it on its next run (within pollInterval).
func (p *Plugin) readIndex() ([]indexEntry, error) {
	var idx []indexEntry
	if _, err := p.kvGet(keyIndex, &idx); err != nil {
		if !errors.Is(err, errCorrupt) {
			return nil, err
		}
		p.corrupted(err)
		return nil, nil
	}
	return idx, nil
}

// corrupted logs an unreadable value (once per key) and makes the next job run rebuild the lists.
func (p *Plugin) corrupted(err error) {
	if _, seen := p.reported.LoadOrStore(err.Error(), true); !seen {
		p.API.LogError("Found an unreadable stored value", "error", err.Error())
	}
	p.needReconcile.Store(true)
}

// sortIndex orders the due index by send time, then ID, so the same set always looks the same.
func sortIndex(idx []indexEntry) {
	slices.SortFunc(idx, func(a, b indexEntry) int {
		if c := cmp.Compare(a.SendAt, b.SendAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func (p *Plugin) indexUpsert(id string, sendAt int64) error {
	idx, err := p.readIndex()
	if err != nil {
		return err
	}
	for i := range idx {
		if idx[i].ID == id {
			if idx[i].SendAt == sendAt {
				return nil
			}
			idx = append(idx[:i], idx[i+1:]...)
			break
		}
	}
	idx = append(idx, indexEntry{ID: id, SendAt: sendAt})
	sortIndex(idx)
	return p.kvSet(keyIndex, idx)
}

func (p *Plugin) indexRemove(id string) error {
	idx, err := p.readIndex()
	if err != nil {
		return err
	}
	for i := range idx {
		if idx[i].ID == id {
			return p.kvSet(keyIndex, append(idx[:i], idx[i+1:]...))
		}
	}
	return nil
}

// dueIDs returns the IDs whose send time is at or before now (milliseconds).
func (p *Plugin) dueIDs(now int64) ([]string, error) {
	idx, err := p.readIndex()
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range idx {
		if e.SendAt > now {
			break
		}
		ids = append(ids, e.ID)
	}
	return ids, nil
}

// reconcile rebuilds the due index and the per-person lists from the messages themselves,
// so a half-finished write (or a lost update on a database replica) cannot lose a message
// or leave a ghost entry. It runs every few minutes from the job.
func (p *Plugin) reconcile() error {
	var keys []string
	const perPage = 1000
	for page := 0; ; page++ {
		batch, appErr := p.API.KVList(page, perPage)
		if appErr != nil {
			return fmt.Errorf("kv list: %s", appErr.Error())
		}
		keys = append(keys, batch...)
		if len(batch) < perPage {
			break
		}
	}

	var idx []indexEntry
	byUser := map[string][]string{}
	for _, key := range keys {
		if !strings.HasPrefix(key, keyMsgPrefix) {
			continue
		}
		m, err := p.getMsg(strings.TrimPrefix(key, keyMsgPrefix))
		if err != nil {
			if errors.Is(err, errCorrupt) {
				// It can't be sent or shown; log it once so an admin can look at the key.
				if _, seen := p.reported.LoadOrStore(key, true); !seen {
					p.API.LogError("Skipping an unreadable scheduled message", "key", key, "error", err.Error())
				}
				continue
			}
			return err
		}
		if m == nil {
			continue
		}
		if m.Status == statusSent {
			// Sent, but its removal failed earlier: finish the clean-up now.
			if err := p.kvDelete(key); err != nil {
				return err
			}
			continue
		}
		byUser[m.UserID] = append(byUser[m.UserID], m.ID)
		if m.Status == statusPending || m.Status == statusSending {
			idx = append(idx, indexEntry{ID: m.ID, SendAt: m.SendAt})
		}
	}

	sortIndex(idx)
	var current []indexEntry
	_, err := p.kvGet(keyIndex, &current)
	damaged := errors.Is(err, errCorrupt)
	if err != nil && !damaged {
		return err
	}
	if damaged || !slices.Equal(current, idx) {
		p.API.LogWarn("Repaired the send-later index", "before", len(current), "after", len(idx))
		if err := p.kvSet(keyIndex, idx); err != nil {
			return err
		}
	}

	for _, key := range keys {
		if !strings.HasPrefix(key, keyUserPrefix) {
			continue
		}
		userID := strings.TrimPrefix(key, keyUserPrefix)
		if _, ok := byUser[userID]; !ok {
			if err := p.kvDelete(key); err != nil {
				return err
			}
		}
	}
	for userID, want := range byUser {
		have, err := p.userIDs(userID)
		if err != nil {
			return err
		}
		if !sameSet(have, want) {
			if err := p.kvSet(keyUserPrefix+userID, want); err != nil {
				return err
			}
		}
	}
	return nil
}

// sentList returns the person's sent messages, newest first, dropping ones older than sentKeep.
func (p *Plugin) sentList(userID string, now int64) ([]sentEntry, error) {
	var list []sentEntry
	if _, err := p.kvGet(keySentPrefix+userID, &list); err != nil {
		if !errors.Is(err, errCorrupt) {
			return nil, err
		}
		p.API.LogError("Found an unreadable sent list; starting a new one", "error", err.Error())
		return nil, nil
	}
	return slices.DeleteFunc(list, func(e sentEntry) bool { return now-e.SentAt > sentKeep.Milliseconds() }), nil
}

// addSent records a sent message (newest first, at most maxSent).
func (p *Plugin) addSent(userID string, e sentEntry, now int64) error {
	list, err := p.sentList(userID, now)
	if err != nil {
		return err
	}
	list = append([]sentEntry{e}, list...)
	if len(list) > maxSent {
		list = list[:maxSent]
	}
	return p.kvSet(keySentPrefix+userID, list)
}

// removeSent forgets a sent message (after it was deleted).
func (p *Plugin) removeSent(userID string, postIDs ...string) error {
	var list []sentEntry
	if _, err := p.kvGet(keySentPrefix+userID, &list); err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(list), func(e sentEntry) bool { return slices.Contains(postIDs, e.PostID) })
	if len(kept) == len(list) {
		return nil
	}
	if len(kept) == 0 {
		return p.kvDelete(keySentPrefix + userID)
	}
	return p.kvSet(keySentPrefix+userID, kept)
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, v := range a {
		seen[v]++
	}
	for _, v := range b {
		if seen[v] == 0 {
			return false
		}
		seen[v]--
	}
	return true
}
