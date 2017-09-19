// Copyright 2013 Julien Schmidt. 
// Copyright 2017 Prasanna V. Loganathar.
// All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package mrouter

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/prasannavl/goerror/httperror"
	"github.com/prasannavl/mchain"
)

type mockResponseWriter struct{}

func (m *mockResponseWriter) Header() (h http.Header) {
	return http.Header{}
}

func (m *mockResponseWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (m *mockResponseWriter) WriteString(s string) (n int, err error) {
	return len(s), nil
}

func (m *mockResponseWriter) WriteHeader(int) {}

func TestParams(t *testing.T) {
	ps := Params{
		Param{"param1", "value1"},
		Param{"param2", "value2"},
		Param{"param3", "value3"},
	}
	for i := range ps {
		if val := ps.ByName(ps[i].Key); val != ps[i].Value {
			t.Errorf("Wrong value for %s: Got %s; Want %s", ps[i].Key, val, ps[i].Value)
		}
	}
	if val := ps.ByName("noKey"); val != "" {
		t.Errorf("Expected empty string for not found key; got: %s", val)
	}
}

func TestRouter(t *testing.T) {
	router := New()

	routed := false
	router.Handle("GET", "/user/:name", func(w http.ResponseWriter, r *http.Request, ps Params) error {
		routed = true
		want := Params{Param{"name", "gopher"}}
		if !reflect.DeepEqual(ps, want) {
			t.Fatalf("wrong wildcard values: want %v, got %v", want, ps)
		}
		return nil
	})

	w := new(mockResponseWriter)

	req, _ := http.NewRequest("GET", "/user/gopher", nil)
	router.ServeHTTP(w, req)

	if !routed {
		t.Fatal("routing failed")
	}
}

type handlerStruct struct {
	handled *bool
}

func (h handlerStruct) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	*h.handled = true
	return nil
}

func TestRouterAPI(t *testing.T) {
	var get, head, options, post, put, patch, delete, handler, handlerFunc bool

	httpHandler := handlerStruct{&handler}

	router := New()
	router.Get("/GET", func(w http.ResponseWriter, r *http.Request, _ Params) error {
		get = true
		return nil
	})
	router.Head("/GET", func(w http.ResponseWriter, r *http.Request, _ Params) error {
		head = true
		return nil

	})
	router.Options("/GET", func(w http.ResponseWriter, r *http.Request, _ Params) error {
		options = true
		return nil
	})
	router.Post("/POST", func(w http.ResponseWriter, r *http.Request, _ Params) error {
		post = true
		return nil
	})
	router.Put("/PUT", func(w http.ResponseWriter, r *http.Request, _ Params) error {
		put = true
		return nil
	})
	router.Patch("/PATCH", func(w http.ResponseWriter, r *http.Request, _ Params) error {
		patch = true
		return nil
	})
	router.Delete("/DELETE", func(w http.ResponseWriter, r *http.Request, _ Params) error {
		delete = true
		return nil
	})
	router.Handler("GET", "/Handler", httpHandler)
	router.HandlerFunc("GET", "/HandlerFunc", func(w http.ResponseWriter, r *http.Request) error {
		handlerFunc = true
		return nil
	})

	w := new(mockResponseWriter)

	r, _ := http.NewRequest("GET", "/GET", nil)
	router.ServeHTTP(w, r)
	if !get {
		t.Error("routing GET failed")
	}

	r, _ = http.NewRequest("HEAD", "/GET", nil)
	router.ServeHTTP(w, r)
	if !head {
		t.Error("routing HEAD failed")
	}

	r, _ = http.NewRequest("OPTIONS", "/GET", nil)
	router.ServeHTTP(w, r)
	if !options {
		t.Error("routing OPTIONS failed")
	}

	r, _ = http.NewRequest("POST", "/POST", nil)
	router.ServeHTTP(w, r)
	if !post {
		t.Error("routing POST failed")
	}

	r, _ = http.NewRequest("PUT", "/PUT", nil)
	router.ServeHTTP(w, r)
	if !put {
		t.Error("routing PUT failed")
	}

	r, _ = http.NewRequest("PATCH", "/PATCH", nil)
	router.ServeHTTP(w, r)
	if !patch {
		t.Error("routing PATCH failed")
	}

	r, _ = http.NewRequest("DELETE", "/DELETE", nil)
	router.ServeHTTP(w, r)
	if !delete {
		t.Error("routing DELETE failed")
	}

	r, _ = http.NewRequest("GET", "/Handler", nil)
	router.ServeHTTP(w, r)
	if !handler {
		t.Error("routing Handler failed")
	}

	r, _ = http.NewRequest("GET", "/HandlerFunc", nil)
	router.ServeHTTP(w, r)
	if !handlerFunc {
		t.Error("routing HandlerFunc failed")
	}
}

func TestRouterRoot(t *testing.T) {
	router := New()
	recv := catchPanic(func() {
		router.Get("noSlashRoot", nil)
	})
	if recv == nil {
		t.Fatal("registering path not beginning with '/' did not panic")
	}
}

func TestRouterChaining(t *testing.T) {
	router1 := New()
	router2 := New()
	router1.NotFound = router2

	fooHit := false
	router1.Post("/foo", func(w http.ResponseWriter, req *http.Request, _ Params) error {
		fooHit = true
		w.WriteHeader(http.StatusOK)
		return nil
	})

	barHit := false
	router2.Post("/bar", func(w http.ResponseWriter, req *http.Request, _ Params) error {
		barHit = true
		w.WriteHeader(http.StatusOK)
		return nil
	})

	r, _ := http.NewRequest("POST", "/foo", nil)
	w := httptest.NewRecorder()
	router1.ServeHTTP(w, r)
	if !(w.Code == http.StatusOK && fooHit) {
		t.Errorf("Regular routing failed with router chaining.")
		t.FailNow()
	}

	r, _ = http.NewRequest("POST", "/bar", nil)
	w = httptest.NewRecorder()
	router1.ServeHTTP(w, r)
	if !(w.Code == http.StatusOK && barHit) {
		t.Errorf("Chained routing failed with router chaining.")
		t.FailNow()
	}

	r, _ = http.NewRequest("POST", "/qax", nil)
	w = httptest.NewRecorder()
	e := router1.ServeHTTP(w, r).(httperror.HttpError)
	if !(e.Code() == http.StatusNotFound) {
		t.Errorf("NotFound behavior failed with router chaining.")
		t.FailNow()
	}
}

func TestRouterOPTIONS(t *testing.T) {
	handlerFunc := func(_ http.ResponseWriter, _ *http.Request, _ Params) error { return nil }

	router := New()
	router.Post("/path", handlerFunc)

	// test not allowed
	// * (server)
	r, _ := http.NewRequest("OPTIONS", "*", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", w.Code, w.Header())
	} else if allow := w.Header().Get("Allow"); allow != "POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// path
	r, _ = http.NewRequest("OPTIONS", "/path", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", w.Code, w.Header())
	} else if allow := w.Header().Get("Allow"); allow != "POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	r, _ = http.NewRequest("OPTIONS", "/doesnotexist", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	e := router.ServeHTTP(w, r).(httperror.HttpError)
	if !(e.Code() == http.StatusNotFound) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", w.Code, w.Header())
	}

	// add another method
	router.Get("/path", handlerFunc)

	// test again
	// * (server)
	r, _ = http.NewRequest("OPTIONS", "*", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", w.Code, w.Header())
	} else if allow := w.Header().Get("Allow"); allow != "POST, GET, OPTIONS" && allow != "GET, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// path
	r, _ = http.NewRequest("OPTIONS", "/path", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", w.Code, w.Header())
	} else if allow := w.Header().Get("Allow"); allow != "POST, GET, OPTIONS" && allow != "GET, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// custom handler
	var custom bool
	router.Options("/path", func(w http.ResponseWriter, r *http.Request, _ Params) error {
		custom = true
		return nil
	})

	// test again
	// * (server)
	r, _ = http.NewRequest("OPTIONS", "*", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", w.Code, w.Header())
	} else if allow := w.Header().Get("Allow"); allow != "POST, GET, OPTIONS" && allow != "GET, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}
	if custom {
		t.Error("custom handler called on *")
	}

	// path
	r, _ = http.NewRequest("OPTIONS", "/path", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == http.StatusOK) {
		t.Errorf("OPTIONS handling failed: Code=%d, Header=%v", w.Code, w.Header())
	}
	if !custom {
		t.Error("custom handler not called")
	}
}

func TestRouterNotAllowed(t *testing.T) {
	handlerFunc := func(_ http.ResponseWriter, _ *http.Request, _ Params) error {
		return nil
	}

	router := New()
	router.Post("/path", handlerFunc)

	// test not allowed
	r, _ := http.NewRequest("GET", "/path", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == http.StatusMethodNotAllowed) {
		t.Errorf("NotAllowed handling failed: Code=%d, Header=%v", w.Code, w.Header())
	} else if allow := w.Header().Get("Allow"); allow != "POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}

	// add another method
	router.Delete("/path", handlerFunc)
	router.Options("/path", handlerFunc) // must be ignored

	// test again
	r, _ = http.NewRequest("GET", "/path", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == http.StatusMethodNotAllowed) {
		t.Errorf("NotAllowed handling failed: Code=%d, Header=%v", w.Code, w.Header())
	} else if allow := w.Header().Get("Allow"); allow != "POST, DELETE, OPTIONS" && allow != "DELETE, POST, OPTIONS" {
		t.Error("unexpected Allow header value: " + allow)
	}
}

func TestRouterNotFound(t *testing.T) {
	handlerFunc := func(_ http.ResponseWriter, _ *http.Request, _ Params) error { return nil }

	router := New()
	router.Get("/path", handlerFunc)
	router.Get("/dir/", handlerFunc)
	router.Get("/", handlerFunc)

	testRoutes := []struct {
		route  string
		code   int
		header string
	}{
		{"/path/", 308, "map[Location:[/path]]"},   // TSR -/
		{"/dir", 308, "map[Location:[/dir/]]"},     // TSR +/
		{"", 308, "map[Location:[/]]"},             // TSR +/
		{"/PATH", 308, "map[Location:[/path]]"},    // Fixed Case
		{"/DIR/", 308, "map[Location:[/dir/]]"},    // Fixed Case
		{"/PATH/", 308, "map[Location:[/path]]"},   // Fixed Case -/
		{"/DIR", 308, "map[Location:[/dir/]]"},     // Fixed Case +/
		{"/../path", 308, "map[Location:[/path]]"}, // CleanPath
		{"/nope", 404, ""},                         // NotFound
	}
	for _, tr := range testRoutes {
		r, _ := http.NewRequest("GET", tr.route, nil)
		w := httptest.NewRecorder()
		if e, ok := router.ServeHTTP(w, r).(httperror.HttpError); ok {
			if !(e.Code() == tr.code && (e.Code() == 404 || fmt.Sprint(w.Header()) == tr.header)) {
				t.Errorf("NotFound handling route %s failed: Code=%d, Header=%v", tr.route, w.Code, w.Header())
			}
		}
	}

	// Test custom not found handler
	var notFound bool
	router.NotFound = mchain.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) error {
		rw.WriteHeader(404)
		notFound = true
		return nil
	})
	r, _ := http.NewRequest("GET", "/nope", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == 404 && notFound == true) {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", w.Code, w.Header())
	}

	// Test other method than GET (want 307 instead of 301)
	router.Patch("/path", handlerFunc)
	r, _ = http.NewRequest("PATCH", "/path/", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	if !(w.Code == 308 && fmt.Sprint(w.Header()) == "map[Location:[/path]]") {
		t.Errorf("Custom NotFound handler failed: Code=%d, Header=%v", w.Code, w.Header())
	}

	// Test special case where no node for the prefix "/" exists
	router = New()
	router.Get("/a", handlerFunc)
	r, _ = http.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	e := router.ServeHTTP(w, r).(httperror.HttpError)
	if !(e.Code() == 404) {
		t.Errorf("NotFound handling route / failed: Code=%d", w.Code)
	}
}

func TestRouterPanicHandler(t *testing.T) {
	router := New()
	panicHandled := false

	router.RecoverPanic = true

	router.Handle("PUT", "/user/:name", func(_ http.ResponseWriter, _ *http.Request, _ Params) error {
		panic("oops!")
	})

	w := new(mockResponseWriter)
	req, _ := http.NewRequest("PUT", "/user/gopher", nil)

	defer func() {
		if rcv := recover(); rcv != nil {
			t.Fatal("handling panic failed")
		}
	}()

	err := router.ServeHTTP(w, req)
	if err != nil {
		panicHandled = true
	}

	if !panicHandled {
		t.Fatal("simulating failed")
	}
}

func TestRouterLookup(t *testing.T) {
	routed := false
	wantHandle := func(_ http.ResponseWriter, _ *http.Request, _ Params) error {
		routed = true
		return nil
	}
	wantParams := Params{Param{"name", "gopher"}}

	router := New()

	// try empty router first
	handle, _, tsr := router.Lookup("GET", "/nope")
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}

	// insert route and try again
	router.Get("/user/:name", wantHandle)

	handle, params, tsr := router.Lookup("GET", "/user/gopher")
	if handle == nil {
		t.Fatal("Got no handle!")
	} else {
		handle(nil, nil, nil)
		if !routed {
			t.Fatal("Routing failed!")
		}
	}

	if !reflect.DeepEqual(params, wantParams) {
		t.Fatalf("Wrong parameter values: want %v, got %v", wantParams, params)
	}

	handle, _, tsr = router.Lookup("GET", "/user/gopher/")
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if !tsr {
		t.Error("Got no TSR recommendation!")
	}

	handle, _, tsr = router.Lookup("GET", "/nope")
	if handle != nil {
		t.Fatalf("Got handle for unregistered pattern: %v", handle)
	}
	if tsr {
		t.Error("Got wrong TSR recommendation!")
	}
}

type mockFileSystem struct {
	opened bool
}

func (mfs *mockFileSystem) Open(name string) (http.File, error) {
	mfs.opened = true
	return nil, errors.New("this is just a mock")
}
