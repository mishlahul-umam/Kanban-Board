export type User = {
  id: string
  email: string
  display_name: string
}

export type Task = {
  id: string
  column_id: string
  title: string
  description?: string | null
  position: number
  assignee_id?: string | null
  due_at?: string | null
  created_at: string
  updated_at: string
}

export type Column = {
  id: string
  board_id: string
  title: string
  position: number
  tasks: Task[]
}

export type BoardSummary = {
  id: string
  title: string
  owner_id: string
  created_at: string
}

export type BoardDetail = BoardSummary & {
  columns: Column[]
  members: User[]
}

export type Comment = {
  id: string
  task_id: string
  user_id: string
  body: string
  created_at: string
  user?: User
}
