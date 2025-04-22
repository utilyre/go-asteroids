package mcp_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"multiplayer/internal/mcp"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

type assertLogHandler struct {
	handler slog.Handler
	fail    func()
}

func (h assertLogHandler) Enabled(ctx context.Context, l slog.Level) bool {
	if l >= slog.LevelWarn {
		h.fail()
	}
	return h.handler.Enabled(ctx, l)
}

func (h assertLogHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.handler.Handle(ctx, r)
}

func (h assertLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return assertLogHandler{
		handler: h.handler.WithAttrs(attrs),
		fail:    h.fail,
	}
}

func (h assertLogHandler) WithGroup(name string) slog.Handler {
	return assertLogHandler{
		handler: h.handler.WithGroup(name),
		fail:    h.fail,
	}
}

func newAssertLogger(fail func()) *slog.Logger {
	return slog.New(assertLogHandler{
		handler: slog.Default().Handler(),
		fail:    fail,
	})
}

func TestListener_one_to_one(t *testing.T) {
	t.Run("ping pong", func(t *testing.T) {
		msgPing := []byte("ping")
		msgPong := []byte("pong")

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// this is such a bad idea because t.Fail is sometimes called after the
		// test is done, which causes a panic in go's toolchain. Let alone
		// t.FailNow() that is forbidden to call in goroutines other than test.
		logger := newAssertLogger(t.Fail)

		server, err := mcp.Listen("127.0.0.1:", mcp.WithLogger(logger))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			err := server.Close(ctx)
			if err != nil {
				t.Fatal(err)
			}
		}()

		// to synchronize sends and receives since we are merely testing the
		// basics of communication right now
		syn := make(chan struct{})
		defer close(syn)

		g, ctx := errgroup.WithContext(ctx)
		g.Go(func() (err error) {
			sess, err := server.Accept(ctx)
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, sess.Close(ctx)) }()

			data, err := sess.Receive(ctx)
			if err != nil {
				return err
			}
			select {
			case <-syn:
			case <-ctx.Done():
				return ctx.Err()
			}
			if !bytes.Equal(msgPing, data) {
				t.Errorf("expected data %q; actual data %q", string(msgPing), string(data))
			}

			err = sess.Send(ctx, msgPong)
			if err != nil {
				return err
			}
			select {
			case syn <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}

			return nil
		})
		g.Go(func() (err error) {
			client, err := mcp.Dial(ctx, server.LocalAddr().String(), mcp.WithLogger(logger))
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, client.Close(ctx)) }()

			err = client.Send(ctx, msgPing)
			if err != nil {
				return err
			}
			select {
			case syn <- struct{}{}:
			case <-ctx.Done():
				return ctx.Err()
			}

			data, err := client.Receive(ctx)
			if err != nil {
				return err
			}
			select {
			case <-syn:
			case <-ctx.Done():
				return ctx.Err()
			}
			if !bytes.Equal(msgPong, data) {
				t.Errorf("expected data %q; actual data %q", string(msgPong), string(data))
			}

			return nil
		})
		if err := g.Wait(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("graceful closure", func(t *testing.T) {
		t.Fatal("TODO")
	})

	t.Run("abuse of closure", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		ln, err := mcp.Listen("127.0.0.1:")
		if err != nil {
			t.Fatal(err)
		}

		err = ln.Close(ctx)
		if err != nil {
			t.Fatal(err)
		}
		err = ln.Close(ctx)
		if !errors.Is(err, mcp.ErrClosed) {
			t.Fatal(err)
		}
		err = ln.Close(ctx)
		if !errors.Is(err, mcp.ErrClosed) {
			t.Fatal(err)
		}
	})

	t.Run("abuse of closure with clients", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		server, err := mcp.Listen("127.0.0.1:")
		if err != nil {
			t.Fatal(err)
		}

		spawnClient := func() error {
			client, err := mcp.Dial(ctx, server.LocalAddr().String())
			if err != nil {
				return err
			}
			err = client.Close(ctx)
			if err != nil {
				return err
			}
			err = client.Close(ctx)
			if !errors.Is(err, mcp.ErrClosed) {
				return err
			}
			err = client.Close(ctx)
			if !errors.Is(err, mcp.ErrClosed) {
				return err
			}
			return nil
		}
		acceptClient := func() error {
			sess, err := server.Accept(ctx)
			if err != nil {
				return err
			}
			err = sess.Close(ctx)
			if err != nil {
				return err
			}
			err = sess.Close(ctx)
			if err != nil {
				return err
			}
			err = sess.Close(ctx)
			if err != nil {
				return err
			}
			return nil
		}
		g, ctx := errgroup.WithContext(ctx)
		g.Go(spawnClient)
		g.Go(spawnClient)
		g.Go(acceptClient)
		g.Go(acceptClient)
		if err := g.Wait(); err != nil {
			t.Fatal(err)
		}
	})
}
