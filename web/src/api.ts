import type {
  AuthClaims,
  ComposeProject,
  DockerState,
  InstallInfo,
  Node,
  Notification,
  Overview,
  Policy,
  RuntimeSettings,
  Task,
  TaskLog,
  User,
  VersionInfo
} from './types'

const TOKEN_KEY = 'dockpilot.token'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const response = await fetch(path, { ...options, headers })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(data.error || response.statusText)
  }
  return data as T
}

function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

export const api = {
  async login(username: string, password: string) {
    return request<{ token: string; user: User }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password })
    })
  },
  me: () => request<AuthClaims>('/api/auth/me'),
  version: () => request<VersionInfo>('/api/version'),
  overview: () => request<Overview>('/api/overview'),
  nodes: async () => asArray(await request<Node[] | null>('/api/nodes')),
  node: (id: string) => request<{ node: Node; online: boolean; docker: DockerState }>(`/api/nodes/${id}`),
  updateNode: (id: string, body: { name: string; note: string }) =>
    request<Node>(`/api/nodes/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: JSON.stringify(body)
    }),
  deleteNode: (id: string) =>
    request<{ status: string }>(`/api/nodes/${encodeURIComponent(id)}`, {
      method: 'DELETE'
    }),
  dockerState: (nodeId: string) => request<DockerState>(`/api/docker/state?node_id=${encodeURIComponent(nodeId)}`),
  saveCompose: (body: {
    node_id: string
    id?: string
    name: string
    path: string
    content: string
    deploy_now: boolean
  }) =>
    request<ComposeProject>('/api/docker/compose', {
      method: 'POST',
      body: JSON.stringify(body)
    }),
  tasks: async () => asArray(await request<Task[] | null>('/api/tasks?limit=100')),
  createTask: (body: {
    node_id: string
    kind: string
    target_type?: string
    target_id?: string
    args?: Record<string, string>
  }) =>
    request<Task>('/api/tasks', {
      method: 'POST',
      body: JSON.stringify(body)
    }),
  taskLogs: async (id: string) => asArray(await request<TaskLog[] | null>(`/api/tasks/${id}/logs`)),
  cancelTask: (id: string) => request<{ status: string }>(`/api/tasks/${id}/cancel`, { method: 'POST' }),
  clearTasks: (scope = 'finished') => request<{ deleted: number }>(`/api/tasks?scope=${encodeURIComponent(scope)}`, { method: 'DELETE' }),
  policies: async () => asArray(await request<Policy[] | null>('/api/policies')),
  savePolicy: (policy: Policy) =>
    request<Policy>('/api/policies', {
      method: 'PUT',
      body: JSON.stringify(policy)
    }),
  notifications: async () => asArray(await request<Notification[] | null>('/api/notifications')),
  saveNotification: (notification: Notification) =>
    request<Notification>('/api/notifications', {
      method: 'PUT',
      body: JSON.stringify(notification)
    }),
  users: async () => asArray(await request<User[] | null>('/api/users')),
  createUser: (body: { username: string; password: string; role: string }) =>
    request<User>('/api/users', {
      method: 'POST',
      body: JSON.stringify(body)
    }),
  runtimeSettings: () => request<RuntimeSettings>('/api/settings/runtime'),
  saveRuntimeSettings: (body: Partial<Pick<RuntimeSettings, 'agent_auto_update' | 'agent_auto_update_version'>>) =>
    request<RuntimeSettings>('/api/settings/runtime', {
      method: 'PUT',
      body: JSON.stringify(body)
    }),
  installInfo: () => request<InstallInfo>('/api/settings/install')
}
