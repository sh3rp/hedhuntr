package broker

import (
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
)

func EnsureJobsStream(js nats.JetStreamContext, streamName string) error {
	subjects := []string{"jobs.>", "applications.>", "automation.>", "notifications.>"}
	info, err := js.StreamInfo(streamName)
	if err == nil {
		if streamHasSubjects(info.Config.Subjects, subjects) {
			return nil
		}
		config := info.Config
		config.Subjects = mergeSubjects(config.Subjects, subjects)
		if _, err := js.UpdateStream(&config); err != nil {
			return fmt.Errorf("update jetstream stream %q subjects: %w", streamName, err)
		}
		return nil
	}
	if !errors.Is(err, nats.ErrStreamNotFound) {
		return fmt.Errorf("inspect jetstream stream %q: %w", streamName, err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: subjects,
		Storage:  nats.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("create jetstream stream %q: %w", streamName, err)
	}
	return nil
}

func streamHasSubjects(existing, required []string) bool {
	seen := map[string]struct{}{}
	for _, subject := range existing {
		seen[subject] = struct{}{}
	}
	for _, subject := range required {
		if _, ok := seen[subject]; !ok {
			return false
		}
	}
	return true
}

func mergeSubjects(existing, required []string) []string {
	seen := map[string]struct{}{}
	merged := make([]string, 0, len(existing)+len(required))
	for _, subject := range existing {
		if _, ok := seen[subject]; ok {
			continue
		}
		seen[subject] = struct{}{}
		merged = append(merged, subject)
	}
	for _, subject := range required {
		if _, ok := seen[subject]; ok {
			continue
		}
		seen[subject] = struct{}{}
		merged = append(merged, subject)
	}
	return merged
}
