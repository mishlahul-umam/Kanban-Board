import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, setToken } from '../api/client'
import type { BoardSummary, User } from '../api/types'

export function BoardsPage() {
  const nav = useNavigate()
  const qc = useQueryClient()
  const [title, setTitle] = useState('')

  const me = useQuery({
    queryKey: ['me'],
    queryFn: () => api<User>('/auth/me'),
  })

  const boards = useQuery({
    queryKey: ['boards'],
    queryFn: () => api<BoardSummary[]>('/boards'),
  })

  const [createError, setCreateError] = useState<string | null>(null)

  const create = useMutation({
    mutationFn: (t: string) =>
      api<BoardSummary>('/boards', { method: 'POST', json: { title: t } }),
    onMutate: () => setCreateError(null),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['boards'] })
      setTitle('')
    },
    onError: (e: Error) => setCreateError(e.message),
  })

  const [deleteError, setDeleteError] = useState<string | null>(null)

  const deleteBoard = useMutation({
    mutationFn: (id: string) => api(`/boards/${id}`, { method: 'DELETE' }),
    onMutate: () => setDeleteError(null),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['boards'] }),
    onError: (e: Error) => setDeleteError(e.message),
  })

  function logout() {
    setToken(null)
    qc.clear()
    nav('/login')
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-10">
      <header className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">Boards</h1>
          {me.data && (
            <p className="text-sm text-zinc-500">Signed in as {me.data.display_name}</p>
          )}
        </div>
        <button
          type="button"
          onClick={logout}
          className="rounded-lg border border-zinc-700 px-3 py-1.5 text-sm text-zinc-300 hover:bg-zinc-900"
        >
          Log out
        </button>
      </header>

      <div className="mb-8 space-y-2">
        <form
          className="flex flex-col gap-2 sm:flex-row"
          onSubmit={(e) => {
            e.preventDefault()
            setCreateError(null)
            const t = title.trim()
            if (!t) {
              setCreateError('Enter a board title first.')
              return
            }
            create.mutate(t)
          }}
        >
          <input
            className="flex-1 rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-zinc-100 outline-none focus:border-violet-500"
            placeholder="New board title"
            value={title}
            onChange={(e) => {
              setTitle(e.target.value)
              if (createError) setCreateError(null)
            }}
            aria-invalid={!!createError}
            aria-describedby={createError ? 'board-create-error' : undefined}
          />
          <button
            type="submit"
            disabled={create.isPending}
            className="rounded-lg bg-violet-600 px-4 py-2 font-medium text-white hover:bg-violet-500 disabled:opacity-50"
          >
            {create.isPending ? 'Creating…' : 'Create board'}
          </button>
        </form>
        {createError && (
          <p id="board-create-error" className="text-sm text-red-400" role="alert">
            {createError}
          </p>
        )}
      </div>

      {boards.isLoading && <p className="text-zinc-500">Loading…</p>}
      {boards.error && (
        <p className="text-red-400">{(boards.error as Error).message}</p>
      )}
      <ul className="space-y-2">
        {boards.data?.map((b) => (
          <li
            key={b.id}
            className="flex items-center rounded-xl border border-zinc-800 bg-zinc-900/50 transition hover:border-violet-500/40 hover:bg-zinc-900"
          >
            <Link
              to={`/boards/${b.id}`}
              className="min-w-0 flex-1 px-4 py-3 text-left"
            >
              <span className="font-medium text-white">{b.title}</span>
            </Link>
            {me.data?.id === b.owner_id && (
              <button
                type="button"
                onClick={() => {
                  if (
                    confirm(
                      `Delete "${b.title}"? Its columns and tasks are deleted too.`,
                    )
                  )
                    deleteBoard.mutate(b.id)
                }}
                disabled={deleteBoard.isPending}
                aria-label={`Delete board ${b.title}`}
                className="mr-2 shrink-0 rounded-lg px-2 py-1 text-lg leading-none text-zinc-600 hover:bg-red-950/40 hover:text-red-400 disabled:opacity-50"
              >
                ×
              </button>
            )}
          </li>
        ))}
      </ul>
      {deleteError && (
        <p className="mt-3 text-sm text-red-400" role="alert">
          {deleteError}
        </p>
      )}
      {boards.data?.length === 0 && !boards.isLoading && (
        <p className="text-zinc-500">No boards yet. Create one above.</p>
      )}
    </div>
  )
}
