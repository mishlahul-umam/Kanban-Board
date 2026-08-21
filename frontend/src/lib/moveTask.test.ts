import { describe, expect, it } from 'vitest'
import type { BoardDetail, Column, Task } from '../api/types'
import { columnDroppableId, computeMove } from './moveTask'

function makeTask(id: string, columnId: string): Task {
  return {
    id,
    column_id: columnId,
    title: id,
    position: 0,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  }
}

function makeColumn(id: string, taskIds: string[]): Column {
  return {
    id,
    board_id: 'board-1',
    title: id,
    position: 0,
    tasks: taskIds.map((tid) => makeTask(tid, id)),
  }
}

function makeBoard(): BoardDetail {
  return {
    id: 'board-1',
    title: 'Test board',
    owner_id: 'user-1',
    created_at: '2026-01-01T00:00:00Z',
    members: [],
    columns: [
      makeColumn('col-a', ['t1', 't2', 't3']),
      makeColumn('col-b', ['t4']),
      makeColumn('col-empty', []),
    ],
  }
}

describe('computeMove', () => {
  describe('invalid or no-op inputs', () => {
    it('returns null when overId is null', () => {
      expect(computeMove(makeBoard(), 't1', null)).toBeNull()
    })

    it('returns null when overId is undefined', () => {
      expect(computeMove(makeBoard(), 't1', undefined)).toBeNull()
    })

    it('returns null when the dragged task does not exist on the board', () => {
      expect(computeMove(makeBoard(), 'ghost-task', 't4')).toBeNull()
    })

    it('returns null when overId matches neither a column nor a task', () => {
      expect(computeMove(makeBoard(), 't1', 'nonexistent-id-xyz')).toBeNull()
    })

    it('returns null when a task is dropped onto itself', () => {
      expect(computeMove(makeBoard(), 't1', 't1')).toBeNull()
    })
  })

  describe('cross-column moves', () => {
    it('moves a task onto another column task at that task position', () => {
      expect(computeMove(makeBoard(), 't1', 't4')).toEqual({
        column_id: 'col-b',
        position: 0,
      })
    })

    it('appends to the end when dropped on a non-empty column background', () => {
      expect(computeMove(makeBoard(), 't1', columnDroppableId('col-b'))).toEqual({
        column_id: 'col-b',
        position: 1,
      })
    })

    it('moves to position 0 when dropped on an empty column background', () => {
      expect(
        computeMove(makeBoard(), 't1', columnDroppableId('col-empty'))
      ).toEqual({ column_id: 'col-empty', position: 0 })
    })
  })

  describe('same-column reordering', () => {
    it('decrements the target position when moving down within a column', () => {
      expect(computeMove(makeBoard(), 't1', 't3')).toEqual({
        column_id: 'col-a',
        position: 1,
      })
    })

    it('keeps the target position when moving up within a column', () => {
      expect(computeMove(makeBoard(), 't3', 't1')).toEqual({
        column_id: 'col-a',
        position: 0,
      })
    })

    it('returns null when dropping onto the immediately following task', () => {
      expect(computeMove(makeBoard(), 't1', 't2')).toBeNull()
    })

    it('returns null when the only task is dropped on its own column background', () => {
      expect(computeMove(makeBoard(), 't4', columnDroppableId('col-b'))).toBeNull()
    })
  })
})
