import { renderHook } from '@testing-library/react'
import type { FileNode, FileReadResponse } from '@/types/api'
import { useFileContentViewer } from '../use-file-content-viewer'

describe('useFileContentViewer', () => {
  const selectedFile: FileNode = {
    name: 'example.txt',
    path: '/example.txt',
    is_dir: false,
    size: 23,
    readable: true,
  }

  function makeFileContent(text: string): FileReadResponse {
    return {
      data: btoa(text),
      total_size: text.length,
      mime_type: 'text/plain',
    }
  }

  it('decodes file contents and computes highlighted search state', () => {
    const { result } = renderHook(() =>
      useFileContentViewer(
        makeFileContent('alpha beta alpha'),
        selectedFile,
        'alpha',
        1,
      ),
    )

    expect(result.current.decodedContent).toBe('alpha beta alpha')
    expect(result.current.effectiveSearchTerm).toBe('alpha')
    expect(result.current.matchCount).toBe(2)
    expect(result.current.highlightedHtml).toContain('alpha beta alpha')
    expect(result.current.searchHighlightedHtml).toContain('data-match-idx="0"')
    expect(result.current.searchHighlightedHtml).toContain('id="current-search-match" data-match-idx="1"')
  })

  it('falls back to syntax-highlight output when no search term is applied', () => {
    const { result } = renderHook(() =>
      useFileContentViewer(
        makeFileContent('const answer = 42'),
        { ...selectedFile, name: 'example.ts' },
        '',
        0,
      ),
    )

    expect(result.current.effectiveSearchTerm).toBe('')
    expect(result.current.matchCount).toBe(0)
    expect(result.current.searchHighlightedHtml).toBe(result.current.highlightedHtml)
    expect(result.current.highlightedHtml).toContain('hljs-keyword')
  })

  it('returns empty output when no file is selected', () => {
    const { result } = renderHook(() =>
      useFileContentViewer(undefined, null, 'alpha', 0),
    )

    expect(result.current.decodedContent).toBeNull()
    expect(result.current.highlightedHtml).toBe('')
    expect(result.current.searchHighlightedHtml).toBe('')
    expect(result.current.matchCount).toBe(0)
  })
})
