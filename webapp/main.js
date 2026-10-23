// Send Later: web app part. Plain JavaScript using the React that Mattermost provides,
// so there is no build step. See README.md.
(function () {
    'use strict';

    const PLUGIN_ID = 'com.waveseoul.send-later';
    const React = window.React;
    const ReactDOM = window.ReactDOM;
    const h = React.createElement;
    const {useState, useEffect, useLayoutEffect, useRef} = React;

    // ---------------------------------------------------------------- text

    const STRINGS = {
        ko: {
            'button': '예약 전송',
            'menu.title': '메시지 예약',
            'menu.custom': '직접 선택…',
            'menu.list': '예약된 메시지',
            'menu.loading': '불러오는 중…',
            'menu.empty_draft': '보낼 메시지를 먼저 입력해 주세요.',
            'menu.uploading': '파일 업로드가 끝난 뒤에 예약해 주세요.',
            'menu.priority': '메시지 우선순위(중요·긴급)가 설정된 메시지는 예약할 수 없습니다. 우선순위를 해제한 뒤 예약해 주세요.',
            'menu.burn': '읽으면 사라지는 메시지는 예약할 수 없습니다.',
            'menu.command': '/로 시작하는 메시지는 명령어라서 예약할 수 없습니다. 글로 보내려면 맨 앞에 띄어쓰기를 넣어 주세요.',
            'menu.cant_clear': '이 Mattermost 버전에서는 첨부 파일이 있는 메시지를 예약할 수 없습니다. 파일 없이 예약하거나 지금 보내 주세요.',
            'custom.date': '날짜',
            'custom.time': '시간',
            'custom.tz': '시간대: {0}',
            'custom.back': '뒤로',
            'custom.submit': '예약',
            'list.title': '예약된 메시지',
            'list.empty': '예약된 메시지가 없습니다.',
            'list.empty_hint': '메시지를 입력한 뒤 입력창 아래의 시계 버튼(⏰)을 누르면 예약할 수 있습니다. 시계 버튼이 보이지 않으면 입력창의 서식 표시(Aa)를 켜 주세요. 휴대폰에서는 /later를 입력해 주세요.',
            'list.reply': '스레드 답글',
            'list.reply_to': '「{0}」에 단 답글',
            'list.failed': '보내지 못함',
            'list.send_now': '지금 보내기',
            'list.edit': '수정',
            'list.reschedule': '다시 예약',
            'list.copy': '복사',
            'list.copied': '복사했습니다',
            'list.delete': '삭제',
            'list.delete_question': '이 예약 메시지를 삭제하시겠습니까?',
            'list.cancel': '취소',
            'list.go': '채널로 이동',
            'tab.scheduled': '예약됨',
            'tab.sent': '보낸 메시지',
            'sent.empty': '최근 30일 동안 예약으로 보낸 메시지가 없습니다.',
            'sent.hint': '보낸 메시지는 일반 메시지와 같아서 채널에서 바로 수정하거나 삭제할 수도 있습니다.',
            'sent.when': '{0}에 보냄',
            'sent.go': '메시지로 이동',
            'sent.edited': '수정됨',
            'sent.files': '첨부 파일 {0}개',
            'sent.delete_question': '이 메시지를 채널에서 삭제하시겠습니까?',
            'edit.when': '보낼 시간',
            'edit.keep': '그대로 ({0})',
            'edit.custom': '직접 선택',
            'edit.pick_time': '새 시간을 골라 주세요.',
            'edit.save': '저장',
            'error.generic': '문제가 생겼습니다. 잠시 후 다시 시도해 주세요.',
        },
        en: {
            'button': 'Send later',
            'menu.title': 'Schedule message',
            'menu.custom': 'Custom time…',
            'menu.list': 'Scheduled messages',
            'menu.loading': 'Loading…',
            'menu.empty_draft': 'Type a message first.',
            'menu.uploading': 'Wait for the files to finish uploading.',
            'menu.priority': 'Messages with a priority (Important or Urgent) can\'t be scheduled. Remove the priority, then schedule it.',
            'menu.burn': 'Burn-on-read messages can\'t be scheduled.',
            'menu.command': 'Messages starting with / are commands and can\'t be scheduled. To send it as text, start it with a space.',
            'menu.cant_clear': 'In this Mattermost version, messages with attachments can\'t be scheduled. Schedule it without the files, or send it now.',
            'custom.date': 'Date',
            'custom.time': 'Time',
            'custom.tz': 'Time zone: {0}',
            'custom.back': 'Back',
            'custom.submit': 'Schedule',
            'list.title': 'Scheduled messages',
            'list.empty': 'No scheduled messages.',
            'list.empty_hint': 'Type a message, then press the clock button (⏰) under the message box to schedule it. If you don\'t see the clock button, turn on formatting (Aa) in the message box. On mobile, type /later.',
            'list.reply': 'Thread reply',
            'list.reply_to': 'Reply to “{0}”',
            'list.failed': 'Not sent',
            'list.send_now': 'Send now',
            'list.edit': 'Edit',
            'list.reschedule': 'Reschedule',
            'list.copy': 'Copy',
            'list.copied': 'Copied',
            'list.delete': 'Delete',
            'list.delete_question': 'Delete this scheduled message?',
            'list.cancel': 'Cancel',
            'list.go': 'Go to channel',
            'tab.scheduled': 'Scheduled',
            'tab.sent': 'Sent',
            'sent.empty': 'No scheduled messages were sent in the last 30 days.',
            'sent.hint': 'Sent messages are normal messages, so you can also edit or delete them in the channel.',
            'sent.when': 'Sent {0}',
            'sent.go': 'Go to message',
            'sent.edited': 'Edited',
            'sent.files': 'Attachments: {0}',
            'sent.delete_question': 'Delete this message from the channel?',
            'edit.when': 'When',
            'edit.keep': 'Keep ({0})',
            'edit.custom': 'Custom',
            'edit.pick_time': 'Pick a new time.',
            'edit.save': 'Save',
            'error.generic': 'Something went wrong. Try again in a moment.',
        },
    };

    let reduxStore = null;
    let rhsActions = null;

    function lang() {
        try {
            const state = reduxStore.getState();
            const users = state.entities.users;
            const me = users.profiles[users.currentUserId];
            const locale = (me && me.locale) || state.entities.general.config.DefaultClientLocale || 'en';
            return locale.indexOf('ko') === 0 ? 'ko' : 'en';
        } catch (e) {
            return 'en';
        }
    }

    function t(key, ...args) {
        const table = STRINGS[lang()];
        let text = table[key] || STRINGS.en[key] || key;
        args.forEach((arg, i) => {
            text = text.replace('{' + i + '}', () => String(arg)); // a function keeps "$" in user text as is
        });
        return text;
    }

    // ---------------------------------------------------------------- server

    function getCookie(name) {
        const match = document.cookie.match(new RegExp('(?:^|; )' + name + '=([^;]*)'));
        return match ? decodeURIComponent(match[1]) : '';
    }

    async function api(method, path, body) {
        const headers = {'X-Requested-With': 'XMLHttpRequest'};
        const csrf = getCookie('MMCSRF');
        if (csrf) {
            headers['X-CSRF-Token'] = csrf;
        }
        if (body) {
            headers['Content-Type'] = 'application/json';
        }
        const res = await fetch((window.basename || '') + '/plugins/' + PLUGIN_ID + '/api/v1' + path, {
            method,
            headers,
            credentials: 'same-origin',
            body: body ? JSON.stringify(body) : undefined,
        });
        let data = null;
        try {
            data = await res.json();
        } catch (e) {
            data = null;
        }
        if (!res.ok) {
            throw new Error((data && data.error) || t('error.generic'));
        }
        return data;
    }

    // One shared copy of the person's scheduled messages, refreshed when the server says it changed.
    const shared = {items: [], loaded: false, error: ''};
    const listeners = new Set();
    let refreshTimer = null;

    let refreshSeq = 0;

    // Answers can arrive out of order; only the newest request may update the list.
    async function refresh() {
        const seq = ++refreshSeq;
        let items = null;
        let error = '';
        try {
            const res = await api('GET', '/scheduled');
            items = res.items || [];
        } catch (e) {
            error = e.message;
        }
        if (seq !== refreshSeq) {
            return;
        }
        if (items) {
            shared.items = items;
        }
        shared.error = error;
        shared.loaded = true;
        listeners.forEach((fn) => fn());
    }

    // Messages already sent: fetched only once the 「보낸 메시지」 tab has been opened.
    const sent = {items: [], loaded: false, error: ''};
    let sentSeq = 0;

    async function refreshSent() {
        const seq = ++sentSeq;
        let items = null;
        let error = '';
        try {
            const res = await api('GET', '/sent');
            items = res.items || [];
        } catch (e) {
            error = e.message;
        }
        if (seq !== sentSeq) {
            return;
        }
        if (items) {
            sent.items = items;
        }
        sent.error = error;
        sent.loaded = true;
        listeners.forEach((fn) => fn());
    }

    let sentViewers = 0; // how many 「보낸 메시지」 tabs are open right now

    function refreshSoon() {
        clearTimeout(refreshTimer);
        refreshTimer = setTimeout(() => {
            refresh();
            if (sentViewers > 0) {
                refreshSent();
            }
        }, 150);
    }

    function useShared() {
        const [, setTick] = useState(0);
        useEffect(() => {
            const fn = () => setTick((n) => n + 1);
            listeners.add(fn);
            return () => listeners.delete(fn);
        }, []);
        return shared;
    }

    function usePresets() {
        const [state, setState] = useState({presets: null, meta: null, error: ''});
        useEffect(() => {
            let alive = true;
            api('GET', '/presets').then(
                (res) => alive && setState({presets: res.presets || [], meta: res, error: ''}),
                (e) => alive && setState({presets: [], meta: null, error: e.message}),
            );
            return () => {
                alive = false;
            };
        }, []);
        return state;
    }

    // ---------------------------------------------------------------- helpers

    function ClockIcon(props) {
        const size = props.size || 18;
        return h('svg', {width: size, height: size, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', strokeWidth: 2, strokeLinecap: 'round', strokeLinejoin: 'round', 'aria-hidden': 'true'},
            h('circle', {cx: 12, cy: 12, r: 9}),
            h('path', {d: 'M12 7v5l3.5 2'}),
        );
    }

    // The editor's updateText keeps the draft's other fields, so the attachments (and any
    // leftover options) are emptied on the same draft object first, then the text is cleared.
    function clearComposer(draft, updateText) {
        try {
            draft.fileInfos = [];
            draft.uploadsInProgress = [];
            if (draft.file_ids) {
                draft.file_ids = [];
            }
            draft.metadata = {};
            delete draft.type;
            draft.props = {};
        } catch (e) {
            // The draft is read-only in some versions; the text is still cleared below.
        }
        updateText('');
    }

    // The files can only be taken out of the message box by changing the draft object
    // (see clearComposer). If Mattermost ever hands over a read-only draft, refuse messages
    // with files rather than leave the files behind to be posted twice.
    function canClearFiles(draft) {
        if (Object.isFrozen(draft)) {
            return false;
        }
        const d = Object.getOwnPropertyDescriptor(draft, 'fileInfos');
        return !d || d.writable !== false || typeof d.set === 'function';
    }

    function hasPriority(draft) {
        const p = draft.metadata && draft.metadata.priority;
        return Boolean(p && (p.priority || p.requested_ack || p.persistent_notifications));
    }

    function focusComposer(draft) {
        const box = document.getElementById(draft.rootId ? 'reply_textbox' : 'post_textbox');
        if (box) {
            box.focus();
        }
    }

    function inPopout() {
        return window.location.pathname.indexOf('/_popout/') !== -1;
    }

    // The item's own team, or the current one (direct and group messages have none).
    function teamNameFor(item) {
        try {
            const state = reduxStore.getState();
            const team = state.entities.teams.teams[state.entities.teams.currentTeamId];
            return item.team_name || (team && team.name) || '';
        } catch (e) {
            return '';
        }
    }

    function copyText(text, setCopied) {
        if (navigator.clipboard) {
            navigator.clipboard.writeText(text).then(() => {
                setCopied(true);
                setTimeout(() => setCopied(false), 1500);
            }, () => null);
        }
    }

    function goTo(item) {
        try {
            const teamName = teamNameFor(item);
            if (!teamName) {
                return;
            }
            let path = '/' + teamName + '/channels/' + item.channel_name;
            if (item.root_id) {
                path = '/' + teamName + '/pl/' + item.root_id;
            } else if (item.channel_type === 'D' && item.dm_username) {
                path = '/' + teamName + '/messages/@' + item.dm_username;
            } else if (item.channel_type === 'G') {
                path = '/' + teamName + '/messages/' + item.channel_name;
            }
            window.WebappUtils.browserHistory.push(path);
        } catch (e) {
            // Navigation is a convenience; ignore failures.
        }
    }

    function openList() {
        if (reduxStore && rhsActions) {
            reduxStore.dispatch(rhsActions.showRHSPlugin);
        }
    }

    function DateTimeFields(props) {
        return h('div', {className: 'sl-custom'},
            h('label', {className: 'sl-field'},
                h('span', null, t('custom.date')),
                h('input', {type: 'date', value: props.date, min: props.min, onChange: (e) => props.setDate(e.target.value)}),
            ),
            h('label', {className: 'sl-field'},
                h('span', null, t('custom.time')),
                h('input', {type: 'time', value: props.time, step: 300, onChange: (e) => props.setTime(e.target.value)}),
            ),
            props.timezone ? h('div', {className: 'sl-hint'}, t('custom.tz', props.timezone)) : null,
        );
    }

    // ---------------------------------------------------------------- composer button and menu

    function Menu(props) {
        const {anchor, getLatest, onClose} = props;
        const draft = getLatest().draft;
        const {presets, meta, error: loadError} = usePresets();
        const [view, setView] = useState('main');
        const [date, setDate] = useState('');
        const [time, setTime] = useState('');
        const [busy, setBusy] = useState(false);
        const [error, setError] = useState('');
        const boxRef = useRef(null);
        const busyRef = useRef(false);
        const data = useShared();

        useEffect(() => {
            if (meta) {
                setDate(meta.custom_date);
                setTime(meta.custom_time);
            }
        }, [meta]);

        // Put the keyboard focus inside the menu once its choices are there.
        useEffect(() => {
            const box = boxRef.current;
            if (!box) {
                return;
            }
            const first = box.querySelector('button:not(:disabled), input');
            (first || box).focus();
        }, [presets, view]);

        useEffect(() => {
            const onDown = (e) => {
                if (busyRef.current) {
                    return; // keep the menu open until the request has finished, so errors can be shown
                }
                if (boxRef.current && !boxRef.current.contains(e.target) && !(anchor && anchor.contains(e.target))) {
                    onClose(false);
                }
            };
            const onKey = (e) => {
                if (e.key === 'Escape' && !busyRef.current) {
                    e.stopPropagation();
                    onClose(true);
                }
            };
            document.addEventListener('mousedown', onDown, true);
            document.addEventListener('keydown', onKey, true);
            return () => {
                document.removeEventListener('mousedown', onDown, true);
                document.removeEventListener('keydown', onKey, true);
            };
        }, [anchor, onClose]);

        const rect = anchor ? anchor.getBoundingClientRect() : {left: 16, top: window.innerHeight - 64};
        const width = 300;
        const bottom = Math.max(8, window.innerHeight - rect.top + 8);
        const style = {
            width,
            left: Math.max(8, Math.min(rect.left, window.innerWidth - width - 8)),
            bottom,
            // In a short window the list scrolls instead of running off the top of the screen.
            maxHeight: Math.max(160, window.innerHeight - bottom - 8),
            overflowY: 'auto',
        };

        const hasText = (draft.message || '').trim() !== '';
        const hasFiles = (draft.fileInfos || []).length > 0;
        let blocked = '';
        if ((draft.uploadsInProgress || []).length > 0) {
            blocked = t('menu.uploading');
        } else if (hasPriority(draft)) {
            blocked = t('menu.priority');
        } else if (draft.type === 'burn_on_read') {
            blocked = t('menu.burn');
        } else if (hasFiles && !canClearFiles(draft)) {
            blocked = t('menu.cant_clear');
        } else if (/^\/(?!\/)/.test(draft.message || '')) {
            blocked = t('menu.command');
        } else if (!hasText && !hasFiles) {
            blocked = t('menu.empty_draft');
        }

        async function schedule(when) {
            if (blocked || busyRef.current) {
                return;
            }
            const sent = getLatest().draft;
            busyRef.current = true;
            setBusy(true);
            setError('');
            try {
                await api('POST', '/scheduled', {
                    channel_id: sent.channelId,
                    root_id: sent.rootId || '',
                    message: sent.message || '',
                    file_ids: (sent.fileInfos || []).map((f) => f.id),
                    when,
                });
            } catch (e) {
                busyRef.current = false;
                setError(e.message);
                setBusy(false);
                return;
            }

            // Clear the box only if nothing was typed while the request was on its way.
            const latest = getLatest();
            if (latest.draft.channelId === sent.channelId && (latest.draft.rootId || '') === (sent.rootId || '') && latest.draft.message === sent.message) {
                clearComposer(latest.draft, latest.updateText);
            }
            busyRef.current = false;
            refreshSoon();
            onClose(false);
            setTimeout(() => focusComposer(sent), 0);
        }

        let body;
        if (view === 'custom') {
            body = h('div', {className: 'sl-menu-custom'},
                h(DateTimeFields, {date, time, setDate, setTime, min: meta && meta.today, timezone: meta && meta.timezone}),
                h('div', {className: 'sl-row'},
                    h('button', {type: 'button', className: 'sl-btn', disabled: busy, onClick: () => setView('main')}, t('custom.back')),
                    h('button', {type: 'button', className: 'sl-btn sl-primary', disabled: busy || Boolean(blocked) || !date || !time, onClick: () => schedule({date, time})}, t('custom.submit')),
                ),
            );
        } else {
            body = h(React.Fragment, null,
                presets === null ? h('div', {className: 'sl-menu-note'}, t('menu.loading')) : null,
                ...(presets || []).map((p) => h('button', {
                    key: p.key,
                    type: 'button',
                    className: 'sl-menu-item',
                    disabled: Boolean(blocked) || busy,
                    onClick: () => schedule({send_at: p.send_at}),
                }, h('span', null, p.label), h('span', {className: 'sl-menu-detail'}, p.detail))),
                h('button', {type: 'button', className: 'sl-menu-item', disabled: Boolean(blocked) || busy || !meta, onClick: () => setView('custom')}, t('menu.custom')),
                inPopout() ? null : h('div', {className: 'sl-menu-sep'}),
                inPopout() ? null : h('button', {type: 'button', className: 'sl-menu-item', disabled: busy, onClick: () => {
                    openList();
                    onClose(false);
                }}, h('span', null, t('menu.list')), data.items.length > 0 ? h('span', {className: 'sl-menu-detail'}, String(data.items.length)) : null),
            );
        }

        return h('div', {className: 'sl-menu', ref: boxRef, style, role: 'dialog', 'aria-label': t('menu.title'), tabIndex: -1},
            h('div', {className: 'sl-menu-title'}, t('menu.title')),
            blocked ? h('div', {className: 'sl-menu-note'}, blocked) : null,
            body,
            error || loadError ? h('div', {className: 'sl-error', role: 'alert'}, error || loadError) : null,
        );
    }

    function EditorButton(props) {
        const draft = props.draft || {};
        const data = useShared();
        const [open, setOpen] = useState(false);
        const [hidden, setHidden] = useState(false);
        const ref = useRef(null);

        // The menu reads the newest draft when it acts, not the one from when it opened.
        const latest = useRef({draft, updateText: props.updateText});
        latest.current = {draft, updateText: props.updateText};

        useLayoutEffect(() => {
            // The same toolbar appears when editing a sent message; scheduling makes no sense there.
            if (ref.current && ref.current.closest('.post--editing, .post')) {
                setHidden(true);
            }
        }, []);

        if (hidden || !draft.channelId) {
            return h('span', {ref, style: {display: 'none'}});
        }

        const here = data.items.filter((i) => i.status === 'pending' && i.channel_id === draft.channelId && (i.root_id || '') === (draft.rootId || '')).length;
        const close = (returnFocus) => {
            setOpen(false);
            if (returnFocus && ref.current) {
                ref.current.focus();
            }
        };

        return h(React.Fragment, null,
            h('button', {
                ref,
                type: 'button',
                className: 'sl-editor-btn' + (open ? ' sl-active' : ''),
                'aria-label': t('button'),
                'aria-haspopup': 'dialog',
                'aria-expanded': open,
                title: t('button'),
                onClick: () => setOpen(!open),
            }, h(ClockIcon, {size: 18}), here > 0 ? h('span', {className: 'sl-badge'}, String(here)) : null),
            open ? ReactDOM.createPortal(h(Menu, {
                anchor: ref.current,
                getLatest: () => latest.current,
                onClose: close,
            }), document.body) : null,
        );
    }

    // ---------------------------------------------------------------- list in the right-hand sidebar

    function EditItem(props) {
        const {item, onDone} = props;
        const failed = item.status === 'failed';
        const {presets, meta} = usePresets();
        const [text, setText] = useState(item.message);
        const [choice, setChoice] = useState(failed ? '' : 'keep');
        const [date, setDate] = useState('');
        const [time, setTime] = useState('');
        const [busy, setBusy] = useState(false);
        const [error, setError] = useState('');

        useEffect(() => {
            if (meta) {
                setDate(meta.custom_date);
                setTime(meta.custom_time);
            }
        }, [meta]);

        async function save() {
            const body = {};
            if (text !== item.message) {
                body.message = text;
            }
            const chosen = (presets || []).find((p) => p.key === choice);
            if (choice === 'custom') {
                body.when = {date, time};
            } else if (chosen) {
                body.when = {send_at: chosen.send_at}; // the time on the chip, even if saved after midnight
            }
            if (failed && !body.when) {
                setError(t('edit.pick_time'));
                return;
            }
            setBusy(true);
            setError('');
            try {
                if (Object.keys(body).length > 0) {
                    await api('PATCH', '/scheduled/' + item.id, body);
                }
                await refresh();
                onDone();
            } catch (e) {
                setError(e.message);
                setBusy(false);
            }
        }

        const chips = [];
        if (!failed) {
            chips.push({key: 'keep', label: t('edit.keep', item.when)});
        }
        (presets || []).forEach((p) => chips.push({key: p.key, label: p.label}));
        chips.push({key: 'custom', label: t('edit.custom')});

        return h('div', {className: 'sl-item sl-editing'},
            h('textarea', {
                className: 'sl-textarea',
                value: text,
                rows: Math.max(3, text.split('\n').length + 1),
                onChange: (e) => setText(e.target.value),
            }),
            h('div', {className: 'sl-label'}, t('edit.when')),
            h('div', {className: 'sl-chips'}, ...chips.map((c) => h('button', {
                key: c.key,
                type: 'button',
                className: 'sl-chip' + (choice === c.key ? ' sl-chip-on' : ''),
                'aria-pressed': choice === c.key,
                onClick: () => setChoice(c.key),
            }, c.label))),
            choice === 'custom' ? h(DateTimeFields, {date, time, setDate, setTime, min: meta && meta.today, timezone: meta && meta.timezone}) : null,
            error ? h('div', {className: 'sl-error'}, error) : null,
            h('div', {className: 'sl-actions'},
                h('button', {type: 'button', className: 'sl-btn sl-primary', disabled: busy, onClick: save}, t('edit.save')),
                h('button', {type: 'button', className: 'sl-btn', disabled: busy, onClick: onDone}, t('list.cancel')),
            ),
        );
    }

    function Item(props) {
        const {item} = props;
        const failed = item.status === 'failed';
        const [mode, setMode] = useState('view');
        const [busy, setBusy] = useState(false);
        const [error, setError] = useState('');
        const [copied, setCopied] = useState(false);

        async function run(fn) {
            setBusy(true);
            setError('');
            try {
                await fn();
                await refresh();
            } catch (e) {
                setError(e.message);
                setBusy(false);
            }
        }

        const copy = () => copyText(item.message, setCopied);

        if (mode === 'edit') {
            return h(EditItem, {item, onDone: () => setMode('view')});
        }

        let actions;
        if (mode === 'delete') {
            actions = h('div', {className: 'sl-confirm'},
                h('div', {className: 'sl-confirm-q'}, t('list.delete_question')),
                h('div', {className: 'sl-actions'},
                    h('button', {type: 'button', className: 'sl-btn sl-danger', disabled: busy, onClick: () => run(() => api('DELETE', '/scheduled/' + item.id))}, t('list.delete')),
                    h('button', {type: 'button', className: 'sl-btn', disabled: busy, onClick: () => setMode('view')}, t('list.cancel')),
                ),
            );
        } else {
            actions = h('div', {className: 'sl-actions'},
                h('button', {type: 'button', className: 'sl-btn', disabled: busy, onClick: () => run(() => api('POST', '/scheduled/' + item.id + '/send'))}, t('list.send_now')),
                h('button', {type: 'button', className: 'sl-btn', disabled: busy, onClick: () => setMode('edit')}, failed ? t('list.reschedule') : t('list.edit')),
                item.message ? h('button', {type: 'button', className: 'sl-btn', onClick: copy}, copied ? t('list.copied') : t('list.copy')) : null,
                h('button', {type: 'button', className: 'sl-btn sl-danger-text', disabled: busy, onClick: () => setMode('delete')}, t('list.delete')),
            );
        }

        return h('div', {className: 'sl-item' + (failed ? ' sl-failed' : '')},
            h('div', {className: 'sl-item-head'},
                h('button', {type: 'button', className: 'sl-link', title: t('list.go'), onClick: () => goTo(item)}, item.channel_label),
                item.root_id ? h('span', {className: 'sl-tag'}, item.root_preview ? t('list.reply_to', item.root_preview) : t('list.reply')) : null,
            ),
            failed ?
                h('div', {className: 'sl-when sl-error-text'}, t('list.failed') + ' · ' + item.error) :
                h('div', {className: 'sl-when'}, h(ClockIcon, {size: 14}), item.when),
            item.message ? h('div', {className: 'sl-msg'}, item.message) : null,
            item.file_names && item.file_names.length > 0 ? h('div', {className: 'sl-files'}, '📎 ' + item.file_names.join(', ')) : null,
            error ? h('div', {className: 'sl-error'}, error) : null,
            actions,
        );
    }

    function goToPost(item) {
        try {
            const teamName = teamNameFor(item);
            if (teamName) {
                window.WebappUtils.browserHistory.push('/' + teamName + '/pl/' + item.post_id);
            }
        } catch (e) {
            // Navigation is a convenience; ignore failures.
        }
    }

    function SentItem(props) {
        const {item} = props;
        const [mode, setMode] = useState('view');
        const [text, setText] = useState(item.message);
        const [busy, setBusy] = useState(false);
        const [error, setError] = useState('');
        const [copied, setCopied] = useState(false);

        async function run(fn, after) {
            setBusy(true);
            setError('');
            try {
                await fn();
                await refreshSent();
                if (after) {
                    after();
                }
                setBusy(false);
            } catch (e) {
                setError(e.message);
                setBusy(false);
            }
        }

        const copy = () => copyText(item.message, setCopied);

        let body;
        if (mode === 'edit') {
            body = h(React.Fragment, null,
                h('textarea', {
                    className: 'sl-textarea',
                    value: text,
                    rows: Math.max(3, text.split('\n').length + 1),
                    onChange: (e) => setText(e.target.value),
                }),
                h('div', {className: 'sl-actions'},
                    h('button', {type: 'button', className: 'sl-btn sl-primary', disabled: busy, onClick: () => run(() => api('PATCH', '/sent/' + item.post_id, {message: text}), () => setMode('view'))}, t('edit.save')),
                    h('button', {type: 'button', className: 'sl-btn', disabled: busy, onClick: () => {
                        setText(item.message);
                        setMode('view');
                    }}, t('list.cancel')),
                ),
            );
        } else {
            body = h(React.Fragment, null,
                item.message ? h('div', {className: 'sl-msg'}, item.message) : null,
                item.file_count ? h('div', {className: 'sl-files'}, '📎 ' + t('sent.files', item.file_count)) : null,
                mode === 'delete' ?
                    h('div', {className: 'sl-confirm'},
                        h('div', {className: 'sl-confirm-q'}, t('sent.delete_question')),
                        h('div', {className: 'sl-actions'},
                            h('button', {type: 'button', className: 'sl-btn sl-danger', disabled: busy, onClick: () => run(() => api('DELETE', '/sent/' + item.post_id))}, t('list.delete')),
                            h('button', {type: 'button', className: 'sl-btn', disabled: busy, onClick: () => setMode('view')}, t('list.cancel')),
                        ),
                    ) :
                    h('div', {className: 'sl-actions'},
                        h('button', {type: 'button', className: 'sl-btn', onClick: () => goToPost(item)}, t('sent.go')),
                        item.can_edit ? h('button', {type: 'button', className: 'sl-btn', disabled: busy, onClick: () => {
                            setText(item.message);
                            setMode('edit');
                        }}, t('list.edit')) : null,
                        item.message ? h('button', {type: 'button', className: 'sl-btn', onClick: copy}, copied ? t('list.copied') : t('list.copy')) : null,
                        h('button', {type: 'button', className: 'sl-btn sl-danger-text', disabled: busy, onClick: () => setMode('delete')}, t('list.delete')),
                    ),
            );
        }

        return h('div', {className: 'sl-item' + (mode === 'edit' ? ' sl-editing' : '')},
            h('div', {className: 'sl-item-head'},
                h('button', {type: 'button', className: 'sl-link', title: t('list.go'), onClick: () => goTo(item)}, item.channel_label),
                item.root_id ? h('span', {className: 'sl-tag'}, item.root_preview ? t('list.reply_to', item.root_preview) : t('list.reply')) : null,
                item.edited ? h('span', {className: 'sl-tag'}, t('sent.edited')) : null,
            ),
            h('div', {className: 'sl-when'}, h(ClockIcon, {size: 14}), t('sent.when', item.when)),
            body,
            error ? h('div', {className: 'sl-error', role: 'alert'}, error) : null,
        );
    }

    function SentList() {
        useShared();
        useEffect(() => {
            sentViewers++;
            refreshSent();
            return () => {
                sentViewers--;
            };
        }, []);

        if (!sent.loaded) {
            return h('div', {className: 'sl-note'}, t('menu.loading'));
        }
        if (sent.error && sent.items.length === 0) {
            return h('div', {className: 'sl-error', role: 'alert'}, sent.error);
        }
        if (sent.items.length === 0) {
            return h('div', {className: 'sl-empty'},
                h(ClockIcon, {size: 32}),
                h('div', {className: 'sl-empty-title'}, t('sent.empty')),
                h('div', {className: 'sl-note'}, t('sent.hint')),
            );
        }
        return h(React.Fragment, null,
            sent.error ? h('div', {className: 'sl-error', role: 'alert'}, sent.error) : null,
            h('div', {className: 'sl-note sl-list-hint'}, t('sent.hint')),
            ...sent.items.map((item) => h(SentItem, {key: item.post_id, item})),
        );
    }

    function ScheduledList() {
        const data = useShared();
        if (!data.loaded) {
            return h('div', {className: 'sl-note'}, t('menu.loading'));
        }
        if (data.error && data.items.length === 0) {
            return h('div', {className: 'sl-error', role: 'alert'}, data.error);
        }
        if (data.items.length === 0) {
            return h('div', {className: 'sl-empty'},
                h(ClockIcon, {size: 32}),
                h('div', {className: 'sl-empty-title'}, t('list.empty')),
                h('div', {className: 'sl-note'}, t('list.empty_hint')),
            );
        }
        return h(React.Fragment, null,
            data.error ? h('div', {className: 'sl-error', role: 'alert'}, data.error) : null,
            ...data.items.map((item) => h(Item, {key: item.id, item})),
        );
    }

    function ListPanel() {
        const data = useShared();
        const [tab, setTab] = useState('scheduled');
        useEffect(() => {
            refresh();
        }, []);

        const tabButton = (key, label) => h('button', {
            type: 'button',
            role: 'tab',
            'aria-selected': tab === key,
            className: 'sl-tab' + (tab === key ? ' sl-tab-on' : ''),
            onClick: () => setTab(key),
        }, label);

        return h('div', {className: 'sl-panel'},
            h('div', {className: 'sl-tabs', role: 'tablist'},
                tabButton('scheduled', t('tab.scheduled') + (data.items.length ? ' ' + data.items.length : '')),
                tabButton('sent', t('tab.sent')),
            ),
            tab === 'sent' ? h(SentList) : h(ScheduledList),
        );
    }

    function RhsTitle() {
        useShared();
        return h('span', null, t('button'));
    }

    function ListLabel() {
        return t('list.title');
    }

    // ---------------------------------------------------------------- styles

    const CSS = `
.sl-editor-btn{position:relative;display:inline-flex;align-items:center;justify-content:center;width:32px;height:32px;padding:0;border:none;border-radius:4px;background:transparent;color:rgba(var(--center-channel-color-rgb),0.56);cursor:pointer}
.sl-editor-btn:hover{background:rgba(var(--center-channel-color-rgb),0.08);color:rgba(var(--center-channel-color-rgb),0.72)}
.sl-editor-btn.sl-active{background:rgba(var(--button-bg-rgb),0.08);color:var(--button-bg)}
.sl-badge{position:absolute;top:2px;right:1px;min-width:14px;height:14px;padding:0 3px;border-radius:7px;background:var(--button-bg);color:var(--button-color);font-size:10px;font-weight:600;line-height:14px;text-align:center}
.sl-menu{position:fixed;z-index:1100;padding:8px 0;border:1px solid rgba(var(--center-channel-color-rgb),0.16);border-radius:4px;background:var(--center-channel-bg);color:var(--center-channel-color);box-shadow:0 8px 24px rgba(0,0,0,0.12);font-size:14px}
.sl-menu-title{padding:4px 16px 8px;color:rgba(var(--center-channel-color-rgb),0.64);font-size:12px;font-weight:600}
.sl-menu-note{padding:4px 16px 8px;color:rgba(var(--center-channel-color-rgb),0.64);font-size:12px}
.sl-menu-item{display:flex;align-items:center;justify-content:space-between;gap:12px;width:100%;min-height:36px;padding:6px 16px;border:none;background:transparent;color:inherit;text-align:left;cursor:pointer}
.sl-menu-item:hover:not(:disabled){background:rgba(var(--center-channel-color-rgb),0.08)}
.sl-menu-item:disabled{opacity:0.48;cursor:default}
.sl-menu-detail{color:rgba(var(--center-channel-color-rgb),0.64);font-size:12px;white-space:nowrap}
.sl-menu-sep{height:1px;margin:8px 0;background:rgba(var(--center-channel-color-rgb),0.16)}
.sl-menu-custom{padding:0 16px}
.sl-custom{display:flex;flex-wrap:wrap;gap:8px;margin:4px 0 8px}
.sl-field{display:flex;flex-direction:column;gap:4px;margin:0;font-size:12px;font-weight:600;color:rgba(var(--center-channel-color-rgb),0.72)}
.sl-field input{height:32px;padding:0 8px;border:1px solid rgba(var(--center-channel-color-rgb),0.24);border-radius:4px;background:var(--center-channel-bg);color:var(--center-channel-color);font-size:14px;font-weight:400}
.sl-hint{flex-basis:100%;color:rgba(var(--center-channel-color-rgb),0.64);font-size:12px}
.sl-row{display:flex;justify-content:flex-end;gap:8px;margin-bottom:4px}
.sl-btn{height:32px;padding:0 12px;border:1px solid rgba(var(--center-channel-color-rgb),0.16);border-radius:4px;background:var(--center-channel-bg);color:var(--center-channel-color);font-size:12px;font-weight:600;cursor:pointer}
.sl-btn:hover:not(:disabled){background:rgba(var(--center-channel-color-rgb),0.08)}
.sl-btn:disabled{opacity:0.48;cursor:default}
.sl-primary{border-color:var(--button-bg);background:var(--button-bg);color:var(--button-color)}
.sl-primary:hover:not(:disabled){background:var(--button-bg);opacity:0.9}
.sl-danger{border-color:var(--error-text);background:var(--error-text);color:#fff}
.sl-danger:hover:not(:disabled){background:var(--error-text);opacity:0.9}
.sl-danger-text{color:var(--error-text)}
.sl-error{margin:8px 16px 0;color:var(--error-text);font-size:12px}
.sl-panel .sl-error,.sl-item .sl-error{margin:8px 0 0}
.sl-panel{padding:12px 16px;color:var(--center-channel-color)}
.sl-empty{display:flex;flex-direction:column;align-items:center;gap:8px;padding:40px 16px;text-align:center;color:rgba(var(--center-channel-color-rgb),0.56)}
.sl-empty-title{color:var(--center-channel-color);font-size:16px;font-weight:600}
.sl-note{color:rgba(var(--center-channel-color-rgb),0.64);font-size:12px;line-height:18px}
.sl-item{padding:12px;margin-bottom:12px;border:1px solid rgba(var(--center-channel-color-rgb),0.16);border-radius:4px}
.sl-failed{border-color:var(--error-text)}
.sl-item-head{display:flex;align-items:center;gap:8px;margin-bottom:4px}
.sl-link{padding:0;border:none;background:none;color:var(--link-color);font-weight:600;cursor:pointer;text-align:left}
.sl-link:hover{text-decoration:underline}
.sl-tag{padding:0 6px;border-radius:4px;background:rgba(var(--center-channel-color-rgb),0.08);color:rgba(var(--center-channel-color-rgb),0.72);font-size:11px;line-height:18px}
.sl-when{display:flex;align-items:center;gap:4px;margin-bottom:8px;color:rgba(var(--center-channel-color-rgb),0.72);font-size:12px;font-weight:600}
.sl-error-text{color:var(--error-text)}
.sl-msg{white-space:pre-wrap;word-break:break-word;font-size:14px;line-height:20px}
.sl-files{margin-top:6px;color:rgba(var(--center-channel-color-rgb),0.72);font-size:12px}
.sl-actions{display:flex;flex-wrap:wrap;gap:8px;margin-top:12px}
.sl-tabs{display:flex;gap:4px;margin:-4px 0 12px;border-bottom:1px solid rgba(var(--center-channel-color-rgb),0.16)}
.sl-tab{padding:8px 12px;margin-bottom:-1px;border:1px solid transparent;border-radius:4px 4px 0 0;background:none;color:rgba(var(--center-channel-color-rgb),0.64);font-size:13px;font-weight:600;cursor:pointer}
.sl-tab:hover{color:var(--center-channel-color)}
.sl-tab-on{border-color:rgba(var(--center-channel-color-rgb),0.16);border-bottom-color:var(--center-channel-bg);background:var(--center-channel-bg);color:var(--center-channel-color)}
.sl-list-hint{margin-bottom:12px}
.sl-confirm{margin-top:12px}
.sl-confirm-q{font-size:13px;font-weight:600}
.sl-confirm .sl-actions{margin-top:8px}
.sl-menu:focus,.sl-menu-item:focus-visible,.sl-btn:focus-visible,.sl-chip:focus-visible,.sl-editor-btn:focus-visible{outline:2px solid rgba(var(--button-bg-rgb),0.5);outline-offset:-2px}
.sl-menu:focus{outline:none}
.sl-textarea{width:100%;padding:8px;border:1px solid rgba(var(--center-channel-color-rgb),0.24);border-radius:4px;background:var(--center-channel-bg);color:var(--center-channel-color);font-size:14px;line-height:20px;resize:vertical}
.sl-label{margin:8px 0 4px;font-size:12px;font-weight:600;color:rgba(var(--center-channel-color-rgb),0.72)}
.sl-chips{display:flex;flex-wrap:wrap;gap:6px;margin-bottom:8px}
.sl-chip{padding:4px 10px;border:1px solid rgba(var(--center-channel-color-rgb),0.16);border-radius:4px;background:var(--center-channel-bg);color:var(--center-channel-color);font-size:12px;cursor:pointer}
.sl-chip-on{border-color:var(--button-bg);background:rgba(var(--button-bg-rgb),0.08);color:var(--button-bg);font-weight:600}
`;

    // Always rewrite the styles: Mattermost loads a new plugin version without reloading the page.
    function addStyles() {
        let style = document.getElementById('sl-styles');
        if (!style) {
            style = document.createElement('style');
            style.id = 'sl-styles';
            document.head.appendChild(style);
        }
        style.textContent = CSS;
    }

    // ---------------------------------------------------------------- registration

    class Plugin {
        initialize(registry, store) {
            reduxStore = store;
            addStyles();

            // No pop-out window: the list's links and buttons act on the main window.
            rhsActions = registry.registerRightHandSidebarComponent({component: ListPanel, title: RhsTitle, showPopout: false});
            registry.registerPostEditorActionComponent(EditorButton);
            registry.registerChannelHeaderButtonAction({
                icon: h(ClockIcon, {size: 18}),
                action: () => store.dispatch(rhsActions.toggleRHSPlugin),
                dropdownText: ListLabel,
                tooltipText: ListLabel,
            });
            registry.registerWebSocketEventHandler('custom_' + PLUGIN_ID + '_changed', refreshSoon);
            registry.registerReconnectHandler(refreshSoon);

            refresh();
        }

        uninitialize() {
            const style = document.getElementById('sl-styles');
            if (style) {
                style.remove();
            }
        }
    }

    window.registerPlugin(PLUGIN_ID, new Plugin());
}());
