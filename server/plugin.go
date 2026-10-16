package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	// Embed the time zone database so user time zones work even on servers without tzdata.
	_ "time/tzdata"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

const (
	pluginID       = "com.waveseoul.send-later"
	botUsername    = "send-later"
	commandTrigger = "later"
	wsEventChanged = "changed"

	// How often the plugin looks for messages that are due.
	pollInterval = 10 * time.Second
)

// locker is satisfied by pluginLock in production and by sync.Mutex in tests.
type locker interface {
	Lock()
	Unlock()
}

// pluginLock takes a local mutex before the cluster mutex: requests on the same server queue
// quickly and never share one cluster lock, and other servers in a cluster still wait their turn.
type pluginLock struct {
	local   sync.Mutex
	cluster *cluster.Mutex
}

func (l *pluginLock) Lock() {
	l.local.Lock()
	l.cluster.Lock()
}

func (l *pluginLock) Unlock() {
	l.cluster.Unlock()
	l.local.Unlock()
}

type Plugin struct {
	plugin.MattermostPlugin

	configLock sync.RWMutex
	config     *configuration

	botID  string
	lock   locker
	job    *cluster.Job
	router http.Handler

	// lastReconcile is when the stored lists were last checked against the messages (ms);
	// needReconcile asks the next job run to check them now (after a failed write or an
	// unreadable value). Both are touched by requests without the lock, hence atomic.
	lastReconcile atomic.Int64
	needReconcile atomic.Bool

	// reported remembers unreadable keys already logged, so each is logged once.
	reported sync.Map

	// now is replaceable in tests.
	now func() time.Time
}

func (p *Plugin) OnActivate() error {
	if p.now == nil {
		p.now = time.Now
	}

	botID, err := p.API.EnsureBotUser(&model.Bot{
		Username:    botUsername,
		DisplayName: "Send Later",
		Description: "예약 메시지를 보냅니다 · Sends your scheduled messages",
	})
	if err != nil {
		return fmt.Errorf("failed to ensure bot: %w", err)
	}
	p.botID = botID
	p.setBotImage()

	mutex, err := cluster.NewMutex(p.API, "send_later_lock")
	if err != nil {
		return fmt.Errorf("failed to create mutex: %w", err)
	}
	p.lock = &pluginLock{cluster: mutex}

	if err := p.API.RegisterCommand(p.command()); err != nil {
		return fmt.Errorf("failed to register /%s: %w", commandTrigger, err)
	}

	p.router = p.newRouter()

	job, err := cluster.Schedule(p.API, "send_later_job", cluster.MakeWaitForInterval(pollInterval), p.runDue)
	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}
	p.job = job

	return nil
}

func (p *Plugin) OnDeactivate() error {
	if p.job != nil {
		return p.job.Close()
	}
	return nil
}

func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

func (p *Plugin) setBotImage() {
	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(bundlePath, "assets", "bot.png"))
	if err != nil {
		return
	}
	if appErr := p.API.SetProfileImage(p.botID, data); appErr != nil {
		p.API.LogWarn("Failed to set the bot's profile image", "error", appErr.Error())
	}
}

func (p *Plugin) publishChanged(userID string) {
	p.API.PublishWebSocketEvent(wsEventChanged, map[string]any{}, &model.WebsocketBroadcast{UserId: userID})
}
