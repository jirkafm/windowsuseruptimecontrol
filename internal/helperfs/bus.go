package helperfs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"wincontrol/internal/helperipc"
)

type Bus struct {
	root string
}

func New(root string) *Bus {
	return &Bus{root: root}
}

func (b *Bus) Speak(ctx context.Context, userSID, message string) error {
	return b.enqueue(ctx, userSID, helperipc.Command{Type: helperipc.CommandSpeak, Message: message})
}

func (b *Bus) enqueue(ctx context.Context, userSID string, cmd helperipc.Command) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	dir := filepath.Join(b.root, userSID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("%d.json", time.Now().UnixNano())
	return os.WriteFile(filepath.Join(dir, name), payload, 0o644)
}

func (b *Bus) Poll(userSID string) ([]helperipc.Command, error) {
	dir := filepath.Join(b.root, userSID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	commands := make([]helperipc.Command, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var cmd helperipc.Command
		if err := json.Unmarshal(data, &cmd); err != nil {
			return nil, err
		}
		commands = append(commands, cmd)
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	return commands, nil
}
