package wave

import (
    "context"
    "math"
    "strings"
    "testing"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

func testProfile() Profile { return NewProfile(0.8, 0.15, 80*time.Millisecond) }

func TestEnvelopeCheckBoundaries(t *testing.T) {
    envelope := Envelope{Section: "section-a", MaxHeight: 0.3, MaxRate: 5}
    at := time.Unix(1700000000, 0).UTC()
    previous := model.DeviceCommand{Setpoint: 0, IssuedAt: at}
    ok := model.DeviceCommand{Setpoint: 0.01, IssuedAt: at.Add(5 * time.Millisecond)}
    if err := envelope.Check(ok, &previous); err != nil {
        t.Fatalf("expected command within envelope, got %v", err)
    }
    if err := envelope.Check(ok, nil); err != nil {
        t.Fatalf("expected first command without previous to pass, got %v", err)
    }
    high := model.DeviceCommand{Setpoint: 0.4, IssuedAt: at.Add(5 * time.Millisecond)}
    if err := envelope.Check(high, &previous); err == nil || !strings.Contains(err.Error(), "wave height") {
        t.Fatalf("expected wave height violation, got %v", err)
    }
    fast := model.DeviceCommand{Setpoint: 0.1, IssuedAt: at.Add(5 * time.Millisecond)}
    if err := envelope.Check(fast, &previous); err == nil || !strings.Contains(err.Error(), "rate") {
        t.Fatalf("expected rate violation, got %v", err)
    }
}

func TestHeightBreachStopsPlan(t *testing.T) {
    controller := NewController()
    run := model.NewRunID()
    envelope := controller.SetEnvelope("section-a", 0.05, 1000, time.Now())
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    controller.Start(ctx, run, "section-a", testProfile(), 1, time.Now())
    if !controller.WaitStopped(2 * time.Second) {
        t.Fatal("expected plan to stop after envelope breach")
    }
    if controller.Active() {
        t.Fatal("expected controller inactive after breach")
    }
    breaches := controller.Breaches()
    if len(breaches) != 1 {
        t.Fatalf("expected one breach, got %d", len(breaches))
    }
    breach := breaches[0]
    if breach.Run != run || breach.Section != "section-a" {
        t.Fatalf("breach not bound to run and section: %+v", breach)
    }
    if math.Abs(breach.Command.Setpoint) <= envelope.MaxHeight {
        t.Fatalf("expected blocked command above envelope, got %v", breach.Command.Setpoint)
    }
    if !strings.Contains(breach.Reason, "wave height") {
        t.Fatalf("expected height reason, got %q", breach.Reason)
    }
    if breach.Envelope.Revision != envelope.Revision {
        t.Fatalf("breach not verifiable against envelope revision %s", envelope.Revision)
    }
    for _, command := range controller.Commands() {
        if math.Abs(command.Setpoint) > envelope.MaxHeight {
            t.Fatalf("issued command above envelope: %v", command.Setpoint)
        }
    }
    issued := len(controller.Commands())
    time.Sleep(20 * time.Millisecond)
    if len(controller.Commands()) != issued {
        t.Fatal("commands continued after plan stopped")
    }
}

func TestRateBreachStopsPlan(t *testing.T) {
    controller := NewController()
    controller.SetEnvelope("section-a", 10, 2, time.Now())
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    controller.Start(ctx, model.NewRunID(), "section-a", testProfile(), 1, time.Now())
    if !controller.WaitStopped(2 * time.Second) {
        t.Fatal("expected plan to stop after rate breach")
    }
    breaches := controller.Breaches()
    if len(breaches) != 1 {
        t.Fatalf("expected one breach, got %d", len(breaches))
    }
    if !strings.Contains(breaches[0].Reason, "rate") {
        t.Fatalf("expected rate reason, got %q", breaches[0].Reason)
    }
}

func TestMidRunEnvelopeChangeStopsPlan(t *testing.T) {
    controller := NewController()
    controller.SetEnvelope("section-a", 0.5, 1000, time.Now())
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    controller.Start(ctx, model.NewRunID(), "section-a", testProfile(), 1, time.Now())
    time.Sleep(40 * time.Millisecond)
    if !controller.Active() {
        t.Fatal("expected plan running under generous envelope")
    }
    before := len(controller.Commands())
    if before < 2 {
        t.Fatalf("expected commands flowing before tightening, got %d", before)
    }
    tightened := controller.SetEnvelope("section-a", 0.03, 1000, time.Now())
    if !controller.WaitStopped(2 * time.Second) {
        t.Fatal("expected plan to stop after envelope tightened mid-run")
    }
    breaches := controller.Breaches()
    if len(breaches) != 1 {
        t.Fatalf("expected one breach, got %d", len(breaches))
    }
    if breaches[0].Envelope.Revision != tightened.Revision || breaches[0].Envelope.MaxHeight != 0.03 {
        t.Fatalf("breach not checked against tightened envelope: %+v", breaches[0].Envelope)
    }
}

func TestNoEnvelopeAllowsFullProfile(t *testing.T) {
    controller := NewController()
    ctx, cancel := context.WithCancel(context.Background())
    controller.Start(ctx, model.NewRunID(), "section-b", testProfile(), 1, time.Now())
    time.Sleep(30 * time.Millisecond)
    cancel()
    if !controller.WaitStopped(2 * time.Second) {
        t.Fatal("expected plan to stop on cancel")
    }
    if len(controller.Commands()) == 0 {
        t.Fatal("expected commands without envelope")
    }
    if len(controller.Breaches()) != 0 {
        t.Fatalf("expected no breaches without envelope, got %d", len(controller.Breaches()))
    }
}
