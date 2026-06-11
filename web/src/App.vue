<template>
  <main v-if="!token" class="login-page">
    <form class="login-panel" @submit.prevent="login">
      <div class="login-brand">
        <div class="brand-mark">D</div>
        <div>
          <p class="eyebrow">DockPilot</p>
          <h1>Docker 运维控制台</h1>
        </div>
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
        <div class="nav-group">
          <p>运行态</p>
          <button :class="{ active: activeView === 'dashboard' }" title="总览" @click="activeView = 'dashboard'">
            <LayoutDashboard :size="18" />
            总览
          </button>
          <button :class="{ active: activeView === 'nodes' }" title="节点" @click="activeView = 'nodes'">
            <Server :size="18" />
            节点
          </button>
          <button :class="{ active: activeView === 'projects' }" title="项目" @click="activeView = 'projects'">
            <FileCode2 :size="18" />
            项目
          </button>
        </div>
        <div class="nav-group">
          <p>自动化</p>
          <button :class="{ active: activeView === 'updates' }" title="更新" @click="activeView = 'updates'">
            <RefreshCw :size="18" />
            更新
          </button>
          <button :class="{ active: activeView === 'tasks' }" title="任务" @click="activeView = 'tasks'">
            <ClipboardList :size="18" />
            任务
          </button>
        </div>
        <div class="nav-group">
          <p>系统</p>
          <button :class="{ active: activeView === 'settings' }" title="设置" @click="activeView = 'settings'">
            <Settings :size="18" />
            设置
          </button>
        </div>
      </nav>

      <button class="ghost bottom-action" title="退出" @click="logout">
        <LogOut :size="18" />
        退出
      </button>
    </aside>

    <section class="main">
      <header class="topbar">
        <div class="title-block">
          <p class="eyebrow">{{ viewSection }}</p>
          <h1>{{ viewTitle }}</h1>
        </div>
        <div class="top-actions">
          <div class="time-chip">
            <Clock3 :size="16" />
            <span>{{ currentClock }}</span>
          </div>
          <div class="theme-picker" title="界面主题">
            <button class="theme-trigger" type="button" @click="themeMenuOpen = !themeMenuOpen">
              <Palette :size="16" />
              <span>{{ currentThemeLabel }}</span>
            </button>
            <div v-if="themeMenuOpen" class="theme-menu">
              <button
                v-for="theme in themes"
                :key="theme.value"
                type="button"
                :class="{ active: theme.value === themeName }"
                @click="chooseTheme(theme.value)"
              >
                {{ theme.label }}
              </button>
            </div>
          </div>
          <select v-model="selectedNodeId" title="选择节点">
            <option value="">全部节点</option>
            <option v-for="node in nodes" :key="node.id" :value="node.id">
              {{ node.name }}
            </option>
          </select>
          <button class="icon-button" title="刷新" :disabled="isPending('refresh')" @click="manualRefresh">
            <RefreshCw :size="18" />
          </button>
        </div>
      </header>

      <p v-if="error" class="error-line">{{ error }}</p>

      <section v-if="activeView === 'dashboard'" class="view-stack">
        <div class="metric-grid">
          <article class="metric-card">
            <div>
              <Server :size="18" />
              <span>在线节点</span>
            </div>
            <strong>{{ overview.nodes_online }}/{{ overview.nodes_total }}</strong>
          </article>
          <article class="metric-card">
            <div>
              <Box :size="18" />
              <span>容器</span>
            </div>
            <strong>{{ overview.containers_total }}</strong>
          </article>
          <article class="metric-card warn">
            <div>
              <RefreshCw :size="18" />
              <span>可更新</span>
            </div>
            <strong>{{ overview.updates_available }}</strong>
          </article>
          <article class="metric-card danger">
            <div>
              <AlertTriangle :size="18" />
              <span>失败任务</span>
            </div>
            <strong>{{ overview.failed_tasks }}</strong>
          </article>
        </div>

        <div class="dashboard-grid">
          <section class="surface span-2">
            <div class="surface-head">
              <h2>节点运行态</h2>
              <button class="secondary" :disabled="!isAdmin || onlineNodes.length === 0 || isPending('detect-all')" @click="detectAllNodes">
                <Search :size="18" />
                全部检测
              </button>
            </div>
            <div class="node-table">
              <button
                v-for="node in nodes"
                :key="node.id"
                class="node-line"
                :class="{ selected: node.id === selectedNodeId }"
                @click="selectNode(node.id, 'nodes')"
              >
                <span class="status-dot" :class="node.status"></span>
                <strong>{{ node.name }}</strong>
                <span>{{ node.os || '-' }}/{{ node.arch || '-' }}</span>
                <span>{{ node.docker_version || 'Docker -' }}</span>
                <em :class="agentVersionBadgeClass(node)">{{ agentVersionLabel(node) }}</em>
              </button>
            </div>
          </section>

          <section class="surface">
            <div class="surface-head">
              <h2>资源</h2>
              <Activity :size="18" />
            </div>
            <div class="telemetry-list">
              <div class="telemetry-line">
                <Cpu :size="18" />
                <span>CPU</span>
                <strong>{{ formatPercent(overview.last_metric.cpu_percent) }}</strong>
                <div class="meter"><span :style="{ width: clampPercent(overview.last_metric.cpu_percent) + '%' }"></span></div>
              </div>
              <div class="telemetry-line">
                <MemoryStick :size="18" />
                <span>内存</span>
                <strong>{{ formatPercent(memoryPercent) }}</strong>
                <div class="meter"><span :style="{ width: clampPercent(memoryPercent) + '%' }"></span></div>
              </div>
              <div class="telemetry-line">
                <HardDrive :size="18" />
                <span>磁盘</span>
                <strong>{{ formatPercent(diskPercent) }}</strong>
                <div class="meter"><span :style="{ width: clampPercent(diskPercent) + '%' }"></span></div>
              </div>
              <div class="telemetry-line">
                <Network :size="18" />
                <span>网络</span>
                <strong>{{ formatBytes(overview.last_metric.network_rx + overview.last_metric.network_tx) }}</strong>
                <div class="meter accent"><span :style="{ width: '64%' }"></span></div>
              </div>
            </div>
          </section>

          <section class="surface">
            <div class="surface-head">
              <h2>任务队列</h2>
              <button class="ghost small-action" @click="activeView = 'tasks'">
                <ClipboardList :size="16" />
                查看
              </button>
            </div>
            <div class="compact-list">
              <button v-for="task in tasks.slice(0, 7)" :key="task.id" class="task-line" @click="openTask(task)">
                <span class="badge" :class="task.status">{{ statusText(task.status) }}</span>
                <strong>{{ taskTitle(task.kind) }}</strong>
                <small>{{ taskNodeName(task.node_id) }}</small>
              </button>
            </div>
          </section>
        </div>
      </section>

      <section v-if="activeView === 'nodes'" class="workbench">
        <aside class="node-rail surface">
          <div class="surface-head">
            <h2>节点</h2>
            <span class="count-chip">{{ filteredNodes.length }}</span>
          </div>
          <label class="search-field">
            <Search :size="16" />
            <input v-model="nodeSearch" placeholder="搜索节点" />
          </label>
          <div class="node-list">
            <button
              v-for="node in filteredNodes"
              :key="node.id"
              class="node-card"
              :class="{ selected: node.id === selectedNodeId }"
              @click="selectNode(node.id)"
            >
              <span class="status-dot" :class="node.status"></span>
              <strong>{{ node.name }}</strong>
              <small>{{ node.os || '-' }}/{{ node.arch || '-' }}</small>
              <em :class="agentVersionBadgeClass(node)">{{ agentVersionLabel(node) }}</em>
            </button>
          </div>
        </aside>

        <section class="node-workspace">
          <section class="surface node-summary">
            <div>
              <p class="eyebrow">{{ selectedNode?.status || '未选择' }}</p>
              <h2>{{ selectedNode?.name || '选择节点' }}</h2>
            </div>
            <div class="summary-facts">
              <span>Docker <strong>{{ selectedNode?.docker_version || '-' }}</strong></span>
              <span>Compose <strong>{{ selectedNode?.compose_version || '-' }}</strong></span>
              <span>Agent <strong>{{ selectedNode?.version ? `v${selectedNode.version}` : '-' }}</strong></span>
              <span>心跳 <strong>{{ selectedNode?.last_seen || '-' }}</strong></span>
            </div>
            <div class="button-row">
              <button class="secondary" :disabled="!selectedNodeId || !isAdmin" @click="createNodeTask('detect_updates')">
                <Search :size="18" />
                检测
              </button>
              <button class="secondary" :disabled="!selectedNodeId || !isAdmin" @click="createNodeTask('prune_images')">
                <Trash2 :size="18" />
                清理镜像
              </button>
              <button class="secondary" :disabled="!selectedNode || !isAdmin || !agentCanUpdate(selectedNode)" @click="upgradeAgent(selectedNode)">
                <RefreshCw :size="18" />
                升级 Agent
              </button>
            </div>
          </section>

          <div class="tabbar">
            <button :class="{ active: nodeDetailTab === 'containers' }" @click="nodeDetailTab = 'containers'">
              <Box :size="16" />
              容器
            </button>
            <button :class="{ active: nodeDetailTab === 'images' }" @click="nodeDetailTab = 'images'">
              <Package :size="16" />
              镜像
            </button>
            <button :class="{ active: nodeDetailTab === 'compose' }" @click="nodeDetailTab = 'compose'">
              <FileCode2 :size="16" />
              Compose
            </button>
            <button :class="{ active: nodeDetailTab === 'profile' }" @click="nodeDetailTab = 'profile'">
              <SlidersHorizontal :size="16" />
              信息
            </button>
          </div>

          <section v-if="nodeDetailTab === 'containers'" class="surface">
            <div class="surface-head">
              <h2>容器</h2>
              <div class="inline-tools">
                <label class="search-field compact-search">
                  <Search :size="16" />
                  <input v-model="containerSearch" placeholder="名称 / 镜像 / Compose" />
                </label>
                <div class="segmented compact-segmented">
                  <button :class="{ active: containerStateFilter === 'all' }" @click="containerStateFilter = 'all'">全部</button>
                  <button :class="{ active: containerStateFilter === 'running' }" @click="containerStateFilter = 'running'">运行</button>
                  <button :class="{ active: containerStateFilter === 'stopped' }" @click="containerStateFilter = 'stopped'">停止</button>
                  <button :class="{ active: containerStateFilter === 'updates' }" @click="containerStateFilter = 'updates'">更新</button>
                </div>
              </div>
            </div>
            <div class="data-table">
              <div class="table-row table-head">
                <span>名称</span>
                <span>镜像</span>
                <span>状态</span>
                <span>Compose</span>
                <span></span>
              </div>
              <div v-for="container in filteredContainers" :key="container.id" class="table-row">
                <strong>{{ container.name }}</strong>
                <span>{{ container.image }}</span>
                <span><em class="badge" :class="container.state">{{ container.state }}</em></span>
                <span>{{ container.compose_project || '-' }}</span>
                <span class="row-actions">
                  <button
                    class="icon-button"
                    title="重启容器"
                    :disabled="!isAdmin"
                    @click="createNodeTask('restart_container', 'container', container.id)"
                  >
                    <RotateCcw :size="16" />
                  </button>
                </span>
              </div>
            </div>
          </section>

          <section v-if="nodeDetailTab === 'images'" class="surface">
            <div class="surface-head">
              <h2>镜像</h2>
              <span class="count-chip">{{ dockerState.images.length }}</span>
            </div>
            <div class="data-table image-table">
              <div class="table-row table-head">
                <span>仓库</span>
                <span>标签</span>
                <span>大小</span>
                <span>创建时间</span>
              </div>
              <div v-for="image in dockerState.images" :key="image.id + image.repository + image.tag" class="table-row">
                <strong>{{ image.repository }}</strong>
                <span>{{ image.tag }}</span>
                <span>{{ image.size }}</span>
                <span>{{ image.created_at }}</span>
              </div>
            </div>
          </section>

          <section v-if="nodeDetailTab === 'compose'" class="surface">
            <div class="surface-head">
              <h2>Compose</h2>
              <button class="secondary" :disabled="!isAdmin" @click="newCompose">
                <Plus :size="18" />
                新建
              </button>
            </div>
            <div class="project-strip">
              <button v-for="project in dockerState.compose_projects" :key="project.id" class="project-pill" @click="openProject(project)">
                <strong>{{ project.name }}</strong>
                <span>{{ project.path }}</span>
                <em :class="detectionBadgeClass(project)">{{ detectionLabel(project) }}</em>
              </button>
            </div>
          </section>

          <section v-if="nodeDetailTab === 'profile'" class="surface">
            <div class="surface-head">
              <h2>节点信息</h2>
              <Save :size="18" />
            </div>
            <form class="form-stack" @submit.prevent="saveNode">
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
                  保存
                </button>
                <button class="secondary danger-action" type="button" :disabled="!selectedNodeId || !isAdmin" @click="deleteNode">
                  <Trash2 :size="18" />
                  删除
                </button>
              </div>
            </form>
          </section>
        </section>
      </section>

      <section v-if="activeView === 'projects'" class="view-stack">
        <section class="surface">
          <div class="surface-head">
            <h2>项目</h2>
            <div class="inline-tools">
              <label class="search-field compact-search">
                <Search :size="16" />
                <input v-model="projectSearch" placeholder="搜索项目 / 路径" />
              </label>
              <div class="segmented compact-segmented">
                <button :class="{ active: projectFilter === 'all' }" @click="projectFilter = 'all'">全部</button>
                <button :class="{ active: projectFilter === 'updates' }" @click="projectFilter = 'updates'">可更新</button>
                <button :class="{ active: projectFilter === 'failed' }" @click="projectFilter = 'failed'">异常</button>
                <button :class="{ active: projectFilter === 'current' }" @click="projectFilter = 'current'">正常</button>
              </div>
            </div>
          </div>
        </section>

        <div class="project-layout">
          <section class="surface project-list-panel">
            <div class="project-list">
              <button
                v-for="project in filteredProjects"
                :key="project.id"
                class="project-row"
                :class="{ selected: project.id === selectedProjectId }"
                @click="selectCompose(project)"
              >
                <strong>{{ project.name }}</strong>
                <span>{{ project.path }}</span>
                <em :class="detectionBadgeClass(project)">{{ detectionLabel(project) }}</em>
                <small v-if="detectionMeta(project)">{{ detectionMeta(project) }}</small>
              </button>
            </div>
          </section>

          <section class="surface project-editor-panel">
            <div class="surface-head">
              <h2>{{ composeForm.name || 'Compose' }}</h2>
              <div class="button-row compact-actions">
                <button class="secondary" type="button" :disabled="!isAdmin" @click="newCompose">
                  <Plus :size="18" />
                  新建
                </button>
                <button class="secondary" type="button" :disabled="!selectedProject || !isAdmin" @click="createSelectedProjectTask('detect_updates')">
                  <Search :size="18" />
                  检测
                </button>
                <button class="secondary" type="button" :disabled="!selectedProject || !isAdmin" @click="createSelectedProjectTask('compose_update')">
                  <Play :size="18" />
                  更新
                </button>
              </div>
            </div>
            <div v-if="selectedProject" class="project-meta">
              <span><strong>状态</strong><em :class="detectionBadgeClass(selectedProject)">{{ detectionLabel(selectedProject) }}</em></span>
              <span><strong>检测</strong>{{ detectionMeta(selectedProject) || '-' }}</span>
              <span v-if="selectedProject.detection_error" class="error-text"><strong>错误</strong>{{ selectedProject.detection_error }}</span>
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
                <button class="primary" type="submit" :disabled="!selectedNodeId || !isAdmin">
                  <Save :size="18" />
                  保存
                </button>
              </div>
            </form>
          </section>
        </div>
      </section>

      <section v-if="activeView === 'updates'" class="view-stack">
        <div class="metric-grid compact-metrics">
          <article class="metric-card">
            <div>
              <Shield :size="18" />
              <span>Server</span>
            </div>
            <strong>v{{ versionInfo.version }}</strong>
            <em :class="releaseStatusClass">{{ releaseStatusText }}</em>
          </article>
          <article class="metric-card">
            <div>
              <RefreshCw :size="18" />
              <span>最新发布</span>
            </div>
            <strong>{{ latestReleaseLabel }}</strong>
            <small>{{ versionInfo.release?.published_at || '-' }}</small>
          </article>
          <article class="metric-card warn">
            <div>
              <Package :size="18" />
              <span>项目更新</span>
            </div>
            <strong>{{ updateProjects.length }}</strong>
          </article>
          <article class="metric-card danger">
            <div>
              <AlertTriangle :size="18" />
              <span>检测异常</span>
            </div>
            <strong>{{ failedProjects.length }}</strong>
          </article>
        </div>

        <section class="surface">
          <div class="surface-head">
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

        <section class="surface">
          <div class="surface-head">
            <h2>Compose 更新</h2>
            <button class="secondary" :disabled="!isAdmin || !selectedNodeId" @click="createNodeTask('detect_updates')">
              <Search :size="18" />
              检测当前节点
            </button>
          </div>
          <div class="update-list">
            <article v-for="row in composePolicyRows" :key="row.project.id" class="update-row">
              <div class="update-main">
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
                <button class="icon-button" title="检测更新" :disabled="!isAdmin" @click="createProjectTask('detect_updates', row.project)">
                  <Search :size="16" />
                </button>
                <button class="icon-button" title="执行更新" :disabled="!isAdmin" @click="createProjectTask('compose_update', row.project)">
                  <Play :size="16" />
                </button>
              </div>
            </article>
          </div>
        </section>

        <section class="surface">
          <div class="surface-head">
            <h2>Agent 版本</h2>
            <button class="secondary" :disabled="!isAdmin || outdatedAgentCount === 0" @click="upgradeOutdatedAgents">
              <RefreshCw :size="18" />
              升级落后节点
            </button>
          </div>
          <div class="data-table agent-table">
            <div class="table-row table-head">
              <span>节点</span>
              <span>Agent</span>
              <span>系统</span>
              <span>状态</span>
              <span></span>
            </div>
            <div v-for="node in nodes" :key="node.id" class="table-row">
              <strong>{{ node.name }}</strong>
              <span>{{ node.version ? `v${node.version}` : '-' }}</span>
              <span>{{ node.os }}/{{ node.arch }}</span>
              <span><em :class="agentVersionBadgeClass(node)">{{ agentVersionLabel(node) }}</em></span>
              <span class="row-actions">
                <button class="icon-button" title="升级 Agent" :disabled="!isAdmin || !agentCanUpdate(node)" @click="upgradeAgent(node)">
                  <RefreshCw :size="16" />
                </button>
              </span>
            </div>
          </div>
        </section>
      </section>

      <section v-if="activeView === 'tasks'" class="task-layout">
        <section class="surface task-panel">
          <div class="surface-head">
            <h2>任务</h2>
            <div class="button-row compact-actions">
              <button class="secondary" :disabled="!isAdmin" @click="clearTasksScope('failed', '失败任务')">
                <Eraser :size="18" />
                清除失败
              </button>
              <button class="secondary" :disabled="!isAdmin" @click="clearTasksScope('finished', '已结束任务')">
                <Eraser :size="18" />
                清除历史
              </button>
            </div>
          </div>
          <div class="task-toolbar">
            <div class="segmented task-filter">
              <button :class="{ active: taskFilter === 'all' }" @click="taskFilter = 'all'">全部</button>
              <button :class="{ active: taskFilter === 'active' }" @click="taskFilter = 'active'">运行中</button>
              <button :class="{ active: taskFilter === 'failed' }" @click="taskFilter = 'failed'">失败</button>
            </div>
            <div class="task-counts">
              <span>{{ activeTasks.length }} 运行</span>
              <span>{{ failedTasks.length }} 失败</span>
            </div>
          </div>
          <div class="task-list">
            <button
              v-for="task in visibleTasks"
              :key="task.id"
              class="task-row"
              :class="{ selected: selectedTask?.id === task.id }"
              @click="openTask(task)"
            >
              <span class="badge" :class="task.status">{{ statusText(task.status) }}</span>
              <strong>{{ taskTitle(task.kind) }}</strong>
              <small>{{ taskMessage(task) || task.target_id || '-' }}</small>
              <small>{{ taskNodeName(task.node_id) }}</small>
              <small>{{ task.created_at }}</small>
            </button>
          </div>
        </section>

        <section class="surface task-detail">
          <div class="surface-head">
            <h2>详情</h2>
            <Terminal :size="18" />
          </div>
          <div class="task-meta" v-if="selectedTask">
            <span><strong>ID</strong>{{ selectedTask.id }}</span>
            <span><strong>节点</strong>{{ taskNodeName(selectedTask.node_id) }}</span>
            <span><strong>状态</strong><em class="badge" :class="selectedTask.status">{{ statusText(selectedTask.status) }}</em></span>
            <span><strong>结果</strong>{{ taskMessage(selectedTask) || '-' }}</span>
          </div>
          <pre class="logs">{{ selectedTaskLogs }}</pre>
        </section>
      </section>

      <section v-if="activeView === 'settings'" class="view-stack">
        <section class="surface">
          <div class="surface-head">
            <h2>版本与运行设置</h2>
            <Shield :size="18" />
          </div>
          <div class="settings-grid">
            <div class="fact-grid">
              <span>Server</span>
              <strong>v{{ versionInfo.version }}</strong>
              <span>最新发布</span>
              <strong>
                <a v-if="versionInfo.release?.url" class="text-link" :href="versionInfo.release.url" target="_blank" rel="noreferrer">
                  {{ latestReleaseLabel }}
                </a>
                <span v-else>{{ latestReleaseLabel }}</span>
              </strong>
              <span>发布状态</span>
              <strong><em :class="releaseStatusClass">{{ releaseStatusText }}</em></strong>
              <span>发布仓库</span>
              <strong>{{ versionInfo.release?.repository || runtimeSettings.release_repo || '-' }}</strong>
              <span>Commit</span>
              <strong>{{ shortCommit }}</strong>
              <span>构建时间</span>
              <strong>{{ versionInfo.build_date || '-' }}</strong>
              <span>服务时间</span>
              <strong>{{ versionInfo.server_time || '-' }}</strong>
              <span>时区</span>
              <strong>{{ versionInfo.time_zone || 'Asia/Shanghai' }}</strong>
            </div>
            <form class="form-stack" @submit.prevent="saveRuntimeSettings">
              <label class="checkline">
                <input v-model="runtimeSettings.agent_auto_update" type="checkbox" :disabled="!isAdmin" />
                Agent 自动升级
              </label>
              <label>
                <span>Agent 目标版本</span>
                <input v-model="runtimeSettings.agent_auto_update_version" :disabled="!isAdmin" placeholder="latest 或 v0.2.0" />
              </label>
              <label>
                <span>扫描间隔</span>
                <input :value="formatDuration(runtimeSettings.agent_auto_update_interval_seconds)" disabled />
              </label>
              <button class="primary aligned" :disabled="!isAdmin">
                <Save :size="18" />
                保存
              </button>
            </form>
          </div>
        </section>

        <section class="surface">
          <div class="surface-head">
            <h2>部署命令</h2>
            <Terminal :size="18" />
          </div>
          <div class="command-grid">
            <div v-for="item in commandItems" :key="item.label" class="command-item">
              <div class="command-title">
                <span>{{ item.label }}</span>
                <button class="icon-button" title="复制" @click="copyCommand(item.value)">
                  <Copy :size="16" />
                </button>
              </div>
              <div class="command-box">{{ item.value || '-' }}</div>
            </div>
          </div>
        </section>

        <section class="surface">
          <div class="surface-head">
            <h2>通知</h2>
            <Bell :size="18" />
          </div>
          <div class="notification-grid">
            <button v-for="item in notifications" :key="item.id" class="project-row" @click="editNotification(item)">
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

        <section class="surface">
          <div class="surface-head">
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

  <div class="toast-stack" aria-live="polite">
    <div v-for="toast in toasts" :key="toast.id" class="toast" :class="toast.type">
      {{ toast.message }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import {
  Activity,
  AlertTriangle,
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
  Package,
  Palette,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  Save,
  Search,
  Server,
  Settings,
  Shield,
  SlidersHorizontal,
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
  RuntimeSettings,
  Task,
  TaskLog,
  User,
  VersionInfo
} from './types'

type ViewName = 'dashboard' | 'nodes' | 'projects' | 'updates' | 'tasks' | 'settings'
type ThemeName = 'operator' | 'graphite' | 'ember' | 'terminal'
type ToastType = 'info' | 'success' | 'error'
type NodeDetailTab = 'containers' | 'images' | 'compose' | 'profile'
type ContainerFilter = 'all' | 'running' | 'stopped' | 'updates'
type ProjectFilter = 'all' | 'updates' | 'failed' | 'current'

interface Toast {
  id: number
  type: ToastType
  message: string
}

const THEME_KEY = 'dockpilot.theme'
const themes: { value: ThemeName; label: string }[] = [
  { value: 'operator', label: '运维' },
  { value: 'graphite', label: '石墨' },
  { value: 'ember', label: '日冕' },
  { value: 'terminal', label: '终端' }
]
const savedTheme = localStorage.getItem(THEME_KEY) as ThemeName | null
const token = ref(getToken())
const user = ref<AuthClaims | null>(null)
const activeView = ref<ViewName>('dashboard')
const themeName = ref<ThemeName>(
  themes.some((theme) => theme.value === savedTheme) ? (savedTheme as ThemeName) : 'operator'
)
const themeMenuOpen = ref(false)
const busy = ref(false)
const error = ref('')
const selectedNodeId = ref('')
const selectedProjectId = ref('')
const selectedTask = ref<Task | null>(null)
const taskLogs = ref<TaskLog[]>([])
const taskFilter = ref<'all' | 'active' | 'failed'>('all')
const nodeDetailTab = ref<NodeDetailTab>('containers')
const nodeSearch = ref('')
const containerSearch = ref('')
const containerStateFilter = ref<ContainerFilter>('all')
const projectSearch = ref('')
const projectFilter = ref<ProjectFilter>('all')
const toasts = ref<Toast[]>([])
const pendingActions = ref<string[]>([])
const currentClock = ref('')
let clockTimer: number | undefined
let refreshTimer: number | undefined
let toastID = 0

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
  server_time: '',
  release: undefined,
  settings: undefined
})
const runtimeSettings = reactive<RuntimeSettings>({
  release_repo: 'RY-zzcn/DockPilot',
  release_cache_seconds: 900,
  agent_auto_update: false,
  agent_auto_update_version: 'latest',
  agent_auto_update_interval_seconds: 3600
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
const selectedProject = computed(() => dockerState.compose_projects.find((project) => project.id === selectedProjectId.value))
const onlineNodes = computed(() => nodes.value.filter((node) => node.status === 'online'))
const activeTasks = computed(() => tasks.value.filter((task) => task.status === 'pending' || task.status === 'running'))
const failedTasks = computed(() => tasks.value.filter((task) => task.status === 'failed'))
const memoryPercent = computed(() => percent(overview.last_metric.memory_used, overview.last_metric.memory_total))
const diskPercent = computed(() => percent(overview.last_metric.disk_used, overview.last_metric.disk_total))
const shortCommit = computed(() => (versionInfo.commit && versionInfo.commit !== 'dev' ? versionInfo.commit.slice(0, 12) : versionInfo.commit || '-'))
const latestReleaseVersion = computed(() => versionInfo.release?.latest_version || '')
const currentThemeLabel = computed(() => themes.find((theme) => theme.value === themeName.value)?.label || '主题')
const agentTargetVersion = computed(() => {
  const configured = runtimeSettings.agent_auto_update_version || 'latest'
  if (configured === 'latest') {
    return latestReleaseVersion.value || 'latest'
  }
  return configured
})
const latestReleaseLabel = computed(() => {
  if (versionInfo.release?.error && !latestReleaseVersion.value) return '获取失败'
  return latestReleaseVersion.value ? `v${latestReleaseVersion.value}` : '-'
})
const releaseStatusText = computed(() => {
  if (versionInfo.release?.error && !latestReleaseVersion.value) return versionInfo.release.error
  if (!latestReleaseVersion.value) return '未获取'
  return versionInfo.release?.update_available ? 'Server 可升级' : 'Server 已是最新'
})
const releaseStatusClass = computed(() => {
  if (versionInfo.release?.error && !latestReleaseVersion.value) return 'mini-danger'
  return versionInfo.release?.update_available ? 'mini-alert' : 'mini-muted'
})
const outdatedAgentCount = computed(() => nodes.value.filter((node) => agentCanUpdate(node)).length)
const viewTitle = computed(() => {
  const titles: Record<ViewName, string> = {
    dashboard: '总览',
    nodes: selectedNode.value?.name || '节点',
    projects: '项目',
    updates: '更新',
    tasks: '任务',
    settings: '设置'
  }
  return titles[activeView.value]
})
const viewSection = computed(() => {
  const sections: Record<ViewName, string> = {
    dashboard: '运行态',
    nodes: '运行态',
    projects: '运行态',
    updates: '自动化',
    tasks: '自动化',
    settings: '系统'
  }
  return sections[activeView.value]
})
const filteredNodes = computed(() => {
  const keyword = nodeSearch.value.trim().toLowerCase()
  if (!keyword) return nodes.value
  return nodes.value.filter((node) =>
    [node.name, node.id, node.note, node.os, node.arch, node.docker_version, node.compose_version]
      .filter(Boolean)
      .some((value) => value.toLowerCase().includes(keyword))
  )
})
const filteredContainers = computed(() => {
  const keyword = containerSearch.value.trim().toLowerCase()
  return dockerState.containers.filter((container) => {
    const stateMatch =
      containerStateFilter.value === 'all' ||
      (containerStateFilter.value === 'running' && container.state === 'running') ||
      (containerStateFilter.value === 'stopped' && container.state !== 'running') ||
      (containerStateFilter.value === 'updates' && container.update_available)
    if (!stateMatch) return false
    if (!keyword) return true
    return [container.name, container.image, container.state, container.status, container.compose_project]
      .filter(Boolean)
      .some((value) => value.toLowerCase().includes(keyword))
  })
})
const filteredProjects = computed(() => {
  const keyword = projectSearch.value.trim().toLowerCase()
  return dockerState.compose_projects.filter((project) => {
    const statusMatch =
      projectFilter.value === 'all' ||
      (projectFilter.value === 'updates' && project.update_available) ||
      (projectFilter.value === 'failed' && (project.detection_status === 'failed' || project.detection_status === 'partial')) ||
      (projectFilter.value === 'current' && !project.update_available && project.detection_status !== 'failed' && project.detection_status !== 'partial')
    if (!statusMatch) return false
    if (!keyword) return true
    return [project.name, project.path, project.detection_status, project.detection_error || '']
      .filter(Boolean)
      .some((value) => value.toLowerCase().includes(keyword))
  })
})
const updateProjects = computed(() => dockerState.compose_projects.filter((project) => project.update_available))
const failedProjects = computed(() =>
  dockerState.compose_projects.filter((project) => project.detection_status === 'failed' || project.detection_status === 'partial')
)
const composePolicyRows = computed(() =>
  dockerState.compose_projects.map((project) => ({
    project,
    policy: policyDraftFor('compose', project.id)
  }))
)
const visibleTasks = computed(() => {
  if (taskFilter.value === 'failed') {
    return failedTasks.value
  }
  if (taskFilter.value === 'active') {
    return activeTasks.value
  }
  return tasks.value
})
const selectedTaskLogs = computed(() => {
  if (!selectedTask.value) {
    return '选择一个任务查看日志'
  }
  if (taskLogs.value.length === 0) {
    return `${selectedTask.value.id}\n暂无日志`
  }
  return taskLogs.value.map((line) => `[${line.created_at}] ${line.line}`).join('\n')
})
const commandItems = computed(() => [
  { label: '交互式部署/卸载', value: installInfo.interactive || '' },
  { label: 'Agent 二进制接入', value: installInfo.agent_binary || installInfo.binary_command || '' },
  { label: 'Agent Docker 接入', value: installInfo.agent_docker || installInfo.docker_command || '' },
  { label: 'Server Docker 部署', value: installInfo.server_docker || '' },
  { label: 'Server 二进制部署', value: installInfo.server_binary || '' },
  { label: '交互式卸载', value: installInfo.uninstall || '' },
  { label: '仅卸载 Agent', value: installInfo.uninstall_agent || '' },
  { label: '仅卸载 Server', value: installInfo.uninstall_server || '' },
  { label: '全部卸载', value: installInfo.uninstall_all || '' },
  { label: '彻底卸载', value: installInfo.uninstall_purge || '' }
])

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
    selectedProjectId.value = ''
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

function chooseTheme(value: ThemeName) {
  themeName.value = value
  themeMenuOpen.value = false
}

function notify(message: string, type: ToastType = 'info') {
  const id = ++toastID
  toasts.value = [...toasts.value, { id, type, message }]
  window.setTimeout(() => {
    toasts.value = toasts.value.filter((toast) => toast.id !== id)
  }, type === 'error' ? 5200 : 3200)
}

function isPending(key: string) {
  return pendingActions.value.includes(key)
}

function setPending(key: string, pending: boolean) {
  if (pending) {
    if (!pendingActions.value.includes(key)) {
      pendingActions.value = [...pendingActions.value, key]
    }
    return
  }
  pendingActions.value = pendingActions.value.filter((item) => item !== key)
}

async function runAction<T>(key: string, startMessage: string, successMessage: string, action: () => Promise<T>) {
  if (isPending(key)) {
    notify('该操作正在处理中', 'info')
    return undefined
  }
  setPending(key, true)
  error.value = ''
  notify(startMessage, 'info')
  try {
    const result = await action()
    notify(successMessage, 'success')
    return result
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    error.value = message
    notify(`操作失败：${message}`, 'error')
    return undefined
  } finally {
    setPending(key, false)
  }
}

async function manualRefresh() {
  await runAction('refresh', '正在刷新', '数据已刷新', refreshAll)
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
    if (versionData.settings) {
      Object.assign(runtimeSettings, versionData.settings)
    }
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
  const [install, userList, settings] = await Promise.all([api.installInfo(), api.users(), api.runtimeSettings()])
  Object.assign(installInfo, install)
  Object.assign(runtimeSettings, settings)
  users.value = userList
}

async function loadDocker(nodeId: string) {
  const state = await api.dockerState(nodeId)
  dockerState.containers = state.containers
  dockerState.images = state.images
  dockerState.compose_projects = state.compose_projects
  syncProjectSelection()
}

function syncProjectSelection() {
  if (selectedProjectId.value) {
    const current = dockerState.compose_projects.find((project) => project.id === selectedProjectId.value)
    if (current) {
      editCompose(current)
      return
    }
  }
  const first = dockerState.compose_projects[0]
  if (first) {
    selectCompose(first)
    return
  }
  selectedProjectId.value = ''
  composeForm.id = ''
  composeForm.name = ''
  composeForm.path = ''
  composeForm.content = ''
  composeForm.deploy_now = false
}

function selectNode(nodeId: string, view?: ViewName) {
  selectedNodeId.value = nodeId
  if (view) activeView.value = view
}

function syncNodeForm() {
  nodeForm.name = selectedNode.value?.name || ''
  nodeForm.note = selectedNode.value?.note || ''
}

async function saveNode() {
  if (!selectedNodeId.value) return
  const saved = await runAction('save-node', '正在保存节点信息', '节点信息已保存', () =>
    api.updateNode(selectedNodeId.value, nodeForm)
  )
  if (!saved) return
  nodes.value = nodes.value.map((node) => (node.id === saved.id ? saved : node))
}

async function deleteNode() {
  if (!selectedNodeId.value || !selectedNode.value) return
  const confirmed = window.confirm(`删除节点 ${selectedNode.value.name}？如果该 Agent 仍在运行，它可能会重新注册。`)
  if (!confirmed) return
  const deletedID = selectedNodeId.value
  const result = await runAction('delete-node', '正在删除节点', '节点已删除', () => api.deleteNode(deletedID))
  if (!result) return
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
  const task = await createTaskForNode(selectedNodeId.value, kind, targetType, targetId)
  if (!task) return
  await refreshTasks()
}

async function createTaskForNode(nodeId: string, kind: string, targetType = '', targetId = '') {
  return runAction(`task:${nodeId}:${kind}:${targetId}`, `正在创建任务：${taskTitle(kind)}`, `任务已创建：${taskTitle(kind)}`, () =>
    api.createTask({ node_id: nodeId, kind, target_type: targetType, target_id: targetId, args: {} })
  )
}

async function detectAllNodes() {
  const targets = nodes.value.filter((node) => node.status === 'online')
  if (targets.length === 0) return
  const created = await runAction('detect-all', '正在创建检测任务', '检测任务已创建', async () => {
    await Promise.all(targets.map((node) => api.createTask({ node_id: node.id, kind: 'detect_updates', args: {} })))
    return true
  })
  if (!created) return
  await refreshTasks()
}

async function upgradeAgent(node?: Node) {
  if (!node) return
  const task = await runAction(`agent-update:${node.id}`, `正在创建 ${node.name} 的 Agent 升级任务`, 'Agent 升级任务已创建', () =>
    createAgentUpdateTask(node)
  )
  if (!task) return
  await refreshTasks()
}

async function upgradeOutdatedAgents() {
  const targets = nodes.value.filter((node) => agentCanUpdate(node))
  if (targets.length === 0) return
  const confirmed = window.confirm(`为 ${targets.length} 个落后节点创建 Agent 升级任务？`)
  if (!confirmed) return
  const created = await runAction('agent-update-batch', '正在创建批量 Agent 升级任务', '批量升级任务已创建', async () => {
    for (const node of targets) {
      await createAgentUpdateTask(node)
    }
    return true
  })
  if (!created) return
  await refreshTasks()
}

function createAgentUpdateTask(node: Node) {
  return api.createTask({
    node_id: node.id,
    kind: 'agent_update',
    target_type: 'node',
    target_id: node.id,
    args: { version: agentTargetVersion.value }
  })
}

async function createProjectTask(kind: string, project: ComposeProject) {
  const task = await runAction(`project-task:${project.id}:${kind}`, `正在创建 ${project.name} 的${taskTitle(kind)}任务`, `任务已创建：${project.name}`, () =>
    api.createTask({
      node_id: project.node_id,
      kind,
      target_type: 'compose',
      target_id: project.id,
      args: { path: project.path, name: project.name }
    })
  )
  if (!task) return
  await refreshTasks()
}

async function createSelectedProjectTask(kind: string) {
  if (!selectedProject.value) return
  await createProjectTask(kind, selectedProject.value)
}

async function refreshTasks() {
  tasks.value = await api.tasks()
}

async function clearTasksScope(scope: 'finished' | 'failed', label: string) {
  const confirmed = window.confirm(`清除${label}？正在运行和排队任务会保留。`)
  if (!confirmed) return
  const result = await runAction(`clear-tasks:${scope}`, `正在清除${label}`, `${label}已清除`, () => api.clearTasks(scope))
  if (!result) return
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

function openProject(project: ComposeProject) {
  activeView.value = 'projects'
  selectCompose(project)
}

function selectCompose(project: ComposeProject) {
  selectedProjectId.value = project.id
  editCompose(project)
}

function editCompose(project: ComposeProject) {
  composeForm.id = project.id
  composeForm.name = project.name
  composeForm.path = project.path
  composeForm.content = project.content || ''
  composeForm.deploy_now = false
}

function newCompose() {
  activeView.value = 'projects'
  selectedProjectId.value = ''
  composeForm.id = ''
  composeForm.name = ''
  composeForm.path = '/opt/app/compose.yml'
  composeForm.content = 'services:\n  app:\n    image: nginx:stable\n    restart: unless-stopped\n'
  composeForm.deploy_now = false
}

async function saveCompose() {
  if (!selectedNodeId.value) return
  const saved = await runAction('save-compose', composeForm.deploy_now ? '正在保存并创建部署任务' : '正在保存 Compose', composeForm.deploy_now ? 'Compose 已保存，部署任务已创建' : 'Compose 已保存', () =>
    api.saveCompose({
      node_id: selectedNodeId.value,
      id: composeForm.id || undefined,
      name: composeForm.name,
      path: composeForm.path,
      content: composeForm.content,
      deploy_now: composeForm.deploy_now
    })
  )
  if (!saved) return
  selectedProjectId.value = saved.id
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
  const saved = await runAction(`policy:${policy.scope}:${policy.scope_id}`, '正在保存策略', '策略已保存', () => api.savePolicy(policy))
  if (!saved) return
  policyDrafts[policyKey(saved.scope, saved.scope_id)] = { ...saved }
  policies.value = await api.policies()
}

async function saveRuntimeSettings() {
  const saved = await runAction('runtime-settings', '正在保存运行设置', '运行设置已保存', () =>
    api.saveRuntimeSettings({
      agent_auto_update: runtimeSettings.agent_auto_update,
      agent_auto_update_version: runtimeSettings.agent_auto_update_version || 'latest'
    })
  )
  if (!saved) return
  Object.assign(runtimeSettings, saved)
  versionInfo.settings = { ...saved }
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
  const saved = await runAction('notification', '正在保存通知渠道', '通知渠道已保存', () => api.saveNotification(notificationForm))
  if (!saved) return
  notifications.value = await api.notifications()
}

async function createUser() {
  const created = await runAction('create-user', '正在创建用户', '用户已创建', () => api.createUser(userForm))
  if (!created) return
  userForm.username = ''
  userForm.password = ''
  users.value = await api.users()
}

async function copyCommand(command: string) {
  if (!command) return
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(command)
      notify('命令已复制', 'success')
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
    notify('命令已复制', 'success')
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    notify(`复制失败：${message}`, 'error')
  }
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

function taskMessage(task: Task) {
  if (!task.result) return ''
  try {
    const result = JSON.parse(task.result) as { message?: string }
    return result.message || ''
  } catch {
    return ''
  }
}

function taskTitle(kind: string) {
  const titles: Record<string, string> = {
    detect_updates: '检测更新',
    compose_update: '执行更新',
    compose_deploy: '部署 Compose',
    restart_container: '重启容器',
    prune_images: '清理镜像',
    agent_update: '升级 Agent'
  }
  return titles[kind] || kind
}

function statusText(status: string) {
  const labels: Record<string, string> = {
    pending: '等待',
    running: '运行',
    success: '成功',
    failed: '失败',
    canceled: '取消'
  }
  return labels[status] || status
}

function taskNodeName(nodeID: string) {
  return nodes.value.find((node) => node.id === nodeID)?.name || nodeID
}

function agentCanUpdate(node?: Node) {
  if (!node || node.status !== 'online') return false
  const latest = latestReleaseVersion.value || (agentTargetVersion.value !== 'latest' ? agentTargetVersion.value : '')
  return !!latest && compareVersions(node.version, latest) < 0
}

function agentVersionLabel(node: Node) {
  if (node.status !== 'online') return '离线'
  const latest = latestReleaseVersion.value
  if (!node.version) return latest ? `可升级到 v${latest}` : '版本未知'
  if (!latest) return `Agent v${node.version}`
  if (compareVersions(node.version, latest) < 0) return `可升级到 v${latest}`
  return '最新'
}

function agentVersionBadgeClass(node: Node) {
  if (node.status !== 'online') return 'mini-danger'
  if (agentCanUpdate(node)) return 'mini-alert'
  return 'mini-muted'
}

function cleanVersion(value: string) {
  return (value || '').trim().replace(/^v/, '')
}

function compareVersions(left: string, right: string) {
  const a = cleanVersion(left)
  const b = cleanVersion(right)
  if (a === b) return 0
  if (!a) return -1
  if (!b) return 1
  const aHasDigit = /\d/.test(a)
  const bHasDigit = /\d/.test(b)
  if (!aHasDigit && bHasDigit) return -1
  if (aHasDigit && !bHasDigit) return 1
  const parse = (value: string) => value.split(/[.+-]/).map((part) => Number.parseInt(part, 10) || 0)
  const ap = parse(a)
  const bp = parse(b)
  const length = Math.max(ap.length, bp.length)
  for (let index = 0; index < length; index += 1) {
    const av = ap[index] || 0
    const bv = bp[index] || 0
    if (av < bv) return -1
    if (av > bv) return 1
  }
  return 0
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

function formatDuration(seconds: number) {
  if (!seconds) return '-'
  if (seconds % 86400 === 0) return `${seconds / 86400} 天`
  if (seconds % 3600 === 0) return `${seconds / 3600} 小时`
  if (seconds % 60 === 0) return `${seconds / 60} 分钟`
  return `${seconds} 秒`
}
</script>
