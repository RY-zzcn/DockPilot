export type Role = 'admin' | 'viewer'
export type PolicyMode = 'manual' | 'scheduled' | 'automatic'

export interface User {
  id: number
  username: string
  role: Role
  created_at: string
}

export interface AuthClaims {
  sub: string
  username: string
  role: Role
  exp: number
}

export interface Overview {
  nodes_total: number
  nodes_online: number
  containers_total: number
  updates_available: number
  failed_tasks: number
  last_metric: Metric
}

export interface Node {
  id: string
  name: string
  note: string
  version: string
  os: string
  arch: string
  docker_version: string
  compose_version: string
  status: string
  last_seen: string
  labels: string
  created_at: string
  updated_at: string
}

export interface Metric {
  id: number
  node_id: string
  cpu_percent: number
  memory_used: number
  memory_total: number
  disk_used: number
  disk_total: number
  network_rx: number
  network_tx: number
  container_count: number
  recorded_at: string
}

export interface Container {
  id: string
  node_id: string
  name: string
  image: string
  state: string
  status: string
  compose_project: string
  update_available: boolean
  updated_at: string
}

export interface Image {
  id: string
  node_id: string
  repository: string
  tag: string
  size: string
  created_at: string
  updated_at: string
}

export interface ComposeProject {
  id: string
  node_id: string
  name: string
  path: string
  managed: boolean
  content: string
  version: number
  update_available: boolean
  checked_at: string
  detection_status: string
  detection_method: string
  detection_platform: string
  detection_error?: string
  last_seen: string
  updated_at: string
}

export interface DockerState {
  containers: Container[]
  images: Image[]
  compose_projects: ComposeProject[]
}

export interface Task {
  id: string
  node_id: string
  kind: string
  target_type: string
  target_id: string
  status: string
  requested_by: string
  policy_id?: string
  payload?: string
  result?: string
  created_at: string
  started_at?: string
  finished_at?: string
}

export interface TaskLog {
  id: number
  task_id: string
  line: string
  created_at: string
}

export interface UpdateRecord {
  id: number
  node_id: string
  task_id: string
  target_type: string
  target_id: string
  name: string
  previous_version?: string
  current_version?: string
  changed: boolean
  created_at: string
}

export interface Policy {
  id?: string
  scope: string
  scope_id: string
  mode: PolicyMode
  schedule: string
  exclude_patterns: string
  enabled: boolean
  updated_at?: string
}

export interface Notification {
  id?: string
  name: string
  channel: 'telegram' | 'webhook' | 'email'
  config: string
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface InstallInfo {
  server_url: string
  registration_token: string
  interactive?: string
  docker_command: string
  binary_command: string
  agent_docker?: string
  agent_binary?: string
  server_docker?: string
  server_binary?: string
  uninstall?: string
  uninstall_agent?: string
  uninstall_server?: string
  uninstall_all?: string
  uninstall_purge?: string
}

export interface ReleaseAsset {
  name: string
  download_url: string
  size: number
}

export interface ReleaseInfo {
  repository: string
  latest_version: string
  latest_tag: string
  name?: string
  url?: string
  published_at?: string
  checked_at: string
  update_available: boolean
  assets?: ReleaseAsset[]
  error?: string
}

export interface RuntimeSettings {
  release_repo: string
  release_cache_seconds: number
  agent_auto_update: boolean
  agent_auto_update_version: string
  agent_auto_update_interval_seconds: number
}

export interface VersionInfo {
  version: string
  commit: string
  build_date: string
  time_zone: string
  server_time: string
  release?: ReleaseInfo
  settings?: RuntimeSettings
}
