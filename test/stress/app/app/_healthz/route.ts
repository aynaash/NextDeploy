export const dynamic = "force-dynamic";

export async function GET() {
  return new Response("healthy\n", {
    status: 200,
    headers: { "Content-Type": "text/plain" },
  });
}
