package tank

import (
    "errors"
    "sync"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

type SectionState struct {
    Name string `json:"name"`
    DepthMeters float64 `json:"depth_meters"`
    DepthRevision model.RevisionID `json:"depth_revision"`
    CalmSince time.Time `json:"calm_since"`
    LastSample time.Time `json:"last_sample"`
    WaveHeight float64 `json:"wave_height"`
    Envelope WaveEnvelope `json:"envelope"`
}

type Sections struct {
    mu sync.Mutex
    values map[string]SectionState
}

func NewSections(names []string, depth float64) *Sections {
    values := make(map[string]SectionState, len(names))
    for _, name := range names {
        values[name] = SectionState{Name: name, DepthMeters: depth, DepthRevision: model.NewRevisionID(), Envelope: DefaultEnvelope(depth, time.Now())}
    }
    return &Sections{values: values}
}

func (s *Sections) UpdateDepth(name string, depth float64) SectionState {
    s.mu.Lock()
    defer s.mu.Unlock()
    state := s.values[name]
    state.Name = name
    state.DepthMeters = depth
    state.DepthRevision = model.NewRevisionID()
    state.CalmSince = time.Time{}
    state.LastSample = time.Time{}
    state.Envelope = DefaultEnvelope(depth, time.Now())
    s.values[name] = state
    return state
}

// SetEnvelope replaces the section envelope with explicit limits, bumping
// the envelope revision so the change is verifiable against abort records.
func (s *Sections) SetEnvelope(name string, maxHeight float64, maxRate float64, basis string, at time.Time) (SectionState, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if maxHeight <= 0 {
        return SectionState{}, errors.New("max height must be positive")
    }
    if maxRate <= 0 {
        return SectionState{}, errors.New("max rate must be positive")
    }
    state, ok := s.values[name]
    if !ok {
        return SectionState{}, errors.New("unknown section " + name)
    }
    state.Envelope = WaveEnvelope{MaxHeight: maxHeight, MaxRate: maxRate, Revision: model.NewRevisionID(), Basis: basis, UpdatedAt: at.UTC()}
    s.values[name] = state
    return state, nil
}

// WaveEnvelope returns the envelope currently in force for the section.
// Command issuers consult it before every command so mid-run condition
// changes take effect on the very next command.
func (s *Sections) WaveEnvelope(name string) (WaveEnvelope, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    state, ok := s.values[name]
    if !ok {
        return WaveEnvelope{}, false
    }
    return state.Envelope, true
}

func (s *Sections) RecordWave(name string, height float64, at time.Time) SectionState {
    s.mu.Lock()
    defer s.mu.Unlock()
    state := s.values[name]
    state.WaveHeight = height
    state.LastSample = at.UTC()
    if height <= 0.02 && state.CalmSince.IsZero() {
        state.CalmSince = at.UTC()
    }
    if height > 0.02 {
        state.CalmSince = time.Time{}
    }
    s.values[name] = state
    return state
}

func (s *Sections) Get(name string) (SectionState, bool) {
    s.mu.Lock()
    defer s.mu.Unlock()
    state, ok := s.values[name]
    return state, ok
}

func (s *Sections) Snapshot() []SectionState {
    s.mu.Lock()
    defer s.mu.Unlock()
    result := make([]SectionState, 0, len(s.values))
    for _, state := range s.values {
        result = append(result, state)
    }
    return result
}
