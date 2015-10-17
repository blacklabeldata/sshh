package router

import (
	"fmt"
	"testing"
)
import "golang.org/x/net/context"

type NoopHandler struct {
}

func (NoopHandler) Handle(*Context) error {
	return nil
}

func BenchmarkHandleSingle(b *testing.B) {
	r := New(nil, nil, nil)
	r.Register("/noop", &NoopHandler{})
	ctx := Context{
		Path:     "/noop",
		Params:   nil,
		Context:  context.Background(),
		Channel:  nil,
		Requests: nil,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Handle(&ctx)
	}
}

func BenchmarkHandleLong(b *testing.B) {
	r := New(nil, nil, nil)
	r.Register("/repos/:owner/:repo/issues/:number/comments", &NoopHandler{})
	ctx := Context{
		Path:     "/repos/:owner/:repo/issues/:number/comments",
		Params:   nil,
		Context:  context.Background(),
		Channel:  nil,
		Requests: nil,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Handle(&ctx)
	}
}

func TestParams(t *testing.T) {
	r := New(nil, nil, nil)
	r.RegisterFunc("/repos/:owner/:repo/issues/:number/comments", func(ctx *Context) error {
		fmt.Println(ctx.Params)
		return nil
	})
	ctx := Context{
		Path:     "/repos/eliquious/32/issues/1/comments",
		Params:   nil,
		Context:  context.Background(),
		Channel:  nil,
		Requests: nil,
	}
	r.Handle(&ctx)
}
