# mango-pflag

[![Latest Release](https://img.shields.io/github/release/muesli/mango-pflag.svg)](https://github.com/muesli/mango-pflag/releases)
[![Build Status](https://github.com/muesli/mango-pflag/workflows/build/badge.svg)](https://github.com/muesli/mango-pflag/actions)
[![Go ReportCard](https://goreportcard.com/badge/muesli/mango-pflag)](https://goreportcard.com/report/muesli/mango-pflag)
[![GoDoc](https://godoc.org/github.com/golang/gddo?status.svg)](https://pkg.go.dev/github.com/muesli/mango-pflag)

pflag adapter for [mango](https://github.com/muesli/mango).

## Example

```go
import (
    "fmt"

    "github.com/muesli/mango"
    mpflag "github.com/muesli/mango-pflag"
    "github.com/muesli/roff"
    flag "github.com/spf13/pflag"
)

func main() {
    flag.Parse()

    manPage := mango.NewManPage(1, "mango", "a man-page generator").
        WithLongDescription("mango is a man-page generator for Go.").
        WithSection("Copyright", "(C) 2022 Christian Muehlhaeuser.\n"+
            "Released under MIT license.")

    flag.VisitAll(mpflag.PFlagVisitor(manPage))
    fmt.Println(manPage.Build(roff.NewDocument()))
}
```
