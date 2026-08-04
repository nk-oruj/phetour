# phetour (փետուր)

A minimal static-site generator written in Go. It reads plain-text post files written in a lightweight custom syntax, compiles them into an intermediate XML document tree, and transforms that tree into any number of output formats (HTML, Gemtext, or anything else) using XSLT stylesheets.

> [!WARNING]
> phetour has been heavily programmed with AI assistance. this is a warning in case you consider to not use AI generated programs. if you consider using it anyways, then review its code and generated output carefully before relying on it for an important or public sites, and expect to maintain it as your needs change.

---

## Contents

- [How it works](#how-it-works)
- [Project structure](#project-structure)
- [Prerequisites](#prerequisites)
- [Build](#build)
- [Deployment](#deployment)
- [RSS publication](#rss-publication)
- [Writing posts](#writing-posts)
- [Adding a stylesheet](#adding-a-stylesheet)
- [Available stylesheets](#available-stylesheets)
- [Identity and lock file](#identity-and-lock-file)
- [Static files](#static-files)

---

## How it works

```
input/   →   Parse   →   Render   →   XSL Transform   →   output/
```

1. **Parse** — each post file is read and parsed into a `<document>` XML element with `<meta>` (title + tags) and `<body>` (content blocks).
2. **Render** — a separate `<document>` XML file is written for each post, each tag index, and the home catalog.
3. **Transform** — the stylesheet mappings in `config.xml` are applied to every XML file in `output/xml/`, each producing its configured output subfolder.
4. **Lock** — post and tag identities are stored in `lock.xml` so that URLs remain stable across rebuilds even when filenames change.

---

## Project structure

```
.
├── input/
│   ├── posts/          # post source files (see syntax below)
│   ├── statics/        # files copied verbatim into every output directory
│   └── styles/         # XSLT stylesheets for page and RSS output
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
| [xsltproc](http://xmlsoft.org/XSLT/) | apply page stylesheets on Linux/macOS; required for RSS publishing on every platform |
| [msxsl.exe](https://www.microsoft.com/en-us/download/details.aspx?id=21714) | fallback for page stylesheets on Windows; it cannot publish RSS |
| [rsync](https://rsync.samba.org/) | synchronize deployed output folders |
| [pandoc](https://pandoc.org/) | render Markdown tables inside ` ``` ` blocks (optional) |

---

## Build

Before the first build, copy `~config.xml` to `config.xml` and complete the configuration described in [Deployment](#deployment). Every command loads the full file, so it needs valid stylesheet mappings, deployment outputs, and an SSH alias even when you only intend to build locally.

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

    <styles>
        <stylesheet output="html" path="input/styles/html.xsl"/>
        <stylesheet output="gmi" path="input/styles/gmi.xsl"/>
    </styles>

    <rss entry-limit="5">
        <publish output="html" path="rss.xml" stylesheet="input/styles/rss.html.xsl"/>
        <publish output="gmi" path="rss.xml" stylesheet="input/styles/rss.gmi.xsl"/>
    </rss>
</config>
```

`config.xml` is ignored by Git; `~config.xml` remains the shareable template. Its RSS section enables two separate feed files: an HTML-description feed in `output/html/rss.xml` and a Gemtext-description feed in `output/gmi/rss.xml`. They contain the same publication history in different description formats; remove one `<publish>` element if you want only one feed. Each `<stylesheet>` maps an XSLT file (`path`) to an output name (`output`), used for both the output subfolder and file extension: `html` creates `output/html/.../*.html`. Phetour uses `rsync` over your SSH alias: it previews the exact upload and deletion plan, asks for confirmation, then makes the configured remote directory match the corresponding local output folder.

```sh
# upload only files whose contents differ, and delete stale remote files
go run ./source deploy-changes

# force every local file to upload, and delete stale remote files
go run ./source deploy-all
```

Use `--force` to skip the confirmation prompt in scripts. `deploy-changes` uses checksums and does not synchronize timestamps or permissions, so it does not re-upload a file merely because a full local build gave it a new timestamp or WSL reports different file modes. Rsync creates new files as the remote SSH user using the server's normal umask, leaves existing ownership and modes untouched, and excludes symlinks, devices, and special files.

## RSS publication

RSS is optional: omit the `<rss>` element to disable it. Each `<publish>` element configures one RSS file. It selects the output folder where the feed is written (`output`), its relative path (`path`), and the XSLT stylesheet which formats its items (`stylesheet`). All configured feed files are rendered during the same `publish` operation from one shared publication history and share the same `entry-limit`; they are not independently curated. Each tag entry contains its complete current catalog body.

For an HTML-styled description, use `output="html"` with `input/styles/rss.html.xsl`. For a Gemtext-styled description, use `output="gmi"` with `input/styles/rss.gmi.xsl` instead. Both files are RSS XML; the choice controls only the item description body. Do not configure both when you want one feed file.

#### First publication workflow

1. Copy and complete `config.xml`, including at least one output and RSS `<publish>` entry.
2. Run `go run ./source build` to generate the site.
3. Run `go run ./source deploy-changes`. The first successful deployment writes `state.xml` but intentionally creates no RSS entries.
4. After a later post edit, build and deploy again.
5. Run `go run ./source publish` and select the changed posts ready to announce. Their affected tag catalogs are added automatically when safe to publish.

```sh
# deploy pages and record their semantic library state
go run ./source deploy-changes

# select changed posts to publish as individual RSS entries
go run ./source publish
```

Page deployment deliberately excludes the configured RSS files, so an ordinary build cannot delete an existing feed. After all page outputs deploy successfully, Phetour saves their semantic post and catalog snapshot in the local, Git-ignored `state.xml`.

When `state.xml` does not exist, the first successful deployment creates it with the current post and tag snapshots marked as already published. It creates no RSS entries, so change tracking starts with the next deployed change. Running `publish` before the first deployment performs the same initialization from `output/xml` and also creates no RSS entries. The file is internal Phetour state: it stores deployed and published snapshots, publication history, and the next private publication number used to make opaque GUID hashes.

`publish` lists changed posts with their `Created` or `Revised` status and their affected tag catalogs. Select the post numbers that are ready to announce. Each selected post becomes one RSS item containing its full current content. Tag catalogs affected only by selected posts are published automatically as their own `Created` or `Revised` entries, containing their full current catalog body. A tag with an outstanding change from an unselected post is held back, preventing an automatic catalog entry from revealing unselected work. Use `publish --force` for noninteractive use; it selects every changed post.

Only the selected posts and their automatic tag entries advance the published baseline after every RSS file uploads successfully. Other changes remain pending. Each entry receives a new persistent GUID and its `pubDate` is the time of publication.

Missing posts and tags never produce an entry. Phetour retains their last published content as a baseline: if the same identity reappears unchanged it remains silent, and if it reappears changed it becomes a `Revised` candidate. Phetour compares canonical intermediate documents and tag relations, so stylesheet changes for HTML or Gemtext never create RSS entries. Old grouped `<library-update>` history is not migrated; it is ignored when the current state format is next saved.

`deploy-all` is a resynchronization command: it uploads every page file and records the deployed library state, just as `deploy-changes` does.

#### RSS item stylesheets

The provided [`input/styles/rss.html.xsl`](input/styles/rss.html.xsl) is a starting point for HTML descriptions. Phetour gives it one publication document per entry:

```xml
<publication guid="f2b2..." published-at="2026-08-04T12:00:00Z" site-url="https://example.com">
    <post id="0x0012" title="New post" status="Created">
        <member id="0x0005" title="Essays"/>
        <body>
            <text>The complete current post content.</text>
        </body>
    </post>
</publication>
```

The stylesheet must produce an `<rss-content>` element with `<title>` and `<description>` children. Phetour stores description child markup safely in the RSS description. The default HTML stylesheet renders only the current page body and omits its generated first title heading.

```xml
<rss-content>
    <title>Created: New post</title>
    <description>
        <p>The complete current post content.</p>
    </description>
</rss-content>
```

Phetour controls the RSS item link, GUID, and publication date. The GUID is an opaque SHA-256 hash for each publication. `rss.html.xsl` imports `html.xsl` and `rss.gmi.xsl` imports `gmi.xsl`; both render only the current page body, omitting its generated first title heading. Each stylesheet has visible `Created` and `Revised` cases in its `post|tag` template in `mode="title"`; edit their text to customize titles. The title, description, and presentation of posts and catalogs remain customizable in XSLT.

```xml
<xsl:template match="post|tag" mode="title">
    <xsl:choose>
        <xsl:when test="@status = 'Created'">Created: <xsl:value-of select="@title"/></xsl:when>
        <xsl:when test="@status = 'Revised'">Revised: <xsl:value-of select="@title"/></xsl:when>
    </xsl:choose>
</xsl:template>
```

---

## Writing posts

Post files live anywhere under `input/posts/`; subfolders are searched recursively and have no technical effect. The filename is the post's permanent identity key — the title displayed to readers comes from the file content, not the filename. Therefore filenames must be unique across all post subfolders.

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
- The contiguous **`>` block** following the title is the tag list. Leading blank lines before that block are ignored, but the first blank line after a tag ends the header. The entire string after `>` becomes the tag label.
- The header also ends when any non-`>` line is encountered. From that point on, everything is content.

#### Content blocks

| Syntax | Intermediate XML element | Notes |
|---|---|---|
| `# Section heading` | `<bold>` | rendered by the stylesheet |
| `- List item` | `<item-group><item>` | consecutive items share one group; a blank line starts another group |
| `> url label` | `<link-group><link href="url">` | first word is the href, rest is label; a blank line starts another group |
| Plain paragraph text | `<text>` | consecutive lines form one block |
| ` ``` … ``` ` | `<code>` | processed by pandoc if available |

Consecutive plain-text lines are collected into a single `<text>` block. Consecutive links and items are stored in `<link-group>` and `<item-group>` elements. A blank line starts a new group; headings, text, and code also end the current group. Generated tag links form their own group, separate from the post body.

> **Note on the `>` sigil:** In the header it means *tag*. In the content body it means *link*, but only when followed by a space (`> url label`). End the tag block with a blank line before the first body link; that line is then parsed as a link instead of a tag.

#### Tables (via pandoc)

Markdown-style tables inside a ` ``` ` block are processed by `pandoc`:

    ```
    |   | _1_ | _2_ |
    |---|:---:|:---:|
    | A | foo | bar |
    ```

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
2. Add a `<stylesheet output="myformat" path="input/styles/myformat.xsl"/>` entry under `<styles>` in `config.xml`.
3. Run `make build`.
4. Find the output in `output/myformat/`.

The approach is to write one stylesheet per target format and map it explicitly to its output directory in `config.xml`.

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
        <link-group>
            <link href="/0x0002/">0x0002 - essays</link>
            <link href="/0x0003/">0x0003 - books</link>
        </link-group>
        <text>Reading is one of the few activities that slows time down.
A good book makes an afternoon feel like a week.</text>
        <item-group>
            <item>it builds vocabulary without deliberate effort</item>
            <item>it trains sustained attention</item>
            <item>it exposes you to ways of thinking you would not reach alone</item>
        </item-group>
        <bold>Where to start</bold>
        <text>Start anywhere. Curiosity is a better guide than a syllabus.</text>
        <link-group>
            <link href="/0x0002/">post on essays</link>
        </link-group>
    </body>
</document>
```

---

## Available stylesheets

The repository ships HTML, Gemtext, and RSS stylesheets in `input/styles/`. They are meant as working references — use them as-is, strip them down, or write your own from scratch alongside them.

### `html.xsl` → `output/html/`

Produces HTML. Element mapping:

| XML element | HTML output |
|---|---|
| `<bold>` | `<p><strong>` |
| `<text>` | `<p>` |
| `<link-group>` | `<a href="…">` lines inside a `<p>` |
| `<item-group>` | `<li>` inside a `<ul>` |
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
