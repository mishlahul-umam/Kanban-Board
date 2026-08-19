import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, wsBoardUrl } from '../api/client'
import type { BoardDetail, Task, User } from '../api/types'
import { KanbanBoard, type ColumnError } from '../components/KanbanBoard'
import { TaskDrawer } from '../components/TaskDrawer'

function initials(name: string): string {
  return name
    .split(' ')
    .filter(Boolean)
    .slice(0, 2)
    .map((p) => p.charAt(0).toUpperCase())
    .join('')
}

export function BoardPage() {
  const { boardId = '' } = useParams()
  const qc = useQueryClient()
  const nav = useNavigate()
  const [selected, setSelected] = useState<Task | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [taskFilter, setTaskFilter] = useState<'all' | 'overdue' | 'soon'>('all')
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteError, setInviteError] = useState<string | null>(null)
  const [inviteNotice, setInviteNotice] = useState<string | null>(null)
  const [editingTitle, setEditingTitle] = useState(false)
  const [titleDraft, setTitleDraft] = useState('')
  const [boardError, setBoardError] = useState<string | null>(null)
  const [columnError, setColumnError] = useState<ColumnError | null>(null)

  const board = useQuery({
    queryKey: ['board', boardId],
    queryFn: () => api<BoardDetail>(`/boards/${boardId}`),
    enabled: !!boardId,
  })

  const me = useQuery({
    queryKey: ['me'],
    queryFn: () => api<User>('/auth/me'),
  })

  const ownerId = board.data?.owner_id
  const members = board.data?.members ?? []
  const isOwner = !!me.data && !!ownerId && me.data.id === ownerId

  const moveMut = useMutation({
    mutationFn: (p: { id: string; column_id: string; position: number }) =>
      api(`/tasks/${p.id}/move`, {
        method: 'PATCH',
        json: { column_id: p.column_id, position: p.position },
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board', boardId] }),
  })

  const createTaskMut = useMutation({
    mutationFn: (p: { columnId: string; title: string }) =>
      api<Task>(`/columns/${p.columnId}/tasks`, {
        method: 'POST',
        json: { title: p.title },
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board', boardId] }),
  })

  const inviteMut = useMutation({
    mutationFn: (email: string) =>
      api<User>(`/boards/${boardId}/members`, {
        method: 'POST',
        json: { email },
      }),
    onMutate: () => {
      setInviteError(null)
      setInviteNotice(null)
    },
    onSuccess: (u) => {
      setInviteEmail('')
      setInviteNotice(`${u.display_name} is on this board.`)
      qc.invalidateQueries({ queryKey: ['board', boardId] })
    },
    onError: (e: Error) => setInviteError(e.message),
  })

  const removeMemberMut = useMutation({
    mutationFn: (userId: string) =>
      api(`/boards/${boardId}/members/${userId}`, { method: 'DELETE' }),
    onMutate: () => {
      setInviteError(null)
      setInviteNotice(null)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board', boardId] }),
    onError: (e: Error) => setInviteError(e.message),
  })

  const renameMut = useMutation({
    mutationFn: (t: string) =>
      api(`/boards/${boardId}`, { method: 'PATCH', json: { title: t } }),
    onMutate: () => setBoardError(null),
    onSuccess: () => {
      setEditingTitle(false)
      qc.invalidateQueries({ queryKey: ['board', boardId] })
      qc.invalidateQueries({ queryKey: ['boards'] })
    },
    onError: (e: Error) => setBoardError(e.message),
  })

  const deleteBoardMut = useMutation({
    mutationFn: () => api(`/boards/${boardId}`, { method: 'DELETE' }),
    onMutate: () => setBoardError(null),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['boards'] })
      nav('/')
    },
    onError: (e: Error) => setBoardError(e.message),
  })

  const addColumnMut = useMutation({
    mutationFn: (t: string) =>
      api(`/boards/${boardId}/columns`, { method: 'POST', json: { title: t } }),
    onMutate: () => setColumnError(null),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board', boardId] }),
    onError: (e: Error) =>
      setColumnError({ columnId: null, message: e.message }),
  })

  const renameColumnMut = useMutation({
    mutationFn: (p: { columnId: string; title: string }) =>
      api(`/columns/${p.columnId}`, {
        method: 'PATCH',
        json: { title: p.title },
      }),
    onMutate: () => setColumnError(null),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board', boardId] }),
    onError: (e: Error, p) =>
      setColumnError({ columnId: p.columnId, message: e.message }),
  })

  const deleteColumnMut = useMutation({
    mutationFn: (columnId: string) =>
      api(`/columns/${columnId}`, { method: 'DELETE' }),
    onMutate: () => setColumnError(null),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board', boardId] }),
    onError: (e: Error, columnId) =>
      setColumnError({
        columnId,
        message:
          e.message === 'column has tasks'
            ? 'This column still has tasks. Move or delete them first.'
            : e.message,
      }),
  })

  useEffect(() => {
    if (!boardId || !board.data) return
    const url = wsBoardUrl(boardId)
    let ws: WebSocket
    try {
      ws = new WebSocket(url)
    } catch {
      return
    }
    ws.onmessage = () => {
      qc.invalidateQueries({ queryKey: ['board', boardId] })
    }
    return () => {
      ws.close()
    }
  }, [boardId, board.data, qc])

  const title = useMemo(() => board.data?.title ?? 'Board', [board.data?.title])

  function openTask(t: Task) {
    setSelected(t)
    setDrawerOpen(true)
  }

  function startEditTitle() {
    setTitleDraft(board.data?.title ?? '')
    setBoardError(null)
    setEditingTitle(true)
  }

  function cancelEditTitle() {
    setEditingTitle(false)
    setBoardError(null)
  }

  return (
    <div className="min-h-screen px-4 py-6">
      <header className="mx-auto mb-6 flex max-w-[1200px] flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <Link to="/" className="text-sm text-violet-400 hover:underline">
            ← Boards
          </Link>
          {editingTitle ? (
            <form
              className="mt-1 flex flex-wrap items-center gap-2"
              onSubmit={(e) => {
                e.preventDefault()
                const t = titleDraft.trim()
                if (!t) {
                  setBoardError('Board title cannot be empty.')
                  return
                }
                if (t === board.data?.title) {
                  cancelEditTitle()
                  return
                }
                renameMut.mutate(t)
              }}
            >
              <input
                autoFocus
                className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-1.5 text-2xl font-semibold text-white outline-none focus:border-violet-500 sm:w-96"
                value={titleDraft}
                aria-label="Board title"
                onChange={(e) => {
                  setTitleDraft(e.target.value)
                  if (boardError) setBoardError(null)
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Escape') cancelEditTitle()
                }}
                aria-invalid={!!boardError}
                aria-describedby={boardError ? 'board-error' : undefined}
              />
              <button
                type="submit"
                disabled={renameMut.isPending}
                className="rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white hover:bg-violet-500 disabled:opacity-50"
              >
                {renameMut.isPending ? 'Saving…' : 'Save'}
              </button>
              <button
                type="button"
                onClick={cancelEditTitle}
                className="rounded-lg border border-zinc-700 px-3 py-2 text-sm text-zinc-300 hover:bg-zinc-900"
              >
                Cancel
              </button>
            </form>
          ) : (
            <h1 className="mt-1 text-2xl font-semibold text-white">
              <button
                type="button"
                onClick={startEditTitle}
                disabled={!board.data}
                aria-label={`Rename board: ${title}`}
                className="group flex max-w-full items-center gap-2 text-left disabled:cursor-default"
              >
                <span className="truncate">{title}</span>
                {board.data && (
                  <span
                    aria-hidden="true"
                    className="shrink-0 text-base text-zinc-600 transition group-hover:text-violet-400"
                  >
                    ✎
                  </span>
                )}
              </button>
            </h1>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs text-zinc-500">Highlight:</span>
          {(['all', 'overdue', 'soon'] as const).map((k) => (
            <button
              key={k}
              type="button"
              onClick={() => setTaskFilter(k)}
              className={`rounded-full px-3 py-1 text-xs font-medium ${
                taskFilter === k
                  ? 'bg-violet-600 text-white'
                  : 'bg-zinc-800 text-zinc-400 hover:bg-zinc-700'
              }`}
            >
              {k === 'all' ? 'All' : k === 'overdue' ? 'Overdue' : 'Due ≤7d'}
            </button>
          ))}
          {isOwner && (
            <button
              type="button"
              onClick={() => {
                if (
                  confirm(
                    `Delete "${board.data?.title ?? 'this board'}"? Its columns and tasks are deleted too.`,
                  )
                )
                  deleteBoardMut.mutate()
              }}
              disabled={deleteBoardMut.isPending}
              className="rounded-lg border border-red-900/60 px-3 py-1 text-xs text-red-400 hover:bg-red-950/40 disabled:opacity-50"
            >
              {deleteBoardMut.isPending ? 'Deleting…' : 'Delete board'}
            </button>
          )}
        </div>
      </header>

      {boardError && (
        <p
          id="board-error"
          className="mx-auto mb-4 max-w-[1200px] text-sm text-red-400"
          role="alert"
        >
          {boardError}
        </p>
      )}

      {board.isLoading && <p className="text-zinc-500">Loading board…</p>}
      {board.error && (
        <p className="text-red-400">{(board.error as Error).message}</p>
      )}
      {board.data && (
        <section className="mx-auto mb-6 max-w-[1200px] rounded-xl border border-zinc-800 bg-zinc-900/50 p-4">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <h2 className="mb-2 text-xs font-medium uppercase tracking-wide text-zinc-500">
                Members ({members.length})
              </h2>
              <ul className="flex flex-wrap gap-2">
                {members.map((m) => (
                  <li
                    key={m.id}
                    className="flex items-center gap-2 rounded-full border border-zinc-800 bg-zinc-900 py-1 pl-1 pr-3"
                  >
                    <span
                      className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-violet-600 text-[11px] font-medium text-white"
                      aria-hidden="true"
                    >
                      {initials(m.display_name)}
                    </span>
                    <span className="text-sm text-zinc-200" title={m.email}>
                      {m.display_name}
                    </span>
                    {m.id === ownerId ? (
                      <span className="rounded-full bg-zinc-800 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wide text-zinc-400">
                        Owner
                      </span>
                    ) : (
                      isOwner && (
                        <button
                          type="button"
                          onClick={() => {
                            if (confirm(`Remove ${m.display_name} from this board?`))
                              removeMemberMut.mutate(m.id)
                          }}
                          disabled={removeMemberMut.isPending}
                          aria-label={`Remove ${m.display_name}`}
                          className="text-xs text-zinc-500 hover:text-red-400 disabled:opacity-50"
                        >
                          ✕
                        </button>
                      )
                    )}
                  </li>
                ))}
              </ul>
            </div>

            {isOwner && (
              <form
                className="flex w-full gap-2 lg:w-auto"
                onSubmit={(e) => {
                  e.preventDefault()
                  setInviteNotice(null)
                  const email = inviteEmail.trim()
                  if (!email) {
                    setInviteError('Enter an email address first.')
                    return
                  }
                  inviteMut.mutate(email)
                }}
              >
                <input
                  type="email"
                  className="w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-sm text-zinc-100 outline-none focus:border-violet-500 lg:w-64"
                  placeholder="teammate@example.com"
                  aria-label="Invite member by email"
                  value={inviteEmail}
                  onChange={(e) => {
                    setInviteEmail(e.target.value)
                    if (inviteError) setInviteError(null)
                  }}
                  aria-invalid={!!inviteError}
                  aria-describedby={inviteError ? 'invite-error' : undefined}
                />
                <button
                  type="submit"
                  disabled={inviteMut.isPending}
                  className="shrink-0 rounded-lg bg-violet-600 px-4 py-2 text-sm font-medium text-white hover:bg-violet-500 disabled:opacity-50"
                >
                  {inviteMut.isPending ? 'Inviting…' : 'Invite'}
                </button>
              </form>
            )}
          </div>

          {inviteError && (
            <p id="invite-error" className="mt-3 text-sm text-red-400" role="alert">
              {inviteError}
            </p>
          )}
          {inviteNotice && (
            <p className="mt-3 text-sm text-emerald-400" role="status">
              {inviteNotice}
            </p>
          )}
        </section>
      )}

      {board.data && (
        <div className="mx-auto max-w-[1200px]">
          <KanbanBoard
            board={board.data}
            taskFilter={taskFilter}
            onMove={(taskId, column_id, position) =>
              moveMut.mutate({ id: taskId, column_id, position })
            }
            onAddTask={(columnId, t) => createTaskMut.mutate({ columnId, title: t })}
            onOpenTask={openTask}
            onAddColumn={(t) => addColumnMut.mutate(t)}
            onRenameColumn={(columnId, t) =>
              renameColumnMut.mutate({ columnId, title: t })
            }
            onDeleteColumn={(columnId) => deleteColumnMut.mutate(columnId)}
            onClearColumnError={() => setColumnError(null)}
            columnError={columnError}
          />
        </div>
      )}

      {board.data && (
        <TaskDrawer
          board={board.data}
          task={selected}
          open={drawerOpen}
          onClose={() => {
            setDrawerOpen(false)
            setSelected(null)
          }}
        />
      )}
    </div>
  )
}
