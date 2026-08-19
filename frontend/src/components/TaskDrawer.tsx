import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client'
import type { BoardDetail, Comment, Task } from '../api/types'

export function TaskDrawer({
  board,
  task,
  open,
  onClose,
}: {
  board: BoardDetail
  task: Task | null
  open: boolean
  onClose: () => void
}) {
  const qc = useQueryClient()
  const boardId = board.id

  const comments = useQuery({
    queryKey: ['comments', task?.id],
    queryFn: () => api<Comment[]>(`/tasks/${task!.id}/comments`),
    enabled: open && !!task,
  })

  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [assigneeId, setAssigneeId] = useState<string>('')
  const [dueAt, setDueAt] = useState('')
  const [commentBody, setCommentBody] = useState('')

  useEffect(() => {
    if (!task) return
    setTitle(task.title)
    setDescription(task.description || '')
    setAssigneeId(task.assignee_id || '')
    setDueAt(task.due_at ? task.due_at.slice(0, 16) : '')
  }, [task])

  const saveTask = useMutation({
    mutationFn: () => {
      if (!task) throw new Error('no task')
      const payload: Record<string, unknown> = {
        title: title.trim() || task.title,
        description,
      }
      if (assigneeId) {
        payload.assignee_id = assigneeId
        payload.clear_assignee = false
      } else {
        payload.clear_assignee = true
      }
      if (dueAt) {
        payload.due_at = new Date(dueAt).toISOString()
        payload.clear_due_at = false
      } else {
        payload.clear_due_at = true
      }
      return api<Task>(`/tasks/${task.id}`, {
        method: 'PATCH',
        json: payload,
      })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['board', boardId] })
    },
  })

  const addComment = useMutation({
    mutationFn: () => {
      if (!task) throw new Error('no task')
      return api<Comment>(`/tasks/${task.id}/comments`, {
        method: 'POST',
        json: { body: commentBody.trim() },
      })
    },
    onSuccess: () => {
      setCommentBody('')
      qc.invalidateQueries({ queryKey: ['comments', task?.id] })
      qc.invalidateQueries({ queryKey: ['board', boardId] })
    },
  })

  const deleteTask = useMutation({
    mutationFn: () => {
      if (!task) throw new Error('no task')
      return api(`/tasks/${task.id}`, { method: 'DELETE' })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['board', boardId] })
      onClose()
    },
  })

  if (!open || !task) return null

  return (
    <div className="fixed inset-0 z-40 flex justify-end bg-black/50" role="presentation">
      <button
        type="button"
        className="h-full flex-1 cursor-default border-0 bg-transparent"
        aria-label="Close panel"
        onClick={onClose}
      />
      <aside className="flex h-full w-full max-w-md flex-col border-l border-zinc-800 bg-zinc-950 shadow-2xl">
        <div className="flex items-center justify-between border-b border-zinc-800 px-4 py-3">
          <h2 className="text-lg font-semibold text-white">Task</h2>
          <button
            type="button"
            onClick={onClose}
            className="rounded p-1 text-zinc-500 hover:bg-zinc-900 hover:text-zinc-200"
          >
            ✕
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-4 py-4">
          <label className="mb-3 block text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
            Title
            <input
              className="mt-1 w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-violet-500"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
          </label>
          <label className="mb-3 block text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
            Description
            <textarea
              className="mt-1 min-h-[100px] w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-violet-500"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </label>
          <label className="mb-3 block text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
            Assignee
            <select
              className="mt-1 w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-violet-500"
              value={assigneeId}
              onChange={(e) => setAssigneeId(e.target.value)}
            >
              <option value="">Unassigned</option>
              {board.members.map((m) => (
                <option key={m.id} value={m.id}>
                  {m.display_name}
                </option>
              ))}
            </select>
          </label>
          <label className="mb-4 block text-left text-xs font-medium uppercase tracking-wide text-zinc-500">
            Due
            <input
              type="datetime-local"
              className="mt-1 w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-violet-500"
              value={dueAt}
              onChange={(e) => setDueAt(e.target.value)}
            />
          </label>

          <div className="mb-6 flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => saveTask.mutate()}
              disabled={saveTask.isPending}
              className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white hover:bg-violet-500 disabled:opacity-50"
            >
              Save changes
            </button>
            <button
              type="button"
              onClick={() => {
                if (confirm('Delete this task?')) deleteTask.mutate()
              }}
              disabled={deleteTask.isPending}
              className="rounded-lg border border-red-900/60 px-4 py-2 text-sm text-red-400 hover:bg-red-950/40"
            >
              Delete
            </button>
          </div>

          <h3 className="mb-2 text-sm font-semibold text-zinc-300">Comments</h3>
          <ul className="mb-4 space-y-3">
            {comments.isLoading && <li className="text-sm text-zinc-500">Loading…</li>}
            {comments.data?.map((c) => (
              <li key={c.id} className="rounded-lg border border-zinc-800 bg-zinc-900/50 p-3 text-left text-sm">
                <div className="mb-1 text-xs text-zinc-500">
                  {c.user?.display_name ?? 'Member'} ·{' '}
                  {new Date(c.created_at).toLocaleString()}
                </div>
                <p className="whitespace-pre-wrap text-zinc-200">{c.body}</p>
              </li>
            ))}
          </ul>
          <form
            className="flex flex-col gap-2"
            onSubmit={(e) => {
              e.preventDefault()
              if (!commentBody.trim()) return
              addComment.mutate()
            }}
          >
            <textarea
              className="min-h-[72px] rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-violet-500"
              placeholder="Write a comment…"
              value={commentBody}
              onChange={(e) => setCommentBody(e.target.value)}
            />
            <button
              type="submit"
              disabled={addComment.isPending}
              className="self-start rounded-lg bg-zinc-800 px-3 py-1.5 text-sm text-white hover:bg-zinc-700 disabled:opacity-50"
            >
              Add comment
            </button>
          </form>
        </div>
      </aside>
    </div>
  )
}
