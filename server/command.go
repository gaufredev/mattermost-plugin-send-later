package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	maxListItems = 20
	// The dialog's message box holds this many characters. (Mattermost's own check counts
	// bytes but only logs a warning; the web and mobile apps count characters.)
	dialogTextMax  = model.DialogElementTextareaMaxLength
	dialogInterval = 15 // minutes between choices in the date-time picker
)

// cutRunes shortens s to at most max characters.
func cutRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

func (p *Plugin) command() *model.Command {
	// Autocomplete text is shown before we know the person, so it carries both languages.
	both := func(key string) string { return tr(true, key) + " · " + tr(false, key) }
	desc := both("cmd.desc")

	hint := both("cmd.hint")

	// Korean words first, then the English ones; ExecuteCommand accepts both.
	data := model.NewAutocompleteData(commandTrigger, hint, desc)
	data.AddCommand(model.NewAutocompleteData("목록", "", tr(true, "cmd.list")))
	data.AddCommand(model.NewAutocompleteData("도움말", "", tr(true, "cmd.help")))
	data.AddCommand(model.NewAutocompleteData("list", "", tr(false, "cmd.list")))
	data.AddCommand(model.NewAutocompleteData("help", "", tr(false, "cmd.help")))

	return &model.Command{
		Trigger:          commandTrigger,
		DisplayName:      "Send Later",
		Description:      desc,
		AutoComplete:     true,
		AutoCompleteDesc: desc,
		AutoCompleteHint: hint,
		AutocompleteData: data,
	}
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	text := strings.TrimSpace(args.Command)
	if fields := strings.Fields(text); len(fields) > 0 {
		text = strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
	}
	up := p.prefsFor(args.UserId)

	switch strings.ToLower(text) {
	case "list", "목록":
		p.sendList(args.UserId, args.ChannelId, args.RootId, up)
		return &model.CommandResponse{}, nil
	case "help", "도움말":
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         tr(up.Ko, "help", up.TZName),
		}, nil
	}

	if err := p.openCreateDialog(args.TriggerId, args.ChannelId, args.RootId, text, up); err != nil {
		p.API.LogError("Failed to open the schedule dialog", "error", err.Error())
		return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: tr(up.Ko, "err.internal")}, nil
	}
	return &model.CommandResponse{}, nil
}

type dialogState struct {
	ChannelID string `json:"channel_id,omitempty"`
	RootID    string `json:"root_id,omitempty"`
	ID        string `json:"id,omitempty"`

	// List is set when the edit started from a /later list, so that list is redrawn afterwards.
	List *listRef `json:"list,omitempty"`
}

// listRef points at a /later list shown to the person (an ephemeral post).
type listRef struct {
	PostID    string `json:"post_id"`
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id,omitempty"`
	CreateAt  int64  `json:"create_at"`
}

// The radio values carry the exact time shown in the label ("at:<ms>"), so a dialog opened
// just before midnight still sends at the time it said.
const atPrefix = "at:"

func (p *Plugin) whenElements(up userPrefs, keepLabel string) []model.DialogElement {
	presets := computePresets(p.now(), up, p.getConfiguration())
	options := make([]*model.PostActionOptions, 0, len(presets)+1)
	def := ""
	if keepLabel != "" {
		options = append(options, &model.PostActionOptions{Text: tr(up.Ko, "dialog.keep", keepLabel), Value: "keep"})
		def = "keep"
	}
	for _, ps := range presets {
		value := atPrefix + strconv.FormatInt(ps.SendAt, 10)
		options = append(options, &model.PostActionOptions{Text: ps.Label + " · " + ps.Detail, Value: value})
		if def == "" && ps.Key == "tomorrow" {
			def = value
		}
	}

	return []model.DialogElement{
		{
			DisplayName: tr(up.Ko, "dialog.when"),
			Name:        "when",
			Type:        "radio",
			Default:     def,
			Options:     options,
		},
		{
			DisplayName: tr(up.Ko, "dialog.custom_field"),
			Name:        "custom",
			Type:        "datetime",
			Optional:    true,
			HelpText:    tr(up.Ko, "dialog.custom_help", up.TZName),
			DateTimeConfig: &model.DialogDateTimeConfig{
				TimeInterval:     dialogInterval,
				LocationTimezone: up.TZName,
				MinDate:          "today",
				MaxDate:          "+365d",
			},
		},
	}
}

func (p *Plugin) openCreateDialog(triggerID, channelID, rootID, text string, up userPrefs) error {
	var notes []string
	if rootID != "" {
		notes = append(notes, tr(up.Ko, "dialog.thread"))
	}
	if cut := cutRunes(text, dialogTextMax); cut != text {
		text = cut
		notes = append(notes, tr(up.Ko, "dialog.cut"))
	}
	state, _ := json.Marshal(dialogState{ChannelID: channelID, RootID: rootID})
	intro := strings.Join(notes, "\n\n")

	elements := append([]model.DialogElement{{
		DisplayName: tr(up.Ko, "dialog.message"),
		Name:        "message",
		Type:        "textarea",
		Default:     text,
		MaxLength:   dialogTextMax,
	}}, p.whenElements(up, "")...)

	appErr := p.API.OpenInteractiveDialog(model.OpenDialogRequest{
		TriggerId: triggerID,
		URL:       "/plugins/" + pluginID + "/dialog/create",
		Dialog: model.Dialog{
			CallbackId:       "create",
			Title:            tr(up.Ko, "dialog.title"),
			IntroductionText: intro,
			Elements:         elements,
			SubmitLabel:      tr(up.Ko, "dialog.submit"),
			State:            string(state),
		},
	})
	if appErr != nil {
		return appErr
	}
	return nil
}

func (p *Plugin) openEditDialog(userID, triggerID, id string, list *listRef, up userPrefs) error {
	m, err := p.ownedMsg(userID, id)
	if err != nil {
		return err
	}

	state, _ := json.Marshal(dialogState{ID: id, List: list})
	keep := whenLabel(m.SendAt, p.now(), up, true)
	title := tr(up.Ko, "dialog.edit_title")
	if m.Status == statusFailed {
		keep = "" // a failed message needs a new time
		title = tr(up.Ko, "dialog.retry_title")
	}

	var elements []model.DialogElement
	var notes []string
	if m.RootID != "" {
		notes = append(notes, tr(up.Ko, "dialog.thread"))
	}
	if utf8.RuneCountInString(m.Message) <= dialogTextMax {
		elements = append(elements, model.DialogElement{
			DisplayName: tr(up.Ko, "dialog.message"),
			Name:        "message",
			Type:        "textarea",
			Default:     m.Message,
			MaxLength:   dialogTextMax,
			Optional:    len(m.FileIDs) > 0,
		})
	} else {
		notes = append(notes, tr(up.Ko, "dialog.long"))
	}
	elements = append(elements, p.whenElements(up, keep)...)
	intro := strings.Join(notes, "\n\n")

	appErr := p.API.OpenInteractiveDialog(model.OpenDialogRequest{
		TriggerId: triggerID,
		URL:       "/plugins/" + pluginID + "/dialog/edit",
		Dialog: model.Dialog{
			CallbackId:       "edit",
			Title:            title,
			IntroductionText: intro,
			Elements:         elements,
			SubmitLabel:      tr(up.Ko, "dialog.save"),
			State:            string(state),
		},
	})
	if appErr != nil {
		return appErr
	}
	return nil
}

// dialogWhen reads the time fields of a submitted dialog. A date and time picked in the
// date-time field always wins over the radio choice. keep=true means "leave the time as it is".
func dialogWhen(sub map[string]any, up userPrefs) (w whenInput, keep bool, field string, err error) {
	choice, _ := sub["when"].(string)
	custom, _ := sub["custom"].(string)

	if strings.TrimSpace(custom) != "" {
		t, perr := parseDialogDateTime(custom, up.Loc)
		if perr != nil {
			return w, false, "custom", failWith("err.bad_time")
		}
		return whenInput{SendAt: model.GetMillisForTime(t)}, false, "custom", nil
	}

	switch {
	case choice == "keep":
		return w, true, "", nil
	case strings.HasPrefix(choice, atPrefix):
		ms, perr := strconv.ParseInt(strings.TrimPrefix(choice, atPrefix), 10, 64)
		if perr != nil {
			return w, false, "when", failWith("err.bad_time")
		}
		return whenInput{SendAt: ms}, false, "when", nil
	case choice == "":
		return w, false, "when", failWith("err.no_time")
	default:
		return whenInput{Preset: choice}, false, "when", nil
	}
}

// fieldFor decides which dialog field an error belongs to.
func fieldFor(code, timeField string) string {
	switch code {
	case "err.empty", "err.too_long", "err.command":
		return "message"
	case "err.past", "err.too_far", "err.no_time", "err.bad_time":
		return timeField
	}
	return ""
}

func (p *Plugin) handleDialog(w http.ResponseWriter, r *http.Request, userID string) {
	var req model.SubmitDialogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.Cancelled {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	up := p.prefsFor(userID)

	var state dialogState
	_ = json.Unmarshal([]byte(req.State), &state)

	when, keep, timeField, err := dialogWhen(req.Submission, up)
	respondErr := func(err error) {
		resp := model.SubmitDialogResponse{}
		if field := fieldFor(errorCode(err), timeField); field != "" {
			resp.Errors = map[string]string{field: errorText(err, up.Ko)}
		} else {
			resp.Error = errorText(err, up.Ko)
		}
		writeJSON(w, http.StatusOK, resp)
	}
	if err != nil {
		respondErr(err)
		return
	}

	message, hasMessage := req.Submission["message"].(string)

	switch r.PathValue("kind") {
	case "create":
		m, err := p.create(userID, createInput{
			ChannelID: state.ChannelID,
			RootID:    state.RootID,
			Message:   message,
			When:      when,
		})
		if err != nil {
			respondErr(err)
			return
		}
		p.sendConfirmation(m, up)

	case "edit":
		in := updateInput{}
		if hasMessage {
			in.Message = &message
		}
		if !keep {
			in.When = &when
		}
		m, err := p.update(userID, state.ID, in)
		if err != nil {
			respondErr(err)
			return
		}
		note := tr(up.Ko, "updated", whenLabel(m.SendAt, p.now(), up, true))
		if l := state.List; l != nil {
			// Redraw the list the edit came from, with the result on top.
			post := p.listPost(userID, l.ChannelID, l.RootID, note, l.CreateAt, up)
			post.Id = l.PostID
			p.API.UpdateEphemeralPost(userID, post)
		} else {
			p.sendEphemeral(userID, m.ChannelID, m.RootID, note, nil)
		}

	default:
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (p *Plugin) button(id, name, op, style string, ctx map[string]any) *model.PostAction {
	return &model.PostAction{
		Id:    id,
		Name:  name,
		Type:  model.PostActionTypeButton,
		Style: style,
		Integration: &model.PostActionIntegration{
			URL:     "/plugins/" + pluginID + "/action/" + op,
			Context: ctx,
		},
	}
}

func (p *Plugin) sendEphemeral(userID, channelID, rootID, text string, attachments []*model.MessageAttachment) *model.Post {
	post := &model.Post{
		UserId:    p.botID,
		ChannelId: channelID,
		RootId:    rootID,
		Message:   text,
		CreateAt:  model.GetMillis(),
	}
	if len(attachments) > 0 {
		post.AddProp(model.PostPropsAttachments, attachments)
	}
	return p.API.SendEphemeralPost(userID, post)
}

// sendConfirmation tells the person, where they wrote it, when the message will go, with quick buttons.
func (p *Plugin) sendConfirmation(m *Scheduled, up userPrefs) {
	if post := p.confirmPost(m, up); post != nil {
		p.API.SendEphemeralPost(m.UserID, post)
	}
}

// confirmPost builds the note "<time>에 보내도록 예약했습니다" with its buttons. Its ID was chosen
// when the message was created (m.Confirm), so the note can be changed later.
func (p *Plugin) confirmPost(m *Scheduled, up userPrefs) *model.Post {
	c := m.Confirm
	if c == nil {
		return nil
	}
	ctx := map[string]any{
		"id":        m.ID,
		"kind":      "confirm",
		"root_id":   c.RootID,
		"create_at": strconv.FormatInt(c.CreateAt, 10),
	}
	post := &model.Post{
		Id:        c.PostID,
		UserId:    p.botID,
		ChannelId: c.ChannelID,
		RootId:    c.RootID,
		Message:   tr(up.Ko, "confirm", whenLabel(m.SendAt, p.now(), up, true)),
		CreateAt:  c.CreateAt,
	}
	post.AddProp(model.PostPropsAttachments, []*model.MessageAttachment{{
		Actions: []*model.PostAction{
			p.button("sendnow", tr(up.Ko, "btn.send_now"), "send-now", "", ctx),
			p.button("delete", tr(up.Ko, "btn.delete"), "delete", "danger", ctx),
		},
	}})
	return post
}

// refreshConfirm redraws the note after the time was changed (same buttons, new time).
func (p *Plugin) refreshConfirm(m *Scheduled) {
	if m.Status != statusPending {
		return
	}
	if post := p.confirmPost(m, p.prefsFor(m.UserID)); post != nil {
		p.API.UpdateEphemeralPost(m.UserID, post)
	}
}

// closeConfirm replaces the note and its buttons with what happened (sent, deleted, not sent),
// so no stale "Send now / Delete" buttons stay behind.
func (p *Plugin) closeConfirm(m *Scheduled, text string) {
	c := m.Confirm
	if c == nil {
		return
	}
	p.API.UpdateEphemeralPost(m.UserID, &model.Post{
		Id:        c.PostID,
		UserId:    p.botID,
		ChannelId: c.ChannelID,
		RootId:    c.RootID,
		CreateAt:  c.CreateAt,
		Message:   text,
	})
}

// postLink is a link that opens a post in whichever team it is in.
func (p *Plugin) postLink(postID string) string {
	cfg := p.API.GetConfig()
	if cfg == nil || cfg.ServiceSettings.SiteURL == nil || *cfg.ServiceSettings.SiteURL == "" {
		return ""
	}
	return strings.TrimRight(*cfg.ServiceSettings.SiteURL, "/") + "/_redirect/pl/" + postID
}

// threadLabel names the thread a reply goes to: 「<start of the first message>」에 단 답글.
func (l *labeler) threadLabel(rootID string) string {
	if text := l.root(rootID); text != "" {
		return tr(l.up.Ko, "list.reply_to", text)
	}
	return tr(l.up.Ko, "list.reply")
}

// listPost builds the /later list: one attachment with buttons per scheduled message.
// note, when set, is shown first (the result of the last button pressed).
func (p *Plugin) listPost(userID, channelID, rootID, note string, createAt int64, up userPrefs) *model.Post {
	post := &model.Post{UserId: p.botID, ChannelId: channelID, RootId: rootID, CreateAt: createAt}
	prefix := ""
	if note != "" {
		prefix = note + "\n\n"
	}

	msgs, err := p.userMsgs(userID)
	if err != nil {
		post.Message = prefix + tr(up.Ko, "err.internal")
		return post
	}
	if len(msgs) == 0 {
		post.Message = prefix + tr(up.Ko, "list.empty")
		return post
	}

	text := prefix + tr(up.Ko, "list.header", len(msgs))
	if len(msgs) > maxListItems {
		text += "\n" + tr(up.Ko, "list.more", maxListItems)
		msgs = msgs[:maxListItems]
	}
	post.Message = text

	now := p.now()
	labels := p.newLabeler(up)
	attachments := make([]*model.MessageAttachment, 0, len(msgs))
	for _, m := range msgs {
		ch := labels.channel(m.ChannelID)
		head := "**" + whenLabel(m.SendAt, now, up, false) + "** · " + ch.Text
		color := ""
		editLabel := tr(up.Ko, "btn.edit")
		if m.Status == statusFailed {
			head = "**" + tr(up.Ko, "list.failed") + "** · " + ch.Text + "\n" + tr(up.Ko, m.Error)
			color = "#d24b4e"
			editLabel = tr(up.Ko, "dialog.retry_title")
		}
		if m.RootID != "" {
			head += " · " + labels.threadLabel(m.RootID)
		}
		body := head
		if strings.TrimSpace(m.Message) != "" {
			body += "\n" + quote(preview(m.Message, 300))
		}
		if len(m.FileIDs) > 0 {
			body += "\n📎 " + tr(up.Ko, "list.files", len(m.FileIDs))
		}

		ctx := map[string]any{
			"id":         m.ID,
			"kind":       "list",
			"channel_id": channelID,
			"root_id":    rootID,
			"create_at":  strconv.FormatInt(createAt, 10),
		}
		attachments = append(attachments, &model.MessageAttachment{
			Text:  body,
			Color: color,
			Actions: []*model.PostAction{
				p.button("sendnow", tr(up.Ko, "btn.send_now"), "send-now", "", ctx),
				p.button("edit", editLabel, "edit", "", ctx),
				p.button("delete", tr(up.Ko, "btn.delete"), "delete", "danger", ctx),
			},
		})
	}
	post.AddProp(model.PostPropsAttachments, attachments)
	return post
}

// sendList shows the person's scheduled messages with buttons (used by /later list, mainly on mobile).
func (p *Plugin) sendList(userID, channelID, rootID string, up userPrefs) {
	p.API.SendEphemeralPost(userID, p.listPost(userID, channelID, rootID, "", model.GetMillis(), up))
}

// handleAction answers the buttons in confirmations and lists.
func (p *Plugin) handleAction(w http.ResponseWriter, r *http.Request, userID string) {
	var req model.PostActionIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	up := p.prefsFor(userID)
	id, _ := req.Context["id"].(string)
	kind, _ := req.Context["kind"].(string)
	rootID, _ := req.Context["root_id"].(string)
	createAt, _ := strconv.ParseInt(fmt.Sprint(req.Context["create_at"]), 10, 64)

	var text string
	switch r.PathValue("op") {
	case "send-now":
		m, _ := p.ownedMsg(userID, id)
		if err := p.sendNow(userID, id); err != nil {
			text = errorText(err, up.Ko)
		} else if kind == "confirm" {
			// Sending already turned this note into "sent … · go to message".
			writeJSON(w, http.StatusOK, model.PostActionIntegrationResponse{})
			return
		} else if m != nil {
			text = tr(up.Ko, "sent.to", p.channelLabel(m.ChannelID, up).Text)
		} else {
			text = tr(up.Ko, "sent")
		}
	case "delete":
		m, err := p.remove(userID, id)
		if err != nil {
			text = errorText(err, up.Ko)
		} else {
			text = tr(up.Ko, "deleted")
			if strings.TrimSpace(m.Message) != "" {
				text += "\n" + tr(up.Ko, "deleted.was") + "\n" + quote(m.Message)
			}
		}
	case "edit":
		var list *listRef
		if kind == "list" {
			list = &listRef{PostID: req.PostId, ChannelID: req.ChannelId, RootID: rootID, CreateAt: createAt}
		}
		if err := p.openEditDialog(userID, req.TriggerId, id, list, up); err != nil {
			writeJSON(w, http.StatusOK, model.PostActionIntegrationResponse{EphemeralText: errorText(err, up.Ko)})
			return
		}
		writeJSON(w, http.StatusOK, model.PostActionIntegrationResponse{})
		return
	default:
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	switch kind {
	case "confirm":
		if m, err := p.ownedMsg(userID, id); err == nil && m.Status != statusSent {
			// The button failed and the message is still scheduled: keep the note and its
			// buttons, and add the reason underneath.
			if post := p.confirmPost(m, up); post != nil {
				post.Message += "\n\n" + text
				p.API.UpdateEphemeralPost(userID, post)
				break
			}
		}
		// Done (deleted) or gone: replace the note and its buttons with the result.
		p.API.UpdateEphemeralPost(userID, &model.Post{
			Id:        req.PostId,
			UserId:    p.botID,
			ChannelId: req.ChannelId,
			RootId:    rootID,
			CreateAt:  createAt,
			Message:   text,
		})
	case "list":
		// Redraw the list in place, with the result on top, so no stale buttons are left.
		post := p.listPost(userID, req.ChannelId, rootID, text, createAt, up)
		post.Id = req.PostId
		p.API.UpdateEphemeralPost(userID, post)
	default:
		writeJSON(w, http.StatusOK, model.PostActionIntegrationResponse{EphemeralText: text})
		return
	}
	writeJSON(w, http.StatusOK, model.PostActionIntegrationResponse{})
}
