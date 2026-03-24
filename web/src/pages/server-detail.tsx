import { useState, useEffect, useMemo, useCallback } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  ArrowLeft,
  ChevronRight,
  ChevronDown,
  Folder,
  File,
  RefreshCw,
  Eye,
  Code,
  Loader2,
  Trash2,
  Plus,
  AlertTriangle,
  FolderOpen,
} from 'lucide-react'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import css from 'highlight.js/lib/languages/css'
import dockerfile from 'highlight.js/lib/languages/dockerfile'
import go from 'highlight.js/lib/languages/go'
import ini from 'highlight.js/lib/languages/ini'
import javascript from 'highlight.js/lib/languages/javascript'
import json from 'highlight.js/lib/languages/json'
import markdown from 'highlight.js/lib/languages/markdown'
import nginx from 'highlight.js/lib/languages/nginx'
import plaintext from 'highlight.js/lib/languages/plaintext'
import python from 'highlight.js/lib/languages/python'
import shell from 'highlight.js/lib/languages/shell'
import sql from 'highlight.js/lib/languages/sql'
import typescript from 'highlight.js/lib/languages/typescript'
import xml from 'highlight.js/lib/languages/xml'
import yaml from 'highlight.js/lib/languages/yaml'
import 'highlight.js/styles/github-dark.css'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { apiFetch } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import type {
  Server,
  ServerStatus,
  FileNode,
  FileListResponse,
  FileReadResponse,
  PathPermission,
  PathListResponse,
  CreatePathRequest,
  User,
} from '@/types/api'

// Register highlight.js languages
hljs.registerLanguage('bash', bash)
hljs.registerLanguage('css', css)
hljs.registerLanguage('dockerfile', dockerfile)
hljs.registerLanguage('go', go)
hljs.registerLanguage('ini', ini)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('json', json)
hljs.registerLanguage('markdown', markdown)
hljs.registerLanguage('nginx', nginx)
hljs.registerLanguage('plaintext', plaintext)
hljs.registerLanguage('python', python)
hljs.registerLanguage('shell', shell)
hljs.registerLanguage('sql', sql)
hljs.registerLanguage('typescript', typescript)
hljs.registerLanguage('xml', xml)
hljs.registerLanguage('yaml', yaml)

// --- Utilities ---

const statusDot: Record<ServerStatus, string> = {
  online: 'text-status-online',
  offline: 'text-status-offline',
  pending: 'text-status-warning',
}

function relativeTime(dateStr: string | null): string {
  if (!dateStr) return '--'
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin} min ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDay = Math.floor(diffHr / 24)
  return `${diffDay}d ago`
}

function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const value = bytes / Math.pow(1024, i)
  return `${value.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function extensionToLanguage(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() ?? ''
  const map: Record<string, string> = {
    js: 'javascript',
    jsx: 'javascript',
    ts: 'typescript',
    tsx: 'typescript',
    py: 'python',
    go: 'go',
    sh: 'bash',
    bash: 'bash',
    zsh: 'bash',
    json: 'json',
    yaml: 'yaml',
    yml: 'yaml',
    xml: 'xml',
    html: 'xml',
    htm: 'xml',
    svg: 'xml',
    css: 'css',
    scss: 'css',
    sql: 'sql',
    md: 'markdown',
    markdown: 'markdown',
    ini: 'ini',
    toml: 'ini',
    conf: 'ini',
    cfg: 'ini',
    properties: 'ini',
    dockerfile: 'dockerfile',
    nginx: 'nginx',
    log: 'plaintext',
    txt: 'plaintext',
    env: 'bash',
  }
  // Handle Dockerfile specifically
  if (filename.toLowerCase() === 'dockerfile' || filename.toLowerCase().startsWith('dockerfile.')) {
    return 'dockerfile'
  }
  return map[ext] || 'plaintext'
}

function isMarkdownFile(path: string): boolean {
  return /\.(md|markdown)$/i.test(path)
}

/**
 * Sanitize highlight.js output.
 *
 * highlight.js already escapes all user-supplied text (& < > " ')
 * and only emits <span class="hljs-..."> wrappers, so its output
 * is safe by design. This helper is a defense-in-depth measure that
 * strips any tag that is not an hljs <span>.
 */
function sanitizeHljsHtml(html: string): string {
  // Allow only <span ...> and </span> tags; strip everything else.
  return html.replace(/<\/?(?!span[\s>]|\/span>)[a-z][^>]*>/gi, '')
}

// --- Tree Node State ---

interface TreeNodeState {
  loading: boolean
  expanded: boolean
  children: FileNode[] | null
  error: string | null
}

// --- FileTreeNode Component ---

function FileTreeNode({
  node,
  depth,
  serverId,
  selectedPath,
  treeState,
  onToggleDir,
  onSelectFile,
}: {
  node: FileNode
  depth: number
  serverId: string
  selectedPath: string | null
  treeState: Record<string, TreeNodeState>
  onToggleDir: (path: string) => void
  onSelectFile: (node: FileNode) => void
}) {
  const state = treeState[node.path]
  const isExpanded = state?.expanded ?? false
  const isLoading = state?.loading ?? false
  const children = state?.children ?? null
  const isSelected = selectedPath === node.path

  if (node.is_dir) {
    return (
      <div>
        <button
          onClick={() => onToggleDir(node.path)}
          className={`w-full flex items-center gap-1 px-2 py-1 text-sm hover:bg-elevated/80 transition-colors text-left ${
            isSelected ? 'bg-elevated text-accent' : 'text-text-secondary'
          }`}
          style={{ paddingLeft: `${depth * 16 + 8}px` }}
          disabled={isLoading}
        >
          {isLoading ? (
            <Loader2 className="w-3.5 h-3.5 shrink-0 animate-spin text-text-faint" />
          ) : isExpanded ? (
            <ChevronDown className="w-3.5 h-3.5 shrink-0 text-text-faint" />
          ) : (
            <ChevronRight className="w-3.5 h-3.5 shrink-0 text-text-faint" />
          )}
          {isExpanded ? (
            <FolderOpen className="w-4 h-4 shrink-0 text-status-warning" />
          ) : (
            <Folder className="w-4 h-4 shrink-0 text-status-warning" />
          )}
          <span className="truncate ml-1">{node.name}</span>
        </button>
        {isExpanded && children && (
          <div>
            {children.length === 0 ? (
              <div
                className="text-text-faint text-xs italic px-2 py-1"
                style={{ paddingLeft: `${(depth + 1) * 16 + 28}px` }}
              >
                Empty directory
              </div>
            ) : (
              children.map((child) => (
                <FileTreeNode
                  key={child.path}
                  node={child}
                  depth={depth + 1}
                  serverId={serverId}
                  selectedPath={selectedPath}
                  treeState={treeState}
                  onToggleDir={onToggleDir}
                  onSelectFile={onSelectFile}
                />
              ))
            )}
          </div>
        )}
        {state?.error && (
          <div
            className="text-status-error text-xs px-2 py-1"
            style={{ paddingLeft: `${(depth + 1) * 16 + 28}px` }}
          >
            {state.error}
          </div>
        )}
      </div>
    )
  }

  return (
    <button
      onClick={() => node.readable && onSelectFile(node)}
      className={`w-full flex items-center gap-1 px-2 py-1 text-sm transition-colors text-left ${
        isSelected
          ? 'bg-accent/15 text-accent'
          : node.readable
            ? 'text-text-secondary hover:bg-elevated/80'
            : 'text-text-faint cursor-not-allowed'
      }`}
      style={{ paddingLeft: `${depth * 16 + 8}px` }}
      disabled={!node.readable}
      title={!node.readable ? 'File not readable' : undefined}
    >
      <span className="w-3.5 shrink-0" />
      <File className="w-4 h-4 shrink-0 text-text-faint" />
      <span className="truncate ml-1">{node.name}</span>
    </button>
  )
}

// --- PathManagement Component (Admin Only) ---

function PathManagement({ serverId }: { serverId: string }) {
  const queryClient = useQueryClient()
  const [expanded, setExpanded] = useState(false)
  const [newUserId, setNewUserId] = useState('')
  const [newPath, setNewPath] = useState('')

  const { data: pathsData, isLoading: pathsLoading } = useQuery({
    queryKey: ['server-paths', serverId],
    queryFn: () => apiFetch<PathListResponse>(`/servers/${serverId}/paths`),
    enabled: expanded,
  })

  const { data: usersData } = useQuery({
    queryKey: ['users'],
    queryFn: () => apiFetch<{ users: User[] }>('/users'),
    enabled: expanded,
  })

  const addMutation = useMutation({
    mutationFn: (data: CreatePathRequest) =>
      apiFetch<PathPermission>(`/servers/${serverId}/paths`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['server-paths', serverId] })
      setNewUserId('')
      setNewPath('')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (pathId: string) =>
      apiFetch(`/servers/${serverId}/paths/${pathId}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['server-paths', serverId] })
    },
  })

  const handleAdd = (e: React.FormEvent) => {
    e.preventDefault()
    if (!newUserId || !newPath) return
    addMutation.mutate({ user_id: newUserId, path: newPath })
  }

  const paths = pathsData?.paths ?? []
  const users = usersData?.users ?? []

  return (
    <div className="border border-border rounded">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-text-primary hover:bg-elevated/50 transition-colors"
      >
        <span>Manage File Access</span>
        {expanded ? (
          <ChevronDown className="w-4 h-4 text-text-muted" />
        ) : (
          <ChevronRight className="w-4 h-4 text-text-muted" />
        )}
      </button>

      {expanded && (
        <div className="border-t border-border px-4 py-3 space-y-4">
          {/* Add Path Form */}
          <form onSubmit={handleAdd} className="flex items-end gap-3">
            <div className="flex-1">
              <label className="block text-xs text-text-muted mb-1">User</label>
              <select
                value={newUserId}
                onChange={(e) => setNewUserId(e.target.value)}
                className="w-full bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:border-accent"
              >
                <option value="">Select user...</option>
                {users.map((u) => (
                  <option key={u.id} value={u.id}>
                    {u.username}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex-1">
              <label className="block text-xs text-text-muted mb-1">Path</label>
              <input
                type="text"
                placeholder="/var/log"
                value={newPath}
                onChange={(e) => setNewPath(e.target.value)}
                className="w-full bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent font-mono"
              />
            </div>
            <button
              type="submit"
              disabled={addMutation.isPending || !newUserId || !newPath}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-accent hover:bg-accent-hover text-white text-sm rounded transition-colors disabled:opacity-50"
            >
              <Plus className="w-3.5 h-3.5" />
              {addMutation.isPending ? 'Adding...' : 'Add Path'}
            </button>
          </form>

          {addMutation.isError && (
            <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2">
              {addMutation.error instanceof Error ? addMutation.error.message : 'Failed to add path'}
            </div>
          )}

          {/* Paths Table */}
          {pathsLoading ? (
            <div className="text-text-muted text-sm py-4 text-center">Loading permissions...</div>
          ) : paths.length === 0 ? (
            <div className="text-text-muted text-sm py-4 text-center">
              No path permissions configured yet.
            </div>
          ) : (
            <div className="border border-border rounded overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="bg-elevated text-text-muted text-xs uppercase tracking-wider">
                    <th className="px-3 py-2 text-left">Username</th>
                    <th className="px-3 py-2 text-left">Path</th>
                    <th className="px-3 py-2 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {paths.map((p) => (
                    <tr key={p.id} className="hover:bg-elevated/50 transition-colors">
                      <td className="px-3 py-2 text-text-primary">{p.username}</td>
                      <td className="px-3 py-2 font-mono text-text-secondary text-xs">{p.path}</td>
                      <td className="px-3 py-2 text-right">
                        <button
                          onClick={() => deleteMutation.mutate(p.id)}
                          disabled={deleteMutation.isPending}
                          className="text-text-muted hover:text-status-offline transition-colors"
                          title="Remove permission"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// --- Main Page Component ---

export function ServerDetailPage() {
  const { id: serverId } = useParams<{ id: string }>()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'

  // File tree state
  const [treeState, setTreeState] = useState<Record<string, TreeNodeState>>({})
  const [selectedFile, setSelectedFile] = useState<FileNode | null>(null)
  const [markdownView, setMarkdownView] = useState<'raw' | 'rendered'>('rendered')

  // Server info query
  const {
    data: server,
    isLoading: serverLoading,
    error: serverError,
  } = useQuery({
    queryKey: ['server', serverId],
    queryFn: () => apiFetch<Server>(`/servers/${serverId}`),
    refetchInterval: 10_000,
    enabled: !!serverId,
  })

  // User's allowed root paths
  const {
    data: myPathsData,
    isLoading: pathsLoading,
  } = useQuery({
    queryKey: ['my-paths', serverId],
    queryFn: () => apiFetch<{ paths: string[] }>(`/servers/${serverId}/my-paths`),
    enabled: !!serverId && server?.status === 'online',
  })

  const rootPaths = myPathsData?.paths ?? []

  // Build root nodes from paths
  const rootNodes: FileNode[] = useMemo(
    () =>
      rootPaths.map((p) => ({
        name: p,
        path: p,
        is_dir: true,
        size: 0,
        readable: true,
      })),
    [rootPaths],
  )

  // File content query
  const {
    data: fileContent,
    isLoading: fileLoading,
    error: fileError,
    refetch: refetchFile,
  } = useQuery({
    queryKey: ['file-content', serverId, selectedFile?.path],
    queryFn: () =>
      apiFetch<FileReadResponse>(
        `/servers/${serverId}/files/read?path=${encodeURIComponent(selectedFile!.path)}`,
      ),
    enabled: !!serverId && !!selectedFile && !selectedFile.is_dir,
  })

  // Decode file content
  const decodedContent = useMemo(() => {
    if (!fileContent?.data) return null
    try {
      return atob(fileContent.data)
    } catch {
      return '[Unable to decode file content]'
    }
  }, [fileContent])

  // Syntax-highlighted HTML
  // highlight.js escapes all HTML entities in source text and only produces
  // <span class="hljs-..."> wrappers, so its output is safe by construction.
  // We additionally strip any non-span tags via sanitizeHljsHtml as defense-in-depth.
  const highlightedHtml = useMemo(() => {
    if (!decodedContent || !selectedFile) return ''
    const lang = extensionToLanguage(selectedFile.name)
    try {
      const result = hljs.highlight(decodedContent, { language: lang })
      return sanitizeHljsHtml(result.value)
    } catch {
      // Fallback to plaintext
      try {
        const result = hljs.highlight(decodedContent, { language: 'plaintext' })
        return sanitizeHljsHtml(result.value)
      } catch {
        return decodedContent
          .replace(/&/g, '&amp;')
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;')
      }
    }
  }, [decodedContent, selectedFile])

  // Toggle directory expansion
  const handleToggleDir = useCallback(
    async (dirPath: string) => {
      const current = treeState[dirPath]
      if (current?.expanded) {
        // Collapse
        setTreeState((prev) => ({
          ...prev,
          [dirPath]: { ...prev[dirPath], expanded: false },
        }))
        return
      }

      // If already loaded, just expand
      if (current?.children !== null && current?.children !== undefined) {
        setTreeState((prev) => ({
          ...prev,
          [dirPath]: { ...prev[dirPath], expanded: true },
        }))
        return
      }

      // Fetch children
      setTreeState((prev) => ({
        ...prev,
        [dirPath]: { loading: true, expanded: true, children: null, error: null },
      }))

      try {
        const data = await apiFetch<FileListResponse>(
          `/servers/${serverId}/files?path=${encodeURIComponent(dirPath)}`,
        )
        // Sort: directories first, then alphabetically
        const sorted = [...data.files].sort((a, b) => {
          if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1
          return a.name.localeCompare(b.name)
        })
        setTreeState((prev) => ({
          ...prev,
          [dirPath]: { loading: false, expanded: true, children: sorted, error: null },
        }))
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'Failed to list directory'
        setTreeState((prev) => ({
          ...prev,
          [dirPath]: { loading: false, expanded: true, children: null, error: msg },
        }))
      }
    },
    [serverId, treeState],
  )

  const handleSelectFile = useCallback((node: FileNode) => {
    setSelectedFile(node)
    setMarkdownView('rendered')
  }, [])

  // Reset tree state when server changes
  useEffect(() => {
    setTreeState({})
    setSelectedFile(null)
  }, [serverId])

  // Breadcrumb segments
  const breadcrumbs = useMemo(() => {
    if (!selectedFile) return []
    const parts = selectedFile.path.split('/').filter(Boolean)
    return parts.map((part, i) => ({
      name: part,
      path: '/' + parts.slice(0, i + 1).join('/'),
    }))
  }, [selectedFile])

  // Partial-file banner
  const isPartialFile =
    fileContent && decodedContent && fileContent.total_size > decodedContent.length

  // --- Render ---

  if (serverLoading) {
    return (
      <div className="flex items-center justify-center py-16">
        <Loader2 className="w-5 h-5 animate-spin text-text-muted" />
        <span className="ml-2 text-text-muted text-sm">Loading server...</span>
      </div>
    )
  }

  if (serverError || !server) {
    return (
      <div className="text-center py-16">
        <AlertTriangle className="w-8 h-8 text-status-error mx-auto mb-3" />
        <p className="text-text-primary text-sm mb-1">Failed to load server</p>
        <p className="text-text-muted text-xs mb-4">
          {serverError instanceof Error ? serverError.message : 'Server not found'}
        </p>
        <Link to="/" className="text-accent text-sm hover:underline">
          Back to dashboard
        </Link>
      </div>
    )
  }

  const isOffline = server.status !== 'online'

  return (
    <div className="flex flex-col h-[calc(100vh-92px)]">
      {/* Header Bar */}
      <div className="flex items-center justify-between px-4 py-3 bg-surface border-b border-border shrink-0">
        <div className="flex items-center gap-4">
          <Link
            to="/"
            className="text-text-muted hover:text-text-secondary transition-colors"
            title="Back to dashboard"
          >
            <ArrowLeft className="w-4 h-4" />
          </Link>
          <div>
            <div className="flex items-center gap-2">
              <span className={statusDot[server.status]}>&#x25CF;</span>
              <h1 className="text-lg font-semibold text-text-primary">{server.name}</h1>
            </div>
            <div className="flex items-center gap-3 text-xs text-text-muted mt-0.5">
              {(server.hostname || server.ip_address) && (
                <span className="font-mono">
                  {server.hostname ?? '--'} / {server.ip_address ?? '--'}
                </span>
              )}
              {server.os && <span>{server.os}</span>}
              {server.agent_version && <span>v{server.agent_version}</span>}
              <span>Last seen: {relativeTime(server.last_seen_at)}</span>
            </div>
          </div>
        </div>
      </div>

      {/* Offline banner */}
      {isOffline && (
        <div className="bg-status-error/10 border-b border-status-error/20 px-4 py-2.5 flex items-center gap-2 shrink-0">
          <AlertTriangle className="w-4 h-4 text-status-error" />
          <span className="text-status-error text-sm">
            Server is offline. File browsing is unavailable.
          </span>
        </div>
      )}

      {/* Main content area */}
      {!isOffline && (
        <div className="flex flex-1 min-h-0">
          {/* Left Sidebar - File Tree */}
          <div className="w-72 border-r border-border flex flex-col bg-surface/30 shrink-0">
            <div className="px-3 py-2 border-b border-border text-xs text-text-muted uppercase tracking-wider font-medium">
              File Explorer
            </div>
            <div className="flex-1 overflow-y-auto">
              {pathsLoading ? (
                <div className="flex items-center gap-2 px-3 py-4 text-text-muted text-sm">
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Loading paths...
                </div>
              ) : rootPaths.length === 0 ? (
                <div className="px-3 py-4 text-text-muted text-sm">
                  No paths configured. Ask an admin to grant access.
                </div>
              ) : (
                <div className="py-1">
                  {rootNodes.map((node) => (
                    <FileTreeNode
                      key={node.path}
                      node={node}
                      depth={0}
                      serverId={serverId!}
                      selectedPath={selectedFile?.path ?? null}
                      treeState={treeState}
                      onToggleDir={handleToggleDir}
                      onSelectFile={handleSelectFile}
                    />
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Right Pane - File Viewer */}
          <div className="flex-1 flex flex-col min-w-0">
            {!selectedFile ? (
              <div className="flex-1 flex items-center justify-center">
                <div className="text-center">
                  <File className="w-10 h-10 text-text-faint mx-auto mb-3" />
                  <p className="text-text-muted text-sm">Select a file to view its contents</p>
                </div>
              </div>
            ) : (
              <>
                {/* Breadcrumb + controls */}
                <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-surface/50 shrink-0">
                  <div className="flex items-center gap-1 text-xs text-text-muted overflow-x-auto min-w-0">
                    {breadcrumbs.map((crumb, i) => (
                      <span key={crumb.path} className="flex items-center gap-1 shrink-0">
                        {i > 0 && <span className="text-text-faint">/</span>}
                        <span
                          className={
                            i === breadcrumbs.length - 1
                              ? 'text-text-primary font-medium'
                              : 'text-text-muted'
                          }
                        >
                          {crumb.name}
                        </span>
                      </span>
                    ))}
                  </div>
                  <div className="flex items-center gap-2 shrink-0 ml-2">
                    {fileContent && (
                      <span className="text-xs text-text-faint">
                        {formatFileSize(fileContent.total_size)}
                        {fileContent.mime_type && ` \u00B7 ${fileContent.mime_type}`}
                      </span>
                    )}
                    {isMarkdownFile(selectedFile.path) && (
                      <button
                        onClick={() =>
                          setMarkdownView((v) => (v === 'raw' ? 'rendered' : 'raw'))
                        }
                        className="flex items-center gap-1 px-2 py-0.5 text-xs bg-elevated border border-border rounded text-text-secondary hover:text-text-primary transition-colors"
                        title={markdownView === 'raw' ? 'Show rendered' : 'Show raw'}
                      >
                        {markdownView === 'raw' ? (
                          <>
                            <Eye className="w-3 h-3" />
                            Rendered
                          </>
                        ) : (
                          <>
                            <Code className="w-3 h-3" />
                            Raw
                          </>
                        )}
                      </button>
                    )}
                    <button
                      onClick={() => refetchFile()}
                      disabled={fileLoading}
                      className="p-1 text-text-muted hover:text-text-primary transition-colors"
                      title="Refresh file"
                    >
                      <RefreshCw
                        className={`w-3.5 h-3.5 ${fileLoading ? 'animate-spin' : ''}`}
                      />
                    </button>
                  </div>
                </div>

                {/* Partial file banner */}
                {isPartialFile && (
                  <div className="bg-status-warning/10 border-b border-status-warning/20 px-4 py-1.5 text-xs text-status-warning shrink-0">
                    Showing last 1 MB of {formatFileSize(fileContent!.total_size)}
                  </div>
                )}

                {/* File content */}
                <div className="flex-1 overflow-auto">
                  {fileLoading ? (
                    <div className="flex items-center justify-center py-16">
                      <Loader2 className="w-5 h-5 animate-spin text-text-muted" />
                      <span className="ml-2 text-text-muted text-sm">Loading file...</span>
                    </div>
                  ) : fileError ? (
                    <div className="flex flex-col items-center justify-center py-16">
                      <AlertTriangle className="w-6 h-6 text-status-error mb-2" />
                      <p className="text-text-primary text-sm mb-1">Failed to load file</p>
                      <p className="text-text-muted text-xs mb-3">
                        {fileError instanceof Error ? fileError.message : 'Unknown error'}
                      </p>
                      <button
                        onClick={() => refetchFile()}
                        className="flex items-center gap-1.5 px-3 py-1.5 bg-elevated border border-border rounded text-sm text-text-secondary hover:text-text-primary transition-colors"
                      >
                        <RefreshCw className="w-3.5 h-3.5" />
                        Retry
                      </button>
                    </div>
                  ) : decodedContent !== null ? (
                    isMarkdownFile(selectedFile.path) && markdownView === 'rendered' ? (
                      <div className="p-6 prose prose-invert prose-sm max-w-none text-text-primary prose-headings:text-text-primary prose-p:text-text-secondary prose-a:text-accent prose-code:text-accent prose-code:bg-elevated prose-code:px-1 prose-code:rounded prose-pre:bg-elevated prose-pre:border prose-pre:border-border prose-blockquote:border-accent prose-strong:text-text-primary prose-li:text-text-secondary">
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>
                          {decodedContent}
                        </ReactMarkdown>
                      </div>
                    ) : (
                      <pre className="p-4 text-sm font-mono leading-relaxed overflow-x-auto">
                        <code
                          className="hljs"
                          dangerouslySetInnerHTML={{ __html: highlightedHtml }}
                        />
                      </pre>
                    )
                  ) : null}
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {/* Admin Path Management */}
      {isAdmin && (
        <div className="shrink-0 border-t border-border p-4 bg-surface/30">
          <PathManagement serverId={serverId!} />
        </div>
      )}
    </div>
  )
}
