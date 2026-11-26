import { findGalleryItem } from "../../../items";
import ModalFrame from "./modal-frame";

export const dynamic = "force-dynamic";

export default async function InterceptedItemModal({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const item = findGalleryItem(id);

  return (
    <ModalFrame title={item.title} itemId={item.id}>
      <p className="text-sm text-surface-300 leading-relaxed">{item.details}</p>
      <div className="text-xs font-mono text-surface-400 bg-surface-950 border border-surface-800 rounded-lg p-3">
        URL changed to <code>/routes/gallery/item/{item.id}</code> without
        unmounting the underlying gallery page.
      </div>
    </ModalFrame>
  );
}
