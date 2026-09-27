package wave

import (
    "context"
    "math"
    "strings"
    "testing"
    "time"

    "github.com/wyw14/solo-037/internal/model"
    "github.com/wyw14/solo-037/internal/tank"
)

func startGuarded(controller *Controller, sections *tank.Sections, section string) context.CancelFunc {
    ctx, cancel := context.WithCancel(context.Background())
    controller.Start(ctx, model.RunID("run-1"), section, sections, NewProfile(0.8, 0.15, 80*time.Millisecond), 1, time.Now())
    return cancel
}

func TestControllerAbortsWhenSectionConditionsTighten(t *testing.T) {
    sections := tank.NewSections([]string{"section-a"}, 2.4)
    controller := NewController()
    cancel := startGuarded(controller, sections, "section-a")
    defer cancel()

    time.Sleep(25 * time.Millisecond)
    before := CommandCount(controller, "run-1")
    if before == 0 {
        t.Fatal("expected commands to flow while inside the envelope")
    }

    // Section conditions change mid-run: shallower water tightens the
    // verifiable envelope to 0.04 m, below the profile's 0.0587 m peak.
    sections.UpdateDepth("section-a", 0.1)

    if !controller.WaitStopped(2 * time.Second) {
        t.Fatal("controller did not stop the plan after the envelope tightened")
    }
    if controller.Active() {
        t.Fatal("controller still active after envelope violation")
    }

    aborts := controller.Aborts()
    if len(aborts) != 1 {
        t.Fatalf("expected exactly one abort record, got %d", len(aborts))
    }
    abort := aborts[0]
    if !strings.Contains(abort.Reason, "exceeds envelope 0.0400") {
        t.Fatalf("abort reason does not name the violated bound: %q", abort.Reason)
    }
    if abort.Command.Run != "run-1" || abort.Command.ID == "" {
        t.Fatalf("abort did not preserve the actual command: %+v", abort.Command)
    }
    if abort.Command.Setpoint <= 0.04 {
        t.Fatalf("withheld command should be the out-of-envelope setpoint, got %v", abort.Command.Setpoint)
    }
    if math.Abs(abort.Envelope.MaxHeight-0.04) > 1e-9 {
        t.Fatalf("abort did not preserve the envelope in force: %+v", abort.Envelope)
    }

    // The plan is stopped: no further commands are issued after the abort.
    issued := CommandCount(controller, "run-1")
    time.Sleep(30 * time.Millisecond)
    if got := CommandCount(controller, "run-1"); got != issued {
        t.Fatalf("commands kept flowing after abort: %d -> %d", issued, got)
    }
    for _, command := range controller.Commands() {
        if command.IssuedAt.After(abort.AbortedAt) {
            t.Fatalf("command %v issued after abort at %v", command.IssuedAt, abort.AbortedAt)
        }
    }
}

func TestControllerRunsInsideEnvelope(t *testing.T) {
    sections := tank.NewSections([]string{"section-a"}, 2.4)
    controller := NewController()
    cancel := startGuarded(controller, sections, "section-a")

    time.Sleep(40 * time.Millisecond)
    cancel()
    if !controller.WaitStopped(2 * time.Second) {
        t.Fatal("controller did not stop on cancel")
    }
    if CommandCount(controller, "run-1") < 3 {
        t.Fatalf("expected several commands inside the envelope, got %d", CommandCount(controller, "run-1"))
    }
    if aborts := controller.Aborts(); len(aborts) != 0 {
        t.Fatalf("unexpected aborts inside the envelope: %+v", aborts)
    }
}

func TestControllerAbortsOnRateViolation(t *testing.T) {
    sections := tank.NewSections([]string{"section-a"}, 2.4)
    if _, err := sections.SetEnvelope("section-a", 5.0, 1.0, "test rate limit", time.Now()); err != nil {
        t.Fatal(err)
    }
    controller := NewController()
    cancel := startGuarded(controller, sections, "section-a")
    defer cancel()

    if !controller.WaitStopped(2 * time.Second) {
        t.Fatal("controller did not stop on rate violation")
    }
    aborts := controller.Aborts()
    if len(aborts) != 1 || !strings.Contains(aborts[0].Reason, "wave rate") {
        t.Fatalf("expected a single rate-violation abort, got %+v", aborts)
    }
}

func TestControllerAbortsWhenSectionUnknown(t *testing.T) {
    sections := tank.NewSections([]string{"section-a"}, 2.4)
    controller := NewController()
    cancel := startGuarded(controller, sections, "missing-section")
    defer cancel()

    if !controller.WaitStopped(2 * time.Second) {
        t.Fatal("controller did not stop for an unknown section")
    }
    aborts := controller.Aborts()
    if len(aborts) != 1 || !strings.Contains(aborts[0].Reason, "no wave envelope") {
        t.Fatalf("expected a missing-envelope abort, got %+v", aborts)
    }
}
