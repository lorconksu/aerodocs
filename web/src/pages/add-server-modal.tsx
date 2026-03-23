import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Copy, Check } from 'lucide-react'
import { apiFetch } from '@/lib/api'
import type { CreateServerRequest, CreateServerResponse } from '@/types/api'

interface AddServerModalProps {
  onClose: () => void
}

export function AddServerModal({ onClose }: AddServerModalProps) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [result, setResult] = useState<CreateServerResponse | null>(null)
  const [copied, setCopied] = useState(false)

  const createMutation = useMutation({
    mutationFn: (req: CreateServerRequest) =>
      apiFetch<CreateServerResponse>('/servers', {
        method: 'POST',
        body: JSON.stringify(req),
      }),
    onSuccess: (data) => {
      setResult(data)
      queryClient.invalidateQueries({ queryKey: ['servers'] })
    },
  })

  const handleGenerate = () => {
    if (!name.trim()) return
    createMutation.mutate({ name: name.trim() })
  }

  const handleCopy = async () => {
    if (!result) return
    await navigator.clipboard.writeText(result.install_command)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-surface border border-border rounded-lg w-full max-w-lg mx-4 p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-text-primary font-semibold">Add Server</h3>
          <button onClick={onClose} className="text-text-muted hover:text-text-primary">
            <X className="w-4 h-4" />
          </button>
        </div>

        {!result ? (
          /* Step 1: Enter name */
          <div>
            <label className="block text-sm text-text-secondary mb-1">Server Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleGenerate()}
              placeholder="e.g., web-prod-1"
              className="w-full px-3 py-2 bg-elevated border border-border rounded text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
              autoFocus
            />
            {createMutation.isError && (
              <p className="text-status-error text-xs mt-2">
                {createMutation.error?.message || 'Failed to create server'}
              </p>
            )}
            <div className="flex justify-end mt-4">
              <button
                onClick={handleGenerate}
                disabled={!name.trim() || createMutation.isPending}
                className="px-4 py-2 bg-accent hover:bg-accent-hover disabled:opacity-50 text-white text-sm rounded transition-colors"
              >
                {createMutation.isPending ? 'Generating...' : 'Generate'}
              </button>
            </div>
          </div>
        ) : (
          /* Step 2: Show install command */
          <div>
            <p className="text-sm text-text-secondary mb-3">
              Run this command on <span className="text-text-primary font-medium">{result.server.name}</span> to install the agent:
            </p>
            <div className="relative">
              <pre className="bg-base border border-border rounded p-3 text-xs font-mono text-text-secondary overflow-x-auto whitespace-pre-wrap break-all">
                {result.install_command}
              </pre>
              <button
                onClick={handleCopy}
                className="absolute top-2 right-2 p-1 bg-elevated border border-border rounded text-text-muted hover:text-text-primary transition-colors"
                title="Copy to clipboard"
              >
                {copied ? <Check className="w-3.5 h-3.5 text-status-online" /> : <Copy className="w-3.5 h-3.5" />}
              </button>
            </div>
            <p className="text-xs text-status-warning mt-2">
              Expires in 1 hour. Token is shown only once.
            </p>
            <div className="flex justify-end mt-4">
              <button
                onClick={onClose}
                className="px-4 py-2 bg-elevated hover:bg-border text-text-primary text-sm rounded transition-colors"
              >
                Close
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
