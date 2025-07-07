# html-to-markdown

[![Go Report Card](https://goreportcard.com/badge/github.com/JohannesKaufmann/html-to-markdown)](https://goreportcard.com/report/github.com/JohannesKaufmann/html-to-markdown)
[![codecov](https://codecov.io/gh/JohannesKaufmann/html-to-markdown/branch/master/graph/badge.svg)](https://codecov.io/gh/JohannesKaufmann/html-to-markdown)
![GitHub MIT License](https://img.shields.io/github/license/JohannesKaufmann/html-to-markdown)
[![GoDoc](https://godoc.org/github.com/JohannesKaufmann/html-to-markdown?status.png)](http://godoc.org/github.com/JohannesKaufmann/html-to-markdown)

![Gopher, the mascot of Golang, is wearing a party hat and holding a balloon. Next to the Gopher is a machine that converts characters associated with HTML to characters associated with Markdown.](/logo_five_years.png)

Convert HTML into Markdown with Go. It is using an [HTML Parser](https://github.com/PuerkitoBio/goquery) to avoid the use of `regexp` as much as possible. That should prevent some [weird cases](https://stackoverflow.com/a/1732454) and allows it to be used for cases where the input is totally unknown.

## Installation

```
go get github.com/JohannesKaufmann/html-to-markdown
```

## Usage

```go
import (
	"fmt"
	"log"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

converter := md.NewConverter("", true, nil)

html := `<strong>Important</strong>`

markdown, err := converter.ConvertString(html)
if err != nil {
  log.Fatal(err)
}
fmt.Println("md ->", markdown)
```

If you are already using [goquery](https://github.com/PuerkitoBio/goquery) you can pass a selection to `Convert`.

```go
markdown, err := converter.Convert(selec)
```

### Using it on the command line

If you want to make use of `html-to-markdown` on the command line without any Go coding, check out [`html2md`](https://github.com/suntong/html2md#usage), a cli wrapper for `html-to-markdown` that has all the following options and plugins builtin.

## Options

The third parameter to `md.NewConverter` is `*md.Options`.

For example you can change the character that is around a bold text ("`**`") to a different one (for example "`__`") by changing the value of `StrongDelimiter`.

```go
opt := &md.Options{
  StrongDelimiter: "__", // default: **
  // ...
}
converter := md.NewConverter("", true, opt)
```

For all the possible options look at [godocs](https://godoc.org/github.com/JohannesKaufmann/html-to-markdown/#Options) and for a example look at the [example](/examples/options/main.go).

## Adding Rules

```go
converter.AddRules(
  md.Rule{
    Filter: []string{"del", "s", "strike"},
    Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
      // You need to return a pointer to a string (md.String is just a helper function).
      // If you return nil the next function for that html element
      // will be picked. For example you could only convert an element
      // if it has a certain class name and fallback if not.
      content = strings.TrimSpace(content)
      return md.String("~" + content + "~")
    },
  },
  // more rules
)
```

For more information have a look at the example [add_rules](/examples/add_rules/main.go).

## Using Plugins

If you want plugins (github flavored markdown like striketrough, tables, ...) you can pass it to `Use`.

```go
import "github.com/JohannesKaufmann/html-to-markdown/plugin"

// Use the `GitHubFlavored` plugin from the `plugin` package.
converter.Use(plugin.GitHubFlavored())
```

Or if you only want to use the `Strikethrough` plugin. You can change the character that distinguishes
the text that is crossed out by setting the first argument to a different value (for example "~~" instead of "~").

```go
converter.Use(plugin.Strikethrough(""))
```

For more information have a look at the example [github_flavored](/examples/github_flavored/main.go).

---

These are the plugins located in the [plugin folder](/plugin) which you can use by importing "github.com/JohannesKaufmann/html-to-markdown/plugin".

| Name                  | Description                                                                                 |
| --------------------- | ------------------------------------------------------------------------------------------- |
| GitHubFlavored        | GitHub's Flavored Markdown contains `TaskListItems`, `Strikethrough` and `Table`.           |
| TaskListItems         | (Included in `GitHubFlavored`). Converts `<input>` checkboxes into `- [x] Task`.            |
| Strikethrough         | (Included in `GitHubFlavored`). Converts `<strike>`, `<s>`, and `<del>` to the `~~` syntax. |
| Table                 | (Included in `GitHubFlavored`). Convert a `<table>` into something like this...             |
| TableCompat           |                                                                                             |
|                       |                                                                                             |
| VimeoEmbed            |                                                                                             |
| YoutubeEmbed          |                                                                                             |
|                       |                                                                                             |
| ConfluenceCodeBlock   | Converts `<ac:structured-macro>` elements that are used in Atlassian’s Wiki "Confluence".   |
| ConfluenceAttachments | Converts `<ri:attachment ri:filename=""/>` elements.                                        |

These are the plugins in other repositories:

| Name                         | Description         |
| ---------------------------- | ------------------- |
| \[Plugin Name\]\(Your Link\) | A short description |

I you write a plugin, feel free to open a PR that adds your Plugin to this list.

## Writing Plugins

Have a look at the [plugin folder](/plugin) for a reference implementation. The most basic one is [Strikethrough](/plugin/strikethrough.go).

## Security

This library produces markdown that is readable and can be changed by humans.

Once you convert this markdown back to HTML (e.g. using [goldmark](https://github.com/yuin/goldmark) or [blackfriday](https://github.com/russross/blackfriday)) you need to be careful of malicious content.

This library does NOT sanitize untrusted content. Use an HTML sanitizer such as [bluemonday](https://github.com/microcosm-cc/bluemonday) before displaying the HTML in the browser.

## Other Methods

[Godoc](https://godoc.org/github.com/JohannesKaufmann/html-to-markdown)

### `func (c *Converter) Keep(tags ...string) *Converter`

Determines which elements are to be kept and rendered as HTML.

### `func (c *Converter) Remove(tags ...string) *Converter`

Determines which elements are to be removed altogether i.e. converted to an empty string.

## Escaping

Some characters have a special meaning in markdown. For example, the character "\*" can be used for lists, emphasis and dividers. By placing a backlash before that character (e.g. "\\\*") you can "escape" it. Then the character will render as a raw "\*" without the _"markdown meaning"_ applied.

But why is "escaping" even necessary?

<!-- prettier-ignore -->
```md
Paragraph 1
-
Paragraph 2
```

The markdown above doesn't seem that problematic. But "Paragraph 1" (with only one hyphen below) will be recognized as a _setext heading_.

```html
<h2>Paragraph 1</h2>
<p>Paragraph 2</p>
```

A well-placed backslash character would prevent that...

<!-- prettier-ignore -->
```md
Paragraph 1
\-
Paragraph 2
```

---

How to configure escaping? Depending on the `EscapeMode` option, the markdown output is going to be different.

```go
opt = &md.Options{
	EscapeMode: "basic", // default
}
```

Lets try it out with this HTML input:

|          |                                                       |
| -------- | ----------------------------------------------------- |
| input    | `<p>fake **bold** and real <strong>bold</strong></p>` |
|          |                                                       |
|          | **With EscapeMode "basic"**                           |
| output   | `fake \*\*bold\*\* and real **bold**`                 |
| rendered | fake \*\*bold\*\* and real **bold**                   |
|          |                                                       |
|          | **With EscapeMode "disabled"**                        |
| output   | `fake **bold** and real **bold**`                     |
| rendered | fake **bold** and real **bold**                       |

With **basic** escaping, we get some escape characters (the backlash "\\") but it renders correctly.

With escaping **disabled**, the fake and real bold can't be distinguished in the markdown. That means it is both going to render as bold.

---

So now you know the purpose of escaping. However, if you encounter some content where the escaping breaks, you can manually disable it. But please also open an issue!

## Issues

If you find HTML snippets (or even full websites) that don't produce the expected results, please open an issue!

## Contributing & Testing

Please first discuss the change you wish to make, by opening an issue. I'm also happy to guide you to where a change is most likely needed.

_Note: The outside API should not change because of backwards compatibility..._

You don't have to be afraid of breaking the converter, since there are many "Golden File Tests":

Add your problematic HTML snippet to one of the `input.html` files in the `testdata` folder. Then run `go test -update` and have a look at which `.golden` files changed in GIT.

You can now change the internal logic and inspect what impact your change has by running `go test -update` again.

_Note: Before submitting your change as a PR, make sure that you run those tests and check the files into GIT..._

## Related Projects

- [turndown (js)](https://github.com/domchristie/turndown), a very good library written in javascript.
- [lunny/html2md](https://github.com/lunny/html2md), which is using [regex instead of goquery](https://stackoverflow.com/a/1732454). I came around a few edge case when using it (leaving some html comments, ...) so I wrote my own.

## License

This project is licensed under the terms of the MIT license.
