import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react'

type ToastVariant = 'success' | 'error'
type Toast = { id: number; message: string; variant: ToastVariant }
type ToastContextValue = { toasts: Toast[]; push: (message: string, variant?: ToastVariant) => void; dismiss: (id: number) => void }

const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const idRef = useRef(0)

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const push = useCallback(
    (message: string, variant: ToastVariant = 'success') => {
      const id = ++idRef.current
      setToasts((prev) => [...prev, { id, message, variant }])
      setTimeout(() => dismiss(id), 3500)
    },
    [dismiss],
  )

  return <ToastContext.Provider value={{ toasts, push, dismiss }}>{children}</ToastContext.Provider>
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}

export function ToastHost() {
  const { toasts, dismiss } = useToast()
  if (toasts.length === 0) return null
  return (
    <div className="pointer-events-none fixed inset-x-0 top-3 z-[100] flex flex-col items-center gap-2 px-4">
      {toasts.map((t) => (
        <button
          key={t.id}
          onClick={() => dismiss(t.id)}
          className={`pointer-events-auto w-full max-w-sm rounded-2xl px-4 py-3 text-left text-sm font-medium shadow-lg transition ${
            t.variant === 'error' ? 'bg-red-600 text-white' : 'bg-brand-500 text-white'
          }`}
        >
          {t.message}
        </button>
      ))}
    </div>
  )
}

/** Small helper: run an async mutation, toast on success/error, rethrow on error so callers can still branch. */
export function useMutationToast() {
  const { push } = useToast()
  return useCallback(
    async <T,>(action: () => Promise<T>, successMessage: string, errorPrefix = 'Failed'): Promise<T> => {
      try {
        const result = await action()
        push(successMessage, 'success')
        return result
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Something went wrong'
        push(`${errorPrefix}: ${message}`, 'error')
        throw err
      }
    },
    [push],
  )
}
