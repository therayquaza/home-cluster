import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { publicClient } from '../api'
import { useToast } from '../useToast'

export default function Login() {
  const navigate = useNavigate()
  const { push } = useToast()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      if (mode === 'register') {
        await publicClient.register({ email, password, display_name: displayName })
        push('Account created', 'success')
      } else {
        await publicClient.login({ email, password })
        push('Signed in', 'success')
      }
      navigate('/today', { replace: true })
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Something went wrong'
      setError(message)
      push(message, 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-6 bg-brand-bg px-6">
      <h1 className="text-4xl font-extrabold text-brand-500">dinks</h1>
      <div className="w-full max-w-sm rounded-3xl bg-white p-6 shadow-lg shadow-brand-100">
        <div className="mb-4 flex rounded-full bg-brand-50 p-1 text-sm font-medium">
          <button type="button" className={`flex-1 rounded-full py-2 ${mode === 'login' ? 'bg-brand-500 text-white' : 'text-brand-700'}`} onClick={() => setMode('login')}>
            Sign in
          </button>
          <button type="button" className={`flex-1 rounded-full py-2 ${mode === 'register' ? 'bg-brand-500 text-white' : 'text-brand-700'}`} onClick={() => setMode('register')}>
            Register
          </button>
        </div>
        <form className="flex flex-col gap-3" onSubmit={submit}>
          {mode === 'register' && (
            <input
              className="rounded-xl border border-brand-100 px-3 py-2 outline-none focus:border-brand-500"
              placeholder="Display name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
            />
          )}
          <input
            className="rounded-xl border border-brand-100 px-3 py-2 outline-none focus:border-brand-500"
            type="email"
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          <input
            className="rounded-xl border border-brand-100 px-3 py-2 outline-none focus:border-brand-500"
            type="password"
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            minLength={8}
            required
          />
          {error && <p className="text-sm text-red-500">{error}</p>}
          <button className="rounded-xl bg-brand-500 py-2 font-semibold text-white disabled:opacity-50" disabled={busy} type="submit">
            {mode === 'register' ? 'Create account' : 'Sign in'}
          </button>
        </form>
        <div className="my-4 flex items-center gap-2 text-xs text-slate-400">
          <div className="h-px flex-1 bg-slate-200" /> or <div className="h-px flex-1 bg-slate-200" />
        </div>
        <a className="block rounded-xl border border-brand-100 py-2 text-center font-medium text-brand-700" href="/auth/login">
          Continue with Keycloak
        </a>
      </div>
      <p className="max-w-sm text-center text-xs text-slate-400">Estimates are not medical advice.</p>
    </div>
  )
}
