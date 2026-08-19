import { useSortable } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { Task } from '../api/types'

function dueLabel(due: string | null | undefined) {
  if (!due) return null
  const d = new Date(due)
  const now = new Date()
  const overdue = d < now
  return (
    <span
      className={`text-xs ${overdue ? 'text-amber-400' : 'text-zinc-500'}`}
      title={d.toLocaleString()}
    >
      {d.toLocaleDateString()}
    </span>
  )
}

export function TaskCard({
  task,
  onOpen,
  dimmed = false,
}: {
  task: Task
  onOpen: () => void
  dimmed?: boolean
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: task.id,
  })
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.35 : 1,
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={`rounded-lg border border-zinc-700/80 bg-zinc-800/80 p-2 shadow-sm transition-opacity ${
        dimmed ? 'opacity-35' : ''
      }`}
    >
      <div className="flex items-start gap-2">
        <button
          type="button"
          className="mt-0.5 cursor-grab touch-none text-zinc-500 hover:text-zinc-300"
          aria-label="Drag task"
          {...attributes}
          {...listeners}
        >
          ⠿
        </button>
        <button
          type="button"
          className="min-w-0 flex-1 text-left"
          onClick={onOpen}
        >
          <span className="block font-medium text-zinc-100">{task.title}</span>
          {task.description && (
            <span className="mt-0.5 line-clamp-2 block text-xs text-zinc-500">{task.description}</span>
          )}
          <div className="mt-1 flex flex-wrap gap-2">{dueLabel(task.due_at)}</div>
        </button>
      </div>
    </div>
  )
}
