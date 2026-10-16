# 예약 전송 · Send Later

Mattermost 메시지를 Slack처럼 원하는 시간에 보내도록 예약합니다. 무료 오픈소스(MIT)이며, 직접 설치한 Mattermost Team Edition을 포함한 모든 에디션에서 쓸 수 있습니다.

Schedule Mattermost messages to send later, the way Slack does it. Free and open source (MIT), for self-hosted Mattermost Team Edition and every other edition.

| 컴퓨터 · Computer | 목록 · List |
|---|---|
| 입력창 아래 시계 버튼: 1시간 후, 2시간 후, 오늘 17:00, 내일 10:00, 월요일 10:00, 또는 직접 선택. | 오른쪽 사이드바의 예약된 메시지: 지금 보내기 · 수정 · 복사 · 삭제. |
| A clock button under the message box: in 1 hour, in 2 hours, today 17:00, tomorrow 10:00, Monday 10:00, or a custom time. | Scheduled messages in the right-hand sidebar: send now, edit, copy, delete. |

---

## 한국어

### 사용법

- **컴퓨터 (웹·데스크톱 앱):** 메시지를 입력하고 입력창 아래의 시계 버튼(⏰)을 누른 뒤 시간을 고릅니다. 첨부 파일과 스레드 답글도 예약할 수 있습니다. 버튼의 숫자는 이 채널에 예약된 메시지 수입니다. 시계 버튼은 서식 막대에 있으므로, 보이지 않으면 입력창의 서식 표시(Aa)를 켜 주세요.
- **예약된 메시지 목록:** 시계 버튼 → 「예약된 메시지」, 또는 오른쪽 앱 바의 시계 아이콘. 「예약됨」 탭에서 지금 보내기 · 수정 · 복사 · 삭제를 할 수 있고, 채널 이름을 누르면 그 채널로 이동합니다.
- **보낸 메시지:** 같은 곳의 「보낸 메시지」 탭에 최근 30일 동안 예약으로 보낸 메시지가 표시됩니다. 메시지로 이동 · 수정 · 복사 · 삭제를 할 수 있습니다 (Mattermost의 수정·삭제 권한과 수정 가능 시간을 그대로 따릅니다).
- 예약할 때 표시되는 확인 메시지(지금 보내기 · 삭제 버튼)는 메시지를 보내거나 삭제하면 「…에 보냈습니다 · 메시지로 이동」처럼 결과로 바뀝니다.
- **휴대폰 앱:** 휴대폰 앱에는 시계 버튼이 없으므로 `/later`를 입력합니다. 예약 창이 열리고 시간을 고를 수 있습니다. `/later 보낼 메시지`처럼 내용을 바로 써도 됩니다. 목록은 `/later list`, 도움말은 `/later help`입니다. 휴대폰에서는 첨부 파일을 예약할 수 없습니다.
- 메시지 우선순위(중요·긴급)가 설정된 메시지와 /로 시작하는 명령어는 예약할 수 없습니다.
- 예약하면 그 자리에 본인만 보이는 확인 메시지가 「지금 보내기」·「삭제」 버튼과 함께 표시됩니다.
- 메시지는 **본인 이름으로** 보내집니다. 봇 이름으로 보내지지 않습니다.
- 시간은 각자 Mattermost에 설정한 표준 시간대를 따릅니다 (설정 → 화면 → 표준 시간대). 12시간/24시간 표시 설정도 따릅니다.

### 보내지 못한 경우

예약 시간에 채널을 나갔거나, 채널이 보관되었거나, 답글을 달 원래 메시지가 삭제되었으면 보내지 않습니다. 이때 **Send Later** 봇이 개인 메시지로 사유와 내용을 알려 주고, 목록에 「보내지 못함」으로 남습니다. 「다시 예약」으로 새 시간을 고르거나 삭제할 수 있습니다. 서버가 24시간 넘게 멈춰 있었던 경우에도 오래된 메시지를 늦게 보내지 않고 같은 방법으로 알려 줍니다.

### 설치 (시스템 관리자)

1. 플러그인 파일을 받습니다: `send-later-<버전>.tar.gz` (릴리스 페이지, 또는 아래 「직접 빌드」).
2. 관리자 도구(System Console) → 플러그인 → 플러그인 관리 → 「플러그인 업로드」에서 「파일 선택」 → 「업로드」로 올리고 **활성화**합니다. 업로드 버튼이 없으면 서버 설정에서 `PluginSettings.EnableUploads`를 켜야 합니다.
3. 관리자 도구 → 플러그인 → Send Later에서 빠른 선택 시간(기본: 1시간 후 · 2시간 후 · 오늘 17:00 · 내일 10:00 · 월요일 10:00)과, 표준 시간대를 설정하지 않은 사람에게 쓸 기본 시간대(예: `Asia/Seoul`)를 정합니다.

Mattermost **11.7 이상**이 필요합니다. 유료 라이선스가 있는 서버에서는 Mattermost 자체의 예약 메시지 기능도 「예약된 메시지」로 표시되므로, 두 가지가 함께 보일 수 있습니다.

---

## English

### How to use it

- **Computer (web and desktop app):** type your message, press the clock button (⏰) under the message box and pick a time. Files and thread replies can be scheduled too. The number on the button is how many messages are scheduled in this channel. The clock sits in the formatting bar, so if you don't see it, turn formatting on (Aa) in the message box.
- **Scheduled messages list:** clock button → *Scheduled messages*, or the clock icon in the App Bar on the right. On the *Scheduled* tab: send now, edit, copy or delete; click the channel name to go there.
- **Sent messages:** the *Sent* tab in the same place lists what was sent from a schedule in the last 30 days: go to message, edit, copy or delete (Mattermost's own edit/delete permissions and edit time limit apply).
- The note shown when you schedule (with *Send now* and *Delete*) turns into the result once the message is sent or deleted, e.g. "Sent today at 8:34 PM · Go to message".
- **Mobile app:** the mobile app has no clock button, so type `/later`. A form opens where you pick the time. You can also write the message straight away: `/later your message`. `/later list` shows the list and `/later help` shows help. Files can't be scheduled from a phone.
- Messages with a priority (Important or Urgent) and slash commands (starting with /) can't be scheduled.
- After scheduling, a note only you can see appears where you wrote, with *Send now* and *Delete* buttons.
- Messages are sent **as you**, not as a bot.
- Times follow each person's Mattermost time zone (Settings → Display → Timezone) and their 12/24-hour clock setting.

### When a message cannot be sent

If, at the scheduled time, you have left the channel, the channel was archived or the message you were replying to was deleted, the message is not sent. The **Send Later** bot sends you a direct message with the reason and the text, and the message stays in the list as *Not sent*, where you can *Reschedule* or *Delete* it. If the server was down for more than 24 hours, old messages are not sent late; you are told the same way.

### Install (system admin)

1. Get the plugin file `send-later-<version>.tar.gz` (from the releases page, or see *Build it yourself* below).
2. System Console → Plugins → Plugin Management → *Upload Plugin*, then **Enable** it. If there is no upload button, turn on `PluginSettings.EnableUploads` in the server configuration.
3. System Console → Plugins → Send Later: set the quick-choice times (default: in 1 hour, in 2 hours, today 17:00, tomorrow 10:00, Monday 10:00) and the default time zone for people who have not set one (for example `Europe/Paris`).

Requires Mattermost **11.7 or later**. On servers with a paid licence, Mattermost's own scheduled messages also appear as *Scheduled messages*, so people may see both.

---

## 업데이트 · Updating

새 버전은 명령어 하나로 설치합니다. 테스트 → 빌드 → 업로드(기존 버전 교체) → 활성화 → 새 버전이 실제로 실행 중인지 확인까지 합니다. 자동으로 업데이트되지는 않으며, 누군가 이 명령을 실행할 때만 바뀝니다.

A new version installs with one command: test → build → upload (replacing the old one) → enable → check the new version is live. It never updates by itself; it changes only when someone runs it.

```sh
MM_URL=https://chat.example.com ./deploy.sh                  # linux-amd64 build
MM_URL=https://chat.example.com ARCH=linux-arm64 ./deploy.sh
# or put MM_URL=https://chat.example.com in a .deploy.env file next to the script (not committed)
```

처음 한 번만: 시스템 관리자(또는 시스템 관리자 역할의 봇)의 토큰을 만들어 macOS 키체인에 저장합니다. Mattermost에서 관리자 도구 → 통합 → 봇 계정 → 봇 → 「새 토큰 만들기」(또는 프로필 → 보안 → 개인 액세스 토큰)로 토큰을 만든 뒤, 터미널에서 아래 명령을 실행하고 토큰을 붙여 넣습니다 (화면에 보이지 않습니다).

Once: create a token for a system admin (or a bot with the system admin role) and save it in the macOS Keychain. In Mattermost: System Console → Integrations → Bot Accounts → bot → Create New Token (or Profile → Security → Personal Access Tokens), then run this in Terminal and paste the token (it stays hidden):

```sh
security add-generic-password -a "$USER" -s mattermost-send-later-deploy -w
```

`MM_TOKEN=… ./deploy.sh` also works (e.g. for automation). The token is passed to curl on standard input, so it never appears in the process list.

## Build it yourself · 직접 빌드

Needs Go 1.26.7+ and Node (only for a syntax check). The web part is plain JavaScript that uses the React Mattermost provides, so there is no npm build step.

```sh
./build.sh        # runs the tests, then writes dist/com.waveseoul.send-later-<version>.tar.gz
```

To try the web part in a browser without a Mattermost server, run the harness (a fake Mattermost serving `dev/harness.html` with the real plugin code behind it):

```sh
npm install --prefix /tmp/react react@18.2.0 react-dom@18.2.0
SL_HARNESS=1 SL_REACT_DIR=/tmp/react/node_modules go test ./server -run TestHarness -v
# open http://127.0.0.1:8765/?lang=ko  (or ?lang=en, &theme=dark)
python3 dev/ui_test.py ko --quit   # clicks through everything (needs Python Playwright)
```

## How it works

- **Server (Go):** messages are kept in the plugin's key-value store. A cluster-safe job checks every 10 seconds and posts due messages as their author, after checking they can still post there. Rules: up to 100 scheduled messages per person, up to one year ahead, at most 10 files.
- **Web app:** a button in the message box toolbar (`registerPostEditorActionComponent`), a right-hand sidebar list, and a channel-header button.
- **Mobile:** the `/later` slash command opens an interactive dialog with a date-time field; confirmations and `/later list` use message buttons, which work in the mobile app.

## Licence

MIT. See [LICENSE](LICENSE).
