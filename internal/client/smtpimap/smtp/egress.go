package smtp

import (
	"sync"
	"time"
)

// smtpsBlockedHosts is how many different servers must have left 465 silent
// while answering on 587 before the worker blames its own network.
const smtpsBlockedHosts = 2

// smtpsEvidenceTTL bounds how long one silent 465 counts as evidence.
const smtpsEvidenceTTL = 72 * time.Hour

// smtpsEgress is what this process has learned about reaching port 465,
// from its own dials only: a 465 that connects anywhere clears it.
var smtpsEgress = &egressState{}

type egressState struct {
	mu    sync.Mutex
	now   func() time.Time
	hosts map[string]time.Time
}

func (s *egressState) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// markSilent records a server whose 465 never answered while its 587 did.
func (s *egressState) markSilent(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hosts == nil {
		s.hosts = make(map[string]time.Time)
	}
	s.hosts[host] = s.clock()
}

// markOpen records a 465 that connected, which no blocked network allows.
func (s *egressState) markOpen() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hosts = nil
}

// Blocked reports whether outbound 465 looks blocked on this worker's network.
func (s *egressState) Blocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := s.clock().Add(-smtpsEvidenceTTL)
	for host, at := range s.hosts {
		if at.Before(cutoff) {
			delete(s.hosts, host)
		}
	}
	return len(s.hosts) >= smtpsBlockedHosts
}

// EgressCondition is the standing fleet warning while outbound 465 is
// blocked here, and empty otherwise.
func EgressCondition() string {
	if !smtpsEgress.Blocked() {
		return ""
	}
	return "Outbound port 465 is blocked by this machine's network: mail servers that accept submission only on 465 cannot be reached from it. Ask the hosting provider to open outbound 465."
}
