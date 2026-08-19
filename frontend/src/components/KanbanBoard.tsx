import { useMemo, useState } from 'react'
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { useDroppable } from '@dnd-kit/core'
import { SortableContext, verticalListSortingStrategy } from '@dnd-kit/sortable'
import type { BoardDetail, Task } from '../api/types'
import { columnDroppableId, computeMove } from '../lib/moveTask'
import { TaskCard } from './TaskCard'

function ColumnDrop({
  columnId,
  title,
  children,
}: {
  columnId: string
  title: string
  children: React.ReactNode
}) {
  const { setNodeRef, isOver } = useDroppable({ id: columnDroppableId(columnId) })
  return (
    <div
      ref={setNodeRef}
      className={`flex min-h-[280px] w-72 shrink-0 flex-col rounded-xl border bg-zinc-900/40 ${
        isOver ? 'border-violet-500/60' : 'border-zinc-800'
      }`}
    >
      <div className="border-b border-zinc-800 px-3 py-2">
        <h2 className="text-sm font-semibold text-zinc-200">{title}</h2>
      </div>
      <div className="flex flex-1 flex-col gap-2 p-2">{children}</div>
    </div>
  )
}

function taskMatchesFilter(
  task: Task,
  mode: 'all' | 'overdue' | 'soon'
): boolean {
  if (mode === 'all') return true
  if (!task.due_at) return false
  const d = new Date(task.due_at).getTime()
  const now = Date.now()
  if (mode === 'overdue') return d < now
  const week = 7 * 864e5
  return d >= now && d <= now + week
}

export function KanbanBoard({
  board,
  onMove,
  onAddTask,
  onOpenTask,
  taskFilter = 'all',
}: {
  board: BoardDetail
  onMove: (taskId: string, column_id: string, position: number) => void
  onAddTask: (columnId: string, title: string) => void
  onOpenTask: (task: Task) => void
  taskFilter?: 'all' | 'overdue' | 'soon'
}) {
  const [activeTask, setActiveTask] = useState<Task | null>(null)
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } })
  )

  const sortedColumns = useMemo(
    () => [...board.columns].sort((a, b) => a.position - b.position),
    [board.columns]
  )

  function findTask(id: string): Task | null {
    for (const c of board.columns) {
      const t = c.tasks.find((x) => x.id === id)
      if (t) return t
    }
    return null
  }

  function handleDragStart(e: DragStartEvent) {
    const id = String(e.active.id)
    const t = findTask(id)
    setActiveTask(t)
  }

  function handleDragEnd(e: DragEndEvent) {
    setActiveTask(null)
    const activeId = String(e.active.id)
    const payload = computeMove(board, activeId, e.over?.id != null ? String(e.over.id) : null)
    if (!payload) return
    onMove(activeId, payload.column_id, payload.position)
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
    >
      <div className="flex gap-4 overflow-x-auto pb-4">
        {sortedColumns.map((col) => (
          <ColumnDrop key={col.id} columnId={col.id} title={col.title}>
            <SortableContext items={col.tasks.map((t) => t.id)} strategy={verticalListSortingStrategy}>
              {col.tasks.map((t) => (
                <TaskCard
                  key={t.id}
                  task={t}
                  dimmed={taskFilter !== 'all' && !taskMatchesFilter(t, taskFilter)}
                  onOpen={() => onOpenTask(t)}
                />
              ))}
            </SortableContext>
            <AddTaskInline onAdd={(title) => onAddTask(col.id, title)} />
          </ColumnDrop>
        ))}
      </div>
      <DragOverlay>
        {activeTask ? (
          <div className="rounded-lg border border-violet-500/50 bg-zinc-800 p-2 shadow-xl">
            <span className="font-medium text-zinc-100">{activeTask.title}</span>
          </div>
        ) : null}
      </DragOverlay>
    </DndContext>
  )
}

function AddTaskInline({ onAdd }: { onAdd: (title: string) => void }) {
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="rounded-lg border border-dashed border-zinc-700 py-2 text-sm text-zinc-500 hover:border-zinc-600 hover:text-zinc-400"
      >
        + Add task
      </button>
    )
  }
  return (
    <form
      className="flex flex-col gap-2"
      onSubmit={(e) => {
        e.preventDefault()
        const t = title.trim()
        if (!t) return
        onAdd(t)
        setTitle('')
        setOpen(false)
      }}
    >
      <input
        autoFocus
        className="rounded border border-zinc-700 bg-zinc-950 px-2 py-1.5 text-sm text-zinc-100 outline-none focus:border-violet-500"
        placeholder="Task title"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <div className="flex gap-2">
        <button
          type="submit"
          className="rounded bg-violet-600 px-2 py-1 text-xs font-medium text-white hover:bg-violet-500"
        >
          Add
        </button>
        <button
          type="button"
          className="rounded px-2 py-1 text-xs text-zinc-500 hover:text-zinc-300"
          onClick={() => {
            setOpen(false)
            setTitle('')
          }}
        >
          Cancel
        </button>
      </div>
    </form>
  )
}
