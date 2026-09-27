// Package releases resolves which version the fleet should be running.
//
// It does not update anything. In the pull model a node asks, on every
// heartbeat, what version it should be, compares that to what it is, and
// updates itself. This package's whole job is to keep the answer current: it
// reads GitHub Releases, picks the head of the configured channel, and writes
// the resolved tag into admin_settings where the heartbeat handler reads it.
//
// All configuration is env-driven so this is safe in self-hosted deployments.
// A self-hoster can point the poller at their own fork
// (RELEASES_GITHUB_REPO), point nodes at their own registry
// (RELEASES_WORKER_IMAGE_REPO), or disable the feature entirely
// (RELEASES_ENABLED=false), in which case nodes are told nothing and leave
// themselves alone.
package releases

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type Config struct {
	Enabled         bool   // RELEASES_ENABLED (default false)
	GithubRepo      string // "owner/repo", e.g. "warmbly/warmbly"
	WorkerImageRepo string // "ghcr.io/warmbly/warmbly/worker"
	GithubToken     string // optional, raises API rate limit
	HTTPClient      *http.Client
}

type Service struct {
	cfg      Config
	settings repository.FleetSettingsRepository
	http     *http.Client

	// In-memory view of the last check, surfaced to the dashboard. The
	// authoritative resolved tag lives in admin_settings, not here, so every
	// backend replica answers heartbeats identically.
	stateMu sync.Mutex
	state   State
}

type State struct {
	LastCheckedAt time.Time              `json:"last_checked_at"`
	LastError     string                 `json:"last_error,omitempty"`
	Channels      map[string]ChannelView `json:"channels"`
	GithubRepo    string                 `json:"github_repo"`
	ImageRepo     string                 `json:"image_repo"`
	Enabled       bool                   `json:"enabled"`
}

type ChannelView struct {
	Channel     string    `json:"channel"`
	Tag         string    `json:"tag"`
	Image       string    `json:"image"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	HTMLURL     string    `json:"html_url,omitempty"`
}

func New(cfg Config, settings repository.FleetSettingsRepository) *Service {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &Service{
		cfg:      cfg,
		settings: settings,
		http:     cfg.HTTPClient,
		state: State{
			Channels:   map[string]ChannelView{},
			GithubRepo: cfg.GithubRepo,
			ImageRepo:  cfg.WorkerImageRepo,
			Enabled:    cfg.Enabled,
		},
	}
}

// CheckGitHub resolves the stable and dev channel heads and, unless the fleet
// is pinned, records the head of the configured channel as the version every
// node should converge on.
//
// It never writes an empty tag: a GitHub outage or an empty release list
// leaves the previous answer in place, because telling the fleet "no version"
// would read as "stop updating" at best and be acted on at worst.
func (s *Service) CheckGitHub(ctx context.Context) (*models.FleetReleaseState, error) {
	if !s.cfg.Enabled {
		return nil, errors.New("releases not enabled")
	}

	releases, err := FetchReleases(ctx, s.http, s.cfg.GithubRepo, s.cfg.GithubToken)
	if err != nil {
		s.recordError(err.Error())
		return nil, err
	}
	stable, dev := PickChannelHeads(releases)

	s.stateMu.Lock()
	s.state.LastCheckedAt = time.Now()
	s.state.LastError = ""
	s.state.Channels = map[string]ChannelView{}
	if stable != nil {
		s.state.Channels[models.FleetChannelStable] = s.channelView(models.FleetChannelStable, stable)
	}
	if dev != nil {
		s.state.Channels[models.FleetChannelDev] = s.channelView(models.FleetChannelDev, dev)
	}
	s.stateMu.Unlock()

	current, err := s.settings.GetRelease(ctx)
	if err != nil {
		return nil, err
	}
	if current == nil {
		current = &models.FleetReleaseState{Channel: models.FleetChannelStable}
	}
	if current.Channel == models.FleetChannelPinned {
		// Explicitly held. Channel heads are still reported to the dashboard so
		// an operator can see what they are declining.
		return current, nil
	}

	var head *Release
	switch current.Channel {
	case models.FleetChannelDev:
		head = dev
	default:
		head = stable
	}
	if head == nil || head.TagName == "" {
		return current, nil
	}
	if head.TagName == current.Tag {
		return current, nil
	}

	next := &models.FleetReleaseState{
		Channel:    current.Channel,
		Tag:        head.TagName,
		ResolvedAt: time.Now(),
		Source:     "github:" + current.Channel,
	}
	if err := s.settings.SetRelease(ctx, next); err != nil {
		return nil, err
	}
	log.Printf("releases: fleet target is now %s (channel %s); nodes will self-update on their next heartbeat",
		next.Tag, next.Channel)
	return next, nil
}

// SetChannel switches which channel the fleet follows and immediately
// re-resolves, so the change takes effect on the next heartbeat rather than
// whenever the next release happens to land.
func (s *Service) SetChannel(ctx context.Context, channel string) (*models.FleetReleaseState, error) {
	switch channel {
	case models.FleetChannelStable, models.FleetChannelDev, models.FleetChannelPinned:
	default:
		return nil, errors.New("unknown channel: " + channel)
	}
	current, err := s.settings.GetRelease(ctx)
	if err != nil {
		return nil, err
	}
	if current == nil {
		current = &models.FleetReleaseState{}
	}
	current.Channel = channel
	if err := s.settings.SetRelease(ctx, current); err != nil {
		return nil, err
	}
	if channel == models.FleetChannelPinned {
		return current, nil
	}
	return s.CheckGitHub(ctx)
}

// SetTag pins the fleet to an explicit tag. Used to roll back: it switches the
// channel to pinned so the next GitHub check does not immediately undo it.
func (s *Service) SetTag(ctx context.Context, tag string) (*models.FleetReleaseState, error) {
	if strings.TrimSpace(tag) == "" {
		return nil, errors.New("tag required")
	}
	next := &models.FleetReleaseState{
		Channel:    models.FleetChannelPinned,
		Tag:        tag,
		ResolvedAt: time.Now(),
		Source:     "manual",
	}
	if err := s.settings.SetRelease(ctx, next); err != nil {
		return nil, err
	}
	return next, nil
}

func (s *Service) GetState() State {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	st := s.state
	chans := make(map[string]ChannelView, len(s.state.Channels))
	for k, v := range s.state.Channels {
		chans[k] = v
	}
	st.Channels = chans
	return st
}

// RunBootCheck syncs once on start so a fleet brought up after a release converges.
func (s *Service) RunBootCheck(ctx context.Context) {
	if !s.cfg.Enabled {
		log.Printf("releases: disabled; nodes will not be told to update")
		return
	}
	if s.cfg.GithubRepo == "" {
		log.Printf("releases: skipping boot check (RELEASES_GITHUB_REPO unset)")
		return
	}
	go func() {
		bctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := s.CheckGitHub(bctx); err != nil {
			log.Printf("releases: boot check failed: %v", err)
		}
	}()
}

func (s *Service) recordError(msg string) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.state.LastError = msg
	s.state.LastCheckedAt = time.Now()
}

func (s *Service) imageFor(tag string) string {
	repo := strings.TrimRight(s.cfg.WorkerImageRepo, "/")
	if repo == "" {
		return tag
	}
	return repo + ":" + tag
}

func (s *Service) channelView(name string, r *Release) ChannelView {
	return ChannelView{
		Channel:     name,
		Tag:         r.TagName,
		Image:       s.imageFor(r.TagName),
		PublishedAt: r.PublishedAt,
		HTMLURL:     r.HTMLURL,
	}
}
