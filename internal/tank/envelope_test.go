package tank

import (
    "math"
    "strings"
    "testing"
    "time"
)

func TestDefaultEnvelopeScalesWithDepth(t *testing.T) {
    at := time.Now()
    envelope := DefaultEnvelope(2.4, at)
    if math.Abs(envelope.MaxHeight-0.96) > 1e-9 {
        t.Fatalf("expected height cap 0.96 for depth 2.4, got %v", envelope.MaxHeight)
    }
    if envelope.MaxRate != DefaultRateLimit {
        t.Fatalf("expected default rate limit, got %v", envelope.MaxRate)
    }
    if envelope.Revision == "" || envelope.Basis == "" || !envelope.UpdatedAt.Equal(at.UTC()) {
        t.Fatalf("envelope is not verifiable: %+v", envelope)
    }
}

func TestEnvelopeCheck(t *testing.T) {
    envelope := WaveEnvelope{MaxHeight: 0.1, MaxRate: 2}
    if reason := envelope.Check(0, false, 0.05, 5*time.Millisecond); reason != "" {
        t.Fatalf("unexpected violation: %q", reason)
    }
    if reason := envelope.Check(0, false, -0.15, 5*time.Millisecond); !strings.Contains(reason, "wave height") {
        t.Fatalf("expected height violation, got %q", reason)
    }
    // 0.08 m over 5 ms is 16 m/s, above the 2 m/s cap.
    if reason := envelope.Check(0, true, 0.08, 5*time.Millisecond); !strings.Contains(reason, "wave rate") {
        t.Fatalf("expected rate violation, got %q", reason)
    }
    // The first command of a plan has no previous setpoint: no rate check.
    if reason := envelope.Check(0, false, 0.08, 5*time.Millisecond); reason != "" {
        t.Fatalf("unexpected rate check on first command: %q", reason)
    }
}

func TestUpdateDepthRevisesEnvelope(t *testing.T) {
    sections := NewSections([]string{"section-a"}, 2.4)
    before, ok := sections.WaveEnvelope("section-a")
    if !ok {
        t.Fatal("expected an envelope for section-a")
    }
    state := sections.UpdateDepth("section-a", 0.1)
    after := state.Envelope
    if math.Abs(after.MaxHeight-0.04) > 1e-9 {
        t.Fatalf("expected tightened height cap 0.04, got %v", after.MaxHeight)
    }
    if after.Revision == before.Revision {
        t.Fatal("envelope revision did not change with section conditions")
    }
}

func TestSetEnvelopeValidatesAndRecords(t *testing.T) {
    sections := NewSections([]string{"section-a"}, 2.4)
    at := time.Now()
    state, err := sections.SetEnvelope("section-a", 0.05, 0.5, "operator limit", at)
    if err != nil {
        t.Fatal(err)
    }
    if state.Envelope.MaxHeight != 0.05 || state.Envelope.MaxRate != 0.5 || state.Envelope.Basis != "operator limit" {
        t.Fatalf("explicit envelope not recorded: %+v", state.Envelope)
    }
    if _, err := sections.SetEnvelope("section-a", 0, 1, "bad", at); err == nil {
        t.Fatal("expected error for non-positive height")
    }
    if _, err := sections.SetEnvelope("section-a", 1, -1, "bad", at); err == nil {
        t.Fatal("expected error for non-positive rate")
    }
    if _, err := sections.SetEnvelope("missing", 1, 1, "bad", at); err == nil {
        t.Fatal("expected error for unknown section")
    }
    if _, ok := sections.WaveEnvelope("missing"); ok {
        t.Fatal("unexpected envelope for unknown section")
    }
}
