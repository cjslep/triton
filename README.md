# Triton

Triton is a cache-enabled static web muxer written in Go. It watches the filesystem for changes, updates its internal cache, and continues to deliver the static site. It defers doing any sort of content management to another solution.

# Installation

[Install golang](http://golang.org/doc/install).

```
go get github.com/cjslep/triton
```

[Read some documentation](http://godoc.org/github.com/cjslep/triton)

Write your custom code setting up your familiar [http.Server](http://golang.org/pkg/net/http/#Server) and mapping any assets you have to their mime-type.

# Example

[Introducing Triton](http://cjslep.com/blog/triton) (hosted on a triton-based web server)

# Contributing

Triton is licensed under the MIT Expat License. Please feel free to contribute!
