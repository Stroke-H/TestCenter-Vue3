export type NodeStatus = 'none' | 'completed' | 'in_progress' | 'fixing'

export interface MindNode {
  id: string
  label: string
  status: NodeStatus
  isRoot?: boolean
  children?: MindNode[]
}

export interface ProcessItem {
  id: string
  name: string
  data: MindNode
  updatedAt: string
}
