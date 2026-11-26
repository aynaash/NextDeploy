export const dynamic = "force-dynamic";

export async function GET() {
  const encoder = new TextEncoder();

  const stream = new ReadableStream({
    async start(controller) {
      const events = [
        { step: 1, stage: "edge-init", message: "Worker stream opened" },
        { step: 2, stage: "proxy", message: "proxy.ts headers verified" },
        { step: 3, stage: "r2-sync", message: "Asset manifest checked" },
        { step: 4, stage: "done", message: "SSE stream completed cleanly" },
      ];

      for (const evt of events) {
        const payload = `event: chunk\ndata: ${JSON.stringify({
          ...evt,
          timestamp: new Date().toISOString(),
        })}\n\n`;
        controller.enqueue(encoder.encode(payload));
        await new Promise((resolve) => setTimeout(resolve, 120));
      }
      controller.close();
    },
  });

  return new Response(stream, {
    status: 200,
    headers: {
      "Content-Type": "text/event-stream; charset=utf-8",
      "Cache-Control": "no-cache, no-transform",
      Connection: "keep-alive",
    },
  });
}
