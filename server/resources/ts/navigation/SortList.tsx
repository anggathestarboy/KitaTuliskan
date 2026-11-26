import { DndContext, KeyboardSensor, PointerSensor, closestCenter, useSensor, useSensors, type Modifier } from '@dnd-kit/core';
import { SortableContext, arrayMove, sortableKeyboardCoordinates, verticalListSortingStrategy } from '@dnd-kit/sortable';
import { type ReactNode } from 'react';

// Drag & drop urut (wire:sort di source): item mengikuti kursor, item lain bergeser menyingkir dengan animasi.
// Tiap list (root, daftar anak, baris replicator) punya DndContext sendiri, jadi item tidak bisa dipindah antar-list (itu lewat "Move to...").
const verticalOnly: Modifier = ({ transform }) => ({ ...transform, x: 0 });

export function SortList({ ids, onSort, children }: { ids: string[]; onSort: (id: string, position: number) => void; children: ReactNode }) {
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 4 } }), useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }));
  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} modifiers={[verticalOnly]} onDragEnd={({ active, over }) => {
      if (!over || active.id === over.id) return;
      onSort(String(active.id), arrayMove(ids, ids.indexOf(String(active.id)), ids.indexOf(String(over.id))).indexOf(String(active.id)));
    }}>
      <SortableContext items={ids} strategy={verticalListSortingStrategy}>{children}</SortableContext>
    </DndContext>
  );
}
