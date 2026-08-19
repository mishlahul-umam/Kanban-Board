import type { BoardDetail } from '../api/types'

const colPrefix = 'column-'

export function columnDroppableId(columnId: string) {
  return `${colPrefix}${columnId}`
}

export function parseColumnDroppable(overId: string): string | null {
  if (overId.startsWith(colPrefix)) return overId.slice(colPrefix.length)
  return null
}

export function taskColumnId(board: BoardDetail, taskId: string): string | null {
  for (const c of board.columns) {
    if (c.tasks.some((t) => t.id === taskId)) return c.id
  }
  return null
}

/** Returns API payload for PATCH /tasks/:id/move or null if no-op / invalid. */
export function computeMove(
  board: BoardDetail,
  activeTaskId: string,
  overId: string | null | undefined
): { column_id: string; position: number } | null {
  if (!overId) return null
  const overRaw = String(overId)
  const sourceColId = taskColumnId(board, activeTaskId)
  if (!sourceColId) return null
  const sourceCol = board.columns.find((c) => c.id === sourceColId)!
  const sourceIndex = sourceCol.tasks.findIndex((t) => t.id === activeTaskId)
  if (sourceIndex < 0) return null

  let targetColId = parseColumnDroppable(overRaw)
  let overTaskId: string | null = null
  if (!targetColId) {
    overTaskId = overRaw
    targetColId = taskColumnId(board, overTaskId)
  }
  if (!targetColId) return null
  const targetCol = board.columns.find((c) => c.id === targetColId)!
  let targetPosition: number
  if (overTaskId) {
    if (overTaskId === activeTaskId) return null
    targetPosition = targetCol.tasks.findIndex((t) => t.id === overTaskId)
    if (targetPosition < 0) return null
    if (sourceColId === targetColId && sourceIndex < targetPosition) {
      targetPosition -= 1
    }
  } else {
    targetPosition = targetCol.tasks.filter((t) => t.id !== activeTaskId).length
  }

  if (sourceColId === targetColId && sourceIndex === targetPosition) return null
  return { column_id: targetColId, position: targetPosition }
}
