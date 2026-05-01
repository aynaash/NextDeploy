# Ideas


Loose collection of in-progress design notes for nextdeploy. Each section is one
self-contained idea; not a roadmap, not committed work. Promote to a real doc
(like `CLOUDFLARE_PARITY.md`) when an idea graduates.

---

## Go sidecars for Next.js runtime gaps

> **Thesis:** when Next.js (running on CF Workers or Lambda) hits a runtime
> wall, the answer should be a tiny single-purpose Go binary deployed alongside
> the app, not a different framework or a different cloud. nextdeploy makes
> declaring + provisioning + binding these sidecars a first-class concern.

### Why this shape

Workers can't run native binaries (libvips, ffmpeg, ghostscript), can't open
arbitrary TCP (no SMTP, no raw protocols), have CPU/memory caps measured in MB
and ms, and can't load arbitrary Node deps (`nodemailer`, `sharp`, `puppeteer`).
Everything in that list is a one-binary problem in Go: stdlib `net/smtp`,
`os/exec` to ffmpeg, `pdfcpu`, etc.

The pattern we want for the user:

```yaml
# nextdeploy.yml
services:
  - name: mailservice
    binary: ./services/mail        # local Go module
    runtime: cf-container          # cf-container | cloud-run | fly | ecs
    proto:   ./services/mail/mail.proto
    bindings:
      - { name: MAIL, into_worker: true }   # inject as env.MAIL on the Worker
```

…and the Worker app code becomes:

```ts
const res = await env.MAIL.send({ from, to, subject, html });
```

No SDK install, no extra config — `nextdeploy ship` provisions the container,
generates the Connect-RPC client, and wires the service binding.

### Why CF Containers + Connect-RPC, not raw gRPC

- **CF Containers** (GA in 2025): Worker can spawn / address a long-running
  container in the same account. Same billing, same dashboard, same auth. No
  second platform to operate.
- **Connect-RPC** (Buf's protocol): HTTP/1.1 + HTTP/2 + JSON, works through a
  Worker `fetch()` without HTTP/2-trailer compatibility issues that bite
  vanilla gRPC at the edge. First-class browser/edge support. Same `.proto`
  files compile to both gRPC servers and Connect handlers.
- **Connect over plain gRPC**: Workers' fetch over HTTP/2 doesn't reliably
  surface trailers, which gRPC depends on for status. Connect's HTTP semantic
  layer avoids the issue.

### Concrete first sidecar: `mailservice`

Driving need. nodemailer doesn't work in Workers (raw SMTP TCP). Provider HTTP
APIs (Resend, Postmark, SES) work but lock the user into one vendor.

```proto
service Mail {
  rpc Send(SendRequest) returns (SendResponse);
}
message SendRequest {
  string from = 1;
  repeated string to = 2;
  repeated string cc = 3;
  repeated string bcc = 4;
  string subject = 5;
  string text = 6;
  string html = 7;
  repeated Attachment attachments = 8;
  oneof transport {
    SMTPConfig smtp = 10;          // host/port/user/pass/starttls
    string     provider_id = 11;   // "ses" | "resend" | "postmark" — uses server-side creds
  }
}
message Attachment {
  string filename = 1;
  string content_type = 2;
  bytes  content = 3;
}
message SendResponse {
  string message_id = 1;
  string accepted_by = 2;
}
```

Implementation: ~150 LOC of Go using `gomail.v2` for SMTP, plus thin adapter
files for SES / Resend / Postmark HTTP APIs. Cold start <100ms. Fits in one
file at first, splits later.

### Concrete second sidecar: `mediaservice`

User has built a video processing pipeline in Go before. Gap: Workers can't
run ffmpeg, can't hold large files in memory.

```proto
service Media {
  rpc Transcode(TranscodeRequest) returns (TranscodeResponse);
  rpc Probe(ProbeRequest) returns (ProbeResponse);
  rpc ExtractThumbnail(ThumbnailRequest) returns (ThumbnailResponse);
}
```

`TranscodeRequest` takes an R2 key (input) and writes the result to another
R2 key — no large blobs over the wire, just metadata. The Worker fires off the
job and gets a job id; long jobs go through CF Queues, callback to the Worker
when done.

### Other obvious sidecars

- `pdfservice` — `pdfcpu`, `gotenberg`, or `chromedp` headless. PDF generation
  / merge / split / fill-form.
- `archiver` — large zip/tar streaming with `archive/zip`. Workers can't
  hold multi-GB blobs in memory.
- `imageservice` — only if the user opts out of Cloudflare Images (CF Images
  is cheaper for the common case).
- `aiservice` — local Whisper / sentence-transformer / OCR running on a CF
  Container with a small GPU/CPU model. CF AI bindings cover the big-model
  case; this fills "small fast model, no per-call billing."

### nextdeploy work needed

1. **Schema** — extend `nextdeploy.yml` with a `services:` block (see top of
   doc). Each entry has runtime target, binary path, proto path, bindings.
2. **Generator** — `nextdeploy generate services`: compile `.proto` → Go
   server stubs + TS Connect client, drop client into `.nextdeploy/clients/`
   for the Worker bundle to import.
3. **Provisioner** — for `runtime: cf-container`, build the binary into a
   minimal scratch/distroless container, push to CF's container registry,
   create the Container service, attach as a Worker binding.
4. **Multi-runtime** — same schema should target `cloud-run` and `fly` so
   users not on CF aren't forced to be.
5. **Local dev** — `nextdeploy dev` runs the Go service locally on a port
   and points the Worker dev server at it; same Connect endpoint as prod.
6. **Templating** — `nextdeploy add service mail` scaffolds the proto, a Go
   `main.go` skeleton with the `Send` handler, and the YAML entry.

### Tradeoffs to keep honest

- **Adds a runtime to operate.** Logs, metrics, scaling, secrets, deploys —
  multiplied per service. Mitigated by keeping each binary tiny and
  declarative; nextdeploy owns the lifecycle, user just writes business
  logic.
- **Latency hop.** Worker → Container is intra-region (~1–5ms) but it's
  still a hop. Don't put hot-path code in a sidecar. Reserve sidecars for
  things Workers genuinely can't do.
- **Auth between Worker and service.** Service binding is in-account, so CF
  auths it for us. For non-CF deploys (Cloud Run / Fly), use signed JWT
  middleware on the Go side; nextdeploy generates the keypair and rotates.
- **Provider lock-in alternative.** For email specifically, "just use
  Resend/SES from the Worker" is 5 lines of fetch. Sidecar is right when
  the user wants SMTP/multi-provider/self-hosted, OR when this is the
  first of many sidecars and we're building the pattern.

### Open questions

- Should the proto live in the user's repo or in the binary's repo? (Probably
  user's, for type-checked Worker calls.)
- Buf vs. raw `protoc-gen-connect-go` toolchain — Buf is nicer but adds a
  dep on `buf` CLI.
- Streaming RPCs (server-stream, bidi) — needed for media transcode
  progress reporting? Connect supports them.
- Cold start on CF Containers when traffic is bursty — does the user pay
  for warm reservations or live with the latency? Probably configurable.
- Versioning: when the user changes the proto, breaking change semantics
  need to fail loud at `nextdeploy ship` rather than at runtime.
