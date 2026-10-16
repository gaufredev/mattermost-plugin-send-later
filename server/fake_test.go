package main

import (
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// fakeAPI implements the parts of plugin.API the plugin uses. Anything else panics,
// which tells us when the plugin starts depending on something new.
type fakeAPI struct {
	plugin.API

	mu         sync.Mutex
	kv         map[string][]byte
	users      map[string]*model.User
	prefs      map[string]string // userID → use_military_time
	channels   map[string]*model.Channel
	members    map[string]bool // channelID+"/"+userID
	readOnly   map[string]bool // channelID+"/"+userID without create_post
	posts      map[string]*model.Post
	files      map[string]*model.FileInfo
	teams      map[string]*model.Team
	created    []*model.Post
	ephemeral  []*model.Post
	updatedEph []*model.Post
	dialogs    []model.OpenDialogRequest
	events     []string
	failPost   bool
	postDown   bool            // CreatePost answers 500 (a passing server problem)
	channelErr map[string]bool // GetChannel answers 500 for these
	noUpload   map[string]bool // channelID+"/"+userID without upload_file
	teamsOf    map[string][]string
	restrictDM bool
	failSet    map[string]int // KVSet on this key fails this many times
	noEdit     bool           // the person lacks edit_post
	editLimit  int            // PostEditTimeLimit in seconds (-1 = none)
	deleted    []string
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{
		kv:         map[string][]byte{},
		users:      map[string]*model.User{},
		prefs:      map[string]string{},
		channels:   map[string]*model.Channel{},
		members:    map[string]bool{},
		readOnly:   map[string]bool{},
		posts:      map[string]*model.Post{},
		files:      map[string]*model.FileInfo{},
		teams:      map[string]*model.Team{},
		channelErr: map[string]bool{},
		noUpload:   map[string]bool{},
		teamsOf:    map[string][]string{},
		failSet:    map[string]int{},
		editLimit:  -1,
	}
}

func (f *fakeAPI) KVList(page, perPage int) ([]string, *model.AppError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.kv))
	for k := range f.kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	start := page * perPage
	if start >= len(keys) {
		return []string{}, nil
	}
	end := start + perPage
	if end > len(keys) {
		end = len(keys)
	}
	return keys[start:end], nil
}

func (f *fakeAPI) GetTeamsForUser(userID string) ([]*model.Team, *model.AppError) {
	var out []*model.Team
	for _, id := range f.teamsOf[userID] {
		out = append(out, &model.Team{Id: id})
	}
	return out, nil
}

func notFound(what string) *model.AppError {
	return model.NewAppError("fake", what+" not found", nil, "", http.StatusNotFound)
}

func (f *fakeAPI) KVGet(key string) ([]byte, *model.AppError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.kv[key]; ok {
		return append([]byte(nil), v...), nil
	}
	return nil, nil
}

func (f *fakeAPI) KVSet(key string, value []byte) *model.AppError {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSet[key] > 0 {
		f.failSet[key]--
		return model.NewAppError("fake", "kv write failed", nil, "", http.StatusInternalServerError)
	}
	f.kv[key] = append([]byte(nil), value...)
	return nil
}

func (f *fakeAPI) KVDelete(key string) *model.AppError {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.kv, key)
	return nil
}

func (f *fakeAPI) GetUser(id string) (*model.User, *model.AppError) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, notFound("user")
}

func (f *fakeAPI) GetConfig() *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	*cfg.ServiceSettings.SiteURL = "https://chat.example.com"
	*cfg.ServiceSettings.PostEditTimeLimit = f.editLimit
	if f.restrictDM {
		*cfg.TeamSettings.RestrictDirectMessage = model.DirectMessageTeam
	}
	return cfg
}

func (f *fakeAPI) GetPreferenceForUser(userID, category, name string) (model.Preference, *model.AppError) {
	if v, ok := f.prefs[userID]; ok {
		return model.Preference{UserId: userID, Category: category, Name: name, Value: v}, nil
	}
	return model.Preference{}, notFound("preference")
}

func (f *fakeAPI) GetChannel(id string) (*model.Channel, *model.AppError) {
	if f.channelErr[id] {
		return nil, model.NewAppError("fake", "database timeout", nil, "", http.StatusInternalServerError)
	}
	if c, ok := f.channels[id]; ok {
		return c, nil
	}
	return nil, notFound("channel")
}

func (f *fakeAPI) GetChannelMember(channelID, userID string) (*model.ChannelMember, *model.AppError) {
	if f.members[channelID+"/"+userID] {
		return &model.ChannelMember{ChannelId: channelID, UserId: userID}, nil
	}
	return nil, notFound("member")
}

func (f *fakeAPI) HasPermissionToChannel(userID, channelID string, perm *model.Permission) bool {
	key := channelID + "/" + userID
	if perm.Id == model.PermissionUploadFile.Id && f.noUpload[key] {
		return false
	}
	if perm.Id == model.PermissionEditPost.Id && f.noEdit {
		return false
	}
	return f.members[key] && !f.readOnly[key]
}

func (f *fakeAPI) GetPost(id string) (*model.Post, *model.AppError) {
	if p, ok := f.posts[id]; ok {
		return p, nil
	}
	return nil, notFound("post")
}

func (f *fakeAPI) GetFileInfo(id string) (*model.FileInfo, *model.AppError) {
	if fi, ok := f.files[id]; ok {
		return fi, nil
	}
	return nil, notFound("file")
}

func (f *fakeAPI) GetTeam(id string) (*model.Team, *model.AppError) {
	if t, ok := f.teams[id]; ok {
		return t, nil
	}
	return nil, notFound("team")
}

func (f *fakeAPI) CreatePost(post *model.Post) (*model.Post, *model.AppError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failPost && post.UserId != "bot" {
		return nil, model.NewAppError("fake", "rejected", nil, "", http.StatusBadRequest)
	}
	if f.postDown && post.UserId != "bot" {
		return nil, model.NewAppError("fake", "database down", nil, "", http.StatusInternalServerError)
	}
	post.Id = model.NewId()
	post.CreateAt = model.GetMillis()
	f.created = append(f.created, post)
	f.posts[post.Id] = post.Clone()
	for _, id := range post.FileIds {
		if fi, ok := f.files[id]; ok {
			fi.PostId = post.Id
		}
	}
	return post, nil
}

func (f *fakeAPI) UpdatePost(post *model.Post) (*model.Post, *model.AppError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.posts[post.Id]; !ok {
		return nil, notFound("post")
	}
	post.EditAt = model.GetMillis()
	f.posts[post.Id] = post.Clone()
	return post, nil
}

func (f *fakeAPI) DeletePost(id string) *model.AppError {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.posts[id]; !ok {
		return notFound("post")
	}
	delete(f.posts, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeAPI) GetDirectChannel(a, b string) (*model.Channel, *model.AppError) {
	return &model.Channel{Id: "dm_" + a + "_" + b, Type: model.ChannelTypeDirect, Name: a + "__" + b}, nil
}

func (f *fakeAPI) SendEphemeralPost(_ string, post *model.Post) *model.Post {
	for _, a := range post.Attachments() {
		if err := a.IsValid(); err != nil {
			panic("invalid attachment: " + err.Error())
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if post.Id == "" { // like Mattermost: an ID chosen by the plugin is kept
		post.Id = model.NewId()
	}
	f.ephemeral = append(f.ephemeral, post)
	return post
}

func (f *fakeAPI) UpdateEphemeralPost(_ string, post *model.Post) *model.Post {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updatedEph = append(f.updatedEph, post)
	return post
}

func (f *fakeAPI) OpenInteractiveDialog(req model.OpenDialogRequest) *model.AppError {
	// Mattermost only logs invalid dialogs, but labels over its limits look broken, so the fake
	// refuses them. The message box is the exception: clients count its limit in characters,
	// so check a copy with the default text cut to the byte limit.
	check := req.Dialog
	check.Elements = append([]model.DialogElement(nil), req.Dialog.Elements...)
	for i := range check.Elements {
		if check.Elements[i].Type == "textarea" && utf8.RuneCountInString(check.Elements[i].Default) <= dialogTextMax {
			check.Elements[i].Default = ""
		}
	}
	if err := check.IsValid(); err != nil {
		return model.NewAppError("fake", "invalid dialog: "+err.Error(), nil, "", http.StatusBadRequest)
	}
	f.dialogs = append(f.dialogs, req)
	return nil
}

func (f *fakeAPI) PublishWebSocketEvent(event string, _ map[string]any, b *model.WebsocketBroadcast) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event+":"+b.UserId)
}

func (f *fakeAPI) LogError(string, ...any) {}
func (f *fakeAPI) LogWarn(string, ...any)  {}
func (f *fakeAPI) LogDebug(string, ...any) {}

// fixture: a Korean user in Seoul, an English user in Paris, one channel both are in.
type fixture struct {
	t     *testing.T
	api   *fakeAPI
	p     *Plugin
	clock time.Time
}

const (
	userKo  = "userko"
	userEn  = "useren"
	chanID  = "chan1"
	teamID  = "team1"
	rootID  = "root1"
	replyID = "reply1"
)

func newFixture(t *testing.T, start time.Time) *fixture {
	t.Helper()
	api := newFakeAPI()
	api.users[userKo] = &model.User{Id: userKo, Username: "minji", Locale: "ko", Timezone: model.StringMap{"useAutomaticTimezone": "true", "automaticTimezone": "Asia/Seoul"}}
	api.users[userEn] = &model.User{Id: userEn, Username: "alex", Locale: "en", Timezone: model.StringMap{"useAutomaticTimezone": "false", "manualTimezone": "Europe/Paris"}}
	api.users["bot"] = &model.User{Id: "bot", Username: botUsername, IsBot: true}
	api.teams[teamID] = &model.Team{Id: teamID, Name: "wave"}
	api.channels[chanID] = &model.Channel{Id: chanID, TeamId: teamID, Type: model.ChannelTypeOpen, Name: "town-square", DisplayName: "Town Square"}
	api.members[chanID+"/"+userKo] = true
	api.members[chanID+"/"+userEn] = true
	api.posts[rootID] = &model.Post{Id: rootID, ChannelId: chanID}
	api.posts[replyID] = &model.Post{Id: replyID, ChannelId: chanID, RootId: rootID}

	f := &fixture{t: t, api: api, clock: start}
	p := &Plugin{botID: "bot", lock: &sync.Mutex{}, now: func() time.Time { return f.clock }}
	p.SetAPI(api)
	p.config = &configuration{TodayTime: "17:00", TomorrowTime: "10:00", MondayTime: "10:00"}
	p.router = p.newRouter()
	f.p = p
	return f
}

func (f *fixture) advance(d time.Duration) { f.clock = f.clock.Add(d) }

func seoul(t *testing.T, value string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		t.Fatal(err)
	}
	tm, err := time.ParseInLocation("2006-01-02 15:04", value, loc)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}
