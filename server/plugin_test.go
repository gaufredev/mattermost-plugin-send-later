package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

// 2026-10-15 is a Thursday.
const thursdayAfternoon = "2026-10-15 14:00"

func presetKeys(ps []preset) []string {
	keys := make([]string, len(ps))
	for i, p := range ps {
		keys[i] = p.Key
	}
	return keys
}

func presetByKey(ps []preset, key string) (preset, bool) {
	for _, p := range ps {
		if p.Key == key {
			return p, true
		}
	}
	return preset{}, false
}

func TestPresets(t *testing.T) {
	cfg := &configuration{TodayTime: "17:00", TomorrowTime: "10:00", MondayTime: "10:00"}
	loc, _ := time.LoadLocation("Asia/Seoul")
	ko := userPrefs{Loc: loc, TZName: "Asia/Seoul", Ko: true}

	t.Run("thursday afternoon: in 1 and 2 hours, today, tomorrow, monday", func(t *testing.T) {
		ps := computePresets(seoul(t, thursdayAfternoon), ko, cfg)
		if got := strings.Join(presetKeys(ps), ","); got != "in1h,in2h,today,tomorrow,monday" {
			t.Fatalf("keys = %s", got)
		}
		want := map[string]string{"in1h": "2026-10-15 15:00", "in2h": "2026-10-15 16:00", "today": "2026-10-15 17:00", "tomorrow": "2026-10-16 10:00", "monday": "2026-10-19 10:00"}
		for _, p := range ps {
			if p.SendAt != seoul(t, want[p.Key]).UnixMilli() {
				t.Errorf("%s = %s, want %s", p.Key, time.UnixMilli(p.SendAt).In(loc), want[p.Key])
			}
		}
		labels := []string{}
		for _, p := range ps {
			labels = append(labels, p.Label+" / "+p.Detail)
		}
		if got := strings.Join(labels, " | "); got != "1시간 후 / 오후 3:00 | 2시간 후 / 오후 4:00 | 오늘 오후 5:00 / 10월 15일 (목) | 내일 오전 10:00 / 10월 16일 (금) | 월요일 오전 10:00 / 10월 19일 (월)" {
			t.Fatalf("labels = %s", got)
		}
	})

	t.Run("in N hours rounds up to the next minute and shows the date after midnight", func(t *testing.T) {
		now := seoul(t, "2026-10-15 23:10").Add(30 * time.Second)
		ps := computePresets(now, ko, cfg)
		in1, _ := presetByKey(ps, "in1h")
		if in1.SendAt != seoul(t, "2026-10-16 00:11").UnixMilli() || in1.Detail != "10월 16일 (금) 오전 12:11" {
			t.Fatalf("in1h = %+v", in1)
		}
	})

	t.Run("after midnight: today at the morning time comes first", func(t *testing.T) {
		ps := computePresets(seoul(t, "2026-10-22 01:36"), ko, cfg)
		if got := strings.Join(presetKeys(ps), ","); got != "in1h,in2h,early,morning,today,tomorrow,monday" {
			t.Fatalf("keys = %s", got)
		}
		m, _ := presetByKey(ps, "morning")
		tm, _ := presetByKey(ps, "tomorrow")
		if m.Label != "오늘 오전 10:00" || m.SendAt != seoul(t, "2026-10-22 10:00").UnixMilli() || tm.SendAt != seoul(t, "2026-10-23 10:00").UnixMilli() {
			t.Fatalf("morning = %+v, tomorrow = %+v", m, tm)
		}
		e, _ := presetByKey(ps, "early")
		if e.Label != "오늘 오전 9:00" || e.SendAt != seoul(t, "2026-10-22 09:00").UnixMilli() {
			t.Fatalf("early = %+v", e)
		}
		at930 := computePresets(seoul(t, "2026-10-22 09:30"), ko, cfg)
		if _, ok := presetByKey(at930, "early"); ok {
			t.Fatal("9:00 still offered at 9:30")
		}
		if _, ok := presetByKey(at930, "morning"); !ok {
			t.Fatal("10:00 should still be offered at 9:30")
		}
		if _, ok := presetByKey(computePresets(seoul(t, "2026-10-22 01:36"), ko, &configuration{EarlyTime: "0"}), "early"); ok {
			t.Fatal(`EarlyTime "0" should hide it`)
		}
		if _, ok := presetByKey(computePresets(seoul(t, "2026-10-22 10:30"), ko, cfg), "morning"); ok {
			t.Fatal("morning still offered after 10:00")
		}
	})

	t.Run("after 17:00 there is no today", func(t *testing.T) {
		ps := computePresets(seoul(t, "2026-10-15 16:59"), ko, cfg)
		if _, ok := presetByKey(ps, "today"); ok {
			t.Fatalf("keys = %v", presetKeys(ps))
		}
	})

	t.Run("sunday: tomorrow is monday, so monday is not repeated", func(t *testing.T) {
		ps := computePresets(seoul(t, "2026-10-18 20:00"), ko, cfg)
		if got := strings.Join(presetKeys(ps), ","); got != "in1h,in2h,tomorrow" {
			t.Fatalf("keys = %s", got)
		}
	})

	t.Run("monday: next monday is a week later", func(t *testing.T) {
		ps := computePresets(seoul(t, "2026-10-19 09:00"), ko, cfg)
		if mon, _ := presetByKey(ps, "monday"); mon.SendAt != seoul(t, "2026-10-26 10:00").UnixMilli() {
			t.Fatalf("monday = %+v", mon)
		}
	})

	t.Run("english, 24-hour", func(t *testing.T) {
		en := userPrefs{Loc: loc, TZName: "Asia/Seoul", H24: true}
		ps := computePresets(seoul(t, thursdayAfternoon), en, cfg)
		in1, _ := presetByKey(ps, "in1h")
		in2, _ := presetByKey(ps, "in2h")
		today, _ := presetByKey(ps, "today")
		mon, _ := presetByKey(ps, "monday")
		if in1.Label != "In 1 hour" || in2.Label != "In 2 hours" || in1.Detail != "15:00" || today.Label != "Today at 17:00" || mon.Detail != "Mon 19 Oct" {
			t.Fatalf("got %q %q %q %q %q", in1.Label, in2.Label, in1.Detail, today.Label, mon.Detail)
		}
	})

	t.Run("invalid setting falls back to the default", func(t *testing.T) {
		ps := computePresets(seoul(t, thursdayAfternoon), ko, &configuration{TodayTime: "five", TomorrowTime: "25:00"})
		today, _ := presetByKey(ps, "today")
		tomorrow, _ := presetByKey(ps, "tomorrow")
		if today.SendAt != seoul(t, "2026-10-15 17:00").UnixMilli() || tomorrow.SendAt != seoul(t, "2026-10-16 10:00").UnixMilli() {
			t.Fatalf("fallbacks not used: %+v", ps)
		}
	})

	t.Run("relative hours setting", func(t *testing.T) {
		cases := map[string]string{
			"":              "in1h,in2h",
			"0":             "",
			"3, 1":          "in3h,in1h",
			"1,1,x,30,-2,4": "in1h,in4h",
			"1,2,3,4,5":     "in1h,in2h,in3h,in4h",
		}
		for setting, want := range cases {
			ps := computePresets(seoul(t, thursdayAfternoon), ko, &configuration{RelativeHours: setting})
			var got []string
			for _, p := range ps {
				if strings.HasPrefix(p.Key, "in") {
					got = append(got, p.Key)
				}
			}
			if strings.Join(got, ",") != want {
				t.Errorf("%q: got %v, want %s", setting, got, want)
			}
		}
	})
}

func TestWhenLabel(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Seoul")
	now := seoul(t, thursdayAfternoon)
	ko := userPrefs{Loc: loc, Ko: true}
	en := userPrefs{Loc: loc}
	cases := []struct {
		at   string
		up   userPrefs
		sent bool
		want string
	}{
		{"2026-10-15 17:00", ko, true, "오늘 오후 5:00"},
		{"2026-10-16 00:30", ko, true, "내일 오전 12:30"},
		{"2026-10-20 12:05", ko, true, "10월 20일 (화) 오후 12:05"},
		{"2027-01-05 10:00", ko, true, "2027년 1월 5일 (화) 오전 10:00"},
		{"2026-10-16 10:00", en, true, "tomorrow at 10:00 AM"},
		{"2026-10-16 10:00", en, false, "Tomorrow at 10:00 AM"},
		{"2026-10-20 18:00", en, false, "Tue 20 Oct at 6:00 PM"},
	}
	for _, c := range cases {
		if got := whenLabel(seoul(t, c.at).UnixMilli(), now, c.up, c.sent); got != c.want {
			t.Errorf("%s: got %q, want %q", c.at, got, c.want)
		}
	}
}

func TestParseDialogDateTime(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Seoul")
	got, err := parseDialogDateTime("2026-10-20T05:30:00Z", loc)
	if err != nil || !got.Equal(seoul(t, "2026-10-20 14:30")) {
		t.Fatalf("UTC value: %v %v", got, err)
	}
	got, err = parseDialogDateTime("2026-10-20T14:30", loc)
	if err != nil || !got.Equal(seoul(t, "2026-10-20 14:30")) {
		t.Fatalf("local value: %v %v", got, err)
	}
	if _, err := parseDialogDateTime("next tuesday", loc); err == nil {
		t.Fatal("expected an error")
	}
}

func TestScheduleAndSend(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	f.api.files["file1"] = &model.FileInfo{Id: "file1", CreatorId: userKo, ChannelId: chanID, Name: "photo.jpg"}

	m, err := f.p.create(userKo, createInput{
		ChannelID: chanID,
		RootID:    replyID, // replying from inside a thread: stored against the root
		Message:   "내일 회의 자료입니다",
		FileIDs:   []string{"file1"},
		When:      whenInput{Preset: "tomorrow"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.RootID != rootID || m.FileNames[0] != "photo.jpg" {
		t.Fatalf("stored = %+v", m)
	}

	f.advance(19*time.Hour + 59*time.Minute) // 09:59 the next day
	f.p.runDue()
	if len(f.api.created) != 0 {
		t.Fatal("sent too early")
	}

	f.advance(time.Minute)
	f.p.runDue()
	if len(f.api.created) != 1 {
		t.Fatalf("created %d posts", len(f.api.created))
	}
	post := f.api.created[0]
	if post.UserId != userKo || post.ChannelId != chanID || post.RootId != rootID || post.Message != "내일 회의 자료입니다" || post.FileIds[0] != "file1" {
		t.Fatalf("post = %+v", post)
	}

	f.p.lock.Lock()
	left, _ := f.p.userMsgs(userKo)
	idx, _ := f.p.readIndex()
	f.p.lock.Unlock()
	if len(left) != 0 || len(idx) != 0 {
		t.Fatalf("still stored: %d messages, %d index entries", len(left), len(idx))
	}

	f.p.runDue()
	if len(f.api.created) != 1 {
		t.Fatal("sent twice")
	}
}

func TestValidation(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	f.api.files["theirs"] = &model.FileInfo{Id: "theirs", CreatorId: userEn, ChannelId: chanID}
	f.api.channels["archived"] = &model.Channel{Id: "archived", TeamId: teamID, Type: model.ChannelTypeOpen, DeleteAt: 1}
	f.api.members["archived/"+userKo] = true
	f.api.channels["readonly"] = &model.Channel{Id: "readonly", TeamId: teamID, Type: model.ChannelTypeOpen}
	f.api.members["readonly/"+userKo] = true
	f.api.readOnly["readonly/"+userKo] = true
	f.api.channels["other"] = &model.Channel{Id: "other", TeamId: teamID, Type: model.ChannelTypeOpen}

	tomorrow := whenInput{Preset: "tomorrow"}
	cases := []struct {
		name string
		in   createInput
		want string
	}{
		{"empty", createInput{ChannelID: chanID, Message: "  ", When: tomorrow}, "err.empty"},
		{"no time", createInput{ChannelID: chanID, Message: "hi"}, "err.no_time"},
		{"past", createInput{ChannelID: chanID, Message: "hi", When: whenInput{Date: "2026-10-15", Time: "13:00"}}, "err.past"},
		{"too far", createInput{ChannelID: chanID, Message: "hi", When: whenInput{Date: "2027-12-01", Time: "10:00"}}, "err.too_far"},
		{"bad date", createInput{ChannelID: chanID, Message: "hi", When: whenInput{Date: "2026-13-01", Time: "10:00"}}, "err.bad_time"},
		{"not a member", createInput{ChannelID: "other", Message: "hi", When: tomorrow}, "fail.not_member"},
		{"archived", createInput{ChannelID: "archived", Message: "hi", When: tomorrow}, "fail.archived"},
		{"read-only", createInput{ChannelID: "readonly", Message: "hi", When: tomorrow}, "fail.no_permission"},
		{"missing channel", createInput{ChannelID: "nope", Message: "hi", When: tomorrow}, "fail.channel_gone"},
		{"deleted thread", createInput{ChannelID: chanID, RootID: "gone", Message: "hi", When: tomorrow}, "fail.thread_deleted"},
		{"someone else's file", createInput{ChannelID: chanID, FileIDs: []string{"theirs"}, When: tomorrow}, "fail.files"},
		{"too long", createInput{ChannelID: chanID, Message: strings.Repeat("가", model.PostMessageMaxRunesV2+1), When: tomorrow}, "err.too_long"},
	}
	for _, c := range cases {
		_, err := f.p.create(userKo, c.in)
		if errorCode(err) != c.want {
			t.Errorf("%s: got %v, want %s", c.name, err, c.want)
		}
	}
	if errorText(failWith("err.past"), true) != "이미 지난 시간입니다. 지금 이후의 시간을 골라 주세요." {
		t.Error("Korean error text")
	}
}

func TestOnlyOwnerCanChange(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, err := f.p.create(userKo, createInput{ChannelID: chanID, Message: "hi", When: whenInput{Preset: "tomorrow"}})
	if err != nil {
		t.Fatal(err)
	}
	text := "hacked"
	if _, err := f.p.update(userEn, m.ID, updateInput{Message: &text}); errorCode(err) != "err.not_found" {
		t.Errorf("update by someone else: %v", err)
	}
	if _, err := f.p.remove(userEn, m.ID); errorCode(err) != "err.not_found" {
		t.Errorf("delete by someone else: %v", err)
	}
	if err := f.p.sendNow(userEn, m.ID); errorCode(err) != "err.not_found" {
		t.Errorf("send by someone else: %v", err)
	}
	if len(f.api.created) != 0 {
		t.Fatal("someone else managed to send it")
	}
}

func TestFailureIsKeptAndReported(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, err := f.p.create(userKo, createInput{ChannelID: chanID, Message: "잘 부탁드립니다", When: whenInput{Preset: "today"}})
	if err != nil {
		t.Fatal(err)
	}

	delete(f.api.members, chanID+"/"+userKo) // left the channel before 17:00
	f.advance(3 * time.Hour)
	f.p.runDue()

	if len(f.api.created) != 1 || f.api.created[0].UserId != "bot" {
		t.Fatalf("expected one DM from the bot, got %+v", f.api.created)
	}
	dm := f.api.created[0].Message
	if !strings.Contains(dm, "이 채널의 멤버가 아닙니다") || !strings.Contains(dm, "~town-square") || !strings.Contains(dm, "> 잘 부탁드립니다") {
		t.Fatalf("DM = %q", dm)
	}

	f.p.lock.Lock()
	got, _ := f.p.getMsg(m.ID)
	idx, _ := f.p.readIndex()
	f.p.lock.Unlock()
	if got.Status != statusFailed || got.Error != "fail.not_member" || len(idx) != 0 {
		t.Fatalf("after failure: %+v, index %v", got, idx)
	}

	// Back in the channel, a new time schedules it again.
	f.api.members[chanID+"/"+userKo] = true
	if _, err := f.p.update(userKo, m.ID, updateInput{When: &whenInput{Preset: "tomorrow"}}); err != nil {
		t.Fatal(err)
	}
	f.advance(24 * time.Hour)
	f.p.runDue()
	if len(f.api.created) != 2 || f.api.created[1].UserId != userKo {
		t.Fatalf("not resent: %+v", f.api.created)
	}
}

func TestLateAndInterruptedSends(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	late, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "late", When: whenInput{Preset: "today"}})
	stuck, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "stuck", When: whenInput{Preset: "tomorrow"}})

	// The server was down for more than a day.
	f.advance(48 * time.Hour)

	// Simulate a send that was interrupted: marked "sending" long ago.
	f.p.lock.Lock()
	s, _ := f.p.getMsg(stuck.ID)
	s.Status = statusSending
	s.UpdateAt = f.clock.Add(-10 * time.Minute).UnixMilli()
	_ = f.p.putMsg(s)
	f.p.lock.Unlock()

	f.p.runDue()

	f.p.lock.Lock()
	l, _ := f.p.getMsg(late.ID)
	s, _ = f.p.getMsg(stuck.ID)
	f.p.lock.Unlock()
	if l.Error != "fail.too_late" || s.Error != "fail.maybe_sent" {
		t.Fatalf("late=%s stuck=%s", l.Error, s.Error)
	}
	for _, p := range f.api.created {
		if p.UserId != "bot" {
			t.Fatalf("a message was posted: %+v", p)
		}
	}
}

func TestSendNowAndRejectedPost(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userEn, createInput{ChannelID: chanID, Message: "now please", When: whenInput{Preset: "monday"}})

	f.api.failPost = true
	err := f.p.sendNow(userEn, m.ID)
	if errorCode(err) != "fail.post_failed" {
		t.Fatalf("got %v", err)
	}
	for _, p := range f.api.created {
		if p.UserId == "bot" {
			t.Fatal("send now should answer directly, not by DM")
		}
	}

	f.api.failPost = false
	if err := f.p.sendNow(userEn, m.ID); err != nil {
		t.Fatal(err)
	}
	if len(f.api.created) != 1 || f.api.created[0].Message != "now please" {
		t.Fatalf("created = %+v", f.api.created)
	}
}

func TestUserLimit(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	for i := 0; i < maxPerUser; i++ {
		if _, err := f.p.create(userKo, createInput{ChannelID: chanID, Message: "x", When: whenInput{Preset: "tomorrow"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.p.create(userKo, createInput{ChannelID: chanID, Message: "x", When: whenInput{Preset: "tomorrow"}}); errorCode(err) != "err.too_many" {
		t.Fatalf("got %v", err)
	}
}

func doJSON(t *testing.T, f *fixture, method, path, userID string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if userID != "" {
		req.Header.Set("Mattermost-User-Id", userID)
	}
	rec := httptest.NewRecorder()
	f.p.router.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestHTTPAPI(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))

	if code, _ := doJSON(t, f, "GET", "/api/v1/scheduled", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("no user: %d", code)
	}

	code, presets := doJSON(t, f, "GET", "/api/v1/presets", userKo, nil)
	if code != 200 || presets["timezone"] != "Asia/Seoul" || presets["custom_date"] != "2026-10-16" || presets["custom_time"] != "10:00" {
		t.Fatalf("presets: %d %v", code, presets)
	}

	code, item := doJSON(t, f, "POST", "/api/v1/scheduled", userKo, map[string]any{
		"channel_id": chanID, "message": "안녕하세요", "when": map[string]any{"date": "2026-10-20", "time": "09:30"},
	})
	if code != 200 || item["when"] != "10월 20일 (화) 오전 9:30" || item["channel_label"] != "Town Square" || item["team_name"] != "wave" {
		t.Fatalf("create: %d %v", code, item)
	}
	id := item["id"].(string)
	if len(f.api.ephemeral) != 1 || !strings.Contains(f.api.ephemeral[0].Message, "10월 20일 (화) 오전 9:30에 보내도록 예약했습니다") {
		t.Fatalf("confirmation: %+v", f.api.ephemeral)
	}

	code, bad := doJSON(t, f, "POST", "/api/v1/scheduled", userKo, map[string]any{"channel_id": chanID, "message": "", "when": map[string]any{"preset": "today"}})
	if code != 400 || bad["error"] != "보낼 메시지를 입력해 주세요." {
		t.Fatalf("empty: %d %v", code, bad)
	}

	code, updated := doJSON(t, f, "PATCH", "/api/v1/scheduled/"+id, userKo, map[string]any{"message": "수정본", "when": map[string]any{"preset": "today"}})
	if code != 200 || updated["message"] != "수정본" || updated["when"] != "오늘 오후 5:00" {
		t.Fatalf("update: %d %v", code, updated)
	}

	code, list := doJSON(t, f, "GET", "/api/v1/scheduled", userKo, nil)
	if code != 200 || len(list["items"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, list)
	}
	if code, _ := doJSON(t, f, "GET", "/api/v1/scheduled", userEn, nil); code != 200 {
		t.Fatal("other user list")
	}
	if code, other := doJSON(t, f, "DELETE", "/api/v1/scheduled/"+id, userEn, nil); code != 404 {
		t.Fatalf("delete by someone else: %d %v", code, other)
	}
	if code, _ := doJSON(t, f, "POST", "/api/v1/scheduled/"+id+"/send", userKo, nil); code != 200 {
		t.Fatalf("send now: %d", code)
	}
	if len(f.api.created) != 1 || f.api.created[0].Message != "수정본" {
		t.Fatalf("created = %+v", f.api.created)
	}
	if code, _ := doJSON(t, f, "DELETE", "/api/v1/scheduled/"+id, userKo, nil); code != 404 {
		t.Fatalf("delete after send: %d", code)
	}
	if !slices.Contains(f.api.events, wsEventChanged+":"+userKo) {
		t.Fatal("no websocket event")
	}
}

func TestDialogFlow(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))

	resp, err := f.p.ExecuteCommand(nil, &model.CommandArgs{Command: "/later  금요일 아침 공지", UserId: userKo, ChannelId: chanID, RootId: rootID, TriggerId: "trig"})
	if err != nil || resp.Text != "" {
		t.Fatalf("command: %v %+v", err, resp)
	}
	if len(f.api.dialogs) != 1 {
		t.Fatal("no dialog opened")
	}
	d := f.api.dialogs[0].Dialog
	tomorrowAt := "at:" + strconv.FormatInt(seoul(t, "2026-10-16 10:00").UnixMilli(), 10)
	if d.Elements[0].Default != "금요일 아침 공지" || d.Elements[1].Default != tomorrowAt || d.Elements[2].Type != "datetime" || d.IntroductionText == "" {
		t.Fatalf("dialog = %+v", d)
	}
	for _, o := range d.Elements[1].Options {
		if !strings.HasPrefix(o.Value, "at:") {
			t.Fatalf("radio value without a time: %+v", o)
		}
	}

	// A broken date-time value is an error on that field.
	code, out := doJSON(t, f, "POST", "/dialog/create", userKo, model.SubmitDialogRequest{
		State: d.State, Submission: map[string]any{"message": "공지", "when": tomorrowAt, "custom": "soon"},
	})
	if code != 200 || out["errors"].(map[string]any)["custom"] != "날짜와 시간을 다시 확인해 주세요." {
		t.Fatalf("bad custom: %v", out)
	}

	// A picked date and time wins over the radio choice (here: tomorrow 10:00 → 09:00).
	code, out = doJSON(t, f, "POST", "/dialog/create", userKo, model.SubmitDialogRequest{
		State: d.State, Submission: map[string]any{"message": "공지", "when": tomorrowAt, "custom": "2026-10-16T00:00:00Z"},
	})
	if code != 200 || len(out) != 0 {
		t.Fatalf("submit: %d %v", code, out)
	}

	f.p.lock.Lock()
	msgs, _ := f.p.userMsgs(userKo)
	f.p.lock.Unlock()
	if len(msgs) != 1 || msgs[0].RootID != rootID || msgs[0].SendAt != seoul(t, "2026-10-16 09:00").UnixMilli() {
		t.Fatalf("stored = %+v", msgs)
	}

	// The confirmation's delete button replaces the confirmation with the result.
	conf := f.api.ephemeral[len(f.api.ephemeral)-1]
	attachments := conf.Attachments()
	ctx := attachments[0].Actions[1].Integration.Context
	code, _ = doJSON(t, f, "POST", "/action/delete", userKo, model.PostActionIntegrationRequest{UserId: userKo, PostId: conf.Id, ChannelId: chanID, Context: ctx})
	last := f.api.updatedEph[len(f.api.updatedEph)-1]
	if code != 200 || last.Id != conf.Id || !strings.Contains(last.Message, "예약 메시지를 삭제했습니다.") || last.RootId != rootID || len(last.Attachments()) != 0 {
		t.Fatalf("delete action: %d %+v", code, last)
	}
}

func TestEditDialogKeepsTime(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userEn, createInput{ChannelID: chanID, Message: "draft", When: whenInput{Preset: "monday"}})

	if err := f.p.openEditDialog(userEn, "trig", m.ID, nil, f.p.prefsFor(userEn)); err != nil {
		t.Fatal(err)
	}
	d := f.api.dialogs[0].Dialog
	if d.Elements[1].Default != "keep" || !strings.HasPrefix(d.Elements[1].Options[0].Text, "Keep the scheduled time (") {
		t.Fatalf("edit dialog = %+v", d.Elements[1])
	}

	code, out := doJSON(t, f, "POST", "/dialog/edit", userEn, model.SubmitDialogRequest{
		State: d.State, Submission: map[string]any{"message": "final", "when": "keep"},
	})
	if code != 200 || len(out) != 0 {
		t.Fatalf("submit: %d %v", code, out)
	}
	f.p.lock.Lock()
	got, _ := f.p.getMsg(m.ID)
	f.p.lock.Unlock()
	if got.Message != "final" || got.SendAt != m.SendAt {
		t.Fatalf("after edit: %+v", got)
	}
}

func TestCutRunes(t *testing.T) {
	if got := cutRunes("가나다", 2); got != "가나" {
		t.Fatalf("got %q", got)
	}
	if got := cutRunes("abc", 10); got != "abc" {
		t.Fatalf("got %q", got)
	}
}

func TestTemporaryErrorsAreRetried(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "retry me", When: whenInput{Preset: "tomorrow"}})
	f.advance(20 * time.Hour)

	f.api.channelErr[chanID] = true // database timeout
	f.p.runDue()
	f.api.channelErr[chanID] = false
	f.api.postDown = true // then the post itself fails with a 500
	f.p.runDue()          // too soon: waits a minute after the first problem
	f.advance(61 * time.Second)
	f.p.runDue()
	if len(f.api.created) != 0 {
		t.Fatalf("nothing should be sent or reported yet: %+v", f.api.created)
	}
	f.p.lock.Lock()
	got, _ := f.p.getMsg(m.ID)
	f.p.lock.Unlock()
	if got.Status != statusPending {
		t.Fatalf("status = %s (%s)", got.Status, got.Error)
	}

	if got, _ := f.p.getMsg(m.ID); got.Attempts != 2 {
		t.Fatalf("attempts = %d", got.Attempts)
	}

	f.api.postDown = false
	f.p.runDue()
	if len(f.api.created) != 0 {
		t.Fatal("retried before the 2-minute wait")
	}
	f.advance(2*time.Minute + time.Second)
	f.p.runDue()
	if len(f.api.created) != 1 || f.api.created[0].UserId != userKo {
		t.Fatalf("not sent after recovery: %+v", f.api.created)
	}
}

func TestKeptFailingForADay(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "x", When: whenInput{Preset: "tomorrow"}})
	f.api.postDown = true
	f.advance(20 * time.Hour)
	for i := 0; i < 60; i++ { // 30 hours in 30-minute steps
		f.p.runDue()
		f.advance(30 * time.Minute)
	}
	got, _ := f.p.getMsg(m.ID)
	if got.Status != statusFailed || got.Error != "fail.kept_failing" || got.Attempts > 60 {
		t.Fatalf("got %+v", got)
	}
}

func TestStaleIndexDoesNotSendEarly(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "monday news", When: whenInput{Preset: "tomorrow"}})

	// Move it to Monday, but the index write fails: the edit is undone and reported.
	f.api.failSet[keyIndex] = 1
	if _, err := f.p.update(userKo, m.ID, updateInput{When: &whenInput{Preset: "monday"}}); err == nil {
		t.Fatal("expected the failed save to be reported")
	}
	got, _ := f.p.getMsg(m.ID)
	if got.SendAt != m.SendAt {
		t.Fatalf("failed edit was kept: %+v", got)
	}

	// Now a stale index entry (tomorrow) with the message itself set to Monday.
	_, _ = f.p.update(userKo, m.ID, updateInput{When: &whenInput{Preset: "monday"}})
	f.p.lock.Lock()
	_ = f.p.kvSet(keyIndex, []indexEntry{{ID: m.ID, SendAt: m.SendAt}})
	f.p.lastReconcile.Store(model.GetMillisForTime(f.clock.Add(24 * time.Hour))) // no repair in between
	f.p.lock.Unlock()
	f.advance(20 * time.Hour) // tomorrow 10:00
	f.p.runDue()
	if len(f.api.created) != 0 {
		t.Fatal("sent three days early")
	}
	idx, _ := f.p.readIndex()
	if len(idx) != 1 || idx[0].SendAt != seoul(t, "2026-10-19 10:00").UnixMilli() {
		t.Fatalf("index not corrected: %v", idx)
	}
}

func TestDamagedValuesAreSkippedAndRepaired(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	good, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "good", When: whenInput{Preset: "tomorrow"}})
	bad, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "bad", When: whenInput{Preset: "tomorrow"}})
	f.api.kv[keyMsgPrefix+bad.ID] = []byte("{not json")
	f.api.kv[keyIndex] = []byte("also not json")

	msgs, err := f.p.userMsgs(userKo)
	if err != nil || len(msgs) != 1 || msgs[0].ID != good.ID {
		t.Fatalf("list with a damaged item: %v %v", msgs, err)
	}
	if _, err := f.p.create(userKo, createInput{ChannelID: chanID, Message: "new", When: whenInput{Preset: "monday"}}); err != nil {
		t.Fatalf("create with a damaged index: %v", err)
	}

	f.advance(20 * time.Hour)
	f.p.runDue() // the damaged index asked for a rebuild, so this run repairs first
	if len(f.api.created) != 1 || f.api.created[0].Message != "good" {
		t.Fatalf("created = %+v", f.api.created)
	}
}

func TestFileOnlyInOneScheduledMessage(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	f.api.files["f1"] = &model.FileInfo{Id: "f1", CreatorId: userKo, ChannelId: chanID, Name: "a.pdf"}
	m, err := f.p.create(userKo, createInput{ChannelID: chanID, FileIDs: []string{"f1", "f1"}, When: whenInput{Preset: "tomorrow"}})
	if err != nil || len(m.FileIDs) != 1 {
		t.Fatalf("duplicate IDs not merged: %v %v", m, err)
	}
	if _, err := f.p.create(userKo, createInput{ChannelID: chanID, FileIDs: []string{"f1"}, When: whenInput{Preset: "monday"}}); errorCode(err) != "err.file_in_use" {
		t.Fatalf("got %v", err)
	}
}

func TestRootPreviewNeedsAccess(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	f.api.posts[rootID].Message = "비밀 이야기"
	if got := f.p.rootPreview(userKo, rootID); got != "비밀 이야기" {
		t.Fatalf("member: %q", got)
	}
	delete(f.api.members, chanID+"/"+userKo)
	if got := f.p.rootPreview(userKo, rootID); got != "" {
		t.Fatalf("after leaving: %q", got)
	}
}

func TestEditFromListRedrawsThatList(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	f.api.channels["chan2"] = &model.Channel{Id: "chan2", TeamId: teamID, Type: model.ChannelTypeOpen, Name: "other"}
	f.api.members["chan2/"+userKo] = true
	m, _ := f.p.create(userKo, createInput{ChannelID: "chan2", Message: "goes to chan2", When: whenInput{Preset: "tomorrow"}})

	_, _ = f.p.ExecuteCommand(nil, &model.CommandArgs{Command: "/later list", UserId: userKo, ChannelId: chanID, TriggerId: "t"})
	list := f.api.ephemeral[len(f.api.ephemeral)-1]
	ctx := list.Attachments()[0].Actions[1].Integration.Context // Edit
	_, _ = doJSON(t, f, "POST", "/action/edit", userKo, model.PostActionIntegrationRequest{UserId: userKo, PostId: list.Id, ChannelId: chanID, TriggerId: "t2", Context: ctx})
	d := f.api.dialogs[len(f.api.dialogs)-1].Dialog

	code, out := doJSON(t, f, "POST", "/dialog/edit", userKo, model.SubmitDialogRequest{
		State: d.State, Submission: map[string]any{"message": "edited", "when": "keep"},
	})
	if code != 200 || len(out) != 0 {
		t.Fatalf("submit: %v", out)
	}
	upd := f.api.updatedEph[len(f.api.updatedEph)-1]
	if upd.Id != list.Id || upd.ChannelId != chanID || !strings.HasPrefix(upd.Message, "예약 메시지를 수정했습니다.") || !strings.Contains(upd.Attachments()[0].Text, "> edited") {
		t.Fatalf("list not redrawn in place: %+v", upd)
	}
	if got, _ := f.p.getMsg(m.ID); got.Message != "edited" {
		t.Fatal("not saved")
	}
}

func TestFailedSendNowKeepsTheSchedule(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "monday", When: whenInput{Preset: "monday"}})

	f.api.readOnly[chanID+"/"+userKo] = true
	if err := f.p.sendNow(userKo, m.ID); errorCode(err) != "fail.no_permission" {
		t.Fatalf("got %v", err)
	}
	f.api.readOnly[chanID+"/"+userKo] = false
	f.api.failPost = true
	if err := f.p.sendNow(userKo, m.ID); errorCode(err) != "fail.post_failed" {
		t.Fatalf("got %v", err)
	}
	f.p.lock.Lock()
	got, _ := f.p.getMsg(m.ID)
	idx, _ := f.p.readIndex()
	f.p.lock.Unlock()
	if got.Status != statusPending || got.SendAt != m.SendAt || len(idx) != 1 || idx[0].SendAt != m.SendAt {
		t.Fatalf("schedule changed: %+v, index %v", got, idx)
	}
}

func TestReconcileRepairsLists(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "lost", When: whenInput{Preset: "tomorrow"}})

	// Simulate half-finished writes: the index lost the message, the list has a ghost.
	f.p.lock.Lock()
	_ = f.p.kvSet(keyIndex, []indexEntry{})
	_ = f.p.kvSet(keyUserPrefix+userKo, []string{m.ID, "ghost"})
	_ = f.p.kvSet(keyUserPrefix+"nobody", []string{"ghost2"})
	f.p.lock.Unlock()

	f.advance(20 * time.Hour) // due now; runDue reconciles first
	f.p.runDue()
	if len(f.api.created) != 1 || f.api.created[0].Message != "lost" {
		t.Fatalf("lost message not sent: %+v", f.api.created)
	}
	f.p.lock.Lock()
	ids, _ := f.p.userIDs(userKo)
	_, other := f.api.kv[keyUserPrefix+"nobody"]
	f.p.lock.Unlock()
	if len(ids) != 0 || other {
		t.Fatalf("ghosts left: %v %v", ids, other)
	}
}

func TestPostingRules(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	f.api.files["f1"] = &model.FileInfo{Id: "f1", CreatorId: userKo, ChannelId: chanID}
	f.api.noUpload[chanID+"/"+userKo] = true
	if _, err := f.p.create(userKo, createInput{ChannelID: chanID, FileIDs: []string{"f1"}, When: whenInput{Preset: "tomorrow"}}); errorCode(err) != "fail.no_upload" {
		t.Fatalf("upload rule: %v", err)
	}

	dm := &model.Channel{Id: "dm1", Type: model.ChannelTypeDirect, Name: userEn + "__" + userKo}
	f.api.channels["dm1"] = dm
	f.api.members["dm1/"+userKo] = true
	f.api.restrictDM = true
	f.api.teamsOf[userKo] = []string{teamID}
	f.api.teamsOf[userEn] = []string{"otherteam"}
	if _, err := f.p.create(userKo, createInput{ChannelID: "dm1", Message: "hi", When: whenInput{Preset: "tomorrow"}}); errorCode(err) != "fail.dm_restricted" {
		t.Fatalf("DM rule: %v", err)
	}
	f.api.teamsOf[userEn] = []string{teamID}
	if _, err := f.p.create(userKo, createInput{ChannelID: "dm1", Message: "hi", When: whenInput{Preset: "tomorrow"}}); err != nil {
		t.Fatalf("shared team should be allowed: %v", err)
	}
}

func TestDialogRadioOnly(t *testing.T) {
	// No date picked: the radio's exact time is used, even if the dialog was opened before midnight.
	f := newFixture(t, seoul(t, "2026-10-15 23:58"))
	_, _ = f.p.ExecuteCommand(nil, &model.CommandArgs{Command: "/later", UserId: userKo, ChannelId: chanID, TriggerId: "trig"})
	d := f.api.dialogs[0].Dialog
	f.advance(3 * time.Minute) // submitted after midnight
	code, out := doJSON(t, f, "POST", "/dialog/create", userKo, model.SubmitDialogRequest{
		State: d.State, Submission: map[string]any{"message": "hi", "when": d.Elements[1].Default},
	})
	if code != 200 || len(out) != 0 {
		t.Fatalf("submit: %v", out)
	}
	f.p.lock.Lock()
	msgs, _ := f.p.userMsgs(userKo)
	f.p.lock.Unlock()
	if len(msgs) != 1 || msgs[0].SendAt != seoul(t, "2026-10-16 10:00").UnixMilli() {
		t.Fatalf("stored = %+v", msgs)
	}
}

func TestListRedrawsInPlace(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	_, _ = f.p.create(userKo, createInput{ChannelID: chanID, Message: "one", When: whenInput{Preset: "tomorrow"}})
	_, _ = f.p.create(userKo, createInput{ChannelID: chanID, Message: "two", When: whenInput{Preset: "monday"}})
	_, _ = f.p.ExecuteCommand(nil, &model.CommandArgs{Command: "/later list", UserId: userKo, ChannelId: chanID})
	list := f.api.ephemeral[len(f.api.ephemeral)-1]
	ctx := list.Attachments()[0].Actions[0].Integration.Context // "send now" on the first item

	code, _ := doJSON(t, f, "POST", "/action/send-now", userKo, model.PostActionIntegrationRequest{UserId: userKo, PostId: list.Id, ChannelId: chanID, Context: ctx})
	upd := f.api.updatedEph[len(f.api.updatedEph)-1]
	if code != 200 || upd.Id != list.Id {
		t.Fatalf("no in-place update: %d", code)
	}
	if upd.Id != list.Id || !strings.HasPrefix(upd.Message, "~town-square에 보냈습니다.") || len(upd.Attachments()) != 1 {
		t.Fatalf("updated list = %q, %d items", upd.Message, len(upd.Attachments()))
	}
	if len(f.api.created) != 1 || f.api.created[0].Message != "one" {
		t.Fatalf("created = %+v", f.api.created)
	}
}

func TestConfirmNoteFollowsTheMessage(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	code, item := doJSON(t, f, "POST", "/api/v1/scheduled", userKo, map[string]any{"channel_id": chanID, "message": "hi", "when": map[string]any{"preset": "today"}})
	if code != 200 {
		t.Fatal(item)
	}
	note := f.api.ephemeral[0]
	if note.Id == "" || len(note.Attachments()) != 1 {
		t.Fatalf("note = %+v", note)
	}

	// Moving the time redraws the same note, buttons kept.
	_, _ = doJSON(t, f, "PATCH", "/api/v1/scheduled/"+item["id"].(string), userKo, map[string]any{"when": map[string]any{"preset": "tomorrow"}})
	upd := f.api.updatedEph[len(f.api.updatedEph)-1]
	if upd.Id != note.Id || !strings.Contains(upd.Message, "내일 오전 10:00") || len(upd.Attachments()) != 1 {
		t.Fatalf("after reschedule: %+v", upd)
	}

	// Once sent, the note says so, links to the message, and has no buttons left.
	f.advance(20 * time.Hour)
	f.p.runDue()
	upd = f.api.updatedEph[len(f.api.updatedEph)-1]
	sent := f.api.created[0]
	if upd.Id != note.Id || !strings.HasPrefix(upd.Message, "오늘 오전 10:00에 보냈습니다.") ||
		!strings.Contains(upd.Message, "https://chat.example.com/_redirect/pl/"+sent.Id) || len(upd.Attachments()) != 0 {
		t.Fatalf("after sending: %+v", upd)
	}
}

func TestConfirmNoteAfterFailure(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "x", When: whenInput{Preset: "today"}})
	f.p.sendConfirmation(m, f.p.prefsFor(userKo))
	delete(f.api.members, chanID+"/"+userKo)
	f.advance(3 * time.Hour)
	f.p.runDue()
	upd := f.api.updatedEph[len(f.api.updatedEph)-1]
	if upd.Id != m.Confirm.PostID || !strings.HasPrefix(upd.Message, "보내지 못했습니다. 사유: 이 채널의 멤버가 아닙니다.") || len(upd.Attachments()) != 0 {
		t.Fatalf("after failure: %+v", upd)
	}
}

func TestSentList(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userKo, createInput{ChannelID: chanID, RootID: rootID, Message: "보낸 메시지", When: whenInput{Preset: "today"}})
	_ = m
	f.advance(3 * time.Hour)
	f.p.runDue()
	postID := f.api.created[0].Id

	code, out := doJSON(t, f, "GET", "/api/v1/sent", userKo, nil)
	items := out["items"].([]any)
	if code != 200 || len(items) != 1 {
		t.Fatalf("sent list: %d %v", code, out)
	}
	it := items[0].(map[string]any)
	if it["post_id"] != postID || it["message"] != "보낸 메시지" || it["when"] != "오늘 오후 5:00" || it["can_edit"] != true || it["root_id"] != rootID {
		t.Fatalf("item = %v", it)
	}

	// Someone else can't touch it.
	if code, _ := doJSON(t, f, "PATCH", "/api/v1/sent/"+postID, userEn, map[string]any{"message": "hacked"}); code != 404 {
		t.Fatalf("edit by someone else: %d", code)
	}
	if code, _ := doJSON(t, f, "DELETE", "/api/v1/sent/"+postID, userEn, nil); code != 404 {
		t.Fatalf("delete by someone else: %d", code)
	}

	// Edit, within Mattermost's rules.
	if code, out := doJSON(t, f, "PATCH", "/api/v1/sent/"+postID, userKo, map[string]any{"message": "고친 메시지"}); code != 200 {
		t.Fatalf("edit: %d %v", code, out)
	}
	if f.api.posts[postID].Message != "고친 메시지" {
		t.Fatal("not edited")
	}
	_, out = doJSON(t, f, "GET", "/api/v1/sent", userKo, nil)
	if it := out["items"].([]any)[0].(map[string]any); it["edited"] != true || it["message"] != "고친 메시지" {
		t.Fatalf("after edit: %v", it)
	}
	f.api.editLimit = 60
	f.api.posts[postID].CreateAt = f.clock.UnixMilli() // the fake stamps posts with the real clock
	f.advance(2 * time.Minute)
	if code, out := doJSON(t, f, "PATCH", "/api/v1/sent/"+postID, userKo, map[string]any{"message": "late"}); code != 400 || out["code"] != "err.edit_too_late" {
		t.Fatalf("edit after the limit: %d %v", code, out)
	}
	f.api.editLimit = -1
	f.api.noEdit = true
	if code, out := doJSON(t, f, "PATCH", "/api/v1/sent/"+postID, userKo, map[string]any{"message": "x"}); code != 400 || out["code"] != "err.no_edit" {
		t.Fatalf("edit without permission: %d %v", code, out)
	}

	// Delete.
	if code, out := doJSON(t, f, "DELETE", "/api/v1/sent/"+postID, userKo, nil); code != 200 {
		t.Fatalf("delete: %d %v", code, out)
	}
	if len(f.api.deleted) != 1 {
		t.Fatal("not deleted")
	}
	_, out = doJSON(t, f, "GET", "/api/v1/sent", userKo, nil)
	if len(out["items"].([]any)) != 0 {
		t.Fatalf("still listed: %v", out)
	}
}

func TestSentListKeepsThirtyDays(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	_, _ = f.p.create(userKo, createInput{ChannelID: chanID, Message: "old", When: whenInput{Preset: "today"}})
	f.advance(3 * time.Hour)
	f.p.runDue()
	f.advance(31 * 24 * time.Hour)
	_, out := doJSON(t, f, "GET", "/api/v1/sent", userKo, nil)
	if len(out["items"].([]any)) != 0 {
		t.Fatalf("kept past 30 days: %v", out)
	}
}

func TestAllTextKeysExist(t *testing.T) {
	// Every key passed as a literal to tr(...) or failWith(...) must have Korean and English.
	re := regexp.MustCompile(`(?:tr\([^,]+,\s*|failWith\()"([a-z_.]+)"\s*[,)]`)
	files, _ := filepath.Glob("*.go")
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, _ := os.ReadFile(file)
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			pair, ok := messages[m[1]]
			if !ok || pair[0] == "" || pair[1] == "" {
				t.Errorf("%s: text %q is missing", file, m[1])
			}
		}
	}
}

func TestSlashCommandsAreRefused(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	for _, msg := range []string{"/away", "/later list"} {
		if _, err := f.p.create(userKo, createInput{ChannelID: chanID, Message: msg, When: whenInput{Preset: "tomorrow"}}); errorCode(err) != "err.command" {
			t.Errorf("%q: got %v", msg, err)
		}
	}
	for _, msg := range []string{"//not a command", " /with a space", "a/b"} {
		if _, err := f.p.create(userKo, createInput{ChannelID: chanID, Message: msg, When: whenInput{Preset: "tomorrow"}}); err != nil {
			t.Errorf("%q should be allowed: %v", msg, err)
		}
	}
}

func TestFailedSendNowKeepsTheNote(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	_, item := doJSON(t, f, "POST", "/api/v1/scheduled", userKo, map[string]any{"channel_id": chanID, "message": "hi", "when": map[string]any{"preset": "monday"}})
	note := f.api.ephemeral[0]
	ctx := note.Attachments()[0].Actions[0].Integration.Context // Send now

	f.api.readOnly[chanID+"/"+userKo] = true
	_, _ = doJSON(t, f, "POST", "/action/send-now", userKo, model.PostActionIntegrationRequest{UserId: userKo, PostId: note.Id, ChannelId: chanID, Context: ctx})
	upd := f.api.updatedEph[len(f.api.updatedEph)-1]
	if upd.Id != note.Id || len(upd.Attachments()) != 1 || !strings.Contains(upd.Message, "보내도록 예약했습니다") || !strings.Contains(upd.Message, "쓸 권한이 없습니다") {
		t.Fatalf("note after a failed send now: %q (%d button rows)", upd.Message, len(upd.Attachments()))
	}
	got, _ := f.p.getMsg(item["id"].(string))
	if got.Status != statusPending {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestSentButNotRemovedIsNotReportedAsFailed(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	m, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: "once", When: whenInput{Preset: "today"}})
	f.advance(3 * time.Hour)
	f.api.failSet[keyUserPrefix+userKo] = 5 // removing it from the person's list fails
	f.p.runDue()
	if len(f.api.created) != 1 {
		t.Fatalf("created = %d", len(f.api.created))
	}
	f.advance(10 * time.Minute) // past the "sending" expiry
	f.p.runDue()
	for _, p := range f.api.created[1:] {
		t.Fatalf("extra post (a failure DM?): %q", p.Message)
	}
	if msgs, _ := f.p.userMsgs(userKo); len(msgs) != 0 {
		t.Fatalf("still listed: %+v", msgs[0])
	}
	if got, _ := f.p.getMsg(m.ID); got != nil {
		t.Fatalf("not cleaned up: %+v", got)
	}
}

func TestIndexOrderIsStable(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	for i := 0; i < 5; i++ {
		_, _ = f.p.create(userKo, createInput{ChannelID: chanID, Message: "same time", When: whenInput{Preset: "tomorrow"}})
	}
	f.api.failSet[keyIndex] = 1 // a rewrite would fail; a correct index needs none
	if err := f.p.reconcile(); err != nil {
		t.Fatalf("reconcile rewrote an index that was fine: %v", err)
	}
}

func TestArchivedChannelSentMessage(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	_, _ = f.p.create(userKo, createInput{ChannelID: chanID, Message: "x", When: whenInput{Preset: "today"}})
	f.advance(3 * time.Hour)
	f.p.runDue()
	postID := f.api.created[0].Id
	f.api.channels[chanID].DeleteAt = 1
	_, out := doJSON(t, f, "GET", "/api/v1/sent", userKo, nil)
	if it := out["items"].([]any)[0].(map[string]any); it["can_edit"] != false {
		t.Fatalf("archived channel still editable: %v", it)
	}
	if code, out := doJSON(t, f, "PATCH", "/api/v1/sent/"+postID, userKo, map[string]any{"message": "y"}); code != 400 || out["code"] != "fail.archived" {
		t.Fatalf("edit: %d %v", code, out)
	}
	if code, out := doJSON(t, f, "DELETE", "/api/v1/sent/"+postID, userKo, nil); code != 400 || out["code"] != "fail.archived" {
		t.Fatalf("delete: %d %v", code, out)
	}
}

func TestDirectMessageLabelFallback(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	f.api.channels["dm2"] = &model.Channel{Id: "dm2", Type: model.ChannelTypeDirect, Name: "gone__" + userKo}
	if got := f.p.channelLabel("dm2", f.p.prefsFor(userKo)).Display; got != "개인 메시지" {
		t.Fatalf("label = %q", got)
	}
}

func TestCommandAutocompleteIsValid(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	cmd := f.p.command()
	if err := cmd.AutocompleteData.IsValid(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cmd.AutoCompleteHint, "[메시지 | 목록 | 도움말]") || cmd.AutocompleteData.SubCommands[0].Trigger != "목록" {
		t.Fatalf("not Korean-first: %q", cmd.AutoCompleteHint)
	}
}

func TestListCommand(t *testing.T) {
	f := newFixture(t, seoul(t, thursdayAfternoon))
	_, _ = f.p.create(userKo, createInput{ChannelID: chanID, RootID: rootID, Message: "하나", When: whenInput{Preset: "tomorrow"}})
	_, _ = f.p.ExecuteCommand(nil, &model.CommandArgs{Command: "/later 목록", UserId: userKo, ChannelId: chanID})
	last := f.api.ephemeral[len(f.api.ephemeral)-1]
	if !strings.Contains(last.Message, "예약된 메시지 1개") {
		t.Fatalf("list = %q", last.Message)
	}
	text := last.Attachments()[0].Text
	if !strings.Contains(text, "**내일 오전 10:00** · ~town-square · 스레드 답글") || !strings.Contains(text, "> 하나") {
		t.Fatalf("item = %q", text)
	}
	f.api.posts[rootID].Message = "다음 주 촬영 일정\n공유드립니다"
	_, _ = f.p.ExecuteCommand(nil, &model.CommandArgs{Command: "/later list", UserId: userKo, ChannelId: chanID})
	text = f.api.ephemeral[len(f.api.ephemeral)-1].Attachments()[0].Text
	if !strings.Contains(text, "「다음 주 촬영 일정 공유드립니다」에 단 답글") {
		t.Fatalf("item = %q", text)
	}
}

func TestDialogsAreValid(t *testing.T) {
	// OpenInteractiveDialog in the fake runs Mattermost's own validation.
	f := newFixture(t, seoul(t, thursdayAfternoon))
	long := strings.Repeat("가", dialogTextMax+50) // over the dialog's limit, under the post limit
	if err := f.p.openCreateDialog("trig", chanID, "", long, f.p.prefsFor(userKo)); err != nil {
		t.Fatal(err)
	}
	m, _ := f.p.create(userKo, createInput{ChannelID: chanID, Message: long, When: whenInput{Preset: "tomorrow"}})
	if err := f.p.openEditDialog(userKo, "trig", m.ID, nil, f.p.prefsFor(userKo)); err != nil {
		t.Fatal(err)
	}
	if f.api.dialogs[1].Dialog.IntroductionText == "" || f.api.dialogs[1].Dialog.Elements[0].Name != "when" {
		t.Fatal("a long message should only allow changing the time")
	}
}
