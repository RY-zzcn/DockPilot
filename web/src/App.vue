<template>
  <main v-if="!token" class="login-page">
    <form class="login-panel" @submit.prevent="login">
      <div>
        <p class="eyebrow">DockPilot</p>
        <h1>Docker 节点管理</h1>
      </div>
      <label>
        <span>用户名</span>
        <input v-model="loginForm.username" autocomplete="username" />
      </label>
      <label>
        <span>密码</span>
        <input v-model="loginForm.password" type="password" autocomplete="current-password" />
      </label>
      <button class="primary" type="submit" :disabled="busy">
        <LogIn :size="18" />
        登录
      </button>
      <p v-if="error" class="error-line">{{ error }}</p>
    </form>
  </main>

  <div v-else class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-mark">D</div>
        <div>
          <strong>DockPilot</strong>
          <span>{{ user?.username }} · v{{ versionInfo.version }}</span>
        </div>
      </div>
      <nav class="nav-list">
        <button :class="{ active: activeView === 'dashboard' }" title="仪表盘" @click="activeView = 'dashboard'">
          <LayoutDashboard :size="18" />
          仪表盘
        </button>
        <button :class="{ active: activeView === 'nodes' }" title="节点" @click="activeView = 'nodes'">
          <Server :size="18" />
          节点
        </button>
        <button :class="{ active: activeView === 'updates' }" title="更新中心" @click="activeView = 'updates'">
          <RefreshCw :size="18" />
          更新中心
        </button>
        <button :class="{ active: activeView === 'tasks' }" title="任务" @click="activeView = 'tasks'">
          <ClipboardList :size="18" />
          任务
        </button>
        <button :class="{ active: activeView === 'settings' }" title="设置" @click="activeView = 'settings'">
          <Settings :size="18" />
          设置
        </button>
      </nav>
      <button class="ghost bottom-action" title="退出" @click="logout">
        <LogOut :size="18" />
        退出
      </button>
    </aside>

    <section class="main">
      <header class="topbar">
        <div>
          <p class="eyebrow">{{ viewTitle }}</p>
          <h1>{{ selectedNode?.name || '多节点 Docker 管理' }}</h1>
        </div>
        <div class="top-actions">
          <div class="time-chip">
            <Clock3 :size="16" />
            <span>{{ currentClock }}</span>
            <small>北京时间</small>
          </div>
          <label class="theme-select" title="界面主题">
            <Palette :size="16" />
            <select v-model="themeName">
              <option v-for="theme in themes" :key="theme.value" :value="theme.value">
                {{ theme.label }}
              </option>
            </select>
          </label>
          <select v-model="selectedNodeId" title="选择节点">
            <option value="">全部节点</option>
            <option v-for="node in nodes" :key="node.id" :value="node.id">
              {{ node.name }}
            </option>
          </select>
          <button class="icon-button" title="刷新" @click="refreshAll">
            <RefreshCw :size="18" />
          </button>
        </div>
      </header>

      <p v-if="error" class="error-line">{{ error }}</p>

      <section v-if="activeView === 'dashboard'" class="view-stack">
        <div class="metric-grid">
          <article class="metric-card">
            <span>在线节点</span>
            <strong>{{ overview.nodes_online }}/{{ overview.nodes_total }}</strong>
          </article>
          <article class="metric-card">
            <span>容器</span>
            <strong>{{ overview.containers_total }}</strong>
          </article>
          <article class="metric-card warn">
            <span>可更新</span>
            <strong>{{ overview.updates_available }}</strong>
          </article>
          <article class="metric-card danger">
            <span>失败任务</span>
            <strong>{{ overview.failed_tasks }}</strong>
          </article>
        </div>

        <div class="telemetry-grid">
          <article class="telemetry-card">
            <div>
              <Cpu :size="18" />
              <span>CPU</span>
            </div>
            <strong>{{ formatPercent(overview.last_metric.cpu_percent) }}</strong>
            <div class="meter"><span :style="{ width: clampPercent(overview.last_metric.cpu_percent) + '%' }"></span></div>
          </article>
          <article class="telemetry-card">
            <div>
              <MemoryStick :size="18" />
              <span>内存</span>
            </div>
            <strong>{{ formatPercent(memoryPercent) }}</strong>
            <small>{{ formatBytes(overview.last_metric.memory_used) }} / {{ formatBytes(overview.last_metric.memory_total) }}</small>
            <div class="meter"><span :style="{ width: clampPercent(memoryPercent) + '%' }"></span></div>
          </article>
          <article class="telemetry-card">
            <div>
              <HardDrive :size="18" />
              <span>磁盘</span>
            </div>
            <strong>{{ formatPercent(diskPercent) }}</strong>
            <small>{{ formatBytes(overview.last_metric.disk_used) }} / {{ formatBytes(overview.last_metric.disk_total) }}</small>
            <div class="meter"><span :style="{ width: clampPercent(diskPercent) + '%' }"></span></div>
          </article>
          <article class="telemetry-card">
            <div>
              <Network :size="18" />
              <span>网络</span>
            </div>
            <strong>{{ formatBytes(overview.last_metric.network_rx + overview.last_metric.network_tx) }}</strong>
            <small>RX {{ formatBytes(overview.last_metric.network_rx) }} / TX {{ formatBytes(overview.last_metric.network_tx) }}</small>
            <div class="meter accent"><span :style="{ width: '64%' }"></span></div>
          </article>
        </div>

        <div class="split">
          <section class="panel">
            <div class="panel-head">
              <h2>节点状态</h2>
              <Server :size="18" />
            </div>
            <div class="node-list">
              <button
                v-for="node in nodes"
                :key="node.id"
                class="node-row"
                :class="{ selected: node.id === selectedNodeId }"
                @click="selectedNodeId = node.id"
              >
                <span class="status-dot" :class="node.status"></span>
                <span>{{ node.name }}</span>
                <small>{{ node.docker_version || 'Docker -' }}</small>
              </button>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <h2>最近任务</h2>
              <ClipboardList :size="18" />
            </div>
            <div class="task-list compact">
              <button v-for="task in tasks.slice(0, 8)" :key="task.id" class="task-row" @click="openTask(task)">
                <span class="badge" :class="task.status">{{ task.status }}</span>
                <span>{{ task.kind }}</span>
                <small>{{ task.node_id }}</small>
              </button>
            </div>
          </section>
        </div>
      </section>

      <section v-if="activeView === 'nodes'" class="view-stack">
        <div class="split">
          <section class="panel">
            <div class="panel-head">
              <h2>节点</h2>
              <Server :size="18" />
            </div>
            <div class="node-list">
              <button
                v-for="node in nodes"
                :key="node.id"
                class="node-row"
                :class="{ selected: node.id === selectedNodeId }"
                @click="selectedNodeId = node.id"
              >
                <span class="status-dot" :class="node.status"></span>
                <span>{{ node.name }}</span>
                <small>{{ node.os }}/{{ node.arch }}</small>
              </button>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <h2>Docker</h2>
              <Terminal :size="18" />
            </div>
            <div class="fact-grid">
              <span>Docker</span>
              <strong>{{ selectedNode?.docker_version || '-' }}</strong>
              <span>Compose</span>
              <strong>{{ selectedNode?.compose_version || '-' }}</strong>
              <span>最近心跳</span>
              <strong>{{ selectedNode?.last_seen || '-' }}</strong>
              <span>备注</span>
              <strong>{{ selectedNode?.note || '-' }}</strong>
            </div>
            <form class="form-stack compact-form" @submit.prevent="saveNode">
              <div class="form-grid">
                <label>
                  <span>节点名称</span>
                  <input v-model="nodeForm.name" :disabled="!selectedNodeId || !isAdmin" />
                </label>
                <label>
                  <span>备注</span>
                  <input v-model="nodeForm.note" :disabled="!selectedNodeId || !isAdmin" />
                </label>
              </div>
              <div class="button-row">
                <button class="primary" :disabled="!selectedNodeId || !isAdmin">
                  <Save :size="18" />
                  保存节点
                </button>
                <button class="secondary danger-action" type="button" :disabled="!selectedNodeId || !isAdmin" @click="deleteNode">
                  <Trash2 :size="18" />
                  删除节点
                </button>
              </div>
            </form>
            <div class="button-row">
              <button class="secondary" :disabled="!selectedNodeId || !isAdmin" @click="createNodeTask('detect_updates')">
                <Search :size="18" />
                检测
              </button>
              <button class="secondary" :disabled="!selectedNodeId || !isAdmin" @click="createNodeTask('prune_images')">
                <Trash2 :size="18" />
                清理镜像
              </button>
            </div>
          </section>
        </div>

        <section class="panel">
          <div class="panel-head">
            <h2>容器</h2>
            <Box :size="18" />
          </div>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>名称</th>
                  <th>镜像</th>
                  <th>状态</th>
                  <th>Compose</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="container in dockerState.containers" :key="container.id">
                  <td>{{ container.name }}</td>
                  <td>{{ container.image }}</td>
                  <td><span class="badge" :class="container.state">{{ container.state }}</span></td>
                  <td>{{ container.compose_project || '-' }}</td>
                  <td>
                    <button
                      class="icon-button"
                      title="重启容器"
                      :disabled="!isAdmin"
                      @click="createNodeTask('restart_container', 'container', container.id)"
                    >
                      <RotateCcw :size="16" />
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <h2>Compose</h2>
            <FileCode2 :size="18" />
          </div>
          <div class="compose-grid">
            <button
              v-for="project in dockerState.compose_projects"
              :key="project.id"
              class="compose-item"
              :class="{ selected: composeForm.id === project.id }"
              @click="editCompose(project)"
            >
              <strong>{{ project.name }}</strong>
              <span>{{ project.path }}</span>
              <em :class="detectionBadgeClass(project)">{{ detectionLabel(project) }}</em>
              <small v-if="detectionMeta(project)">{{ detectionMeta(project) }}</small>
            </button>
          </div>
          <form class="compose-editor" @submit.prevent="saveCompose">
            <div class="form-grid">
              <label>
                <span>名称</span>
                <input v-model="composeForm.name" :disabled="!isAdmin" />
              </label>
              <label>
                <span>路径</span>
                <input v-model="composeForm.path" :disabled="!isAdmin" />
              </label>
            </div>
            <textarea v-model="composeForm.content" :disabled="!isAdmin" spellcheck="false"></textarea>
            <div class="button-row">
              <label class="checkline">
                <input v-model="composeForm.deploy_now" type="checkbox" :disabled="!isAdmin" />
                立即部署
              </label>
              <button class="secondary" type="button" :disabled="!isAdmin" @click="newCompose">
                <Plus :size="18" />
                新建
              </button>
              <button class="primary" type="submit" :disabled="!selectedNodeId || !isAdmin">
                <Save :size="18" />
                保存
              </button>
            </div>
          </form>
        </section>
      </section>

      <section v-if="activeView === 'updates'" class="view-stack">
        <section class="panel">
          <div class="panel-head">
            <h2>策略</h2>
            <Shield :size="18" />
          </div>
          <div class="policy-grid">
            <PolicyEditor title="全局" :policy="policyDraftFor('global', '')" :disabled="!isAdmin" @save="savePolicy" />
            <PolicyEditor
              v-if="selectedNodeId"
              title="当前节点"
              :policy="policyDraftFor('node', selectedNodeId)"
              :disabled="!isAdmin"
              @save="savePolicy"
            />
          </div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <h2>Compose 更新</h2>
            <RefreshCw :size="18" />
          </div>
          <div class="update-list">
            <article v-for="row in composePolicyRows" :key="row.project.id" class="update-row">
              <div>
                <strong>{{ row.project.name }}</strong>
                <span>{{ row.project.path }}</span>
                <em :class="detectionBadgeClass(row.project)">{{ detectionLabel(row.project) }}</em>
                <small v-if="detectionMeta(row.project)">{{ detectionMeta(row.project) }}</small>
                <small v-if="row.project.detection_error" class="error-text">{{ row.project.detection_error }}</small>
              </div>
              <div class="segmented">
                <button :class="{ active: row.policy.mode === 'manual' }" @click="row.policy.mode = 'manual'">手动</button>
                <button :class="{ active: row.policy.mode === 'scheduled' }" @click="row.policy.mode = 'scheduled'">定时</button>
                <button :class="{ active: row.policy.mode === 'automatic' }" @click="row.policy.mode = 'automatic'">全自动</button>
              </div>
              <input v-model="row.policy.schedule" class="schedule-input" placeholder="@daily / interval:6h" />
              <input v-model="row.policy.exclude_patterns" class="schedule-input" placeholder="排除关键字" />
              <div class="button-row compact-actions">
                <button class="icon-button" title="保存策略" :disabled="!isAdmin" @click="savePolicy(row.policy)">
                  <Save :size="16" />
                </button>
                <button
                  class="icon-button"
                  title="检测更新"
                  :disabled="!isAdmin"
                  @click="createProjectTask('detect_updates', row.project)"
                >
                  <Search :size="16" />
                </button>
                <button
                  class="icon-button"
                  title="执行更新"
                  :disabled="!isAdmin"
                  @click="createProjectTask('compose_update', row.project)"
                >
                  <Play :size="16" />
                </button>
              </div>
            </article>
          </div>
        </section>
      </section>

      <section v-if="activeView === 'tasks'" class="view-stack">
        <section class="panel">
          <div class="panel-head">
            <h2>任务</h2>
            <div class="button-row compact-actions">
              <button class="secondary" :disabled="!isAdmin" @click="clearFinishedTasks">
                <Eraser :size="18" />
                清除历史
              </button>
              <ClipboardList :size="18" />
            </div>
          </div>
          <div class="task-list">
            <button v-for="task in tasks" :key="task.id" class="task-row" @click="openTask(task)">
              <span class="badge" :class="task.status">{{ task.status }}</span>
              <span>{{ task.kind }}</span>
              <small>{{ task.created_at }}</small>
              <small>{{ task.node_id }}</small>
            </button>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <h2>日志</h2>
            <Terminal :size="18" />
          </div>
          <pre class="logs">{{ selectedTaskLogs }}</pre>
        </section>
      </section>

      <section v-if="activeView === 'settings'" class="view-stack">
        <section class="panel">
          <div class="panel-head">
            <h2>版本与发布</h2>
            <Shield :size="18" />
          </div>
          <div class="fact-grid">
            <span>Server</span>
            <strong>v{{ versionInfo.version }}</strong>
            <span>Commit</span>
            <strong>{{ shortCommit }}</strong>
            <span>构建时间</span>
            <strong>{{ versionInfo.build_date || '-' }}</strong>
            <span>服务时间</span>
            <strong>{{ versionInfo.server_time || '-' }}</strong>
            <span>时区</span>
            <strong>{{ versionInfo.time_zone || 'Asia/Shanghai' }}</strong>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <h2>Agent</h2>
            <Terminal :size="18" />
          </div>
          <div class="command-stack">
            <div class="command-item">
              <div class="command-title">
                <span>交互式部署/卸载</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.interactive || '')">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.interactive || '-' }}</div>
            </div>
            <div class="command-item">
              <div class="command-title">
                <span>Agent 二进制接入</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.agent_binary || installInfo.binary_command)">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.agent_binary || installInfo.binary_command }}</div>
            </div>
            <div class="command-item">
              <div class="command-title">
                <span>Agent Docker 接入</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.agent_docker || installInfo.docker_command)">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.agent_docker || installInfo.docker_command }}</div>
            </div>
            <div class="command-item">
              <div class="command-title">
                <span>Server Docker 部署</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.server_docker || '')">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.server_docker || '-' }}</div>
            </div>
            <div class="command-item">
              <div class="command-title">
                <span>Server 二进制部署</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.server_binary || '')">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.server_binary || '-' }}</div>
            </div>
            <div class="command-item">
              <div class="command-title">
                <span>交互式卸载</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.uninstall || '')">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.uninstall || '-' }}</div>
            </div>
            <div class="command-item">
              <div class="command-title">
                <span>仅卸载 Agent</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.uninstall_agent || '')">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.uninstall_agent || '-' }}</div>
            </div>
            <div class="command-item">
              <div class="command-title">
                <span>仅卸载 Server</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.uninstall_server || '')">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.uninstall_server || '-' }}</div>
            </div>
            <div class="command-item">
              <div class="command-title">
                <span>全部卸载</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.uninstall_all || '')">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.uninstall_all || '-' }}</div>
            </div>
            <div class="command-item">
              <div class="command-title">
                <span>彻底卸载</span>
                <button class="icon-button" title="复制" @click="copyCommand(installInfo.uninstall_purge || '')">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ installInfo.uninstall_purge || '-' }}</div>
            </div>
          </div>
        </section>

        <section class="panel">
          <div class="panel-head">
            <h2>通知</h2>
            <Bell :size="18" />
          </div>
          <div class="notification-grid">
            <button v-for="item in notifications" :key="item.id" class="compose-item" @click="editNotification(item)">
              <strong>{{ item.name }}</strong>
              <span>{{ item.channel }} · {{ item.enabled ? '启用' : '停用' }}</span>
            </button>
          </div>
          <form class="form-stack" @submit.prevent="saveNotification">
            <div class="form-grid">
              <label>
                <span>名称</span>
                <input v-model="notificationForm.name" :disabled="!isAdmin" />
              </label>
              <label>
                <span>渠道</span>
                <select v-model="notificationForm.channel" :disabled="!isAdmin" @change="setNotificationTemplate">
                  <option value="telegram">Telegram</option>
                  <option value="webhook">Webhook</option>
                  <option value="email">Email</option>
                </select>
              </label>
            </div>
            <textarea v-model="notificationForm.config" :disabled="!isAdmin" spellcheck="false"></textarea>
            <div class="button-row">
              <label class="checkline">
                <input v-model="notificationForm.enabled" type="checkbox" :disabled="!isAdmin" />
                启用
              </label>
              <button class="primary" type="submit" :disabled="!isAdmin">
                <Save :size="18" />
                保存
              </button>
            </div>
          </form>
        </section>

        <section class="panel">
          <div class="panel-head">
            <h2>用户</h2>
            <Users :size="18" />
          </div>
          <div class="user-list">
            <span v-for="item in users" :key="item.id" class="user-chip">{{ item.username }} · {{ item.role }}</span>
          </div>
          <form class="form-grid" @submit.prevent="createUser">
            <label>
              <span>用户名</span>
              <input v-model="userForm.username" :disabled="!isAdmin" />
            </label>
            <label>
              <span>密码</span>
              <input v-model="userForm.password" type="password" :disabled="!isAdmin" />
            </label>
            <label>
              <span>角色</span>
              <select v-model="userForm.role" :disabled="!isAdmin">
                <option value="viewer">viewer</option>
                <option value="admin">admin</option>
              </select>
            </label>
            <button class="primary aligned" :disabled="!isAdmin">
              <Plus :size="18" />
              添加
            </button>
          </form>
        </section>
      </section>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import {
  Bell,
  Box,
  ClipboardList,
  Clock3,
  Copy,
  Cpu,
  Eraser,
  FileCode2,
  HardDrive,
  LayoutDashboard,
  LogIn,
  LogOut,
  MemoryStick,
  Network,
  Palette,
  Plus,
  Play,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
  Server,
  Settings,
  Shield,
  Terminal,
  Trash2,
  Users
} from 'lucide-vue-next'
import { api, clearToken, getToken, setToken } from './api'
import type {
  AuthClaims,
  ComposeProject,
  DockerState,
  InstallInfo,
  Node,
  Notification,
  Overview,
  Policy,
  Task,
  TaskLog,
  User,
  VersionInfo
} from './types'

type ViewName = 'dashboard' | 'nodes' | 'updates' | 'tasks' | 'settings'
type ThemeName = 'aurora' | 'graphite' | 'ember' | 'terminal'

const THEME_KEY = 'dockpilot.theme'
const themes: { value: ThemeName; label: string }[] = [
  { value: 'aurora', label: '极光' },
  { value: 'graphite', label: '石墨' },
  { value: 'ember', label: '日冕' },
  { value: 'terminal', label: '终端' }
]
const savedTheme = localStorage.getItem(THEME_KEY) as ThemeName | null
const token = ref(getToken())
const user = ref<AuthClaims | null>(null)
const activeView = ref<ViewName>('dashboard')
const themeName = ref<ThemeName>(
  themes.some((theme) => theme.value === savedTheme) ? (savedTheme as ThemeName) : 'aurora'
)
const busy = ref(false)
const error = ref('')
const selectedNodeId = ref('')
const selectedTask = ref<Task | null>(null)
const taskLogs = ref<TaskLog[]>([])
const currentClock = ref('')
let clockTimer: number | undefined
let refreshTimer: number | undefined

const loginForm = reactive({ username: 'admin', password: 'admin' })
const overview = reactive<Overview>({
  nodes_total: 0,
  nodes_online: 0,
  containers_total: 0,
  updates_available: 0,
  failed_tasks: 0,
  last_metric: {
    id: 0,
    node_id: '',
    cpu_percent: 0,
    memory_used: 0,
    memory_total: 0,
    disk_used: 0,
    disk_total: 0,
    network_rx: 0,
    network_tx: 0,
    container_count: 0,
    recorded_at: ''
  }
})
const nodes = ref<Node[]>([])
const dockerState = reactive<DockerState>({ containers: [], images: [], compose_projects: [] })
const tasks = ref<Task[]>([])
const policies = ref<Policy[]>([])
const notifications = ref<Notification[]>([])
const users = ref<User[]>([])
const versionInfo = reactive<VersionInfo>({
  version: '0.1.0',
  commit: 'dev',
  build_date: 'unknown',
  time_zone: 'Asia/Shanghai',
  server_time: ''
})
const installInfo = reactive<InstallInfo>({
  server_url: '',
  registration_token: '',
  interactive: '',
  docker_command: '',
  binary_command: '',
  agent_docker: '',
  agent_binary: '',
  server_docker: '',
  server_binary: '',
  uninstall: '',
  uninstall_agent: '',
  uninstall_server: '',
  uninstall_all: '',
  uninstall_purge: ''
})

const nodeForm = reactive({ name: '', note: '' })
const composeForm = reactive({
  id: '',
  name: '',
  path: '',
  content: '',
  deploy_now: false
})
const notificationForm = reactive<Notification>({
  name: '',
  channel: 'webhook',
  config: '{\n  "url": ""\n}',
  enabled: true
})
const userForm = reactive({ username: '', password: '', role: 'viewer' })
const policyDrafts = reactive<Record<string, Policy>>({})

const isAdmin = computed(() => user.value?.role === 'admin')
const selectedNode = computed(() => nodes.value.find((node) => node.id === selectedNodeId.value))
const memoryPercent = computed(() => percent(overview.last_metric.memory_used, overview.last_metric.memory_total))
const diskPercent = computed(() => percent(overview.last_metric.disk_used, overview.last_metric.disk_total))
const shortCommit = computed(() => (versionInfo.commit && versionInfo.commit !== 'dev' ? versionInfo.commit.slice(0, 12) : versionInfo.commit || '-'))
const viewTitle = computed(() => {
  const titles: Record<ViewName, string> = {
    dashboard: '总览',
    nodes: '节点',
    updates: '更新',
    tasks: '任务',
    settings: '设置'
  }
  return titles[activeView.value]
})
const composePolicyRows = computed(() =>
  dockerState.compose_projects.map((project) => ({
    project,
    policy: policyDraftFor('compose', project.id)
  }))
)
const selectedTaskLogs = computed(() => {
  if (!selectedTask.value) {
    return '选择一个任务查看日志'
  }
  if (taskLogs.value.length === 0) {
    return `${selectedTask.value.id}\n暂无日志`
  }
  return taskLogs.value.map((line) => `[${line.created_at}] ${line.line}`).join('\n')
})

const PolicyEditor = defineComponent({
  props: {
    title: { type: String, required: true },
    policy: { type: Object as () => Policy, required: true },
    disabled: { type: Boolean, default: false }
  },
  emits: ['save'],
  setup(props, { emit }) {
    return () =>
      h('div', { class: 'policy-editor' }, [
        h('strong', props.title),
        h('select', {
          value: props.policy.mode,
          disabled: props.disabled,
          onChange: (event: Event) => (props.policy.mode = (event.target as HTMLSelectElement).value as Policy['mode'])
        }, [
          h('option', { value: 'manual' }, '手动'),
          h('option', { value: 'scheduled' }, '定时'),
          h('option', { value: 'automatic' }, '全自动')
        ]),
        h('input', {
          value: props.policy.schedule,
          disabled: props.disabled,
          placeholder: '@daily / interval:6h',
          onInput: (event: Event) => (props.policy.schedule = (event.target as HTMLInputElement).value)
        }),
        h('input', {
          value: props.policy.exclude_patterns,
          disabled: props.disabled,
          placeholder: 'mysql,postgres,redis',
          onInput: (event: Event) => (props.policy.exclude_patterns = (event.target as HTMLInputElement).value)
        }),
        h('label', { class: 'checkline' }, [
          h('input', {
            type: 'checkbox',
            checked: props.policy.enabled,
            disabled: props.disabled,
            onChange: (event: Event) => (props.policy.enabled = (event.target as HTMLInputElement).checked)
          }),
          '启用'
        ]),
        h('button', { class: 'secondary', disabled: props.disabled, onClick: () => emit('save', props.policy) }, [
          h(Save, { size: 16 }),
          '保存'
        ])
      ])
  }
})

watch(
  themeName,
  (value) => {
    document.documentElement.dataset.theme = value
    localStorage.setItem(THEME_KEY, value)
  },
  { immediate: true }
)

onMounted(() => {
  tickClock()
  clockTimer = window.setInterval(tickClock, 1000)
  bootstrap()
  refreshTimer = window.setInterval(() => {
    if (token.value) {
      refreshAll()
    }
  }, 30000)
})
onUnmounted(() => {
  if (clockTimer) window.clearInterval(clockTimer)
  if (refreshTimer) window.clearInterval(refreshTimer)
})
watch(selectedNodeId, async (nodeId) => {
  if (nodeId) {
    await loadDocker(nodeId)
  } else {
    dockerState.containers = []
    dockerState.images = []
    dockerState.compose_projects = []
  }
  syncNodeForm()
})

watch(selectedNode, () => syncNodeForm())

async function bootstrap() {
  if (!token.value) {
    return
  }
  try {
    user.value = await api.me()
    await refreshAll()
  } catch {
    logout()
  }
}

function tickClock() {
  currentClock.value = new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  }).format(new Date())
}

async function login() {
  error.value = ''
  busy.value = true
  try {
    const result = await api.login(loginForm.username, loginForm.password)
    setToken(result.token)
    token.value = result.token
    user.value = await api.me()
    await refreshAll()
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    busy.value = false
  }
}

function logout() {
  clearToken()
  token.value = ''
  user.value = null
}

async function refreshAll() {
  error.value = ''
  try {
    const [overviewData, nodesData, tasksData, policiesData, notificationsData, versionData] = await Promise.all([
      api.overview(),
      api.nodes(),
      api.tasks(),
      api.policies(),
      api.notifications(),
      api.version()
    ])
    Object.assign(overview, overviewData)
    nodes.value = nodesData
    tasks.value = tasksData
    policies.value = policiesData
    notifications.value = notificationsData
    Object.assign(versionInfo, versionData)
    syncPolicyDrafts()
    if (!selectedNodeId.value && nodes.value.length > 0) {
      selectedNodeId.value = nodes.value[0].id
    } else if (selectedNodeId.value) {
      await loadDocker(selectedNodeId.value)
    }
    if (isAdmin.value) {
      await loadAdminSettings()
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadAdminSettings() {
  const [install, userList] = await Promise.all([api.installInfo(), api.users()])
  Object.assign(installInfo, install)
  users.value = userList
}

async function loadDocker(nodeId: string) {
  const state = await api.dockerState(nodeId)
  dockerState.containers = state.containers
  dockerState.images = state.images
  dockerState.compose_projects = state.compose_projects
}

function syncNodeForm() {
  nodeForm.name = selectedNode.value?.name || ''
  nodeForm.note = selectedNode.value?.note || ''
}

async function saveNode() {
  if (!selectedNodeId.value) return
  const saved = await api.updateNode(selectedNodeId.value, nodeForm)
  nodes.value = nodes.value.map((node) => (node.id === saved.id ? saved : node))
}

async function deleteNode() {
  if (!selectedNodeId.value || !selectedNode.value) return
  const confirmed = window.confirm(`删除节点 ${selectedNode.value.name}？如果该 Agent 仍在运行，它可能会重新注册。`)
  if (!confirmed) return
  const deletedID = selectedNodeId.value
  await api.deleteNode(deletedID)
  nodes.value = nodes.value.filter((node) => node.id !== deletedID)
  selectedNodeId.value = nodes.value[0]?.id || ''
  if (!selectedNodeId.value) {
    dockerState.containers = []
    dockerState.images = []
    dockerState.compose_projects = []
  }
  await refreshAll()
}

async function createNodeTask(kind: string, targetType = '', targetId = '') {
  if (!selectedNodeId.value) return
  await api.createTask({ node_id: selectedNodeId.value, kind, target_type: targetType, target_id: targetId, args: {} })
  await refreshTasks()
}

async function createProjectTask(kind: string, project: ComposeProject) {
  await api.createTask({
    node_id: project.node_id,
    kind,
    target_type: 'compose',
    target_id: project.id,
    args: { path: project.path, name: project.name }
  })
  await refreshTasks()
}

async function refreshTasks() {
  tasks.value = await api.tasks()
}

async function clearFinishedTasks() {
  const confirmed = window.confirm('清除已结束的任务历史？正在运行和排队任务会保留。')
  if (!confirmed) return
  await api.clearTasks('finished')
  selectedTask.value = null
  taskLogs.value = []
  await refreshTasks()
  Object.assign(overview, await api.overview())
}

async function openTask(task: Task) {
  selectedTask.value = task
  taskLogs.value = await api.taskLogs(task.id)
  activeView.value = 'tasks'
}

function editCompose(project: ComposeProject) {
  composeForm.id = project.id
  composeForm.name = project.name
  composeForm.path = project.path
  composeForm.content = project.content || ''
  composeForm.deploy_now = false
}

function newCompose() {
  composeForm.id = ''
  composeForm.name = ''
  composeForm.path = '/opt/app/compose.yml'
  composeForm.content = 'services:\n  app:\n    image: nginx:stable\n    restart: unless-stopped\n'
  composeForm.deploy_now = false
}

async function saveCompose() {
  if (!selectedNodeId.value) return
  await api.saveCompose({
    node_id: selectedNodeId.value,
    id: composeForm.id || undefined,
    name: composeForm.name,
    path: composeForm.path,
    content: composeForm.content,
    deploy_now: composeForm.deploy_now
  })
  await loadDocker(selectedNodeId.value)
  await refreshTasks()
}

function policyKey(scope: string, scopeId: string) {
  return `${scope}:${scopeId}`
}

function policyDraftFor(scope: string, scopeId: string): Policy {
  const key = policyKey(scope, scopeId)
  if (!policyDrafts[key]) {
    const existing = policies.value.find((policy) => policy.scope === scope && policy.scope_id === scopeId)
    policyDrafts[key] = existing
      ? { ...existing }
      : {
          scope,
          scope_id: scopeId,
          mode: 'manual',
          schedule: scope === 'global' ? '@daily' : '',
          exclude_patterns: 'mysql,postgres,mariadb,redis',
          enabled: true
        }
  }
  return policyDrafts[key]
}

function syncPolicyDrafts() {
  for (const policy of policies.value) {
    policyDrafts[policyKey(policy.scope, policy.scope_id)] = { ...policy }
  }
  policyDraftFor('global', '')
}

async function savePolicy(policy: Policy) {
  const saved = await api.savePolicy(policy)
  policyDrafts[policyKey(saved.scope, saved.scope_id)] = { ...saved }
  policies.value = await api.policies()
}

function editNotification(item: Notification) {
  Object.assign(notificationForm, item)
}

function setNotificationTemplate() {
  if (notificationForm.channel === 'telegram') {
    notificationForm.config = '{\n  "bot_token": "",\n  "chat_id": ""\n}'
  } else if (notificationForm.channel === 'email') {
    notificationForm.config =
      '{\n  "smtp_host": "",\n  "smtp_port": "587",\n  "username": "",\n  "password": "",\n  "from": "",\n  "to": ""\n}'
  } else {
    notificationForm.config = '{\n  "url": "",\n  "method": "POST",\n  "headers": {}\n}'
  }
}

async function saveNotification() {
  await api.saveNotification(notificationForm)
  notifications.value = await api.notifications()
}

async function createUser() {
  await api.createUser(userForm)
  userForm.username = ''
  userForm.password = ''
  users.value = await api.users()
}

async function copyCommand(command: string) {
  if (!command) return
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(command)
    return
  }
  const textarea = document.createElement('textarea')
  textarea.value = command
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  textarea.remove()
}

function detectionLabel(project: ComposeProject) {
  if (project.detection_status === 'partial') return '部分失败'
  if (project.detection_status === 'failed') return '检测失败'
  if (project.update_available || project.detection_status === 'update_available') return '可更新'
  if (project.detection_status === 'checked' || project.detection_status === 'current') return '已检测'
  if (project.checked_at) return '已检测'
  return '未检测'
}

function detectionBadgeClass(project: ComposeProject) {
  if (project.detection_status === 'partial' || project.detection_status === 'failed') return 'mini-danger'
  if (project.update_available || project.detection_status === 'update_available') return 'mini-alert'
  return 'mini-muted'
}

function detectionMeta(project: ComposeProject) {
  return [project.detection_method, project.detection_platform, project.checked_at].filter(Boolean).join(' · ')
}

function percent(used: number, total: number) {
  if (!total) return 0
  return (used / total) * 100
}

function clampPercent(value: number) {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, value))
}

function formatPercent(value: number) {
  return `${clampPercent(value).toFixed(1)}%`
}

function formatBytes(value: number) {
  if (!value) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let size = value
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  return `${size >= 10 ? size.toFixed(0) : size.toFixed(1)} ${units[unit]}`
}
</script>
