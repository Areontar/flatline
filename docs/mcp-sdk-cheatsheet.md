# Official Go MCP SDK - Client Cheat-Sheet

Source of truth: `github.com/modelcontextprotocol/go-sdk`, package `mcp`.
Verified directly against raw source at tag **v1.7.0** (latest stable, released
2026-07-28; supports MCP spec `2026-07-28` and is backward compatible down to
`2024-11-05`). All code below is quoted or adapted from the actual `mcp/*.go`
source and the repo's `examples/client/listfeatures` and `examples/client/loadtest`
programs - not guessed.

## 1. Module path / version

```
go get github.com/modelcontextprotocol/go-sdk@v1.7.0
```

```go
import "github.com/modelcontextprotocol/go-sdk/mcp"
```

Other importable packages in the module: `.../jsonrpc` (custom transports),
`.../auth`, `.../oauthex` (OAuth helpers) - not needed for a basic HTTP tool-calling client.

## 2. Client construction

```go
func NewClient(impl *Implementation, options *ClientOptions) *Client
```

```go
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}
```

`options` may be `nil` for a plain client. Relevant `ClientOptions` fields (there are more,
mostly deprecated sampling/elicitation handlers):

```go
type ClientOptions struct {
	Logger *slog.Logger // enable logging of client activity
	// CreateMessageHandler, ElicitationHandler, etc. - only needed if the
	// server calls back into the client (sampling/elicitation). Not needed
	// for a plain "call a tool" client. Sampling/roots/logging are deprecated
	// as of protocol 2026-07-28 (SEP-2577) but still supported.
	Capabilities *ClientCapabilities
}
```

Usage:

```go
client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
```

## 3. HTTP transport

### Streamable HTTP - the current, recommended transport

```go
type StreamableClientTransport struct {
	Endpoint             string        // MCP server URL
	HTTPClient           *http.Client  // nil -> http.DefaultClient
	MaxRetries           int           // reconnection attempts; default 5, negative disables retries
	DisableStandaloneSSE bool          // if true, skip the persistent SSE stream for server->client msgs
	OAuthHandler         auth.OAuthHandler // optional, from the auth package
}

func (t *StreamableClientTransport) Connect(ctx context.Context) (Connection, error)
```

No `New...` constructor exists - it's a plain struct literal:

```go
transport := &mcp.StreamableClientTransport{Endpoint: "https://example.com/mcp"}
```

This implements the MCP spec's "Streamable HTTP" transport (introduced 2025-03-26),
which is what current MCP servers expose over HTTP.

### SSE - legacy transport, still supported

```go
type SSEClientTransport struct {
	Endpoint   string       // the SSE endpoint to connect to
	HTTPClient *http.Client // nil -> http.DefaultClient
}

func (c *SSEClientTransport) Connect(ctx context.Context) (Connection, error)
```

This implements the *original* HTTP+SSE transport from spec version `2024-11-05`,
superseded by Streamable HTTP in `2025-03-26`. Only use it against older servers
that haven't upgraded. **For new work, use `StreamableClientTransport`.**

Both transport types satisfy the `mcp.Transport` interface (`Connect(ctx) (Connection, error)`),
so you can hold a `var transport mcp.Transport` and pick the concrete type at runtime
(this is exactly what `examples/client/listfeatures/main.go` does).

## 4. Connecting

```go
func (c *Client) Connect(ctx context.Context, t Transport, opts *ClientSessionOptions) (cs *ClientSession, err error)
```

Doc comment: *"Connect begins an MCP session by connecting to a server over the given
transport. The resulting session is initialized, and ready to use."*

So **yes, the `initialize` handshake happens automatically inside `Connect`** - by
the time it returns successfully you have a ready `*ClientSession`; you never call
`initialize` yourself. `opts` can be `nil` (its one field, `protocolVersion`, is
unexported / testing-only as of v1.7.0).

```go
cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
if err != nil {
	log.Fatal(err)
}
defer cs.Close()
```

`cs.InitializeResult()` returns `*InitializeResult` if you need the server's
declared capabilities/protocol version after connecting.

## 5. Calling a tool

```go
func (cs *ClientSession) CallTool(ctx context.Context, params *CallToolParams) (*CallToolResult, error)
```

```go
type CallToolParams struct {
	Meta      `json:"_meta,omitempty"`
	Name      string `json:"name"`
	Arguments any    `json:"arguments,omitempty"` // NOT map[string]any - any JSON-marshalable value
	// (plus InputResponses/RequestState - multi-round-trip plumbing, ignore for basic calls)
}
```

**Gotcha:** `Arguments` is typed `any`, not `map[string]any`. In practice you pass
a `map[string]any` (most common) or a struct, and the SDK JSON-marshals it. You can
also pass `json.RawMessage` directly (the `loadtest` example does this when forwarding
raw JSON args from a CLI flag):

```go
res, err := cs.CallTool(ctx, &mcp.CallToolParams{
	Name:      "search",
	Arguments: map[string]any{"query": "halctf", "limit": 10},
})
```

```go
type CallToolResult struct {
	Meta               `json:"_meta,omitempty"`
	Content            []Content `json:"content"`
	StructuredContent  any       `json:"structuredContent,omitempty"`
	IsError            bool      `json:"isError,omitempty"`
	// + InputRequests/RequestState (multi-round-trip)
}
```

## 6. Reading the result

`Content` is a **sealed interface** (it has an unexported `fromWire` method), so
only SDK-defined types implement it: `*TextContent`, `*ImageContent`, `*AudioContent`,
`*EmbeddedResource`, `*ResourceLink`, plus sampling-only `*ToolUseContent`/`*ToolResultContent`.
You must type-switch/type-assert on concrete pointer types - you cannot implement
`Content` yourself.

```go
type TextContent struct {
	Text        string
	Meta        Meta
	Annotations *Annotations
}
```

Extracting text:

```go
for _, c := range res.Content {
	if tc, ok := c.(*mcp.TextContent); ok {
		fmt.Println(tc.Text)
	}
}
```

If the tool declares structured output, prefer `res.StructuredContent` (an `any`
you can type-assert or re-marshal/unmarshal into your own struct) - `Content` is
then auto-populated with the JSON text as a fallback for older clients.

**Errors:** `res.IsError == true` means the *tool itself* reported a failure (this
is a normal, successful RPC - check `IsError`, don't rely on `err`). The `Content`
slice on an error result typically still contains a `TextContent` describing the
error - read it the same way. A non-nil `error` return from `CallTool` means a
transport/protocol-level failure (network error, invalid params, session broken),
distinct from a tool-level error.

```go
res, err := cs.CallTool(ctx, params)
if err != nil {
	// transport / protocol error
}
if res.IsError {
	// tool-level error; res.Content[0].(*mcp.TextContent).Text usually has the message
}
```

## 7. Session teardown

```go
func (cs *ClientSession) Close() error
func (cs *ClientSession) Wait() error // blocks until the session's connection is closed
```

Standard pattern: `defer cs.Close()` right after a successful `Connect`.

## 8. Gotchas / notes

- **Initialize is automatic** - don't call it manually; `Connect` won't return
  until the session is initialized.
- **Listing tools first is optional**, not required - you can call `CallTool`
  directly if you already know the tool name/schema. To discover tools:
  `cs.ListTools(ctx, params)` (single page, `*ListToolsResult`) or the newer
  iterator `cs.Tools(ctx, params) iter.Seq2[*mcp.Tool, error]` which auto-paginates
  (used with `for t, err := range cs.Tools(ctx, nil) { ... }`).
- **`Arguments` is `any`**, not `map[string]any` - pass whatever JSON-marshals
  correctly for the tool's input schema.
- **Content is a closed interface** - always type-assert to the concrete pointer
  types (`*mcp.TextContent`, etc.); you can't add your own implementations.
- **Thread-safety:** a single `*ClientSession` is safe for concurrent `CallTool`/
  `ListTools`/etc. calls from multiple goroutines - `examples/client/loadtest`
  spins up many workers per session-per-worker but each worker uses its own
  session; the SDK doesn't document per-session concurrent-call safety explicitly
  in the excerpts checked, so if you need concurrent calls on one session, verify
  against `mcp/client.go` locking or just use one session per goroutine (as the
  load-test example does) to be safe.
- **`StreamableClientTransport.MaxRetries`** defaults to 5 automatic reconnects;
  set negative to disable. There's a known open issue (#683) where transient
  errors can leave the transport in a poisoned state for subsequent requests -
  worth a retry/backoff wrapper at the call site if you need high reliability.
- **Context per call**: pass a fresh/derived `context.Context` (with timeout) to
  each `CallTool`, as shown in `loadtest`'s `context.WithTimeout(parentCtx, *timeout)`
  pattern - don't reuse a single long-lived context with no deadline for calls you
  want to be able to time out independently.
- **Deprecated features**: roots, sampling, and logging capabilities are deprecated
  as of protocol `2026-07-28` (SEP-2577) but the SDK still supports them for a
  ≥12-month compatibility window - irrelevant for a simple tool-calling client.
- **Security advisories** exist against some pre-v1.7.0 releases (case-sensitivity
  handling, null-Unicode JSON parsing, a "Cross-Site Tool Execution" issue for HTTP
  servers, DNS-rebinding protection). Pin to v1.7.0+ to pick up fixes.

## Minimal end-to-end example

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx := context.Background()

	client := mcp.NewClient(&mcp.Implementation{Name: "halctf-client", Version: "v1.0.0"}, nil)

	transport := &mcp.StreamableClientTransport{Endpoint: "https://example.com/mcp"}
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "some_tool",
		Arguments: map[string]any{"key": "value"},
	})
	if err != nil {
		log.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				log.Fatalf("tool error: %s", tc.Text)
			}
		}
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			fmt.Println(tc.Text)
		}
	}
}
```

## Sources checked

- `github.com/modelcontextprotocol/go-sdk` GitHub repo, tag `v1.7.0` (raw source):
  `mcp/client.go`, `mcp/protocol.go`, `mcp/content.go`, `mcp/streamable.go`, `mcp/sse.go`, `README.md`
- `examples/client/listfeatures/main.go`, `examples/client/loadtest/main.go` (real client code using `StreamableClientTransport`)
- GitHub Releases API (`/repos/modelcontextprotocol/go-sdk/releases`) and pkg.go.dev version list - cross-checked v1.7.0 as latest stable, published 2026-07-28
