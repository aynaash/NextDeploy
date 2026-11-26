import { revalidatePath } from "next/cache";
import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function POST() {
  revalidatePath("/routes/isr");
  return NextResponse.json({
    revalidated: true,
    path: "/routes/isr",
    timestamp: new Date().toISOString(),
  });
}
