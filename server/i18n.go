package main

import (
	"fmt"
	"strings"
)

// messages holds every server-side text as {Korean, English}.
// Korean uses 합니다체 with 「~해 주세요」 for requests.
var messages = map[string][2]string{
	// Quick choices and time labels.
	"preset.hours":    {"%d시간 후", "In %d hours"},
	"preset.hour":     {"1시간 후", "In 1 hour"},
	"preset.today":    {"오늘 %s", "Today at %s"},
	"preset.tomorrow": {"내일 %s", "Tomorrow at %s"},
	"preset.monday":   {"월요일 %s", "Monday at %s"},
	"when.today":      {"오늘 %s", "today at %s"},
	"when.tomorrow":   {"내일 %s", "tomorrow at %s"},
	"when.date":       {"%s %s", "%s at %s"},

	// Confirmations.
	"confirm":        {"%s에 보내도록 예약했습니다. ⏰", "Scheduled for %s. ⏰"},
	"sent":           {"보냈습니다.", "Sent."},
	"sent.to":        {"%s에 보냈습니다.", "Sent to %s."},
	"deleted":        {"예약 메시지를 삭제했습니다.", "Scheduled message deleted."},
	"confirm.sent":   {"%s에 보냈습니다.", "Sent %s."},
	"confirm.failed": {"보내지 못했습니다. 사유: %s", "Not sent. Reason: %s"},
	"sent.go":        {"메시지로 이동", "Go to message"},
	"deleted.was":    {"삭제한 내용:", "What it said:"},
	"updated":        {"예약 메시지를 수정했습니다. %s에 보냅니다.", "Scheduled message updated. Now set for %s."},

	// Buttons.
	"btn.send_now": {"지금 보내기", "Send now"},
	"btn.edit":     {"수정", "Edit"},
	"btn.delete":   {"삭제", "Delete"},

	// Errors shown while scheduling or editing.
	"err.empty":          {"보낼 메시지를 입력해 주세요.", "Type a message first."},
	"err.command":        {"/로 시작하는 메시지는 명령어라서 예약할 수 없습니다. 글로 보내려면 맨 앞에 띄어쓰기를 넣어 주세요.", "Messages starting with / are commands and can't be scheduled. To send it as text, start it with a space."},
	"err.edit_failed":    {"Mattermost에서 메시지를 수정하지 못했습니다.", "Mattermost could not edit the message."},
	"err.delete_failed":  {"Mattermost에서 메시지를 삭제하지 못했습니다.", "Mattermost could not delete the message."},
	"err.too_long":       {"메시지가 너무 깁니다.", "The message is too long."},
	"err.past":           {"이미 지난 시간입니다. 지금 이후의 시간을 골라 주세요.", "That time has already passed. Pick a later time."},
	"err.too_far":        {"1년 안의 시간만 예약할 수 있습니다.", "You can schedule up to one year ahead."},
	"err.too_many":       {"예약 메시지는 한 사람당 %d개까지입니다. 필요 없는 예약을 삭제한 뒤 다시 시도해 주세요.", "You can have up to %d scheduled messages. Delete one you no longer need, then try again."},
	"err.no_time":        {"보낼 시간을 골라 주세요.", "Pick a time."},
	"err.bad_time":       {"날짜와 시간을 다시 확인해 주세요.", "Check the date and time."},
	"err.file_in_use":    {"이 파일은 이미 다른 예약 메시지에 첨부되어 있습니다.", "This file is already attached to another scheduled message."},
	"err.too_many_files": {"파일은 10개까지 첨부할 수 있습니다.", "You can attach up to 10 files."},
	"err.not_found":      {"예약 메시지를 찾을 수 없습니다. 이미 보냈거나 삭제한 메시지입니다.", "Scheduled message not found. It was already sent or deleted."},
	"err.busy":           {"지금 보내는 중입니다. 잠시 후 다시 확인해 주세요.", "It is being sent right now. Check again in a moment."},
	"err.sent_gone":      {"메시지를 찾을 수 없습니다. 이미 삭제되었을 수 있습니다.", "Message not found. It may have been deleted."},
	"err.edit_too_late":  {"보낸 지 오래되어 더 이상 수정할 수 없습니다.", "It is too late to edit this message."},
	"err.no_edit":        {"이 메시지를 수정할 권한이 없습니다.", "You do not have permission to edit this message."},
	"err.no_delete":      {"이 메시지를 삭제할 권한이 없습니다.", "You do not have permission to delete this message."},
	"err.internal":       {"문제가 생겼습니다. 잠시 후 다시 시도해 주세요.", "Something went wrong. Try again in a moment."},

	// Why a message could not be scheduled or sent.
	"fail.channel_gone":   {"채널을 찾을 수 없습니다.", "The channel could not be found."},
	"fail.archived":       {"보관된 채널입니다.", "The channel is archived."},
	"fail.not_member":     {"이 채널의 멤버가 아닙니다.", "You are not a member of this channel."},
	"fail.no_permission":  {"이 채널에 메시지를 쓸 권한이 없습니다.", "You do not have permission to post in this channel."},
	"fail.thread_deleted": {"답글을 달 원래 메시지가 삭제되었습니다.", "The message you were replying to was deleted."},
	"fail.user_inactive":  {"계정이 비활성화되었습니다.", "Your account is deactivated."},
	"fail.files":          {"첨부 파일을 찾을 수 없습니다.", "The attached files could not be found."},
	"fail.no_upload":      {"이 채널에 파일을 올릴 권한이 없습니다.", "You do not have permission to upload files in this channel."},
	"fail.dm_restricted":  {"같은 팀에 있는 사람에게만 개인 메시지를 보낼 수 있습니다.", "You can only send direct messages to people on one of your teams."},
	"fail.too_late":       {"예약 시간에 서버가 멈춰 있었고, 하루가 넘게 지나 보내지 않았습니다.", "The server was down at the scheduled time and it was over a day late, so it was not sent."},
	"fail.kept_failing":   {"일시적인 서버 문제가 계속되어 하루가 넘도록 보내지 못했습니다.", "A temporary server problem kept it from being sent for over a day."},
	"fail.maybe_sent":     {"보내는 도중 서버가 멈췄습니다. 채널에 메시지가 올라갔는지 확인해 주세요.", "The server stopped while sending. Check whether the message was posted."},
	"fail.post_failed":    {"Mattermost에서 메시지를 올리지 못했습니다.", "Mattermost could not post the message."},

	// Direct message when sending fails.
	"dm.failed": {
		"예약 메시지를 보내지 못했습니다. ⏰\n보낼 곳: %s\n사유: %s",
		"A scheduled message could not be sent. ⏰\nWhere: %s\nReason: %s",
	},
	"dm.failed.next": {
		"다시 예약하거나 삭제하려면 입력창 아래의 시계 버튼을 누르고 「예약된 메시지」를 열어 주세요. 휴대폰에서는 `/later list`를 입력해 주세요.",
		"To reschedule or delete it, press the clock button under the message box and open Scheduled messages. On mobile, type `/later list`.",
	},

	// Dialog (mobile and /later).
	"dialog.title":        {"메시지 예약", "Schedule message"},
	"dialog.edit_title":   {"예약 메시지 수정", "Edit scheduled message"},
	"dialog.retry_title":  {"다시 예약", "Reschedule message"},
	"dialog.message":      {"메시지", "Message"},
	"dialog.when":         {"보낼 시간", "When"},
	"dialog.custom_field": {"날짜와 시간", "Date and time"},
	"dialog.custom_help":  {"여기서 날짜와 시간을 고르면 위의 선택 대신 이 시간에 보냅니다. 시간대: %s", "If you pick a date and time here, it is used instead of the choice above. Time zone: %s"},
	"dialog.keep":         {"예약된 시간 그대로 (%s)", "Keep the scheduled time (%s)"},
	"dialog.submit":       {"예약", "Schedule"},
	"dialog.save":         {"저장", "Save"},
	"dialog.thread":       {"이 스레드에 답글로 보냅니다.", "This will be sent as a reply in this thread."},
	"dialog.long":         {"메시지가 길어 여기서는 시간만 바꿀 수 있습니다. 내용은 컴퓨터에서 수정해 주세요.", "The message is too long to edit here, so only the time can be changed. Edit the text on a computer."},
	"dialog.cut":          {"메시지가 길어 앞부분만 넣었습니다. 전체를 보내려면 컴퓨터에서 예약해 주세요.", "The message is too long for this form, so only the beginning was kept. To send all of it, schedule it on a computer."},

	// /later list and help.
	"list.empty":    {"예약된 메시지가 없습니다. `/later`로 예약할 수 있습니다.", "You have no scheduled messages. Use `/later` to schedule one."},
	"list.header":   {"예약된 메시지 %d개 ⏰", "Scheduled messages: %d ⏰"},
	"list.more":     {"처음 %d개만 표시했습니다. 전체 목록은 컴퓨터의 「예약된 메시지」에서 볼 수 있습니다.", "Showing the first %d. See them all under Scheduled messages on a computer."},
	"list.reply":    {"스레드 답글", "thread reply"},
	"list.reply_to": {"「%s」에 단 답글", "reply to “%s”"},
	"list.failed":   {"보내지 못함", "Not sent"},
	"list.files":    {"첨부 파일 %d개", "Attachments: %d"},
	"label.group":   {"그룹 메시지", "group message"},
	"label.dm":      {"개인 메시지", "direct message"},
	"help": {
		"**예약 전송 사용법**\n" +
			"- 컴퓨터: 메시지를 입력한 뒤 입력창 아래의 시계 버튼(⏰)을 눌러 시간을 고릅니다.\n" +
			"- 휴대폰: `/later`를 입력하면 예약 창이 열립니다. `/later 보낼 메시지`처럼 내용을 바로 써도 됩니다.\n" +
			"- 목록: `/later list` (지금 보내기 · 수정 · 삭제)\n" +
			"- 시간은 Mattermost의 표준 시간대(%s)를 따릅니다. 설정 → 화면 → 표준 시간대에서 바꿀 수 있습니다.\n" +
			"- 파일 첨부는 컴퓨터에서만 예약할 수 있습니다.",
		"**How to send later**\n" +
			"- Computer: type your message, then press the clock button (⏰) under the message box and pick a time.\n" +
			"- Mobile: type `/later` to open the schedule form. You can also write the message straight away: `/later your message`.\n" +
			"- List: `/later list` (send now · edit · delete)\n" +
			"- Times follow your Mattermost time zone (%s). Change it under Settings → Display → Timezone.\n" +
			"- Files can only be scheduled from a computer.",
	},

	// Slash command autocomplete.
	"cmd.desc": {"메시지를 예약해서 나중에 보냅니다", "Schedule a message to send later"},
	"cmd.hint": {"[메시지 | 목록 | 도움말]", "[message | list | help]"},
	"cmd.list": {"예약된 메시지 목록", "Scheduled messages"},
	"cmd.help": {"사용법", "How to use"},
}

// tr returns the text for key in the user's language, formatted with args.
func tr(ko bool, key string, args ...any) string {
	pair, ok := messages[key]
	if !ok {
		return key
	}
	text := pair[1]
	if ko {
		text = pair[0]
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

// quote turns text into a Markdown block quote.
func quote(text string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		lines[i] = "> " + line
	}
	return strings.Join(lines, "\n")
}

// preview shortens text for lists, keeping whole characters.
func preview(text string, max int) string {
	text = strings.TrimSpace(text)
	if cut := cutRunes(text, max); cut != text {
		return strings.TrimSpace(cut) + "…"
	}
	return text
}
