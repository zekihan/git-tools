package main

import (
	"context"
	"sync"

	tea "charm.land/bubbletea/v2"
)

type pullEntry struct {
	path   string
	result pullResult
}

type pullScanMsg struct {
	err   error
	paths []string
}
type (
	pullStartedMsg  int
	pullFinishedMsg struct {
		result pullResult
		index  int
	}
)
type pullDoneMsg struct{}

//nolint:containedctx // A session owns cancellation for its workers and subsequent retries.
type pullSession struct {
	ctx    context.Context
	events chan tea.Msg
	done   chan struct{}
}

func startPullSession(ctx context.Context, dir string, threads int) *pullSession {
	session := &pullSession{ctx: ctx, events: make(chan tea.Msg), done: make(chan struct{})}
	go func() {
		defer close(session.done)
		defer close(session.events)

		paths, err := findGitRepos(ctx, dir)
		if !session.send(ctx, pullScanMsg{paths: paths, err: err}) || err != nil {
			return
		}

		jobs := make(chan int)

		var workers sync.WaitGroup
		for range min(threads, len(paths)) {
			workers.Go(func() {
				for index := range jobs {
					if ctx.Err() != nil || !session.send(ctx, pullStartedMsg(index)) {
						return
					}

					result := pullRepo(ctx, paths[index])
					if !session.send(ctx, pullFinishedMsg{index: index, result: result}) {
						return
					}
				}
			})
		}

		defer workers.Wait()
		defer close(jobs)

		for index := range paths {
			select {
			case jobs <- index:
			case <-ctx.Done():
				return
			}
		}
	}()

	return session
}

func (session *pullSession) send(ctx context.Context, msg tea.Msg) bool {
	select {
	case session.events <- msg:
		return true
	case <-ctx.Done():
		return false
	}
}

//nolint:ireturn // Bubble Tea commands return messages.
func (session *pullSession) next() tea.Msg {
	msg, ok := <-session.events
	if !ok {
		return pullDoneMsg{}
	}

	return msg
}

func (session *pullSession) retry(path string, index int) *pullSession {
	retry := &pullSession{ctx: session.ctx, events: make(chan tea.Msg), done: make(chan struct{})}
	go func() {
		defer close(retry.done)
		defer close(retry.events)

		result := pullRepo(retry.ctx, path)
		retry.send(retry.ctx, pullFinishedMsg{index: index, result: result})
	}()

	return retry
}
