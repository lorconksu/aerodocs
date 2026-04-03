import { useDeferredValue, useMemo } from 'react'
import type { FileNode, FileReadResponse } from '@/types/api'
import { decodeBase64Utf8, escapeHtml, highlightFileContent } from '@/lib/file-viewer'

interface FileContentViewerState {
  decodedContent: string | null
  effectiveSearchTerm: string
  highlightedHtml: string
  searchHighlightedHtml: string
  matchCount: number
}

export function useFileContentViewer(
  fileContent: FileReadResponse | undefined,
  selectedFile: FileNode | null,
  searchInput: string,
  currentMatch: number,
): FileContentViewerState {
  const deferredSearchInput = useDeferredValue(searchInput)

  const decodedContent = useMemo(
    () => decodeBase64Utf8(fileContent?.data),
    [fileContent?.data],
  )

  const highlightedHtml = useMemo(() => {
    if (!decodedContent || !selectedFile) return ''
    return highlightFileContent(decodedContent, selectedFile.name)
  }, [decodedContent, selectedFile])

  const searchMatchPositions = useMemo(() => {
    if (!deferredSearchInput || !decodedContent) return []

    const lowerContent = decodedContent.toLowerCase()
    const lowerSearch = deferredSearchInput.toLowerCase()
    const positions: number[] = []
    let pos = 0
    while ((pos = lowerContent.indexOf(lowerSearch, pos)) !== -1) {
      positions.push(pos)
      pos += 1
    }
    return positions
  }, [deferredSearchInput, decodedContent])

  const searchBaseHtml = useMemo(() => {
    if (!deferredSearchInput || !decodedContent || searchMatchPositions.length === 0) return null

    let result = ''
    let lastEnd = 0
    searchMatchPositions.forEach((matchPos, idx) => {
      const matchEnd = matchPos + deferredSearchInput.length
      const before = escapeHtml(decodedContent.substring(lastEnd, matchPos))
      const matchText = escapeHtml(decodedContent.substring(matchPos, matchEnd))
      result += before + `<mark class="search-match" data-match-idx="${idx}">${matchText}</mark>`
      lastEnd = matchEnd
    })
    result += escapeHtml(decodedContent.substring(lastEnd))

    return result
  }, [deferredSearchInput, decodedContent, searchMatchPositions])

  const searchHighlightedHtml = useMemo(() => {
    if (!searchBaseHtml) return highlightedHtml

    return searchBaseHtml.replace(
      `class="search-match" data-match-idx="${currentMatch}"`,
      `class="search-match-current" id="current-search-match" data-match-idx="${currentMatch}"`,
    )
  }, [searchBaseHtml, highlightedHtml, currentMatch])

  return {
    decodedContent,
    effectiveSearchTerm: deferredSearchInput,
    highlightedHtml,
    searchHighlightedHtml,
    matchCount: searchMatchPositions.length,
  }
}
