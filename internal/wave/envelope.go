package wave

import (
    "fmt"
    "math"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

type Envelope struct {
    Section string `json:"section"`
    MaxHeight float64 `json:"max_height"`
    MaxRate float64 `json:"max_rate"`
    Revision model.RevisionID `json:"revision"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Breach struct {
    Run model.RunID `json:"run"`
    Section string `json:"section"`
    Command model.DeviceCommand `json:"command"`
    Envelope Envelope `json:"envelope"`
    Reason string `json:"reason"`
    RecordedAt time.Time `json:"recorded_at"`
}

func (e Envelope) Check(command model.DeviceCommand, previous *model.DeviceCommand) error {
    height := math.Abs(command.Setpoint)
    if height > e.MaxHeight {
        return fmt.Errorf("wave height %.4f exceeds section %q envelope %.4f", height, e.Section, e.MaxHeight)
    }
    if previous != nil {
        elapsed := command.IssuedAt.Sub(previous.IssuedAt).Seconds()
        if elapsed > 0 {
            rate := math.Abs(command.Setpoint-previous.Setpoint) / elapsed
            if rate > e.MaxRate {
                return fmt.Errorf("setpoint rate %.4f exceeds section %q envelope %.4f", rate, e.Section, e.MaxRate)
            }
        }
    }
    return nil
}

func (c *Controller) SetEnvelope(section string, maxHeight float64, maxRate float64, at time.Time) Envelope {
    c.mu.Lock()
    defer c.mu.Unlock()
    envelope := Envelope{Section: section, MaxHeight: maxHeight, MaxRate: maxRate, Revision: model.NewRevisionID(), UpdatedAt: at.UTC()}
    c.envelopes[section] = envelope
    return envelope
}

func (c *Controller) Envelope(section string) (Envelope, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    envelope, ok := c.envelopes[section]
    return envelope, ok
}

func (c *Controller) Envelopes() []Envelope {
    c.mu.Lock()
    defer c.mu.Unlock()
    result := make([]Envelope, 0, len(c.envelopes))
    for _, envelope := range c.envelopes {
        result = append(result, envelope)
    }
    return result
}

func (c *Controller) Breaches() []Breach {
    c.mu.Lock()
    defer c.mu.Unlock()
    result := make([]Breach, len(c.breaches))
    copy(result, c.breaches)
    return result
}

func (c *Controller) verify(command model.DeviceCommand) (Breach, bool) {
    envelope, ok := c.envelopes[c.section]
    if !ok {
        return Breach{}, true
    }
    var previous *model.DeviceCommand
    if c.hasLast {
        last := c.last
        previous = &last
    }
    if err := envelope.Check(command, previous); err != nil {
        return Breach{Run: c.run, Section: c.section, Command: command, Envelope: envelope, Reason: err.Error(), RecordedAt: command.IssuedAt}, false
    }
    return Breach{}, true
}
