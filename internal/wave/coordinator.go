package wave

import (
    "context"
    "sync"
    "time"

    "github.com/wyw14/solo-037/internal/model"
    "github.com/wyw14/solo-037/internal/tank"
)

// EnvelopeSource supplies the wave envelope currently in force for a
// section. *tank.Sections satisfies it.
type EnvelopeSource interface {
    WaveEnvelope(section string) (tank.WaveEnvelope, bool)
}

// AbortRecord captures a stopped plan: the command that was about to be
// issued when the check ran, the envelope it was checked against, and the
// reason the command was withheld.
type AbortRecord struct {
    Command model.DeviceCommand `json:"command"`
    Envelope tank.WaveEnvelope `json:"envelope"`
    Reason string `json:"reason"`
    AbortedAt time.Time `json:"aborted_at"`
}

type Controller struct {
    mu sync.Mutex
    run model.RunID
    section string
    envelopes EnvelopeSource
    commands []model.DeviceCommand
    aborts []AbortRecord
    active bool
    done chan struct{}
    lastSetpoint float64
    hasLast bool
    lastIssuedAt time.Time
}

func NewController() *Controller { return &Controller{done: make(chan struct{})} }

// Start begins issuing the profile for the section. When envelopes is
// non-nil, the current section envelope is checked before every command;
// the first command that falls outside the envelope stops the plan and is
// preserved as an AbortRecord.
func (c *Controller) Start(ctx context.Context, run model.RunID, section string, envelopes EnvelopeSource, profile Profile, epoch uint64, at time.Time) {
    c.mu.Lock()
    c.run = run
    c.section = section
    c.envelopes = envelopes
    c.active = true
    c.done = make(chan struct{})
    c.hasLast = false
    c.mu.Unlock()
    go c.loop(ctx, profile, epoch, at)
}

func (c *Controller) loop(ctx context.Context, profile Profile, epoch uint64, at time.Time) {
    ticker := time.NewTicker(5 * time.Millisecond)
    defer ticker.Stop()
    index := 0
    samples := profile.Samples(profile.Duration / 3)
    for {
        select {
        case <-ctx.Done():
            c.mu.Lock()
            c.active = false
            close(c.done)
            c.mu.Unlock()
            return
        case now := <-ticker.C:
            c.mu.Lock()
            if !c.active {
                c.mu.Unlock()
                return
            }
            value := samples[index%len(samples)]
            command := model.DeviceCommand{ID: string(model.NewEventID()), Run: c.run, Epoch: epoch, Setpoint: value, IssuedAt: now.UTC()}
            if reason, envelope, violated := c.checkEnvelope(command, now); violated {
                c.aborts = append(c.aborts, AbortRecord{Command: command, Envelope: envelope, Reason: reason, AbortedAt: now.UTC()})
                c.active = false
                close(c.done)
                c.mu.Unlock()
                return
            }
            c.commands = append(c.commands, command)
            c.lastSetpoint = value
            c.hasLast = true
            c.lastIssuedAt = now
            index++
            c.mu.Unlock()
            _ = at
        }
    }
}

// checkEnvelope verifies a candidate command against the section envelope
// in force right now. Callers hold c.mu.
func (c *Controller) checkEnvelope(command model.DeviceCommand, now time.Time) (string, tank.WaveEnvelope, bool) {
    if c.envelopes == nil {
        return "", tank.WaveEnvelope{}, false
    }
    envelope, ok := c.envelopes.WaveEnvelope(c.section)
    if !ok {
        return "no wave envelope for section " + c.section, tank.WaveEnvelope{}, true
    }
    reason := envelope.Check(c.lastSetpoint, c.hasLast, command.Setpoint, now.Sub(c.lastIssuedAt))
    return reason, envelope, reason != ""
}

func (c *Controller) Stop() {
    c.mu.Lock()
    c.active = false
    c.mu.Unlock()
}

func (c *Controller) Active() bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.active
}

func (c *Controller) Commands() []model.DeviceCommand {
    c.mu.Lock()
    defer c.mu.Unlock()
    result := make([]model.DeviceCommand, len(c.commands))
    copy(result, c.commands)
    return result
}

// Aborts returns the plans stopped by envelope violations, each with the
// withheld command and the reason.
func (c *Controller) Aborts() []AbortRecord {
    c.mu.Lock()
    defer c.mu.Unlock()
    result := make([]AbortRecord, len(c.aborts))
    copy(result, c.aborts)
    return result
}

func (c *Controller) WaitStopped(timeout time.Duration) bool {
    c.mu.Lock()
    done := c.done
    c.mu.Unlock()
    select {
    case <-done:
        return true
    case <-time.After(timeout):
        return false
    }
}
