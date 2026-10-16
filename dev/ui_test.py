"""Clicks through the Send Later web part on the local harness (see README → Build it yourself).

    SL_HARNESS=1 SL_LANG=ko SL_REACT_DIR=... go test ./server -run TestHarness   # in one terminal
    python3 dev/ui_test.py ko --quit                                               # in another

Needs Python Playwright with Chromium. Screenshots go to dev/shots/ (not committed).
"""
import json
import os
import sys
import time
import urllib.request

from playwright.sync_api import sync_playwright

BASE = "http://127.0.0.1:8765"
OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "shots")
os.makedirs(OUT, exist_ok=True)
LANG = sys.argv[1] if len(sys.argv) > 1 and not sys.argv[1].startswith("-") else "ko"
QUIT = "--quit" in sys.argv

KO = LANG == "ko"
T = {
    "empty": "보낼 메시지를 먼저 입력해 주세요." if KO else "Type a message first.",
    "tomorrow": "내일" if KO else "Tomorrow",
    "custom": "직접 선택…" if KO else "Custom time…",
    "edit": "수정" if KO else "Edit",
    "delete": "삭제" if KO else "Delete",
    "question": "삭제하시겠습니까?" if KO else "Delete this scheduled message?",
    "send_now": "지금 보내기" if KO else "Send now",
    "confirm_text": "보내도록 예약했습니다" if KO else "Scheduled for",
    "past": "지난 시간" if KO else "already passed",
    "tab_sent": "보낸 메시지" if KO else "Sent",
    "save": "저장" if KO else "Save",
    "edited": "수정됨" if KO else "Edited",
    "sent_question": "채널에서 삭제하시겠습니까?" if KO else "Delete this message from the channel?",
    "sent_empty": "보낸 메시지가 없습니다" if KO else "No scheduled messages were sent",
    "priority": "우선순위" if KO else "priority",
    "cant_clear": "첨부 파일이 있는 메시지를 예약할 수 없습니다" if KO else "messages with attachments can't be scheduled",
    "command": "명령어라서 예약할 수 없습니다" if KO else "are commands and can't be scheduled",
}


def state():
    with urllib.request.urlopen(BASE + "/__state") as r:
        data = json.load(r)
    return {k: v or [] for k, v in data.items()}


def wait_server():
    for _ in range(90):
        try:
            state()
            return
        except Exception:
            time.sleep(1)
    sys.exit("harness not reachable")


failed = 0


def check(cond, msg):
    global failed
    print(("PASS " if cond else "FAIL ") + msg)
    if not cond:
        failed += 1


def open_menu(page, wait_for=None):
    page.click(".sl-editor-btn")
    page.wait_for_selector(".sl-menu .sl-menu-sep")
    # Quick times arrive a moment later and push the other items down; wait for them.
    page.wait_for_selector(wait_for or ('.sl-menu-item:has-text("%s")' % T["tomorrow"]))


wait_server()
with sync_playwright() as p:
    browser = p.chromium.launch()
    page = browser.new_page(viewport={"width": 1360, "height": 800})
    errors = []
    page.on("console", lambda m: errors.append(m.text) if m.type == "error" else None)
    page.on("pageerror", lambda e: errors.append(str(e)))
    page.goto(BASE + "/?lang=" + LANG)
    page.wait_for_selector(".sl-editor-btn")

    # 1. Empty message: a hint, and the quick times are disabled.
    open_menu(page)
    check(T["empty"] in page.inner_text(".sl-menu"), "empty draft shows the hint")
    disabled = page.eval_on_selector_all(".sl-menu-item", "els => els.slice(0, -1).map(e => e.disabled)")
    check(all(disabled), "quick times disabled when empty: %s" % disabled)
    page.screenshot(path=os.path.join(OUT, LANG + "-01-empty.png"))
    page.keyboard.press("Escape")
    check(page.query_selector(".sl-menu") is None, "Escape closes the menu")

    # 2. Refusals: a slash command; priority set; read-only draft with files.
    page.fill("#post-box", "/away")
    open_menu(page)
    check(T["command"] in page.inner_text(".sl-menu"), "slash command refused")
    page.keyboard.press("Escape")
    page.fill("#post-box", "x")
    page.evaluate("window.__setDraft(d => ({...d, metadata: {priority: {priority: 'urgent'}}}))")
    open_menu(page)
    check(T["priority"] in page.inner_text(".sl-menu"), "message with a priority refused")
    page.keyboard.press("Escape")
    page.evaluate("window.__setDraft(d => Object.freeze({...d, metadata: {}, fileInfos: [{id: 'x', name: 'a.pdf'}]}))")
    open_menu(page)
    check(T["cant_clear"] in page.inner_text(".sl-menu"), "read-only draft with files refused")
    page.keyboard.press("Escape")
    page.evaluate("window.__setDraft(d => ({...d, message: '', fileInfos: [], metadata: {}}))")

    # 3. Message + file, then "tomorrow" (by keyboard: focus starts on the first choice).
    page.fill("#post-box", "내일 아침 회의 자료 공유드립니다" if KO else "Sharing the slides for tomorrow")
    page.evaluate("window.__addFile()")
    page.wait_for_selector(".file")
    open_menu(page)
    page.wait_for_timeout(150)
    check(page.evaluate("document.activeElement.closest('.sl-menu') !== null"), "focus moves into the menu")
    page.screenshot(path=os.path.join(OUT, LANG + "-02-menu.png"))
    page.click('.sl-menu-item:has-text("%s")' % T["tomorrow"])
    page.wait_for_selector(".sl-menu", state="detached")
    check(page.input_value("#post-box") == "", "message box cleared")
    check(page.query_selector(".file") is None, "attachment cleared")
    page.wait_for_selector(".sl-badge")
    check(page.inner_text(".sl-badge") == "1", "badge shows 1")
    eph = state()["ephemeral"]
    check(len(eph) == 1 and T["confirm_text"] in eph[0]["message"], "confirmation posted: %s" % (eph[-1]["message"] if eph else None))

    # 4. Custom date and time.
    page.fill("#post-box", "직접 고른 시간" if KO else "Custom time message")
    open_menu(page)
    page.click('.sl-menu-item:has-text("%s")' % T["custom"])
    page.wait_for_selector('.sl-menu input[type="date"]')
    page.fill('.sl-menu input[type="time"]', "14:30")
    page.screenshot(path=os.path.join(OUT, LANG + "-03-custom.png"))
    page.click(".sl-menu .sl-primary")
    page.wait_for_selector(".sl-menu", state="detached")
    page.wait_for_function("() => document.querySelector('.sl-badge') && document.querySelector('.sl-badge').innerText === '2'")
    check(True, "custom time scheduled, badge 2")

    # A past time is refused with a readable error.
    page.fill("#post-box", "past")
    open_menu(page)
    page.click('.sl-menu-item:has-text("%s")' % T["custom"])
    page.fill('.sl-menu input[type="date"]', "2020-01-01")
    page.click(".sl-menu .sl-primary")
    page.wait_for_selector(".sl-menu .sl-error")
    check(T["past"] in page.inner_text(".sl-menu .sl-error"), "past time refused: " + page.inner_text(".sl-menu .sl-error"))
    page.keyboard.press("Escape")
    page.fill("#post-box", "")

    # 5. The list in the right-hand sidebar.
    open_menu(page)
    page.locator(".sl-menu .sl-menu-item").last.click()
    page.wait_for_selector(".sl-item")
    check(len(page.query_selector_all(".sl-item")) == 2, "list shows 2 items")
    check("📎" in page.inner_text(".sl-panel"), "file name shown")
    page.screenshot(path=os.path.join(OUT, LANG + "-04-list.png"))

    # 6. Edit the custom one: new text, new time.
    item = page.locator(".sl-item", has_text="직접 고른 시간" if KO else "Custom time message")
    item.locator("button", has_text=T["edit"]).click()
    page.wait_for_selector(".sl-editing .sl-chip")
    page.fill(".sl-editing textarea", "수정한 내용" if KO else "Edited text")
    page.locator(".sl-editing .sl-chip").nth(1).click()  # first quick time after "keep"
    page.screenshot(path=os.path.join(OUT, LANG + "-05-edit.png"))
    page.click(".sl-editing .sl-primary")
    page.wait_for_selector(".sl-editing", state="detached")
    check(("수정한 내용" if KO else "Edited text") in page.inner_text(".sl-panel"), "edit saved")

    # 7. Delete it: it asks first.
    item = page.locator(".sl-item", has_text="수정한 내용" if KO else "Edited text")
    item.locator("button", has_text=T["delete"]).click()
    check(T["question"] in item.inner_text(), "delete asks a question")
    item.locator(".sl-confirm .sl-danger").click()
    page.wait_for_function("() => document.querySelectorAll('.sl-item').length === 1")
    check(True, "deleted")

    # 8. Send the other one now.
    before = len(state()["created"])
    page.locator(".sl-item").first.locator("button", has_text=T["send_now"]).click()
    page.wait_for_selector(".sl-empty")
    created = state()["created"]
    check(len(created) == before + 1 and created[-1]["user_id"] == "userko" and len(created[-1].get("file_ids") or []) == 1,
          "sent now as the person, with the file")
    check(page.query_selector(".sl-badge") is None, "badge gone")
    page.screenshot(path=os.path.join(OUT, LANG + "-06-empty-list.png"))

    # 8b. The sent tab: the message is there; edit it, then delete it from the channel.
    page.click('.sl-tab:has-text("%s")' % T["tab_sent"])
    page.wait_for_selector(".sl-item")
    sent_text = "내일 아침 회의 자료 공유드립니다" if KO else "Sharing the slides for tomorrow"
    check(sent_text in page.inner_text(".sl-panel"), "sent tab shows the sent message")
    page.screenshot(path=os.path.join(OUT, LANG + "-06b-sent.png"))
    page.locator(".sl-item button", has_text=T["edit"]).click()
    page.fill(".sl-item textarea", "수정한 보낸 메시지" if KO else "Edited sent message")
    page.click('.sl-item .sl-primary')
    page.wait_for_selector('.sl-tag:has-text("%s")' % T["edited"])
    check(("수정한 보낸 메시지" if KO else "Edited sent message") in page.inner_text(".sl-panel"), "sent message edited")
    page.locator(".sl-item button", has_text=T["delete"]).click()
    check(T["sent_question"] in page.inner_text(".sl-item"), "deleting a sent message asks first")
    page.locator(".sl-item .sl-confirm .sl-danger").click()
    page.wait_for_selector(".sl-empty")
    check(T["sent_empty"] in page.inner_text(".sl-panel"), "sent message deleted")
    page.click('.sl-tab >> nth=0')

    # 9. The channel header button toggles the list.
    page.click(".hdr-btn")
    check(not page.is_visible(".rhs.open"), "header button closes the list")
    page.click(".hdr-btn")
    check(page.is_visible(".rhs.open"), "header button opens the list")

    # 10. Phone width and dark theme.
    page.set_viewport_size({"width": 390, "height": 760})
    page.click(".rhs-close")
    page.fill("#post-box", "폰에서 예약" if KO else "From a phone")
    open_menu(page)
    box = page.eval_on_selector(".sl-menu", "e => { const r = e.getBoundingClientRect(); return [r.left, r.right, window.innerWidth]; }")
    check(box[0] >= 0 and box[1] <= box[2], "menu fits the phone screen: %s" % box)
    page.screenshot(path=os.path.join(OUT, LANG + "-07-phone-menu.png"))
    page.click('.sl-menu-item:has-text("%s")' % T["tomorrow"])
    page.wait_for_selector(".sl-badge")

    page2 = browser.new_page(viewport={"width": 1360, "height": 800})
    page2.goto(BASE + "/?lang=" + LANG + "&theme=dark")
    page2.wait_for_selector(".sl-badge")
    page2.fill("#post-box", "dark")
    open_menu(page2)
    page2.screenshot(path=os.path.join(OUT, LANG + "-08-dark-menu.png"))
    page2.keyboard.press("Escape")
    page2.click(".hdr-btn")
    page2.wait_for_selector(".sl-item")
    page2.screenshot(path=os.path.join(OUT, LANG + "-09-dark-list.png"))
    page2.locator(".sl-item").first.locator("button", has_text=T["delete"]).click()
    page2.locator(".sl-item").first.locator(".sl-confirm .sl-danger").click()
    page2.wait_for_selector(".sl-empty")

    errors = [e for e in errors if "400 (Bad Request)" not in e]  # the deliberate past-time test
    check(not errors, "no console errors: %s" % errors[:5])
    browser.close()

if QUIT:
    urllib.request.urlopen(urllib.request.Request(BASE + "/__quit", method="POST"))
print("FAILED: %d" % failed)
sys.exit(1 if failed else 0)
