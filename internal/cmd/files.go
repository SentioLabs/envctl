// Package cmd implements the CLI commands for envctl.
// This file materializes file sinks and merges their path variables.
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/sentiolabs/envctl/internal/env"
	"github.com/sentiolabs/envctl/internal/filesink"
)

// sinkMode controls which file sinks a command materializes.
type sinkMode int

const (
	// sinkModeRun writes ephemeral and persistent sinks. Only envctl run uses it.
	sinkModeRun sinkMode = iota
	// sinkModePersistentOnly writes persistent sinks and skips ephemeral ones
	// with a stderr warning. Commands that print to stdout use it.
	sinkModePersistentOnly
)

// fileSourcePrefix marks path entries that came from a file sink.
const fileSourcePrefix = "file:"

// overrideSource is the Source label the builder assigns to --set entries.
const overrideSource = "override"

// materialized holds the path entries produced by materializeFiles and the
// ephemeral directory, if one was created.
type materialized struct {
	entries []env.Entry
	dir     *filesink.Dir
}

// Close removes the ephemeral directory. Safe on nil and safe to call twice.
func (m *materialized) Close() {
	if m != nil && m.dir != nil {
		_ = m.dir.Close()
	}
}

// materializeFiles writes each resolved file according to mode and returns
// one path entry per written file plus, when an ephemeral directory was
// created and dirAs is set, an entry naming that directory. Content buffers
// are zeroed after use. Any failure removes the ephemeral directory.
//
//nolint:revive // stderr writes always succeed
func materializeFiles(files []env.ResolvedFile, dirAs, configDir string, mode sinkMode) (*materialized, error) {
	m := &materialized{}
	var skipped []string

	for i := range files {
		f := &files[i]
		bits, err := f.Sink.FileMode()
		if err != nil {
			m.Close()
			return nil, err
		}

		var path string
		switch {
		case f.Sink.Persistent():
			path, err = filesink.WritePersistent(f.Sink.Path, configDir, f.Content, bits)
		case mode == sinkModeRun:
			if m.dir == nil {
				m.dir, err = filesink.NewEphemeral()
				if err != nil {
					return nil, err
				}
			}
			path, err = m.dir.WriteEphemeral(f.Sink.Name, f.Content, bits)
		default:
			skipped = append(skipped, f.Sink.PathAs)
			clear(f.Content)
			continue
		}
		clear(f.Content)
		if err != nil {
			m.Close()
			return nil, err
		}

		verboseLog("Wrote file sink %s -> %s", f.Sink.PathAs, path)
		m.entries = append(m.entries, env.Entry{
			Key:    f.Sink.PathAs,
			Value:  path,
			Source: fileSourcePrefix + f.Secret,
		})
	}

	if m.dir != nil && dirAs != "" {
		m.entries = append(m.entries, env.Entry{Key: dirAs, Value: m.dir.Path(), Source: "file"})
	}

	if len(skipped) > 0 {
		fmt.Fprintf(os.Stderr,
			"[envctl] skipped %d ephemeral file sink(s) (%s); only 'envctl run' materializes them\n",
			len(skipped), strings.Join(skipped, ", "))
	}

	return m, nil
}

// mergeEntries adds extra entries to entries. A path entry replaces a
// secret-derived entry with the same key but never displaces a --set override.
func mergeEntries(entries, extra []env.Entry) []env.Entry {
	index := make(map[string]int, len(entries))
	for i, e := range entries {
		index[e.Key] = i
	}
	for _, e := range extra {
		if i, ok := index[e.Key]; ok {
			if entries[i].Source == overrideSource {
				continue
			}
			entries[i] = e
			continue
		}
		index[e.Key] = len(entries)
		entries = append(entries, e)
	}
	return entries
}
