# Phetour

A minimal static-site generator written in Go. It reads plain-text post files written in a lightweight custom syntax, compiles them into an intermediate XML document tree, and transforms that tree into any number of output formats (HTML, Gemtext, or anything else) using XSLT stylesheets.

---

## How it works

```
input/   →   Parse   →   Render   →   XSL Transform   →   output/
```

1. **Parse** — each post file is read and parsed into a `<document>` XML element with `<meta>` (title + tags) and `<body>` (content blocks).
2. **Render** — a separate `<document>` XML file is written for each post, each tag index, and the home catalog.
3. **Transform** — every `.xsl` stylesheet in `input/styles/` is applied to every XML file in `output/xml/`, producing a parallel output directory named after the stylesheet (e.g. `html.xsl` → `output/html/`).
4. **Lock** — post and tag identities are stored in `lock.xml` so that URLs remain stable across rebuilds even when filenames change.

---

## Project structure

```
.
├── input/
│   ├── posts/          # post source files (see syntax below)
│   ├── statics/        # files copied verbatim into every output directory
│   └── styles/         # XSLT stylesheets, one per output format
├── output/             # generated — do not edit by hand
│   ├── xml/            # intermediate XML (one folder per document)
│   └── .../            # produced by given XSLT stylesheets
├── source/             # Go source code
├── lock.xml            # stable ID registry — commit this file
└── makefile
```

---

## Setup

### Prerequisites

| Tool | Purpose |
|---|---|
| [Go](https://go.dev/) 1.23+ | build the generator |
| [xsltproc](http://xmlsoft.org/XSLT/) | apply stylesheets (Linux/macOS) |
| [msxsl.exe](https://www.microsoft.com/en-us/download/details.aspx?id=21714) | apply stylesheets (Windows) |
| [pandoc](https://pandoc.org/) | render Markdown tables inside ` ``` ` blocks (optional) |


### Build

```sh
# generate the site
make build

# or directly
go run ./source
```

Output lands in `output/`.

---

## Writing posts

Post files live in `input/posts/`. The filename is the post's permanent identity key — the title displayed to readers comes from the file content, not the filename.

### Filenames

Post files use plain names, `.md` extension optional. Prefix the filename with `~` to mark it as a draft — draft files are skipped during build and can be left in the folder safely.

| Convention | Meaning |
|---|---|
| `my_post.md` | **published** — included in the build |
| `~my_post.md` | **draft** — skipped during build |

The filename is the post's permanent identity key stored in `lock.xml`. But the title that readers see comes from the file content, not the filename.

### Syntax

A post file has two sections separated implicitly by the parser: a **header** at the top, and **content** below.

#### Header

```
# Title of the post

> first tag
> second tag
> third tag
```

- The **first line starting with `#`** (anywhere in the file, leading blank lines are ignored) is the title. Everything after the `#` and its trailing space is taken as the title string.
- Every **line starting with `>`** immediately following the title (blank lines between them are ignored) is treated as a single tag. The entire string after `>` becomes the tag label.
- The header ends as soon as any other non-empty, non-`>` line is encountered. From that point on, everything is content.

#### Content blocks

| Syntax | Intermediate XML element | Notes |
|---|---|---|
| `# Section heading` | `<bold>` | rendered by the stylesheet |
| `- List item` | `<item>` | consecutive items form one list |
| `> url label` | `<link href="url">` | first word is the href, rest is label |
| Plain paragraph text | `<text>` | consecutive lines form one block |
| ` ``` … ``` ` | `<code>` | processed by pandoc if available |

Consecutive plain-text lines are collected into a single `<text>` block. A blank line or any special prefix line breaks the collection.

> **Note on the `>` sigil:** In the header it means *tag*. In the content body it means *link*, but only when followed by a space (`> url label`). The parser switches modes after the first non-`>` content line, so the two uses are always unambiguous.

#### Tables (via pandoc)

Markdown-style tables inside a ` ``` ` block are processed by `pandoc`:

````
```
|   | _1_ | _2_ |
|---|:---:|:---:|
| A | foo | bar |
```
````

If `pandoc` is not installed the raw content is preserved as a plain `<code>` block.

### Example

File: `on_reading.md`

```
# On Reading

> essays
> books

Reading is one of the few activities that slows time down.
A good book makes an afternoon feel like a week.

- it builds vocabulary without deliberate effort
- it trains sustained attention
- it exposes you to ways of thinking you would not reach alone

# Where to start

Start anywhere. Curiosity is a better guide than a syllabus.

> /0x0002/ post on essays
```

This produces:

- **title**: `On Reading`
- **tags**: `essays`, `books`
- **body**: one paragraph (two lines joined), a three-item list, a section heading, one link

---

## Adding a stylesheet

1. Create `input/styles/myformat.xsl` — an XSLT 1.0 stylesheet that transforms `/document`.
2. Run `make build`.
3. Find the output in `output/myformat/`.

A common approach is to write one stylesheet per target format. For example, `html.xsl` could render each `<bold>` as `<strong>`, each `<item>` as an `<li>` inside a grouped `<ul>`, each `<link>` as an `<a href>`, and each `<text>` as a `<p>`. A `gmi.xsl` for [Gemini](https://geminiprotocol.net/) would instead emit plain `###`, `*`, `=>`, and paragraph lines. The same XML source drives both without any changes to the posts.

The XML document every stylesheet receives for the [example post above](#example):

```xml
<document>
    <meta>
        <title value="On Reading"/>
        <tag label="essays" id="0x0002"/>
        <tag label="books" id="0x0003"/>
    </meta>
    <body>
        <bold>On Reading</bold>
        <link href="/0x0002/">0x0002 - essays</link>
        <link href="/0x0003/">0x0003 - books</link>
        <text>Reading is one of the few activities that slows time down.
A good book makes an afternoon feel like a week.</text>
        <item>it builds vocabulary without deliberate effort</item>
        <item>it trains sustained attention</item>
        <item>it exposes you to ways of thinking you would not reach alone</item>
        <bold>Where to start</bold>
        <text>Start anywhere. Curiosity is a better guide than a syllabus.</text>
        <link href="/0x0002/">post on essays</link>
    </body>
</document>
```

---

## Available Stylesheets

The repository ships two stylesheets in `input/styles/` that show how the XML schema maps to real output formats. They are meant as working references — use them as-is, strip them down, or write your own from scratch alongside them.

### `html.xsl` → `output/html/`

Produces HTML. Element mapping:

| XML element | HTML output |
|---|---|
| `<bold>` | `<strong><p>` |
| `<text>` | `<p>` |
| `<link href="…">` | `<a href="…">` |
| `<item>` | `<li>` inside a `<ul>`, consecutive items grouped into one list |
| `<code>` (plain) | `<pre><code>` |
| `<code>` containing `<table>` | `<table>` with `<tr>` / `<td>` and optional inline `style` attributes |

The page `<title>` is pulled from `meta/title/@value`.

### `gmi.xsl` → `output/gmi/`

Produces [Gemtext](https://geminiprotocol.net/docs/gemtext.gmi), the native document format for the Gemini protocol. Element mapping:

| XML element | Gemtext output |
|---|---|
| `<bold>` | `### heading` |
| `<text>` | plain paragraph line |
| `<link href="…">` | `=> url label` |
| `<item>` | `* item`, consecutive items grouped under one blank-line separator |
| `<code>` (plain) | ` ``` … ``` ` preformatted block |
| `<code>` containing `<table>` | ASCII box table (see below) |

#### ASCII table rendering in Gemtext

Gemtext has no native table syntax. When a `<code>` block contains a pandoc-generated `<table>`, `gmi.xsl` renders it as a fixed-width ASCII grid. Column widths are computed by measuring the longest cell in each column across all rows, then every cell is padded to that width:

```
+-----+-----+-----+
|  1  |  2  |  3  |
+-----+-----+-----+
| a   | foo | bar |
+-----+-----+-----+
| b   | baz | qux |
+-----+-----+-----+
```

The border line (`+---+`) is redrawn after every row. The calculation is done entirely in XSLT 1.0 using recursive named templates (`draw-border`, `render-row`, `get-max-width`) — no extensions beyond EXSLT `exsl:common` are required.

---

## Identity and lock file

Every post and tag is assigned an ID by `lock.xml` the first time it is seen. These IDs are hex-formatted (`0x0001`, `0x0002`, …) and used as directory names in the output, making URLs stable regardless of filename changes.

**Always commit `lock.xml`.** Deleting it will reassign IDs and break existing inbound links.

```xml
<lock>
    <key id="1" value="POST:on_reading.md"/>
    <key id="2" value="TAG:essays"/>
    <key id="3" value="TAG:books"/>
</lock>
```

---

## Static files

Any file placed in `input/statics/` is copied verbatim into `output/xml/` and then propagated into every style output directory alongside the transformed files. Use this for `favicon.ico`, images, fonts, etc.
