package broker

import (
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
)

func EnsureJobsStream(js nats.JetStreamContext, streamName string) error {
	_, err := js.StreamInfo(streamName)
	if err == nil {
		return nil
	}
	if !errors.Is(err, nats.ErrStreamNotFound) {
		return fmt.Errorf("inspect jetstream stream %q: %w", streamName, err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{"jobs.>"},
		Storage:  nats.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("create jetstream stream %q: %w", streamName, err)
	}
	return nil
}
