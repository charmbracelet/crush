# Glamour Style Guide

The JSON files in this directory are generated from the default styles. To
re-generate them, run:

    go generate ..

## Block Elements

Block elements contain other elements and are rendered around them. All block
elements support the following style settings:

| Attribute        | Value  | Description                                                  |
| ---------------- | ------ | ------------------------------------------------------------ |
| block_prefix     | string | Printed before the block's first element (in parent's style) |
| block_suffix     | string | Printed after the block's last element (in parent's style)   |
| prefix           | string | Printed before the block's first element                     |
| suffix           | string | Printed after the block's last element                       |
| indent           | number | Specifies the indentation of the block                       |
| indent_token     | string | Specifies the indentation format                             |
| margin           | number | Specifies the margin around the block                        |
| color            | color  | Defines the default text color for the block                 |
| background_color | color  | Defines the default background color for the block           |

Elements inside a block inherit the block's following style settings:

| Attribute        | Value | Description                                        |
| ---------------- | ----- | -------------------------------------------------- |
| color            | color | Defines the default text color for the block       |
| background_color | color | Defines the default background color for the block |
| bold             | bool  | Increases text intensity                           |
| faint            | bool  | Decreases text intensity                           |
| italic           | bool  | Prints the text in italic                          |
| crossed_out      | bool  | Enables strikethrough as text decoration           |
| underline        | bool  | Enables underline as text decoration               |
| overlined        | bool  | Enables overline as text decoration                |
| blink            | bool  | Enables blinking text                              |
| conceal          | bool  | Conceals / hides the text                          |
| inverse          | bool  | Swaps fore- & background colors                    |

### document

The `document` element represents the markdown's body.

#### Example

Style:

```json
"document": {
    "indent": 2,
    "background_color": "234",
    "block_prefix": "\n",
    "block_suffix": "\n"
}
```

---

### paragraph

The `paragraph` element represents a paragraph in the document.

#### Example

Style:

```json
"paragraph": {
    "margin": 4,
    "color": "15",
    "background_color": "235"
}
```

---

### heading

The `heading` element represents a heading.

### h1 - h6

The `h1` to `h6` elements represent headings. `h1` defines the most important
heading, `h6` the least important heading. Undefined attributes are inherited
from the `heading` element.

#### Example

Markdown:

```markdown
# h1

## h2

### h3
```

Style:

```json
"heading": {
    "color": "15",
    "background_color": "57"
},
"h1": {
    "prefix": "=> ",
    "suffix": " <=",
    "margin": 2,
    "bold": true,
    "background_color": "69"
},
"h2": {
    "prefix": "## ",
    "margin": 4
},
"h3": {
    "prefix": "### ",
    "margin": 6
}
```

Output:

![Heading Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/heading.png)

---

### block_quote

The `block_quote` element represents a quote.

#### Example

Style:

```json
"block_quote": {
    "color": "200",
    "indent": 1,
    "indent_token": "=> "
}
```

Output:

![Block Quote Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/block_quote.png)

---

### list

The `list` element represents a list in the document.

| Attribute    | Value  | Description                                |
| ------------ | ------ | ------------------------------------------ |
| level_indent | number | Specifies the indentation for nested lists |

#### Example

Style:

```json
"list": {
    "color": "15",
    "background_color": "52",
    "level_indent": 4
}
```

---

### code_block

The `code_block` element represents a block of code.

| Attribute | Value  | Description                                                     |
| --------- | ------ | --------------------------------------------------------------- |
| theme     | string | Defines the [Chroma][chroma] theme used for syntax highlighting |

[chroma]: https://github.com/alecthomas/chroma

#### Example

Style:

```json
"code_block": {
    "color": "200",
    "theme": "solarized-dark"
}
```

Output:

![Code Block Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/code_block.png)

---

### table

The `table` element represents a table of data.

#### Example

Markdown:

```markdown
| Label  | Value |
| ------ | ----- |
| First  | foo   |
| Second | bar   |
```

Style:

```json
"table": {
    "margin": 4
}
```

Output:

![Table Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/table.png)

## Inline Elements

All inline elements support the following style settings:

| Attribute        | Value  | Description                                           |
| ---------------- | ------ | ----------------------------------------------------- |
| block_prefix     | string | Printed before the element (in parent's style)        |
| block_suffix     | string | Printed after the element (in parent's style)         |
| prefix           | string | Printed before the element                            |
| suffix           | string | Printed after the element                             |
| color            | color  | Defines the default text color for the document       |
| background_color | color  | Defines the default background color for the document |
| bold             | bool   | Increases text intensity                              |
| faint            | bool   | Decreases text intensity                              |
| italic           | bool   | Prints the text in italic                             |
| crossed_out      | bool   | Enables strikethrough as text decoration              |
| underline        | bool   | Enables underline as text decoration                  |
| overlined        | bool   | Enables overline as text decoration                   |
| blink            | bool   | Enables blinking text                                 |
| conceal          | bool   | Conceals / hides the text                             |
| inverse          | bool   | Swaps fore- & background colors                       |

### text

The `text` element represents a block of text.

#### Example

Style:

```json
"text": {
    "bold": true,
    "color": "15",
    "background_color": "57"
}
```

---

### item

The `item` element represents an item in a list.

#### Example

Markdown:

```markdown
- First Item
    - Nested List Item
- Second Item
```

Style:

```json
"item": {
    "block_prefix": "• "
}
```

Output:

![List Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/list.png)

---

### enumeration

The `enumeration` element represents an item in an ordered list.

#### Example

Markdown:

```markdown
1. First Item
2. Second Item
```

Style:

```json
"enumeration": {
    "block_prefix": ". "
}
```

Output:

![Enumeration Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/enumeration.png)

---

### task

The `task` element represents a task item.

| Attribute | Value  | Description                 |
| --------- | ------ | --------------------------- |
| ticked    | string | Prefix for finished tasks   |
| unticked  | string | Prefix for unfinished tasks |

#### Example

Markdown:

```markdown
- [x] Finished Task
- [ ] Outstanding Task
```

Style:

```json
"task": {
    "ticked": "✓ ",
    "unticked": "✗ "
}
```

Output:

![Task Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/task.png)

---

### link

The `link` element represents a link.

#### Example

Markdown:

```markdown
This is a [link](https://charm.sh).
```

Style:

```json
"link": {
    "color": "123",
    "underline": true,
    "block_prefix": "(",
    "block_suffix": ")"
}
```

Output:

![Link Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/link.png)

---

### link_text

The `link_text` element represents the text associated with a link.

#### Example

Style:

```json
"link_text": {
    "color": "123",
    "bold": true
}
```

---

### image

The `image` element represents an image.

#### Example

Markdown:

```markdown
![Image](https://charm.sh/logo.png).
```

Style:

```json
"image": {
    "color": "123",
    "block_prefix": "[Image: ",
    "block_suffix": "]"
}
```

Output:

![Image Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/image.png)

---

### image_text

The `image_text` element represents the text associated with an image.

#### Example

Style:

```json
"image_text": {
    "color": "8"
}
```

---

### code

The `code` element represents an inline code segment.

#### Example

Style:

```json
"code": {
    "color": "200"
}
```

Output:

![Code Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/code.png)

---

### emph

The `emph` element represents an emphasized text.

#### Example

Markdown:

```markdown
This text is *emphasized*.
```

Style:

```json
"emph": {
    "italic": true
}
```

Output:

![Emph Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/emph.png)

---

### strong

The `strong` element represents important text.

#### Example

Markdown:

```markdown
This text is **strong**.
```

Style:

```json
"strong": {
    "bold": true
}
```

Output:

![Strong Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/strong.png)

---

### strikethrough

The `strikethrough` element represents strikethrough text.

#### Example

Markdown:

```markdown
~~Scratch this~~.
```

Style:

```json
"strikethrough": {
    "crossed_out": true
}
```

Output:

![Strikethrough Example](https://github.com/charmbracelet/glamour/raw/master/styles/examples/strikethrough.png)

---

### hr

The `hr` element represents a horizontal rule.

#### Example

Markdown:

```markdown
---
```

Style:

```json
"hr": {
    "block_prefix": "---"
}
```

## html_block
## html_span
