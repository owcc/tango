// Copyright 2015 The Tango Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tango

import (
	"net/http"
	"os"
	"sync"
)

func Version() string {
	return "0.4.5.0507"
}

type Tango struct {
	Router
	handlers   []Handler
	logger     Logger
	ErrHandler Handler
	ctxPool    sync.Pool
	respPool   sync.Pool
}

var (
	ClassicHandlers = []Handler{
		Logging(),
		Recovery(true),
		Compresses([]string{}),
		Static(StaticOptions{Prefix: "public"}),
		Return(),
		Param(),
		Contexts(),
	}
)

func (t *Tango) Logger() Logger {
	return t.logger
}

func (t *Tango) Get(url string, c interface{}) {
	t.Route([]string{"GET", "HEAD"}, url, c)
}

func (t *Tango) Post(url string, c interface{}) {
	t.Route([]string{"POST"}, url, c)
}

func (t *Tango) Head(url string, c interface{}) {
	t.Route([]string{"HEAD"}, url, c)
}

func (t *Tango) Options(url string, c interface{}) {
	t.Route([]string{"OPTIONS"}, url, c)
}

func (t *Tango) Trace(url string, c interface{}) {
	t.Route([]string{"TRACE"}, url, c)
}

func (t *Tango) Patch(url string, c interface{}) {
	t.Route([]string{"PATCH"}, url, c)
}

func (t *Tango) Delete(url string, c interface{}) {
	t.Route([]string{"DELETE"}, url, c)
}

func (t *Tango) Put(url string, c interface{}) {
	t.Route([]string{"PUT"}, url, c)
}

func (t *Tango) Any(url string, c interface{}) {
	t.Route(SupportMethods, url, c)
}

func (t *Tango) Use(handlers ...Handler) {
	t.handlers = append(t.handlers, handlers...)
}

func (t *Tango) Run(addrs ...string) {
	var addr string
	if len(addrs) == 0 {
		addr = ":8000"
	} else {
		addr = addrs[0]
	}

	t.logger.Info("Listen on http", addr)

	err := http.ListenAndServe(addr, t)
	if err != nil {
		t.logger.Error(err)
	}
}

func (t *Tango) RunTLS(certFile, keyFile string, addrs ...string) {
	var addr string
	if len(addrs) == 0 {
		addr = ":8000"
	} else {
		addr = addrs[0]
	}

	t.logger.Info("Listen on https", addr)

	err := http.ListenAndServeTLS(addr, certFile, keyFile, t)
	if err != nil {
		t.logger.Error(err)
	}
}

type HandlerFunc func(ctx *Context)

func (h HandlerFunc) Handle(ctx *Context) {
	h(ctx)
}

func WrapBefore(handler http.Handler) HandlerFunc {
	return func(ctx *Context) {
		handler.ServeHTTP(ctx.ResponseWriter, ctx.Req())

		ctx.Next()
	}
}

func WrapAfter(handler http.Handler) HandlerFunc {
	return func(ctx *Context) {
		ctx.Next()

		handler.ServeHTTP(ctx.ResponseWriter, ctx.Req())
	}
}

func (t *Tango) UseHandler(handler http.Handler) {
	t.Use(WrapBefore(handler))
}

func (t *Tango) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	resp := t.respPool.Get().(*responseWriter)
	resp.reset(w)

	ctx := t.ctxPool.Get().(*Context)
	ctx.reset(req, resp)

	ctx.Invoke()

	// if there is no logging or error handle, so the last written check.
	if !ctx.Written() {
		p := req.URL.Path
		if len(req.URL.RawQuery) > 0 {
			p = p + "?" + req.URL.RawQuery
		}

		if ctx.Route() != nil {
			if ctx.Result == nil {
				ctx.Write([]byte(""))
				t.logger.Info(req.Method, ctx.Status(), p)
				t.ctxPool.Put(ctx)
				t.respPool.Put(resp)
				return
			}
			panic("result should be handler before")
		}

		if ctx.Result == nil {
			ctx.Result = NotFound()
		}

		ctx.HandleError()

		t.logger.Error(req.Method, ctx.Status(), p)
	}

	t.ctxPool.Put(ctx)
	t.respPool.Put(resp)
}

func NewWithLog(logger Logger, handlers ...Handler) *Tango {
	tan := &Tango{
		Router:     NewRouter(),
		logger:     logger,
		handlers:   make([]Handler, 0),
		ErrHandler: Errors(),
	}

	tan.ctxPool.New = func() interface{} {
		return &Context{
			tan:    tan,
			Logger: tan.logger,
		}
	}

	tan.respPool.New = func() interface{} {
		return &responseWriter{}
	}

	tan.Use(handlers...)

	return tan
}

func New(handlers ...Handler) *Tango {
	return NewWithLog(NewLogger(os.Stdout), handlers...)
}

func Classic(l ...Logger) *Tango {
	var logger Logger
	if len(l) == 0 {
		logger = NewLogger(os.Stdout)
	} else {
		logger = l[0]
	}

	return NewWithLog(
		logger,
		ClassicHandlers...,
	)
}
