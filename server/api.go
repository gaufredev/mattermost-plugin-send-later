package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

const maxBodyBytes = 1 << 20

type ctxHandler func(w http.ResponseWriter, r *http.Request, userID string)

func (p *Plugin) newRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/presets", p.withUser(p.apiPresets))
	mux.HandleFunc("GET /api/v1/scheduled", p.withUser(p.apiList))
	mux.HandleFunc("POST /api/v1/scheduled", p.withUser(p.apiCreate))
	mux.HandleFunc("PATCH /api/v1/scheduled/{id}", p.withUser(p.apiUpdate))
	mux.HandleFunc("DELETE /api/v1/scheduled/{id}", p.withUser(p.apiDelete))
	mux.HandleFunc("POST /api/v1/scheduled/{id}/send", p.withUser(p.apiSendNow))
	mux.HandleFunc("GET /api/v1/sent", p.withUser(p.apiSent))
	mux.HandleFunc("PATCH /api/v1/sent/{id}", p.withUser(p.apiSentEdit))
	mux.HandleFunc("DELETE /api/v1/sent/{id}", p.withUser(p.apiSentDelete))
	mux.HandleFunc("POST /action/{op}", p.withUser(p.handleAction))
	mux.HandleFunc("POST /dialog/{kind}", p.withUser(p.handleDialog))
	return mux
}

// withUser lets only logged-in requests through. Mattermost sets the header after checking the session.
func (p *Plugin) withUser(next ctxHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-Id")
		if userID == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next(w, r, userID)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func badRequest(w http.ResponseWriter, up userPrefs) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": tr(up.Ko, "err.internal"), "code": "err.bad_request"})
}

func (p *Plugin) writeError(w http.ResponseWriter, err error, ko bool) {
	status := http.StatusBadRequest
	code := errorCode(err)
	switch code {
	case "err.not_found", "err.sent_gone":
		status = http.StatusNotFound
	case "err.internal":
		status = http.StatusInternalServerError
		p.API.LogError("Request failed", "error", err.Error())
	}
	writeJSON(w, status, map[string]string{"error": errorText(err, ko), "code": code})
}

// itemView is a scheduled message as the web app shows it.
// whereView says where a message goes, for both lists in the web app.
type whereView struct {
	ChannelID    string `json:"channel_id"`
	ChannelLabel string `json:"channel_label"`
	ChannelType  string `json:"channel_type"`
	ChannelName  string `json:"channel_name"`
	TeamName     string `json:"team_name,omitempty"`
	DMUsername   string `json:"dm_username,omitempty"`
	RootID       string `json:"root_id,omitempty"`
	RootPreview  string `json:"root_preview,omitempty"`
}

func (l *labeler) where(channelID, rootID string) whereView {
	ch := l.channel(channelID)
	v := whereView{
		ChannelID:    channelID,
		ChannelLabel: ch.Display,
		ChannelType:  ch.Type,
		ChannelName:  ch.Name,
		TeamName:     ch.TeamName,
		DMUsername:   ch.DMUsername,
		RootID:       rootID,
	}
	if rootID != "" {
		v.RootPreview = l.root(rootID)
	}
	return v
}

type itemView struct {
	ID string `json:"id"`
	whereView
	Message   string   `json:"message"`
	FileNames []string `json:"file_names,omitempty"`
	SendAt    int64    `json:"send_at"`
	When      string   `json:"when"`
	Status    string   `json:"status"`
	Error     string   `json:"error,omitempty"`
}

// channelInfo describes where a message goes.
type channelInfo struct {
	Text       string // for messages: "~town-square", "@minji", "group message"
	Display    string // for the web list: "Town Square", "@minji"
	Type       string
	Name       string
	TeamName   string
	DMUsername string
}

func (p *Plugin) channelLabel(channelID string, up userPrefs) channelInfo {
	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		return channelInfo{Text: "?", Display: "?"}
	}
	info := channelInfo{Type: string(channel.Type), Name: channel.Name}

	switch channel.Type {
	case model.ChannelTypeDirect:
		other := channel.GetOtherUserIdForDM(up.userID)
		if other == "" {
			other = up.userID // a direct message to oneself
		}
		if user, appErr := p.API.GetUser(other); appErr == nil {
			info.DMUsername = user.Username
			info.Text = "@" + user.Username
			info.Display = "@" + user.Username
		} else {
			info.Text = tr(up.Ko, "label.dm")
			info.Display = info.Text
		}
	case model.ChannelTypeGroup:
		info.Text = tr(up.Ko, "label.group")
		info.Display = channel.DisplayName
	default:
		info.Text = "~" + channel.Name
		info.Display = channel.DisplayName
		if team, appErr := p.API.GetTeam(channel.TeamId); appErr == nil {
			info.TeamName = team.Name
		}
	}
	if info.Display == "" {
		info.Display = tr(up.Ko, "label.group")
		info.Text = info.Display
	}
	return info
}

// labeler looks up channel names and thread previews once per list, not once per message.
type labeler struct {
	p        *Plugin
	up       userPrefs
	channels map[string]channelInfo
	roots    map[string]string
}

func (p *Plugin) newLabeler(up userPrefs) *labeler {
	return &labeler{p: p, up: up, channels: map[string]channelInfo{}, roots: map[string]string{}}
}

func (l *labeler) channel(id string) channelInfo {
	if info, ok := l.channels[id]; ok {
		return info
	}
	info := l.p.channelLabel(id, l.up)
	l.channels[id] = info
	return info
}

func (l *labeler) root(id string) string {
	if text, ok := l.roots[id]; ok {
		return text
	}
	text := l.p.rootPreview(l.up.userID, id)
	l.roots[id] = text
	return text
}

func (p *Plugin) view(m *Scheduled, l *labeler) itemView {
	up := l.up
	v := itemView{
		ID:        m.ID,
		whereView: l.where(m.ChannelID, m.RootID),
		Message:   m.Message,
		FileNames: m.FileNames,
		SendAt:    m.SendAt,
		When:      whenLabel(m.SendAt, p.now(), up, false),
		Status:    m.Status,
	}
	if m.Status == statusFailed {
		v.Error = tr(up.Ko, m.Error)
	}
	return v
}

// rootPreview returns the start of a thread's first message, on one line, only while the
// person can still read that channel.
func (p *Plugin) rootPreview(userID, rootID string) string {
	root, appErr := p.API.GetPost(rootID)
	if appErr != nil || !p.API.HasPermissionToChannel(userID, root.ChannelId, model.PermissionReadChannel) {
		return ""
	}
	return preview(strings.Join(strings.Fields(root.Message), " "), 30)
}

func (p *Plugin) apiPresets(w http.ResponseWriter, _ *http.Request, userID string) {
	up := p.prefsFor(userID)
	now := p.now().In(up.Loc)
	tomorrow := now.AddDate(0, 0, 1)
	h, m := parseClock(p.getConfiguration().TomorrowTime, 10, 0)
	writeJSON(w, http.StatusOK, map[string]any{
		"presets":     computePresets(now, up, p.getConfiguration()),
		"timezone":    up.TZName,
		"today":       now.Format("2006-01-02"),
		"custom_date": tomorrow.Format("2006-01-02"),
		"custom_time": fmt.Sprintf("%02d:%02d", h, m),
	})
}

func (p *Plugin) apiList(w http.ResponseWriter, _ *http.Request, userID string) {
	up := p.prefsFor(userID)

	// Reads don't take the lock: a list never has to wait for the job.
	msgs, err := p.userMsgs(userID)
	if err != nil {
		p.writeError(w, err, up.Ko)
		return
	}

	// Failed ones first, since they need attention.
	l := p.newLabeler(up)
	items := make([]itemView, 0, len(msgs))
	for _, m := range msgs {
		if m.Status == statusFailed {
			items = append(items, p.view(m, l))
		}
	}
	for _, m := range msgs {
		if m.Status != statusFailed {
			items = append(items, p.view(m, l))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (p *Plugin) apiCreate(w http.ResponseWriter, r *http.Request, userID string) {
	up := p.prefsFor(userID)
	var in createInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		badRequest(w, up)
		return
	}
	m, err := p.create(userID, in)
	if err != nil {
		p.writeError(w, err, up.Ko)
		return
	}
	p.sendConfirmation(m, up)
	writeJSON(w, http.StatusOK, p.view(m, p.newLabeler(up)))
}

func (p *Plugin) apiUpdate(w http.ResponseWriter, r *http.Request, userID string) {
	up := p.prefsFor(userID)
	var in updateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		badRequest(w, up)
		return
	}
	m, err := p.update(userID, r.PathValue("id"), in)
	if err != nil {
		p.writeError(w, err, up.Ko)
		return
	}
	writeJSON(w, http.StatusOK, p.view(m, p.newLabeler(up)))
}

func (p *Plugin) apiDelete(w http.ResponseWriter, r *http.Request, userID string) {
	up := p.prefsFor(userID)
	if _, err := p.remove(userID, r.PathValue("id")); err != nil {
		p.writeError(w, err, up.Ko)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (p *Plugin) apiSendNow(w http.ResponseWriter, r *http.Request, userID string) {
	up := p.prefsFor(userID)
	if err := p.sendNow(userID, r.PathValue("id")); err != nil {
		p.writeError(w, err, up.Ko)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// sentView is a message the plugin already sent, as the web app shows it.
type sentView struct {
	PostID string `json:"post_id"`
	whereView
	Message   string `json:"message"`
	FileCount int    `json:"file_count,omitempty"`
	SentAt    int64  `json:"sent_at"`
	When      string `json:"when"`
	Edited    bool   `json:"edited,omitempty"`
	CanEdit   bool   `json:"can_edit"`
}

// apiSent lists what the plugin sent for the person in the last 30 days, with the posts'
// current text. Posts deleted in the meantime drop out of the list.
func (p *Plugin) apiSent(w http.ResponseWriter, _ *http.Request, userID string) {
	up := p.prefsFor(userID)
	now := model.GetMillisForTime(p.now())
	list, err := p.sentList(userID, now)
	if err != nil {
		p.writeError(w, err, up.Ko)
		return
	}

	l := p.newLabeler(up)
	limit := p.editTimeLimit()
	items := make([]sentView, 0, len(list))
	var gone []string
	for _, e := range list {
		post, appErr := p.API.GetPost(e.PostID)
		if appErr != nil || post.DeleteAt != 0 {
			if appErr == nil || appErr.StatusCode == http.StatusNotFound {
				gone = append(gone, e.PostID)
			}
			continue
		}
		items = append(items, sentView{
			PostID:    post.Id,
			whereView: l.where(e.ChannelID, e.RootID),
			Message:   post.Message,
			FileCount: len(post.FileIds),
			SentAt:    e.SentAt,
			When:      whenLabel(e.SentAt, p.now(), up, true),
			Edited:    post.EditAt > 0,
			CanEdit:   p.canEditSent(userID, post, limit) == nil,
		})
	}

	if len(gone) > 0 {
		p.lock.Lock()
		if err := p.removeSent(userID, gone...); err != nil {
			p.API.LogWarn("Could not tidy the sent list", "error", err.Error())
		}
		p.lock.Unlock()
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ownSent returns a post the plugin sent for this person, if it still exists and is theirs.
func (p *Plugin) ownSent(userID, postID string) (*model.Post, error) {
	list, err := p.sentList(userID, model.GetMillisForTime(p.now()))
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(list, func(e sentEntry) bool { return e.PostID == postID }) {
		return nil, failWith("err.sent_gone")
	}
	post, appErr := p.API.GetPost(postID)
	if appErr != nil {
		return nil, lookupErr(appErr, "err.sent_gone")
	}
	if post.DeleteAt != 0 || post.UserId != userID {
		return nil, failWith("err.sent_gone")
	}
	return post, nil
}

// editTimeLimit is Mattermost's "edit within N seconds" setting (-1 = no limit).
func (p *Plugin) editTimeLimit() int {
	if cfg := p.API.GetConfig(); cfg != nil && cfg.ServiceSettings.PostEditTimeLimit != nil {
		return *cfg.ServiceSettings.PostEditTimeLimit
	}
	return -1
}

// canEditSent applies Mattermost's own rules: the channel is still open, the edit_post
// permission, and the edit time limit.
func (p *Plugin) canEditSent(userID string, post *model.Post, limit int) error {
	if err := p.channelOpen(post.ChannelId); err != nil {
		return err
	}
	if !p.API.HasPermissionToChannel(userID, post.ChannelId, model.PermissionEditPost) {
		return failWith("err.no_edit")
	}
	if limit >= 0 && model.GetMillisForTime(p.now())-post.CreateAt > int64(limit)*1000 {
		return failWith("err.edit_too_late")
	}
	return nil
}

// channelOpen refuses changes in a channel that was deleted or archived since.
func (p *Plugin) channelOpen(channelID string) error {
	channel, appErr := p.API.GetChannel(channelID)
	if appErr != nil {
		return lookupErr(appErr, "fail.channel_gone")
	}
	if channel.DeleteAt != 0 {
		return failWith("fail.archived")
	}
	return nil
}

func (p *Plugin) apiSentEdit(w http.ResponseWriter, r *http.Request, userID string) {
	up := p.prefsFor(userID)
	var in struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		badRequest(w, up)
		return
	}
	post, err := p.ownSent(userID, r.PathValue("id"))
	if err == nil {
		err = p.canEditSent(userID, post, p.editTimeLimit())
	}
	if err == nil {
		err = checkMessage(in.Message, len(post.FileIds))
	}
	if err != nil {
		p.writeError(w, err, up.Ko)
		return
	}
	post.Message = in.Message
	if _, appErr := p.API.UpdatePost(post); appErr != nil {
		code := "err.edit_failed"
		if appErr.StatusCode == http.StatusForbidden {
			code = "err.no_edit"
		}
		p.API.LogWarn("Could not edit a sent message", "post_id", post.Id, "error", appErr.Error())
		p.writeError(w, lookupErr(appErr, code), up.Ko)
		return
	}
	p.publishChanged(userID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (p *Plugin) apiSentDelete(w http.ResponseWriter, r *http.Request, userID string) {
	up := p.prefsFor(userID)
	post, err := p.ownSent(userID, r.PathValue("id"))
	if err == nil {
		err = p.channelOpen(post.ChannelId)
	}
	if err == nil && !p.API.HasPermissionToChannel(userID, post.ChannelId, model.PermissionDeletePost) {
		err = failWith("err.no_delete")
	}
	if err != nil {
		p.writeError(w, err, up.Ko)
		return
	}
	if appErr := p.API.DeletePost(post.Id); appErr != nil {
		code := "err.delete_failed"
		if appErr.StatusCode == http.StatusForbidden {
			code = "err.no_delete"
		}
		p.API.LogWarn("Could not delete a sent message", "post_id", post.Id, "error", appErr.Error())
		p.writeError(w, lookupErr(appErr, code), up.Ko)
		return
	}
	p.lock.Lock()
	if err := p.removeSent(userID, post.Id); err != nil {
		p.API.LogWarn("Could not update the sent list", "error", err.Error())
	}
	p.lock.Unlock()
	p.publishChanged(userID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
