# mrouter [![Build Status](https://travis-ci.org/prasannavl/mrouter.svg?branch=master)](https://travis-ci.org/prasannavl/mrouter) [![Coverage Status](https://coveralls.io/repos/github/prasannavl/mrouter/badge.svg?branch=master)](https://coveralls.io/github/prasannavl/mrouter?branch=master) [![GoDoc](https://godoc.org/github.com/prasannavl/mrouter?status.svg)](http://godoc.org/github.com/prasannavl/mrouter)

mrouter is a lightweight high performance HTTP request router (also called *multiplexer* or just *mux* for short) for [Go](https://golang.org/), using the `mchain` style handlers.

Read about `mchain` here: https://github.com/prasannavl/mchain

**This is a fork of the excellent [httprouter](https://github.com/julienschmidt/httprouter) by [julienschmidt](https://github.com/julienschmidt) adapted to the mchain philosophy of returning errors.** Core routing works exactly the same way and features that are redundant like PanicHandler, MethodNotAllowedHandler, etc have been removed - since they are solved in an elegant manner by the mchain pattern in idiomatic Go, by returning errors. Simply just handle the errors - `ErrRedirect` to write your own redirect code, `ErrNotFound` to do custom 404 handling etc.

In contrast to the [default mux](https://golang.org/pkg/net/http/#ServeMux) of Go's `net/http` package, this router supports variables in the routing pattern and matches against the request method. It also scales better.

The router is optimized for high performance and a small memory footprint. It scales well even with very long paths and a large number of routes. A compressing dynamic trie (radix tree) structure is used for efficient matching.

## Features

**Only explicit matches:** With other routers, like [`http.ServeMux`](https://golang.org/pkg/net/http/#ServeMux), a requested URL path could match multiple patterns. Therefore they have some awkward pattern priority rules, like *longest match* or *first registered, first matched*. By design of this router, a request can only match exactly one or no route. As a result, there are also no unintended matches, which makes it great for SEO and improves the user experience.

**Stop caring about trailing slashes:** Choose the URL style you like, the router automatically redirects the client if a trailing slash is missing or if there is one extra. Of course it only does so, if the new path has a handler. If you don't like it, you can [turn off this behavior](https://godoc.org/github.com/prasannavl/mrouter#Router.RedirectTrailingSlash).

**Path auto-correction:** Besides detecting the missing or additional trailing slash at no extra cost, the router can also fix wrong cases and remove superfluous path elements (like `../` or `//`). Is [CAPTAIN CAPS LOCK](http://www.urbandictionary.com/define.php?term=Captain+Caps+Lock) one of your users? mrouter can help him by making a case-insensitive look-up and redirecting him to the correct URL.

**Parameters in your routing pattern:** Stop parsing the requested URL path, just give the path segment a name and the router delivers the dynamic value to you. Because of the design of the router, path parameters are very cheap.

**Zero Garbage:** The matching and dispatching process generates zero bytes of garbage. In fact, the only heap allocations that are made, is by building the slice of the key-value pairs for path parameters. If the request path contains no parameters, not a single heap allocation is necessary.

**Best Performance:** [Benchmarks speak for themselves](https://github.com/julienschmidt/go-http-routing-benchmark). See below for technical details of the implementation. mrouter is a direct derivative of httprouter - its simply a port to use mchain. All performance benefits of the excellent httprouter applies the same way.

**Deal with errors and panic far more elegantly:** You can simply set [RecoverPanic](https://godoc.org/github.com/prasannavl/mrouter#Router.RecoverPanic) to true, to automatically converts any panic into the error that's returned by the `mchain` handler during a HTTP request. Simply just handle them in the middleware chain, and do whatever you want with them.

**Perfect for APIs:** The router design encourages to build sensible, hierarchical RESTful APIs. Moreover it has builtin native support for [OPTIONS requests](http://zacstewart.com/2012/04/14/http-options-method.html) and `405 Method Not Allowed` replies, or simply handle the errors, and do whatever you want.

## Usage

This is just a quick introduction, view the [GoDoc](http://godoc.org/github.com/prasannavl/mrouter) for details.

A trivial example:

```go
package main

import (
    "fmt"
    "github.com/prasannavl/mrouter"
	"github.com/prasannavl/mchain/hconv"
    "net/http"
    "log"
)

func Index(w http.ResponseWriter, r *http.Request, _ mrouter.Params) error {
    fmt.Fprint(w, "Welcome!\n")
	return nil
}

func Hello(w http.ResponseWriter, r *http.Request, ps mrouter.Params) error {
    fmt.Fprintf(w, "hello, %s!\n", ps.ByName("name"))
	return nil
}

func main() {
    router := mrouter.New()
    router.Get("/", Index)
    router.Get("/hello/:name", Hello)

    log.Fatal(http.ListenAndServe(":8080", hconv.ToHttp(router, nil))
}
```

### Named parameters

As you can see, `:name` is a *named parameter*. The values are accessible via `mrouter.Params`, which is just a slice of `mrouter.Param`s. You can get the value of a parameter either by its index in the slice, or by using the `ByName(name)` method: `:name` can be retrived by `ByName("name")`.

Named parameters only match a single path segment:

```
Pattern: /user/:user

 /user/gordon              match
 /user/you                 match
 /user/gordon/profile      no match
 /user/                    no match
```

**Note:** Since this router has only explicit matches, you can not register static routes and parameters for the same path segment. For example you can not register the patterns `/user/new` and `/user/:user` for the same request method at the same time. The routing of different request methods is independent from each other.

### Catch-All parameters

The second type are *catch-all* parameters and have the form `*name`. Like the name suggests, they match everything. Therefore they must always be at the **end** of the pattern:

```
Pattern: /src/*filepath

 /src/                     match
 /src/somefile.go          match
 /src/subdir/somefile.go   match
```

## How does it work?

The router relies on a tree structure which makes heavy use of *common prefixes*, it is basically a *compact* [*prefix tree*](https://en.wikipedia.org/wiki/Trie) (or just [*Radix tree*](https://en.wikipedia.org/wiki/Radix_tree)). Nodes with a common prefix also share a common parent. Here is a short example what the routing tree for the `GET` request method could look like:

```
Priority   Path             Handle
9          \                *<1>
3          ├s               nil
2          |├earch\         *<2>
1          |└upport\        *<3>
2          ├blog\           *<4>
1          |    └:post      nil
1          |         └\     *<5>
2          ├about-us\       *<6>
1          |        └team\  *<7>
1          └contact\        *<8>
```

Every `*<num>` represents the memory address of a handler function (a pointer). If you follow a path trough the tree from the root to the leaf, you get the complete route path, e.g `\blog\:post\`, where `:post` is just a placeholder ([*parameter*](#named-parameters)) for an actual post name. Unlike hash-maps, a tree structure also allows us to use dynamic parts like the `:post` parameter, since we actually match against the routing patterns instead of just comparing hashes. [As benchmarks show](https://github.com/julienschmidt/go-http-routing-benchmark), this works very well and efficient.

Since URL paths have a hierarchical structure and make use only of a limited set of characters (byte values), it is very likely that there are a lot of common prefixes. This allows us to easily reduce the routing into ever smaller problems. Moreover the router manages a separate tree for every request method. For one thing it is more space efficient than holding a method->handle map in every single node, for another thing is also allows us to greatly reduce the routing problem before even starting the look-up in the prefix-tree.

For even better scalability, the child nodes on each tree level are ordered by priority, where the priority is just the number of handles registered in sub nodes (children, grandchildren, and so on..). This helps in two ways:

1. Nodes which are part of the most routing paths are evaluated first. This helps to make as much routes as possible to be reachable as fast as possible.
2. It is some sort of cost compensation. The longest reachable path (highest cost) can always be evaluated first. The following scheme visualizes the tree structure. Nodes are evaluated from top to bottom and from left to right.

```
├------------
├---------
├-----
├----
├--
├--
└-
```

## Related links

`mchain`: https://github.com/prasannavl/mchain  
`httprouter`: https://github.com/julienschmidt/httprouter  

