package tank

import (
    "fmt"
    "math"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

// DefaultRateLimit bounds the commanded wave-height slew when no tighter
// limit is configured. The stock 0.8 Hz / 0.15 m profile slews at roughly
// 12 m/s on the 5 ms command tick, so the default sits above that.
const DefaultRateLimit = 20.0

// WaveEnvelope is the verifiable bound a section places on commanded wave
// height and its rate of change. Revision and Basis make the envelope
// auditable: every change of section conditions produces a new revision and
// records what the limits were derived from.
type WaveEnvelope struct {
    MaxHeight float64 `json:"max_height"`
    MaxRate float64 `json:"max_rate"`
    Revision model.RevisionID `json:"revision"`
    Basis string `json:"basis"`
    UpdatedAt time.Time `json:"updated_at"`
}

// DefaultEnvelope derives the envelope from the section depth: wave height
// is capped at 40% of the depth so commanded waves stay clear of breaking.
func DefaultEnvelope(depth float64, at time.Time) WaveEnvelope {
    return WaveEnvelope{
        MaxHeight: 0.4 * depth,
        MaxRate: DefaultRateLimit,
        Revision: model.NewRevisionID(),
        Basis: fmt.Sprintf("height=0.4*depth(%.3f), rate=%.3f", depth, DefaultRateLimit),
        UpdatedAt: at.UTC(),
    }
}

// Check validates the next candidate setpoint against the envelope, given
// the previously issued setpoint and the elapsed time since it was issued.
// It returns an empty string when the command stays inside the envelope and
// a human-readable reason otherwise.
func (e WaveEnvelope) Check(previous float64, hasPrevious bool, next float64, elapsed time.Duration) string {
    if math.Abs(next) > e.MaxHeight {
        return fmt.Sprintf("wave height %.4f m exceeds envelope %.4f m", math.Abs(next), e.MaxHeight)
    }
    if hasPrevious && elapsed > 0 {
        rate := math.Abs(next-previous) / elapsed.Seconds()
        if rate > e.MaxRate {
            return fmt.Sprintf("wave rate %.4f m/s exceeds envelope %.4f m/s", rate, e.MaxRate)
        }
    }
    return ""
}
