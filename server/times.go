package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

// userPrefs holds what is needed to show and read times the way a person expects.
type userPrefs struct {
	userID string
	Loc    *time.Location
	TZName string
	Ko     bool
	H24    bool
}

var (
	koWeekdays = [...]string{"일", "월", "화", "수", "목", "금", "토"}
	enWeekdays = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	enMonths   = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
)

func (p *Plugin) prefsFor(userID string) userPrefs {
	up := userPrefs{userID: userID, Loc: time.UTC, TZName: "UTC"}

	tz := ""
	locale := ""
	if user, appErr := p.API.GetUser(userID); appErr == nil {
		tz = user.GetPreferredTimezone()
		locale = user.Locale
	}
	if locale == "" {
		if cfg := p.API.GetConfig(); cfg != nil && cfg.LocalizationSettings.DefaultClientLocale != nil {
			locale = *cfg.LocalizationSettings.DefaultClientLocale
		}
	}
	up.Ko = strings.HasPrefix(locale, "ko")

	if tz == "" {
		tz = strings.TrimSpace(p.getConfiguration().DefaultTimezone)
	}
	if tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			up.Loc = loc
			up.TZName = tz
		}
	}

	if pref, appErr := p.API.GetPreferenceForUser(userID, model.PreferenceCategoryDisplaySettings, model.PreferenceNameUseMilitaryTime); appErr == nil {
		up.H24 = pref.Value == "true"
	}

	return up
}

// preset is one quick choice in the schedule menu.
type preset struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
	SendAt int64  `json:"send_at"`
}

// computePresets returns the quick choices (today, tomorrow, Monday) in the person's time zone.
func computePresets(now time.Time, up userPrefs, cfg *configuration) []preset {
	n := now.In(up.Loc)
	y, mo, d := n.Date()
	var out []preset

	add := func(key string, at time.Time) {
		out = append(out, preset{
			Key:    key,
			Label:  tr(up.Ko, "preset."+key, clockLabel(at, up)),
			Detail: dateLabel(at, n, up),
			SendAt: model.GetMillisForTime(at),
		})
	}

	// "In 1 hour", "In 2 hours": the detail shows the clock time it lands on.
	for _, h := range cfg.relativeHours() {
		at := n.Add(time.Duration(h) * time.Hour).Truncate(time.Minute)
		if at.Before(n.Add(time.Duration(h) * time.Hour)) {
			at = at.Add(time.Minute) // round up, never earlier than promised
		}
		label := tr(up.Ko, "preset.hours", h)
		if !up.Ko && h == 1 {
			label = tr(false, "preset.hour")
		}
		detail := clockLabel(at, up)
		if dayDiff(n, at) != 0 {
			detail = dateLabel(at, n, up) + " " + detail
		}
		out = append(out, preset{Key: fmt.Sprintf("in%dh", h), Label: label, Detail: detail, SendAt: model.GetMillisForTime(at)})
	}

	h, m := parseClock(cfg.TodayTime, 17, 0)
	if today := time.Date(y, mo, d, h, m, 0, 0, up.Loc); today.After(n.Add(time.Minute)) {
		add("today", today)
	}

	h, m = parseClock(cfg.TomorrowTime, 10, 0)
	tomorrow := time.Date(y, mo, d+1, h, m, 0, 0, up.Loc)
	add("tomorrow", tomorrow)

	h, m = parseClock(cfg.MondayTime, 10, 0)
	days := (8 - int(n.Weekday())) % 7
	if days == 0 {
		days = 7
	}
	if monday := time.Date(y, mo, d+days, h, m, 0, 0, up.Loc); !monday.Equal(tomorrow) {
		add("monday", monday)
	}

	return out
}

func clockLabel(t time.Time, up userPrefs) string {
	if up.H24 {
		return t.Format("15:04")
	}
	h := t.Hour()
	h12 := h % 12
	if h12 == 0 {
		h12 = 12
	}
	if up.Ko {
		half := "오전"
		if h >= 12 {
			half = "오후"
		}
		return fmt.Sprintf("%s %d:%02d", half, h12, t.Minute())
	}
	half := "AM"
	if h >= 12 {
		half = "PM"
	}
	return fmt.Sprintf("%d:%02d %s", h12, t.Minute(), half)
}

// dateLabel writes a short date, adding the year only when it differs from now's.
func dateLabel(t, now time.Time, up userPrefs) string {
	if up.Ko {
		if t.Year() != now.Year() {
			return fmt.Sprintf("%d년 %d월 %d일 (%s)", t.Year(), int(t.Month()), t.Day(), koWeekdays[t.Weekday()])
		}
		return fmt.Sprintf("%d월 %d일 (%s)", int(t.Month()), t.Day(), koWeekdays[t.Weekday()])
	}
	if t.Year() != now.Year() {
		return fmt.Sprintf("%s %d %s %d", enWeekdays[t.Weekday()], t.Day(), enMonths[t.Month()-1], t.Year())
	}
	return fmt.Sprintf("%s %d %s", enWeekdays[t.Weekday()], t.Day(), enMonths[t.Month()-1])
}

// dayDiff counts calendar days from now to t in t's location.
func dayDiff(now, t time.Time) int {
	y1, m1, d1 := now.Date()
	y2, m2, d2 := t.Date()
	a := time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)
	b := time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC)
	return int(math.Round(b.Sub(a).Hours() / 24))
}

// whenLabel describes a send time: "내일 오전 10:00", "today at 5:00 PM", "10월 20일 (월) 오전 10:00".
// In English, sentence=false capitalises the first letter for use as a heading.
func whenLabel(sendAt int64, now time.Time, up userPrefs, sentence bool) string {
	t := time.UnixMilli(sendAt).In(up.Loc)
	n := now.In(up.Loc)
	clock := clockLabel(t, up)

	var label string
	switch dayDiff(n, t) {
	case 0:
		label = tr(up.Ko, "when.today", clock)
	case 1:
		label = tr(up.Ko, "when.tomorrow", clock)
	default:
		label = tr(up.Ko, "when.date", dateLabel(t, n, up), clock)
	}
	if !up.Ko && !sentence && label != "" {
		label = strings.ToUpper(label[:1]) + label[1:]
	}
	return label
}

// parseLocal reads "2026-10-20" and "14:30" as a time in loc.
func parseLocal(date, clock string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(date)+" "+strings.TrimSpace(clock), loc)
}

// parseDialogDateTime reads the value of a dialog "datetime" field.
// Clients send UTC ("2026-10-20T05:30:00Z"); values without a zone are read in loc.
func parseDialogDateTime(value string, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date-time %q", value)
}
