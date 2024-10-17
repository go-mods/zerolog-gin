# zerolog-gin

[![Go Reference](https://pkg.go.dev/badge/github.com/go-mods/zerolog-gin.svg)](https://pkg.go.dev/github.com/go-mods/zerolog-gin)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-mods/zerolog-gin)](https://goreportcard.com/report/github.com/go-mods/zerolog-gin)
[![Release](https://img.shields.io/github/release/go-mods/zerolog-gin.svg?style=flat)](https://github.com/go-mods/zerolog-gin/releases)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/go-mods/zerolog-gin/blob/master/LICENSE.md)

`zerolog-gin` is a middleware for the Gin web framework that integrates the `zerolog` logger, providing a fast and efficient logging solution for your web applications. Designed to be lightweight and performant, `zerolog-gin` captures and structures logs optimally, making debugging and performance analysis easier. With its compatibility with the advanced features of `zerolog`, it allows for customizing logs according to the specific needs of your application while maintaining a minimal memory footprint.

## Installation

To install the package, use the following command:

```bash
go get github.com/go-mods/zerolog-gin
```

## Usage

Here is an example of how to use `zerolog-gin` with a Gin application:

```go
package main

import (
    "bytes"
    "fmt"
    "github.com/gin-gonic/gin"
    zerologgin "github.com/go-mods/zerolog-gin"
    "github.com/rs/zerolog"
    "io"
    "net/http"
    "os"
)

func main() {
    // logger to use with gin
    logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

    // Create an instance of gin router
    r := gin.New()
    r.SetTrustedProxies([]string{"::1"})

    // Add zerolog-gin as a middleware
    r.Use(zerologgin.LoggerWithOptions(&zerologgin.Options{Name: "server", Logger: &logger}))

    r.GET("/", func(c *gin.Context) {
        c.String(http.StatusOK, "hello, zerolog-gin example")
    })

    r.GET("/ping", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"message": "pong"})
    })

    r.POST("/echo", func(c *gin.Context) {
        body, err := io.ReadAll(c.Request.Body)
        c.Request.Body = io.NopCloser(bytes.NewReader(body))
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "undefined"})
        } else {
            c.String(http.StatusOK, string(body))
        }
    })

    if err := r.Run(":8080"); err != nil {
        fmt.Println(err.Error())
    }
}
```

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.
