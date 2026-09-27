package wave

import (
    "context"
    "time"

    "github.com/wyw14/solo-037/internal/model"
)

func StartForRun(ctx context.Context, run model.RunID, revision model.RevisionID, controller *Controller, section string, envelopes EnvelopeSource, at time.Time) Profile {
    profile := NewProfile(0.8, 0.15, 80*time.Millisecond)
    controller.Start(ctx, run, section, envelopes, profile, 1, at)
    _ = revision
    return profile
}

func StopForAbort(controller *Controller) {
    controller.Stop()
}

func CommandCount(controller *Controller, run model.RunID) int {
    count := 0
    for _, command := range controller.Commands() {
        if command.Run == run {
            count++
        }
    }
    return count
}

func AbortCount(controller *Controller, run model.RunID) int {
    count := 0
    for _, abort := range controller.Aborts() {
        if abort.Command.Run == run {
            count++
        }
    }
    return count
}
