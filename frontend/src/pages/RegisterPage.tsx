import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation } from '@tanstack/react-query'
import { api, setToken } from '../api/client'
import type { User } from '../api/types'

export function RegisterPage() {
  const nav = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [err, setErr] = useState<string | null>(null)

  const m = useMutation({
    mutationFn: () =>
      api<{ token: string; user: User }>('/auth/register', {
        method: 'POST',
        json: { email, password, display_name: displayName },
      }),
    onSuccess: (d) => {
      setToken(d.token)
      nav('/')
    },
    onError: (e: Error) => setErr(e.message),
  })

  return (
    <div className="mx-auto flex min-h-screen max-w-md flex-col justify-center px-4">
      <h1 className="mb-6 text-2xl font-semibold tracking-tight text-white">Create account</h1>
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault()
          setErr(null)
          m.mutate()
        }}
      >
        <label className="block text-left text-sm text-zinc-400">
          Display name
          <input
            className="mt-1 w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-zinc-100 outline-none focus:border-violet-500"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            required
          />
        </label>
        <label className="block text-left text-sm text-zinc-400">
          Email
          <input
            className="mt-1 w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-zinc-100 outline-none focus:border-violet-500"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            required
          />
        </label>
        <label className="block text-left text-sm text-zinc-400">
          Password (min 8 characters)
          <input
            className="mt-1 w-full rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2 text-zinc-100 outline-none focus:border-violet-500"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
            minLength={8}
            required
          />
        </label>
        {err && <p className="text-sm text-red-400">{err}</p>}
        <button
          type="submit"
          disabled={m.isPending}
          className="rounded-lg bg-violet-600 px-4 py-2.5 font-medium text-white hover:bg-violet-500 disabled:opacity-50"
        >
          {m.isPending ? 'Creating…' : 'Register'}
        </button>
      </form>
      <p className="mt-6 text-center text-sm text-zinc-500">
        Already have an account?{' '}
        <Link className="text-violet-400 hover:underline" to="/login">
          Sign in
        </Link>
      </p>
    </div>
  )
}
