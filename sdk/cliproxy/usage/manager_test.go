package usage

import (
	"context"
	"testing"
	"time"
)

func TestManagerCanRestartAfterStop(t *testing.T) {
	manager := NewManager(1)
	plugin := &recordingPlugin{records: make(chan Record, 1)}
	manager.Register(plugin)
	manager.Start(context.Background())
	manager.Stop()

	manager.Start(context.Background())
	manager.Publish(context.Background(), Record{Model: "restarted"})

	select {
	case record := <-plugin.records:
		if record.Model != "restarted" {
			t.Fatalf("record model = %q, want restarted", record.Model)
		}
	case <-time.After(time.Second):
		t.Fatal("manager did not dispatch a record after restart")
	}
	manager.Stop()
}

type recordingPlugin struct {
	records chan Record
}

func (p *recordingPlugin) HandleUsage(_ context.Context, record Record) {
	p.records <- record
}
