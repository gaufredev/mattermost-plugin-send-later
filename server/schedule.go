package main

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	maxPerUser     = 100
	maxFiles       = 10
	maxAhead       = 366 * 24 * time.Hour
	minAhead       = 5 * time.Second
	maxLateness    = 24 * time.Hour  // older than this when the job finally runs → not sent
	sendingExpiry  = 5 * time.Minute // a "sending" mark older than this means the server stopped mid-send
	reconcileEvery = time.Hour       // also right after any failed write or unreadable value
)

// codeTemporary marks a passing server problem (database timeout, 5xx): try again later.
const codeTemporary = "temporary"

// codeError carries a message key so callers can show it in the person's language.
type codeError struct {
	Code   string
	Args   []any
	Detail string
}

func (e *codeError) Error() string { return e.Code }

func failWith(code string, args ...any) error { return &codeError{Code: code, Args: args} }

// errorText turns any error into a sentence for the person.
func errorText(err error, ko bool) string {
	var ce *codeError
	if errors.As(err, &ce) {
		if strings.HasPrefix(ce.Code, "fail.") || strings.HasPrefix(ce.Code, "err.") {
			return tr(ko, ce.Code, ce.Args...)
		}
	}
	return tr(ko, "err.internal")
}

// lookupErr turns a failed lookup into a lasting reason (4xx: really not there or not allowed)
// or a temporary problem worth retrying.
func lookupErr(appErr *model.AppError, code string) error {
	if appErr.StatusCode >= 400 && appErr.StatusCode < 500 {
		return failWith(code)
	}
	return &codeError{Code: codeTemporary, Detail: appErr.Error()}
}

func isTemporary(err error) bool {
	return errorCode(err) == codeTemporary
}

func errorCode(err error) string {
	var ce *codeError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return "err.internal"
}

// whenInput is one way of choosing a send time.
type whenInput struct {
	Preset string `json:"preset,omitempty"`
	Date   string `json:"date,omitempty"`
	Time   string `json:"time,omitempty"`
	SendAt int64  `json:"send_at,omitempty"`
}

func (p *Plugin) resolveWhen(w whenInput, up userPrefs, now time.Time) (int64, error) {
	var sendAt int64
	switch {
	case w.Preset != "":
		for _, ps := range computePresets(now, up, p.getConfiguration()) {
			if ps.Key == w.Preset {
				sendAt = ps.SendAt
			}
		}
		if sendAt == 0 {
			return 0, failWith("err.past")
		}
	case w.Date != "" || w.Time != "":
		t, err := parseLocal(w.Date, w.Time, up.Loc)
		if err != nil {
			return 0, failWith("err.bad_time")
		}
		sendAt = model.GetMillisForTime(t)
	case w.SendAt > 0:
		sendAt = w.SendAt
	default:
		return 0, failWith("err.no_time")
	}
	return sendAt, checkTime(sendAt, now)
}

func checkTime(sendAt int64, now time.Time) error {
	if sendAt <= model.GetMillisForTime(now.Add(minAhead)) {
		return failWith("err.past")
	}
	if sendAt > model.GetMillisForTime(now.Add(maxAhead)) {
		return failWith("err.too_far")
	}
	return nil
}

// isCommand follows Mattermost's own rule: a message starting with "/" (but not "//") is
// run as a slash command, so it can't be posted later as text.
func isCommand(message string) bool {
	return strings.HasPrefix(message, "/") && !strings.HasPrefix(message, "//")
}

func checkMessage(message string, fileCount int) error {
	if strings.TrimSpace(message) == "" && fileCount == 0 {
		return failWith("err.empty")
	}
	if isCommand(message) {
		return failWith("err.command")
	}
	if utf8.RuneCountInString(message) > model.PostMessageMaxRunesV2 {
		return failWith("err.too_long")
	}
	if fileCount > maxFiles {
		return failWith("err.too_many_files")
	}
	return nil
}

// checkTarget makes sure the person can still post there (and upload, when there are files),
// with the same rules as posting by hand. It returns the thread's root ID.
func (p *Plugin) checkTarget(userID, channelID, rootID string, withFiles bool) (string, error) {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return "", lookupErr(appErr, "fail.user_inactive")
	}
	if user.DeleteAt != 0 {
		return "", failWith("fail.user_inactive")
	}
	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		return "", lookupErr(appErr, "fail.channel_gone")
	}
	if channel.DeleteAt != 0 {
		return "", failWith("fail.archived")
	}
	if _, appErr := p.API.GetChannelMember(channelID, userID); appErr != nil {
		return "", lookupErr(appErr, "fail.not_member")
	}
	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionCreatePost) {
		return "", failWith("fail.no_permission")
	}
	if withFiles && !p.API.HasPermissionToChannel(userID, channelID, model.PermissionUploadFile) {
		return "", failWith("fail.no_upload")
	}
	if channel.Type == model.ChannelTypeDirect {
		if err := p.checkDirectMessage(userID, channel); err != nil {
			return "", err
		}
	}
	if rootID == "" {
		return "", nil
	}
	root, appErr := p.API.GetPost(rootID)
	if appErr != nil {
		return "", lookupErr(appErr, "fail.thread_deleted")
	}
	if root.DeleteAt != 0 || root.ChannelId != channelID {
		return "", failWith("fail.thread_deleted")
	}
	if root.RootId != "" {
		return root.RootId, nil
	}
	return root.Id, nil
}

// checkDirectMessage applies "Enable users to open Direct Message channels with: Any member of the team".
func (p *Plugin) checkDirectMessage(userID string, channel *model.Channel) error {
	cfg := p.API.GetConfig()
	if cfg == nil || cfg.TeamSettings.RestrictDirectMessage == nil || *cfg.TeamSettings.RestrictDirectMessage != model.DirectMessageTeam {
		return nil
	}
	other := channel.GetOtherUserIdForDM(userID)
	if other == "" || other == userID {
		return nil
	}
	mine, appErr := p.API.GetTeamsForUser(userID)
	if appErr != nil {
		return lookupErr(appErr, "fail.dm_restricted")
	}
	theirs, appErr := p.API.GetTeamsForUser(other)
	if appErr != nil {
		return lookupErr(appErr, "fail.dm_restricted")
	}
	for _, a := range mine {
		for _, b := range theirs {
			if a.Id == b.Id {
				return nil
			}
		}
	}
	return failWith("fail.dm_restricted")
}

// checkFiles makes sure the files were uploaded by this person to this channel and are not used yet.
func (p *Plugin) checkFiles(userID, channelID string, fileIDs []string) ([]string, error) {
	if len(fileIDs) > maxFiles {
		return nil, failWith("err.too_many_files")
	}
	names := make([]string, 0, len(fileIDs))
	for _, id := range fileIDs {
		info, appErr := p.API.GetFileInfo(id)
		if appErr != nil {
			return nil, lookupErr(appErr, "fail.files")
		}
		if info.CreatorId != userID || info.PostId != "" || info.DeleteAt != 0 {
			return nil, failWith("fail.files")
		}
		if info.ChannelId != "" && info.ChannelId != channelID {
			return nil, failWith("fail.files")
		}
		names = append(names, info.Name)
	}
	return names, nil
}

type createInput struct {
	ChannelID string    `json:"channel_id"`
	RootID    string    `json:"root_id"`
	Message   string    `json:"message"`
	FileIDs   []string  `json:"file_ids"`
	When      whenInput `json:"when"`
}

func (p *Plugin) create(userID string, in createInput) (*Scheduled, error) {
	now := p.now()
	up := p.prefsFor(userID)
	in.FileIDs = slices.Compact(slices.Sorted(slices.Values(in.FileIDs)))

	if err := checkMessage(in.Message, len(in.FileIDs)); err != nil {
		return nil, err
	}
	sendAt, err := p.resolveWhen(in.When, up, now)
	if err != nil {
		return nil, err
	}
	rootID, err := p.checkTarget(userID, in.ChannelID, in.RootID, len(in.FileIDs) > 0)
	if err != nil {
		return nil, err
	}
	names, err := p.checkFiles(userID, in.ChannelID, in.FileIDs)
	if err != nil {
		return nil, err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	ids, err := p.userIDs(userID)
	if err != nil {
		return nil, err
	}
	if len(ids) >= maxPerUser {
		return nil, failWith("err.too_many", maxPerUser)
	}
	if len(in.FileIDs) > 0 {
		// A file can be attached to one post only, so it can't wait in two scheduled messages.
		mine, err := p.userMsgs(userID)
		if err != nil {
			return nil, err
		}
		for _, other := range mine {
			for _, f := range other.FileIDs {
				if slices.Contains(in.FileIDs, f) {
					return nil, failWith("err.file_in_use")
				}
			}
		}
	}

	millis := model.GetMillisForTime(now)
	m := &Scheduled{
		ID:        model.NewId(),
		UserID:    userID,
		ChannelID: in.ChannelID,
		RootID:    rootID,
		Message:   in.Message,
		FileIDs:   in.FileIDs,
		FileNames: names,
		SendAt:    sendAt,
		CreateAt:  millis,
		UpdateAt:  millis,
		Status:    statusPending,
		Confirm:   &listRef{PostID: model.NewId(), ChannelID: in.ChannelID, RootID: rootID, CreateAt: millis},
	}
	if err := p.putMsg(m); err != nil {
		// The person is told it failed, so it must not be sent later by the repair either.
		if derr := p.deleteMsg(m); derr != nil {
			p.API.LogError("Failed to undo a half-saved message", "id", m.ID, "error", derr.Error())
		}
		return nil, err
	}
	p.publishChanged(userID)
	return m, nil
}

type updateInput struct {
	Message *string    `json:"message,omitempty"`
	When    *whenInput `json:"when,omitempty"`
}

// update changes the text and/or time. Giving a failed message a new time schedules it again.
func (p *Plugin) update(userID, id string, in updateInput) (*Scheduled, error) {
	now := p.now()
	up := p.prefsFor(userID)

	p.lock.Lock()
	defer p.lock.Unlock()

	m, err := p.ownedMsg(userID, id)
	if err != nil {
		return nil, err
	}
	if m.Status == statusSending {
		return nil, failWith("err.busy")
	}
	original := *m

	if in.Message != nil {
		if err := checkMessage(*in.Message, len(m.FileIDs)); err != nil {
			return nil, err
		}
		m.Message = *in.Message
	}

	if in.When != nil {
		sendAt, err := p.resolveWhen(*in.When, up, now)
		if err != nil {
			return nil, err
		}
		m.SendAt = sendAt
		if m.Status == statusFailed {
			rootID, err := p.checkTarget(userID, m.ChannelID, m.RootID, len(m.FileIDs) > 0)
			if err != nil {
				return nil, err
			}
			if _, err := p.checkFiles(userID, m.ChannelID, m.FileIDs); err != nil {
				return nil, err
			}
			m.RootID = rootID
			m.Status = statusPending
			m.Error = ""
			m.ErrorDetail = ""
		}
		m.Attempts = 0
		m.RetryAt = 0
	}

	m.UpdateAt = model.GetMillisForTime(now)
	if err := p.putMsg(m); err != nil {
		// The person is told it failed, so put everything back as it was.
		if rerr := p.putMsg(&original); rerr != nil {
			p.API.LogError("Failed to undo a half-saved edit", "id", id, "error", rerr.Error())
		}
		return nil, err
	}
	if in.When != nil {
		p.refreshConfirm(m)
	}
	p.publishChanged(userID)
	return m, nil
}

// remove deletes a scheduled message and returns it so its text can be shown once more.
func (p *Plugin) remove(userID, id string) (*Scheduled, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	m, err := p.ownedMsg(userID, id)
	if err != nil {
		return nil, err
	}
	if m.Status == statusSending {
		return nil, failWith("err.busy")
	}
	if err := p.deleteMsg(m); err != nil {
		return nil, err
	}
	p.closeConfirm(m, tr(p.prefsFor(userID).Ko, "deleted"))
	p.publishChanged(userID)
	return m, nil
}

// sendNow sends a pending or failed message straight away. If it can't be sent, nothing
// changes (same time, same state, same note) and the reason is returned.
func (p *Plugin) sendNow(userID, id string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	m, err := p.ownedMsg(userID, id)
	if err != nil {
		return err
	}
	if m.Status == statusSending {
		return failWith("err.busy")
	}

	original := *m
	now := model.GetMillisForTime(p.now())
	m.Status = statusSending // if the server stops mid-send, this is reported instead of sent twice
	m.UpdateAt = now
	if err := p.putMsg(m); err != nil {
		return err
	}
	created, rootID, err := p.post(m)
	if err != nil {
		if perr := p.putMsg(&original); perr != nil {
			p.API.LogError("Failed to restore a message after a failed send", "id", id, "error", perr.Error())
		}
		return err
	}
	p.finishSent(m, created, rootID, now)
	return nil
}

func (p *Plugin) ownedMsg(userID, id string) (*Scheduled, error) {
	m, err := p.getMsg(id)
	if err != nil {
		return nil, err
	}
	if m == nil || m.UserID != userID {
		return nil, failWith("err.not_found")
	}
	return m, nil
}

// runDue is called by the cluster job and sends everything that is due.
func (p *Plugin) runDue() {
	p.lock.Lock()
	defer p.lock.Unlock()

	now := model.GetMillisForTime(p.now())
	if p.needReconcile.Swap(false) || now-p.lastReconcile.Load() >= reconcileEvery.Milliseconds() {
		next := now
		if err := p.reconcile(); err != nil {
			p.API.LogError("Failed to check the stored lists", "error", err.Error())
			next = now - reconcileEvery.Milliseconds() + time.Minute.Milliseconds() // try again in a minute
		}
		p.lastReconcile.Store(next)
	}

	ids, err := p.dueIDs(now)
	if err != nil {
		p.API.LogError("Failed to read the due index", "error", err.Error())
		return
	}

	for _, id := range ids {
		m, err := p.getMsg(id)
		if err != nil {
			p.API.LogError("Failed to read a scheduled message", "id", id, "error", err.Error())
			if errors.Is(err, errCorrupt) {
				_ = p.indexRemove(id)
			}
			continue
		}
		if m == nil {
			_ = p.indexRemove(id)
			continue
		}
		switch m.Status {
		case statusPending:
			if m.SendAt > now {
				// The index was behind (a half-finished write): the message's own time decides.
				_ = p.indexUpsert(m.ID, m.SendAt)
				continue
			}
			if m.RetryAt > now {
				continue // waiting after a temporary problem
			}
			p.deliver(m, now)
		case statusSending:
			if now-m.UpdateAt > sendingExpiry.Milliseconds() {
				_ = p.markFailed(m, "fail.maybe_sent", "")
			}
		default:
			_ = p.indexRemove(id)
		}
	}
}

// deliver is the job's send: it marks the message as sending, posts it, and records the
// result (sent, retry later, or failed with a direct message to the author).
func (p *Plugin) deliver(m *Scheduled, now int64) {
	if now-m.SendAt > maxLateness.Milliseconds() {
		code := "fail.too_late"
		if m.Attempts > 0 {
			code = "fail.kept_failing"
		}
		_ = p.markFailed(m, code, "")
		return
	}

	m.Status = statusSending
	m.UpdateAt = now
	if err := p.putMsg(m); err != nil {
		p.API.LogError("Failed to mark a message as sending", "id", m.ID, "error", err.Error())
		return
	}

	created, rootID, err := p.post(m)
	if err != nil {
		if isTemporary(err) {
			_ = p.retryLater(m, err)
			return
		}
		detail := ""
		var ce *codeError
		if errors.As(err, &ce) {
			detail = ce.Detail
		}
		_ = p.markFailed(m, errorCode(err), detail)
		return
	}
	p.finishSent(m, created, rootID, now)
}

// post sends m as its author right now, after checking they can still post there.
// It changes nothing in the store.
func (p *Plugin) post(m *Scheduled) (*model.Post, string, error) {
	rootID, err := p.checkTarget(m.UserID, m.ChannelID, m.RootID, len(m.FileIDs) > 0)
	if err == nil {
		_, err = p.checkFiles(m.UserID, m.ChannelID, m.FileIDs)
	}
	if err != nil {
		return nil, "", err
	}
	created, appErr := p.API.CreatePost(&model.Post{
		UserId:    m.UserID,
		ChannelId: m.ChannelID,
		RootId:    rootID,
		Message:   m.Message,
		FileIds:   m.FileIDs,
	})
	if appErr != nil {
		p.API.LogWarn("Failed to send a scheduled message", "id", m.ID, "error", appErr.Error())
		err := lookupErr(appErr, "fail.post_failed")
		var ce *codeError
		if errors.As(err, &ce) && !isTemporary(err) {
			ce.Detail = appErr.Error()
		}
		return nil, "", err
	}
	return created, rootID, nil
}

// finishSent records a sent message: it leaves the scheduled list, joins the sent list,
// and its note turns into "sent · go to message". The "sent" mark is saved before the
// record is removed, so a failed removal can't later be reported as "maybe not sent".
func (p *Plugin) finishSent(m *Scheduled, created *model.Post, rootID string, now int64) {
	m.Status = statusSent
	m.UpdateAt = now
	if err := p.putMsg(m); err != nil {
		p.API.LogError("Sent a scheduled message but could not mark it as sent", "id", m.ID, "error", err.Error())
	}
	if err := p.deleteMsg(m); err != nil {
		p.API.LogError("Sent a scheduled message but could not remove it", "id", m.ID, "error", err.Error())
	}
	if err := p.addSent(m.UserID, sentEntry{PostID: created.Id, ChannelID: m.ChannelID, RootID: rootID, SentAt: now}, now); err != nil {
		p.API.LogWarn("Could not add a message to the sent list", "id", m.ID, "error", err.Error())
	}
	up := p.prefsFor(m.UserID)
	text := tr(up.Ko, "confirm.sent", whenLabel(now, p.now(), up, true))
	if link := p.postLink(created.Id); link != "" {
		text += " [" + tr(up.Ko, "sent.go") + "](" + link + ")"
	}
	p.closeConfirm(m, text)
	p.publishChanged(m.UserID)
}

// retryDelay waits longer after each temporary failure: 1, 2, 5, 10, then every 30 minutes.
func retryDelay(attempts int) time.Duration {
	steps := []time.Duration{time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute}
	if attempts >= 1 && attempts <= len(steps) {
		return steps[attempts-1]
	}
	return 30 * time.Minute
}

// retryLater puts the message back as pending after a temporary problem; the job tries again
// after retryDelay, until it is more than maxLateness late.
func (p *Plugin) retryLater(m *Scheduled, err error) error {
	m.Attempts++
	m.RetryAt = model.GetMillisForTime(p.now().Add(retryDelay(m.Attempts)))
	p.API.LogWarn("A scheduled message will be retried", "id", m.ID, "attempt", m.Attempts, "error", err.Error())
	m.Status = statusPending
	if perr := p.putMsg(m); perr != nil {
		p.API.LogError("Failed to put a message back as pending", "id", m.ID, "error", perr.Error())
	}
	return err
}

func (p *Plugin) markFailed(m *Scheduled, code, detail string) error {
	m.Status = statusFailed
	m.Error = code
	m.ErrorDetail = detail
	m.UpdateAt = model.GetMillisForTime(p.now())
	if err := p.putMsg(m); err != nil {
		p.API.LogError("Failed to mark a message as failed", "id", m.ID, "error", err.Error())
	}
	up := p.prefsFor(m.UserID)
	p.closeConfirm(m, tr(up.Ko, "confirm.failed", tr(up.Ko, code)))
	p.publishChanged(m.UserID)
	p.notifyFailure(m)
	return &codeError{Code: code, Detail: detail}
}

func (p *Plugin) notifyFailure(m *Scheduled) {
	up := p.prefsFor(m.UserID)
	dm, appErr := p.API.GetDirectChannel(m.UserID, p.botID)
	if appErr != nil {
		p.API.LogWarn("Could not open a direct message to report a failed send", "user_id", m.UserID, "error", appErr.Error())
		return
	}
	text := tr(up.Ko, "dm.failed", p.channelLabel(m.ChannelID, up).Text, tr(up.Ko, m.Error)) +
		"\n\n" + quote(preview(m.Message, 500)) +
		"\n\n" + tr(up.Ko, "dm.failed.next")
	if _, appErr := p.API.CreatePost(&model.Post{UserId: p.botID, ChannelId: dm.Id, Message: text}); appErr != nil {
		p.API.LogWarn("Could not report a failed send", "user_id", m.UserID, "error", appErr.Error())
	}
}
