# phetour (փետուր)

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

## Prerequisites

| Tool | Purpose |
|---|---|
| [Go](https://go.dev/) 1.23+ | build the generator |
| [xsltproc](http://xmlsoft.org/XSLT/) | apply stylesheets (Linux/macOS) |
| [msxsl.exe](https://www.microsoft.com/en-us/download/details.aspx?id=21714) | apply stylesheets (Windows) |
| [rsync](https://rsync.samba.org/) | synchronize deployed output folders |
| [pandoc](https://pandoc.org/) | render Markdown tables inside ` ``` ` blocks (optional) |

---

## Build

```sh
# generate the site
make build

# or directly
go run ./source build
```

Output lands in `output/`.

Phetour intentionally rebuilds the complete local site. For a small static site, this keeps building simple and lets `rsync` avoid unnecessary uploads by comparing generated file contents.

---

## Deployment

Copy the tracked `~config.xml` template to `config.xml`, then set your existing SSH alias and remote destination for each output directory you want to publish:

```sh
cp ~config.xml config.xml
```

```xml
<config>
    <site
        title="My Site"
        url="https://example.com/"
        description="Recent publications."/>

    <deployment ssh-alias="myServer">
        <output name="html" remote="/var/html"/>
        <output name="gmi" remote="/var/gmi"/>
    </deployment>

    <rss entry-limit="5">
        <publish output="html" path="rss.xml" stylesheet="input/rss/default.xsl"/>
    </rss>
</config>
```

`config.xml` is ignored by Git; `~config.xml` remains the shareable template. Phetour uses `rsync` over your SSH alias: it previews the exact upload and deletion plan, asks for confirmation, then makes the configured remote directory match the corresponding local output folder.

```sh
# upload only files whose contents differ, and delete stale remote files
go run ./source deploy-changes

# force every local file to upload, and delete stale remote files
go run ./source deploy-all
```

Use `--force` to skip the confirmation prompt in scripts. `deploy-changes` uses checksums and does not synchronize timestamps or permissions, so it does not re-upload a file merely because a full local build gave it a new timestamp or WSL reports different file modes. Rsync creates new files as the remote SSH user using the server's normal umask, leaves existing ownership and modes untouched, and excludes symlinks, devices, and special files.

### RSS publication

RSS is optional: omit the `<rss>` element to disable it. A `<publish>` element selects the output folder where the feed is written (`output`), its relative path (`path`), and the XSLT stylesheet which formats its items (`stylesheet`). `entry-limit` controls how many previously published library updates remain in each feed.

```sh
# deploy pages and record their semantic library state
go run ./source deploy-changes

# publish one grouped update for every net change since the last publication
go run ./source publish
```

Page deployment deliberately excludes the configured RSS files, so an ordinary build cannot delete an existing feed. After all page outputs deploy successfully, Phetour saves their semantic post and catalog snapshot in the local, Git-ignored `state.xml`. `publish` compares that deployed snapshot to the last published snapshot and creates one RSS item which groups added, revised, and removed posts plus catalog restructuring. It uploads only the RSS files and updates `state.xml` after every feed upload succeeds.

`deploy-all` is a resynchronization command: it uploads every page file but deliberately does not update `state.xml`, so it cannot create a bulk RSS publication. Use `deploy-changes` for deployments that should be eligible for publication.

If a post or catalog is changed and then restored before `publish`, it produces no update. Several deployments before publishing are bundled into one item using their final deployed state. The RSS item GUID identifies that semantic change-set, while its `pubDate` is the time of publication.

#### RSS item stylesheets

The provided [`input/rss/default.xsl`](input/rss/default.xsl) is a starting point. Phetour gives it one grouped update document:

```xml
<library-update guid="..." published-at="2026-07-31T12:00:00Z" site-url="https://example.com">
    <posts>
        <added><post id="0x0012" title="New post"/></added>
        <revised><post id="0x0004" title="Existing post"/></revised>
    </posts>
    <catalogs>
        <revised>
            <tag id="0x0005" title="Essays">
                <added-member id="0x0012" title="New post"/>
            </tag>
        </revised>
    </catalogs>
</library-update>
```

The stylesheet must produce an `<rss-content>` element with `<title>` and `<description>` children. Description children may be HTML/XML markup, which Phetour stores safely as the RSS description.

```xml
<rss-content>
    <title>Library update</title>
    <description>
        <p><strong>Added posts</strong></p>
        <ul><li><a href="https://example.com/0x0012/">[0x0012] - New post</a></li></ul>
    </description>
</rss-content>
```

Phetour controls the RSS item link, GUID, and publication date. The title, description, and presentation of changed posts and catalogs remain customizable in XSLT.

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

The approach is to write one stylesheet per target format.

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

## Available stylesheets

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
