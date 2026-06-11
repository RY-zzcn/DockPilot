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

        <div class="dashboard-grid dashboard-layout">
          <section class="surface node-resource-panel span-2">
            <div class="surface-head">
              <div>
                <h2>节点与资源</h2>
                <p class="muted-line">资源每 {{ formatDuration(METRICS_PROBE_INTERVAL_SECONDS) }} 上报，显示 CPU、内存、硬盘。</p>
              </div>
              <div class="button-row compact-actions">
                <span class="count-chip">{{ dashboardNodeRows.length }} 个节点</span>
                <button class="secondary" :disabled="!isAdmin || onlineNodes.length === 0 || isPending('detect-all')" @click="detectAllNodes">
                  <Search :size="18" />
                  全部检测
                </button>
              </div>
            </div>
            <div class="node-resource-list">
              <button
                v-for="row in dashboardNodeRows"
                :key="row.node.id"
                class="node-resource-row"
                :class="{ selected: row.node.id === selectedNodeId }"
                :title="nodeResourceTitle(row)"
                @click="selectNode(row.node.id, 'nodes')"
              >
                <div class="node-resource-main">
                  <span class="status-dot" :class="row.node.status"></span>
                  <div>
                    <strong :title="row.node.name">{{ row.node.name }}</strong>
                    <small :title="`${agentInstallModeLabel(row.node)} · ${row.node.os || '-'}/${row.node.arch || '-'}`">
                      {{ agentInstallModeLabel(row.node) }} · {{ row.node.os || '-' }}/{{ row.node.arch || '-' }}
                    </small>
                  </div>
                </div>
                <div class="node-resource-meta">
                  <em :class="agentVersionBadgeClass(row.node)">{{ agentVersionLabel(row.node) }}</em>
                  <span :title="row.node.last_seen || '-'">心跳 {{ shortTime(row.node.last_seen) }}</span>
                </div>
                <div class="node-resource-metrics">
                  <div class="resource-mini" :class="{ muted: !row.metric }" :title="row.metric ? metricDetailTitle(row.metric, 'cpu') : '等待节点上报 CPU 数据'">
                    <div>
                      <Cpu :size="15" />
                      <span>CPU</span>
                      <strong>{{ row.metric ? formatPercent(row.metric.cpu_percent) : '等待' }}</strong>
                    </div>
                    <div class="meter"><span :style="{ width: row.metric ? clampPercent(row.metric.cpu_percent) + '%' : '0%' }"></span></div>
                  </div>
                  <div class="resource-mini" :class="{ muted: !row.metric }" :title="row.metric ? metricDetailTitle(row.metric, 'memory') : '等待节点上报内存数据'">
                    <div>
                      <MemoryStick :size="15" />
                      <span>内存</span>
                      <strong>{{ row.metric ? formatPercent(metricMemoryPercent(row.metric)) : '等待' }}</strong>
                    </div>
                    <div class="meter"><span :style="{ width: row.metric ? clampPercent(metricMemoryPercent(row.metric)) + '%' : '0%' }"></span></div>
                  </div>
                  <div class="resource-mini" :class="{ muted: !row.metric }" :title="row.metric ? metricDetailTitle(row.metric, 'disk') : '等待节点上报硬盘数据'">
                    <div>
                      <HardDrive :size="15" />
                      <span>磁盘</span>
                      <strong>{{ row.metric ? formatPercent(metricDiskPercent(row.metric)) : '等待' }}</strong>
                    </div>
                    <div class="meter"><span :style="{ width: row.metric ? clampPercent(metricDiskPercent(row.metric)) + '%' : '0%' }"></span></div>
                  </div>
                </div>
                <div class="node-resource-details">
                  <span :title="row.node.docker_version || '-'">Docker {{ shortLabel(row.node.docker_version) }}</span>
                  <span :title="row.node.compose_version || '-'">Compose {{ shortLabel(row.node.compose_version) }}</span>
                  <span :title="row.metric?.recorded_at || '等待节点资源上报'">资源 {{ row.metric ? shortTime(row.metric.recorded_at) : '等待上报' }}</span>
                </div>
              </button>
              <p v-if="dashboardNodeRows.length === 0" class="empty-hint compact-empty">暂无节点。</p>
            </div>
          </section>

          <section class="surface attention-panel" :class="{ quiet: attentionItems.length === 0 }">
            <div class="surface-head">
              <div class="attention-summary">
                <AlertTriangle :size="18" />
                <div>
                  <h2>异常与更新</h2>
                  <p>{{ attentionItems.length > 0 ? `需要处理 ${attentionItems.length} 项` : '当前状态正常' }}</p>
                </div>
              </div>
              <button class="ghost small-action" @click="activeView = 'updates'">
                <RefreshCw :size="16" />
                查看
              </button>
            </div>
            <div class="attention-actions compact-attention-actions">
              <button v-for="item in attentionItems" :key="item.key" class="attention-pill" :title="`${item.title}\n${item.detail}`" @click="item.action">
                <span class="badge pending">{{ item.kind }}</span>
                <strong>{{ item.title }}</strong>
                <small>{{ item.detail }}</small>
              </button>
              <p v-if="attentionItems.length === 0" class="empty-hint compact-empty">当前没有需要处理的项目。</p>
            </div>
          </section>

          <section class="surface dashboard-task-panel">
            <div class="surface-head">
              <h2>任务队列</h2>
              <button class="ghost small-action" @click="activeView = 'tasks'">
                <ClipboardList :size="16" />
                查看
              </button>
            </div>
            <div class="compact-list">
              <button v-for="task in dashboardTaskItems" :key="task.id" class="task-line dashboard-task-line" @click="openTask(task)">
                <span class="badge" :class="task.status">{{ statusText(task.status) }}</span>
                <strong>{{ taskTitle(task.kind) }}</strong>
                <small>{{ taskNodeName(task.node_id) }}</small>
              </button>
              <p v-if="dashboardTaskItems.length === 0" class="empty-hint compact-empty">暂无任务。</p>
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
              :title="nodeTitle(node)"
              @click="selectNode(node.id)"
            >
              <span class="status-dot" :class="node.status"></span>
              <strong>{{ node.name }}</strong>
              <small>{{ agentInstallModeLabel(node) }} · {{ node.os || '-' }}/{{ node.arch || '-' }}</small>
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
              <span title="Docker 版本">Docker <strong :title="selectedNode?.docker_version || '-'">{{ selectedNode?.docker_version || '-' }}</strong></span>
              <span title="Docker Compose 版本">Compose <strong :title="selectedNode?.compose_version || '-'">{{ selectedNode?.compose_version || '-' }}</strong></span>
              <span title="Agent 版本">Agent <strong :title="selectedNode?.version ? `v${selectedNode.version}` : '-'">{{ selectedNode?.version ? `v${selectedNode.version}` : '-' }}</strong></span>
              <span title="Agent 安装方式">安装 <strong>{{ agentInstallModeLabel(selectedNode) }}</strong></span>
              <span title="最近心跳">心跳 <strong :title="selectedNode?.last_seen || '-'">{{ selectedNode?.last_seen || '-' }}</strong></span>
            </div>
            <div class="button-row">
              <button class="secondary" :disabled="!selectedNodeId || !isAdmin || !canRunTask(selectedNode, 'detect_updates')" @click="createNodeTask('detect_updates')">
                <Search :size="18" />
                检测
              </button>
              <button class="secondary" :disabled="!selectedNodeId || !isAdmin || !canRunTask(selectedNode, 'prune_images')" :title="capabilityWarning('prune_images')" @click="createNodeTask('prune_images')">
                <Trash2 :size="18" />
                清理镜像
              </button>
              <button class="secondary" :disabled="!selectedNode || !isAdmin || !agentCanUpdate(selectedNode)" :title="capabilityWarning('agent_update')" @click="upgradeAgent(selectedNode)">
                <RefreshCw :size="18" />
                升级 Agent
              </button>
            </div>
          </section>

          <details class="surface node-install-drawer">
            <summary>
              <span><Server :size="18" /> 添加节点</span>
              <small>生成带本地能力限制的 Agent 安装命令</small>
            </summary>
            <div class="node-install-builder compact-builder">
              <div class="form-grid">
                <label>
                  <span>节点名称</span>
                  <input v-model="agentInstallForm.node_name" :disabled="!isAdmin" placeholder="默认使用主机名" />
                </label>
                <label>
                  <span>Compose 扫描目录</span>
                  <input v-model="agentInstallForm.compose_dirs" :disabled="!isAdmin" placeholder="/opt,/srv,/var/www" />
                </label>
                <label>
                  <span>Agent 版本</span>
                  <input v-model="agentInstallForm.version" :disabled="!isAdmin" placeholder="latest 或 v0.2.20" />
                </label>
                <div class="install-mode">
                  <span>安装方式</span>
                  <div class="segmented compact-segmented">
                    <button type="button" :class="{ active: agentInstallForm.mode === 'docker' }" :disabled="!isAdmin" @click="agentInstallForm.mode = 'docker'">Docker</button>
                    <button type="button" :class="{ active: agentInstallForm.mode === 'binary' }" :disabled="!isAdmin" @click="agentInstallForm.mode = 'binary'">二进制</button>
                  </div>
                </div>
              </div>
              <div class="permission-grid">
                <label class="checkline capability-toggle">
                  <input v-model="agentInstallForm.self_update" type="checkbox" :disabled="!isAdmin" />
                  Agent 自更新
                </label>
                <label v-for="item in agentCapabilityOptions" :key="item.key" class="checkline capability-toggle">
                  <input v-model="agentInstallForm[item.key]" type="checkbox" :disabled="!isAdmin" />
                  {{ item.label }}
                </label>
              </div>
              <div class="install-summary">
                <span v-for="item in agentInstallSummary" :key="item" class="capability-chip enabled">{{ item }}</span>
                <span v-if="agentInstallSummary.length === 0" class="capability-chip">仅监控与检测</span>
              </div>
              <div class="command-item install-command">
                <div class="command-title">
                  <span>{{ agentInstallForm.mode === 'docker' ? 'Agent Docker 安装命令' : 'Agent 二进制安装命令' }}</span>
                  <button class="icon-button" title="复制" :disabled="!agentInstallCommand" @click="copyCommand(agentInstallCommand)">
                    <Copy :size="16" />
                  </button>
                </div>
                <div class="command-box">{{ agentInstallCommand || '-' }}</div>
              </div>
            </div>
          </details>

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
                <strong :title="container.name">{{ container.name }}</strong>
                <span :title="container.image">{{ container.image }}</span>
                <span :title="container.status || container.state"><em class="badge" :class="container.state">{{ container.state }}</em></span>
                <span :title="container.compose_project || '-'">{{ container.compose_project || '-' }}</span>
                <span class="row-actions">
                  <button
                    class="icon-button"
                    title="重启容器"
                    :disabled="!isAdmin || !canRunTask(selectedNode, 'restart_container')"
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
                <strong :title="image.repository">{{ image.repository }}</strong>
                <span :title="image.tag">{{ image.tag }}</span>
                <span :title="image.size">{{ image.size }}</span>
                <span :title="image.created_at">{{ image.created_at }}</span>
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
              <button v-for="project in dockerState.compose_projects" :key="project.id" class="project-pill" :title="composeProjectTitle(project)" @click="openProject(project)">
                <strong :title="project.name">{{ project.name }}</strong>
                <span :title="project.path">{{ project.path }}</span>
                <em :class="detectionBadgeClass(project)" :title="detectionTitle(project)">{{ detectionLabel(project) }}</em>
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
              <div class="capability-grid">
                <span class="capability-chip" :class="{ enabled: agentInstallMode(selectedNode) !== 'unknown' }">
                  安装 · {{ agentInstallModeLabel(selectedNode) }}
                </span>
                <span v-for="item in capabilityItems(selectedNode)" :key="item.label" class="capability-chip" :class="{ enabled: item.enabled }">
                  {{ item.label }} · {{ item.enabled ? '允许' : '关闭' }}
                </span>
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
                <button :class="{ active: projectFilter === 'failed' }" @click="projectFilter = 'failed'">需处理</button>
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
                :title="composeProjectTitle(project)"
                @click="selectCompose(project)"
              >
                <strong :title="project.name">{{ project.name }}</strong>
                <span :title="project.path">{{ project.path }}</span>
                <small>{{ composeOwnershipLabel(project) }}</small>
                <em :class="detectionBadgeClass(project)" :title="detectionTitle(project)">{{ detectionLabel(project) }}</em>
                <small v-if="detectionMeta(project)" :title="detectionMeta(project)">{{ detectionMeta(project) }}</small>
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
                <button class="secondary" type="button" :disabled="!selectedProject || !isAdmin || !canRunTask(selectedNode, 'detect_updates')" @click="createSelectedProjectTask('detect_updates')">
                  <Search :size="18" />
                  检测
                </button>
                <button class="secondary" type="button" :disabled="!selectedProject || !isAdmin || !canRunTask(selectedNode, 'compose_update')" :title="capabilityWarning('compose_update')" @click="createSelectedProjectTask('compose_update')">
                  <Play :size="18" />
                  更新
                </button>
              </div>
            </div>
            <p v-if="selectedProject && !selectedProject.managed" class="security-note">
              扫描到的 Compose 默认只读，面板不显示文件内容，也不能直接覆盖修改。转为托管时必须粘贴节点原文件完整内容，哈希一致才会通过。
            </p>
            <details v-if="selectedProject && !selectedProject.managed" class="advanced-box">
              <summary>高级操作</summary>
              <p class="muted-line">
                源文件 SHA256：{{ selectedProject.content_hash ? shortHash(selectedProject.content_hash) : '等待 Agent 刷新后可校验' }}
              </p>
              <div class="button-row">
                <button class="secondary" type="button" :disabled="!isAdmin || selectedProject.imported" @click="importSelectedCompose('read_only')">
                  <FileCode2 :size="16" />
                  导入只读
                </button>
                <button class="secondary danger-action" type="button" :disabled="!isAdmin || !selectedProject.content_hash" @click="importSelectedCompose('managed')">
                  <Shield :size="16" />
                  转为托管
                </button>
              </div>
            </details>
            <div v-if="selectedProject" class="project-meta">
              <span><strong>归属</strong>{{ composeOwnershipLabel(selectedProject) }}</span>
              <span><strong>状态</strong><em :class="detectionBadgeClass(selectedProject)" :title="detectionTitle(selectedProject)">{{ detectionLabel(selectedProject) }}</em></span>
              <span><strong>检测</strong>{{ detectionMeta(selectedProject) || '-' }}</span>
              <span v-if="detectionReason(selectedProject)" class="error-text"><strong>原因</strong>{{ detectionReason(selectedProject) }}</span>
            </div>
            <form class="compose-editor" @submit.prevent="saveCompose">
              <div class="form-grid">
                <label>
                  <span>名称</span>
                  <input v-model="composeForm.name" :disabled="!canEditCompose" />
                </label>
                <label>
                  <span>路径</span>
                  <input v-model="composeForm.path" :disabled="!canEditCompose" />
                </label>
              </div>
              <template v-if="selectedProject && !selectedProject.managed">
                <pre v-if="selectedProject.content_preview" class="compose-preview">{{ selectedProject.content_preview }}</pre>
                <p v-else class="empty-hint">当前项目没有可安全展示的 Compose 预览。</p>
              </template>
              <textarea v-else v-model="composeForm.content" :disabled="!canEditCompose" spellcheck="false"></textarea>
              <div class="button-row">
                <label class="checkline">
                  <input v-model="composeForm.deploy_now" type="checkbox" :disabled="!canEditCompose || !canRunTask(selectedNode, 'compose_deploy')" />
                  立即部署
                </label>
                <button class="primary" type="submit" :disabled="!selectedNodeId || !canEditCompose">
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
              <span>需处理</span>
            </div>
            <strong>{{ failedProjects.length }}</strong>
          </article>
        </div>

        <section class="surface">
          <div class="surface-head">
            <h2>自动更新策略</h2>
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
            <button
              class="secondary"
              :disabled="!isAdmin || (selectedNodeId ? !canRunTask(selectedNode, 'detect_updates') : onlineNodes.length === 0)"
              @click="selectedNodeId ? createNodeTask('detect_updates') : detectAllNodes()"
            >
              <Search :size="18" />
              {{ selectedNodeId ? '检测当前节点' : '全部检测' }}
            </button>
          </div>
          <div class="update-list">
            <article v-for="row in composePolicyRows" :key="row.project.id" class="update-row">
              <div class="update-main">
                <strong :title="row.project.name">{{ row.project.name }}</strong>
                <span :title="row.project.path">{{ row.project.path }}</span>
                <em :class="detectionBadgeClass(row.project)" :title="detectionTitle(row.project)">{{ detectionLabel(row.project) }}</em>
                <small v-if="detectionMeta(row.project)" :title="detectionMeta(row.project)">{{ detectionMeta(row.project) }}</small>
                <small v-if="detectionReason(row.project)" class="error-text" :title="detectionReason(row.project)">{{ detectionReason(row.project) }}</small>
              </div>
              <div class="segmented">
                <button :class="{ active: row.policy.mode === 'manual' }" @click="row.policy.mode = 'manual'">不自动</button>
                <button :class="{ active: row.policy.mode === 'scheduled' }" @click="row.policy.mode = 'scheduled'">按计划</button>
                <button :class="{ active: row.policy.mode === 'automatic' }" @click="row.policy.mode = 'automatic'">自动更新</button>
              </div>
              <select v-model="row.policy.schedule" class="schedule-input" title="自动更新间隔">
                <option v-for="option in policyScheduleOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
              <input v-model="row.policy.maintenance_window" class="schedule-input" placeholder="维护窗口 02:00-05:00" />
              <input v-model="row.policy.healthcheck_url" class="schedule-input" placeholder="健康检查 URL，可留空" />
              <input v-model="row.policy.exclude_patterns" class="schedule-input" placeholder="不自动更新的关键字" />
              <label class="checkline compact-check">
                <input v-model="row.policy.rollback_on_failure" type="checkbox" />
                失败回滚
              </label>
              <div class="button-row compact-actions">
                <button class="icon-button" title="保存策略" :disabled="!isAdmin" @click="savePolicy(row.policy)">
                  <Save :size="16" />
                </button>
                <button class="icon-button" title="检测更新" :disabled="!isAdmin || !canRunTask(nodes.find((node) => node.id === row.project.node_id), 'detect_updates')" @click="createProjectTask('detect_updates', row.project)">
                  <Search :size="16" />
                </button>
                <button class="icon-button" title="执行更新" :disabled="!isAdmin || !canRunTask(nodes.find((node) => node.id === row.project.node_id), 'compose_update')" @click="createProjectTask('compose_update', row.project)">
                  <Play :size="16" />
                </button>
              </div>
            </article>
          </div>
        </section>

        <section class="surface">
          <div class="surface-head">
            <h2>更新记录</h2>
            <History :size="18" />
          </div>
          <div class="data-table update-record-table">
            <div class="table-row table-head">
              <span>时间</span>
              <span>容器</span>
              <span>节点</span>
              <span>镜像变化</span>
              <span>状态</span>
            </div>
            <div v-for="record in updateRecords" :key="record.id" class="table-row">
              <span :title="record.created_at">{{ record.created_at }}</span>
              <strong :title="record.name || '-'">{{ record.name || '-' }}</strong>
              <span :title="taskNodeName(record.node_id)">{{ taskNodeName(record.node_id) }}</span>
              <span :title="`${record.previous_version || '-'} -> ${record.current_version || '-'}`">{{ shortVersion(record.previous_version) }} -> {{ shortVersion(record.current_version) }}</span>
              <span><em class="badge" :class="record.changed ? 'success' : 'pending'">{{ record.changed ? '已更新' : '未变化' }}</em></span>
            </div>
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
              <strong :title="node.name">{{ node.name }}</strong>
              <span :title="node.version ? `v${node.version}` : '-'">{{ node.version ? `v${node.version}` : '-' }}</span>
              <span :title="`${node.os}/${node.arch}`">{{ node.os }}/{{ node.arch }}</span>
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
            <div class="segmented task-view-switch">
              <button :class="{ active: taskView === 'current' }" @click="taskView = 'current'">当前</button>
              <button :class="{ active: taskView === 'history' }" @click="taskView = 'history'">历史</button>
            </div>
            <div v-if="taskView === 'history'" class="segmented task-status-filter">
              <button :class="{ active: taskFilter === 'all' }" @click="taskFilter = 'all'">全部</button>
              <button :class="{ active: taskFilter === 'failed' }" @click="taskFilter = 'failed'">失败</button>
            </div>
            <div class="task-counts">
              <span>{{ activeTasks.length }} 运行</span>
              <span>{{ historyTasks.length }} 历史</span>
              <span>{{ failedTasks.length }} 失败</span>
            </div>
          </div>
          <div class="task-list">
            <button
              v-for="task in visibleTasks"
              :key="task.id"
              class="task-row"
              :class="{ selected: selectedTask?.id === task.id }"
              :title="taskTitleText(task)"
              @click="openTask(task)"
            >
              <span class="badge" :class="task.status">{{ statusText(task.status) }}</span>
              <div class="task-main">
                <div class="task-summary">
                  <strong>{{ taskTitle(task.kind) }}</strong>
                  <small>{{ task.created_at }}</small>
                </div>
                <p class="task-message" :title="taskMessage(task) || task.target_id || '-'">{{ taskMessage(task) || task.target_id || '-' }}</p>
                <div class="task-meta-row">
                  <span :title="taskNodeName(task.node_id)">{{ taskNodeName(task.node_id) }}</span>
                  <span :title="task.target_type || '任务'">{{ task.target_type || '任务' }}</span>
                  <span :title="task.id">{{ shortTaskId(task.id) }}</span>
                </div>
              </div>
            </button>
            <p v-if="visibleTasks.length === 0" class="empty-hint">{{ taskView === 'current' ? '当前没有运行中的任务。' : '当前筛选没有历史任务。' }}</p>
          </div>
        </section>

        <section class="surface task-detail">
          <div class="surface-head">
            <h2>详情</h2>
            <div class="button-row compact-actions">
              <label class="search-field compact-search">
                <Search :size="16" />
                <input v-model="taskLogSearch" placeholder="搜索日志" />
              </label>
              <button class="icon-button" title="复制日志" :disabled="!selectedTask" @click="copyTaskLogs">
                <Copy :size="16" />
              </button>
              <button class="icon-button" title="折叠日志" :disabled="!selectedTask" @click="taskLogsCollapsed = !taskLogsCollapsed">
                <Terminal :size="16" />
              </button>
            </div>
          </div>
          <template v-if="selectedTask">
            <div class="task-meta">
              <span><strong>ID</strong>{{ selectedTask.id }}</span>
              <span><strong>节点</strong>{{ taskNodeName(selectedTask.node_id) }}</span>
              <span><strong>状态</strong><em class="badge" :class="selectedTask.status">{{ statusText(selectedTask.status) }}</em></span>
              <span><strong>结果</strong>{{ taskMessage(selectedTask) || '-' }}</span>
            </div>
            <pre v-if="!taskLogsCollapsed" class="logs">{{ selectedTaskLogs }}</pre>
            <p v-else class="empty-hint">日志已折叠。</p>
          </template>
          <p v-else class="empty-hint detail-empty">选择左侧任务查看详情。</p>
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
                面板代管 Agent 升级
              </label>
              <label>
                <span>代管目标版本</span>
                <input v-model="runtimeSettings.agent_auto_update_version" :disabled="!isAdmin" placeholder="latest 或 v0.2.0" />
              </label>
              <label>
                <span>代管扫描间隔</span>
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
          <div class="segmented command-tabs">
            <button
              v-for="tab in commandTabs"
              :key="tab.value"
              type="button"
              :class="{ active: commandCategory === tab.value }"
              @click="commandCategory = tab.value"
            >
              {{ tab.label }}
            </button>
          </div>
          <div class="command-list">
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
  History,
  LayoutDashboard,
  LogIn,
  LogOut,
  MemoryStick,
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
  Metric,
  Node,
  Notification,
  Overview,
  Policy,
  RuntimeSettings,
  Task,
  TaskLog,
  UpdateRecord,
  User,
  VersionInfo
} from './types'

type ViewName = 'dashboard' | 'nodes' | 'projects' | 'updates' | 'tasks' | 'settings'
type ThemeName = 'system' | 'light' | 'dark'
type ToastType = 'info' | 'success' | 'error'
type NodeDetailTab = 'containers' | 'images' | 'compose' | 'profile'
type ContainerFilter = 'all' | 'running' | 'stopped' | 'updates'
type ProjectFilter = 'all' | 'updates' | 'failed' | 'current'
type TaskView = 'current' | 'history'
type CommandCategory = 'quick' | 'agent' | 'server' | 'remove'
type AgentInstallMode = 'docker' | 'binary'
type AgentCapabilityKey =
  | 'allow_agent_update'
  | 'allow_compose_update'
  | 'allow_deploy'
  | 'allow_container_restart'
  | 'allow_image_prune'

interface Toast {
  id: number
  type: ToastType
  message: string
}

interface DashboardNodeRow {
  node: Node
  metric?: Metric
}

const THEME_KEY = 'dockpilot.theme'
const REFRESH_INTERVAL_MS = 30000
const METRICS_PROBE_INTERVAL_SECONDS = 5
const OVERVIEW_REFRESH_INTERVAL_MS = METRICS_PROBE_INTERVAL_SECONDS * 1000
const themes: { value: ThemeName; label: string }[] = [
  { value: 'system', label: '跟随系统' },
  { value: 'light', label: '蓝白' },
  { value: 'dark', label: '夜间' }
]
const policyScheduleOptions = [
  { value: 'interval:1h', label: '每 1 小时' },
  { value: 'interval:2h', label: '每 2 小时' },
  { value: 'interval:3h', label: '每 3 小时' },
  { value: 'interval:6h', label: '每 6 小时' },
  { value: 'interval:12h', label: '每 12 小时' },
  { value: '@daily', label: '每天一次' }
]
const commandTabs: { value: CommandCategory; label: string }[] = [
  { value: 'quick', label: '快速' },
  { value: 'agent', label: 'Agent' },
  { value: 'server', label: 'Server' },
  { value: 'remove', label: '卸载' }
]
const agentCapabilityOptions: { key: AgentCapabilityKey; label: string; flag: string }[] = [
  { key: 'allow_agent_update', label: '面板触发 Agent 升级', flag: '--allow-agent-update' },
  { key: 'allow_compose_update', label: 'Compose 更新', flag: '--allow-compose-update' },
  { key: 'allow_deploy', label: 'Compose 部署', flag: '--allow-deploy' },
  { key: 'allow_container_restart', label: '重启容器', flag: '--allow-container-restart' },
  { key: 'allow_image_prune', label: '清理镜像', flag: '--allow-image-prune' }
]
const savedTheme = normalizeTheme(localStorage.getItem(THEME_KEY))
const token = ref(getToken())
const user = ref<AuthClaims | null>(null)
const activeView = ref<ViewName>('dashboard')
const themeName = ref<ThemeName>(savedTheme || 'system')
const themeMenuOpen = ref(false)
const systemPrefersDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)
const busy = ref(false)
const error = ref('')
const selectedNodeId = ref('')
const selectedProjectId = ref('')
const selectedTask = ref<Task | null>(null)
const taskLogs = ref<TaskLog[]>([])
const taskView = ref<TaskView>('current')
const taskFilter = ref<'all' | 'failed'>('all')
const taskLogSearch = ref('')
const taskLogsCollapsed = ref(false)
const nodeDetailTab = ref<NodeDetailTab>('containers')
const nodeSearch = ref('')
const containerSearch = ref('')
const containerStateFilter = ref<ContainerFilter>('all')
const projectSearch = ref('')
const projectFilter = ref<ProjectFilter>('all')
const commandCategory = ref<CommandCategory>('quick')
const toasts = ref<Toast[]>([])
const pendingActions = ref<string[]>([])
const currentClock = ref('')
let clockTimer: number | undefined
let refreshTimer: number | undefined
let overviewTimer: number | undefined
let refreshPromise: Promise<void> | null = null
let overviewPromise: Promise<void | undefined> | null = null
let dockerLoadSerial = 0
let toastID = 0
const systemThemeQuery = window.matchMedia('(prefers-color-scheme: dark)')

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
  },
  node_metrics: []
})
const nodes = ref<Node[]>([])
const dockerState = reactive<DockerState>({ containers: [], images: [], compose_projects: [] })
const dashboardDockerState = reactive<DockerState>({ containers: [], images: [], compose_projects: [] })
const tasks = ref<Task[]>([])
const updateRecords = ref<UpdateRecord[]>([])
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
  install_script: '',
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
const agentInstallForm = reactive<Record<AgentCapabilityKey, boolean> & {
  mode: AgentInstallMode
  node_name: string
  compose_dirs: string
  version: string
  self_update: boolean
}>({
  mode: 'docker',
  node_name: '',
  compose_dirs: '/opt,/srv,/var/www',
  version: 'latest',
  self_update: true,
  allow_agent_update: false,
  allow_compose_update: false,
  allow_deploy: false,
  allow_container_restart: false,
  allow_image_prune: false
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
const metricCache = reactive<Record<string, Metric>>({})

const isAdmin = computed(() => user.value?.role === 'admin')
const selectedNode = computed(() => nodes.value.find((node) => node.id === selectedNodeId.value))
const selectedProject = computed(
  () =>
    dockerState.compose_projects.find((project) => project.id === selectedProjectId.value) ||
    dashboardDockerState.compose_projects.find((project) => project.id === selectedProjectId.value)
)
const canEditCompose = computed(() => isAdmin.value && (!selectedProject.value || selectedProject.value.managed))
const onlineNodes = computed(() => nodes.value.filter((node) => node.status === 'online'))
const activeTasks = computed(() => tasks.value.filter((task) => task.status === 'pending' || task.status === 'running'))
const failedTasks = computed(() => tasks.value.filter((task) => task.status === 'failed'))
const overviewMetricList = computed(() => {
  const metrics = overview.node_metrics?.length
    ? overview.node_metrics
    : overview.last_metric.node_id
      ? [overview.last_metric]
      : []
  return metrics
})
const overviewMetricByNode = computed(() => {
  const values = new Map<string, Metric>()
  for (const metric of Object.values(metricCache)) {
    if (metric.node_id) {
      values.set(metric.node_id, metric)
    }
  }
  for (const metric of overviewMetricList.value) {
    values.set(metric.node_id, metric)
  }
  return values
})
const dashboardNodeRows = computed<DashboardNodeRow[]>(() => {
  return nodes.value.map((node) => ({
    node,
    metric: overviewMetricByNode.value.get(node.id)
  }))
})
const shortCommit = computed(() => (versionInfo.commit && versionInfo.commit !== 'dev' ? versionInfo.commit.slice(0, 12) : versionInfo.commit || '-'))
const latestReleaseVersion = computed(() => versionInfo.release?.latest_version || '')
const currentThemeLabel = computed(() => themes.find((theme) => theme.value === themeName.value)?.label || '主题')
const effectiveTheme = computed(() => (themeName.value === 'system' ? (systemPrefersDark.value ? 'dark' : 'light') : themeName.value))
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
    [
      node.name,
      node.id,
      node.note,
      node.os,
      node.arch,
      node.docker_version,
      node.compose_version,
      agentInstallMode(node),
      agentInstallModeLabel(node)
    ]
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
  return projectSource.value.compose_projects.filter((project) => {
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
const projectSource = computed(() => (selectedNodeId.value ? dockerState : dashboardDockerState))
const updateProjects = computed(() => projectSource.value.compose_projects.filter((project) => project.update_available))
const failedProjects = computed(() =>
  projectSource.value.compose_projects.filter((project) => project.detection_status === 'failed' || project.detection_status === 'partial')
)
const attentionItems = computed(() => {
  const items = [
    ...failedProjects.value.slice(0, 3).map((project) => ({
      key: `project:${project.id}`,
      title: project.name,
      detail: detectionReason(project) || project.detection_error || detectionLabel(project),
      kind: '项目',
      action: () => openProject(project)
    })),
    ...updateProjects.value.slice(0, 3).map((project) => ({
      key: `update:${project.id}`,
      title: project.name,
      detail: `${detectionLabel(project)} · ${project.path}`,
      kind: '更新',
      action: () => openProject(project)
    })),
    ...failedTasks.value.slice(0, 3).map((task) => ({
      key: `task:${task.id}`,
      title: taskTitle(task.kind),
      detail: taskMessage(task) || task.id,
      kind: '任务',
      action: () => openTask(task)
    }))
  ]
  if (items.length === 0 && overview.updates_available > 0) {
    items.push({
      key: 'overview-updates',
      title: `${overview.updates_available} 个项目可更新`,
      detail: '打开更新页查看',
      kind: '更新',
      action: () => (activeView.value = 'updates')
    })
  }
  if (items.length === 0 && overview.failed_tasks > 0) {
    items.push({
      key: 'overview-failed-tasks',
      title: `${overview.failed_tasks} 个失败任务`,
      detail: '打开任务页查看',
      kind: '任务',
      action: () => (activeView.value = 'tasks')
    })
  }
  return items
})
const composePolicyRows = computed(() =>
  projectSource.value.compose_projects.map((project) => ({
    project,
    policy: policyDraftFor('compose', project.id)
  }))
)
const dashboardTaskItems = computed(() => {
  const seen = new Set<string>()
  return [...activeTasks.value, ...tasks.value].filter((task) => {
    if (seen.has(task.id)) return false
    seen.add(task.id)
    return true
  }).slice(0, 4)
})
const historyTasks = computed(() => tasks.value.filter((task) => task.status !== 'pending' && task.status !== 'running'))
const visibleTasks = computed(() => {
  if (taskView.value === 'current') {
    return activeTasks.value
  }
  if (taskFilter.value === 'failed') {
    return historyTasks.value.filter((task) => task.status === 'failed')
  }
  return historyTasks.value
})
const selectedTaskLogs = computed(() => {
  if (!selectedTask.value) {
    return '选择一个任务查看日志'
  }
  if (taskLogs.value.length === 0) {
    return `${selectedTask.value.id}\n暂无日志`
  }
  const keyword = taskLogSearch.value.trim().toLowerCase()
  const lines = keyword
    ? taskLogs.value.filter((line) => `${line.created_at} ${line.line}`.toLowerCase().includes(keyword))
    : taskLogs.value
  return lines.map((line) => `[${line.created_at}] ${line.line}`).join('\n')
})
const agentInstallSummary = computed(() => [
  ...(agentInstallForm.self_update ? ['Agent 自更新'] : []),
  ...agentCapabilityOptions.filter((item) => agentInstallForm[item.key]).map((item) => item.label)
])
const agentInstallCommand = computed(() => {
  const script = installInfo.install_script || 'https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh'
  if (!installInfo.server_url || !installInfo.registration_token) {
    return ''
  }
  const action = agentInstallForm.mode === 'docker' ? 'install-agent-docker' : 'install-agent-binary'
  const parts = [
    `curl -fsSL ${shellArg(script)} | bash -s -- ${action}`,
    `--server-url ${shellArg(installInfo.server_url)}`,
    `--registration-token ${shellArg(installInfo.registration_token)}`
  ]
  const nodeName = agentInstallForm.node_name.trim()
  if (nodeName) {
    parts.push(`--node-name ${shellArg(nodeName)}`)
  }
  const composeDirs = agentInstallForm.compose_dirs.trim()
  if (composeDirs) {
    parts.push(`--compose-dirs ${shellArg(composeDirs)}`)
  }
  const version = agentInstallForm.version.trim()
  if (version && version !== 'latest') {
    parts.push(`--version ${shellArg(version)}`)
  }
  if (!agentInstallForm.self_update) {
    parts.push('--disable-agent-self-update')
  }
  for (const item of agentCapabilityOptions) {
    if (agentInstallForm[item.key]) {
      parts.push(item.flag)
    }
  }
  return parts.join(' ')
})
const commandItems = computed(() => [
  ...commandGroups.value[commandCategory.value]
])
const commandGroups = computed<Record<CommandCategory, { label: string; value: string }[]>>(() => ({
  quick: [
    { label: '交互式部署', value: installInfo.interactive || '' }
  ],
  agent: [
    { label: 'Agent Docker 接入', value: installInfo.agent_docker || installInfo.docker_command || '' },
    { label: 'Agent 二进制接入', value: installInfo.agent_binary || installInfo.binary_command || '' }
  ],
  server: [
    { label: 'Server Docker 部署', value: installInfo.server_docker || '' },
    { label: 'Server 二进制部署', value: installInfo.server_binary || '' }
  ],
  remove: [
    { label: '交互式卸载', value: installInfo.uninstall || '' },
    { label: '彻底卸载（删除数据）', value: installInfo.uninstall_purge || '' }
  ]
}))

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
          h('option', { value: 'manual' }, '不自动更新'),
          h('option', { value: 'scheduled' }, '按计划更新'),
          h('option', { value: 'automatic' }, '检测到更新后自动更新')
        ]),
        h('select', {
          value: props.policy.schedule,
          disabled: props.disabled,
          title: '自动更新间隔',
          onChange: (event: Event) => (props.policy.schedule = (event.target as HTMLSelectElement).value)
        }, policyScheduleOptions.map((option) => h('option', { value: option.value }, option.label))),
        h('input', {
          value: props.policy.maintenance_window || '',
          disabled: props.disabled,
          placeholder: '维护窗口 02:00-05:00，可留空',
          onInput: (event: Event) => (props.policy.maintenance_window = (event.target as HTMLInputElement).value)
        }),
        h('input', {
          value: props.policy.healthcheck_url || '',
          disabled: props.disabled,
          placeholder: '健康检查 URL，可留空',
          onInput: (event: Event) => (props.policy.healthcheck_url = (event.target as HTMLInputElement).value)
        }),
        h('input', {
          value: props.policy.exclude_patterns,
          disabled: props.disabled,
          placeholder: '不自动更新的关键字',
          onInput: (event: Event) => (props.policy.exclude_patterns = (event.target as HTMLInputElement).value)
        }),
        h('label', { class: 'checkline' }, [
          h('input', {
            type: 'checkbox',
            checked: props.policy.rollback_on_failure,
            disabled: props.disabled,
            onChange: (event: Event) => (props.policy.rollback_on_failure = (event.target as HTMLInputElement).checked)
          }),
          '健康检查失败后回滚镜像'
        ]),
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
  effectiveTheme,
  (value) => {
    document.documentElement.dataset.theme = value
  },
  { immediate: true }
)
watch(
  themeName,
  (value) => {
    localStorage.setItem(THEME_KEY, value)
  },
  { immediate: true }
)

onMounted(() => {
  systemThemeQuery.addEventListener('change', syncSystemTheme)
  tickClock()
  clockTimer = window.setInterval(tickClock, 1000)
  bootstrap()
  refreshTimer = window.setInterval(() => {
    if (token.value) {
      refreshAll()
    }
  }, REFRESH_INTERVAL_MS)
  overviewTimer = window.setInterval(() => {
    if (token.value && activeView.value === 'dashboard') {
      refreshOverviewOnly()
    }
  }, OVERVIEW_REFRESH_INTERVAL_MS)
})
onUnmounted(() => {
  systemThemeQuery.removeEventListener('change', syncSystemTheme)
  if (clockTimer) window.clearInterval(clockTimer)
  if (refreshTimer) window.clearInterval(refreshTimer)
  if (overviewTimer) window.clearInterval(overviewTimer)
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

function normalizeTheme(value: string | null): ThemeName | null {
  if (value === 'system' || value === 'light' || value === 'dark') {
    return value
  }
  if (value === 'sky') return 'light'
  if (value === 'operator' || value === 'mono' || value === 'graphite' || value === 'ember' || value === 'terminal') return 'dark'
  return null
}

function syncSystemTheme(event: MediaQueryListEvent) {
  systemPrefersDark.value = event.matches
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
  await runAction('refresh', '正在刷新', '数据已刷新', () => refreshAll(true))
}

async function refreshAll(forceReleaseRefresh = false) {
  if (refreshPromise) {
    return refreshPromise
  }
  refreshPromise = doRefreshAll(forceReleaseRefresh).finally(() => {
    refreshPromise = null
  })
  return refreshPromise
}

function applyOverviewData(overviewData: Overview) {
  Object.assign(overview, overviewData)
  const metrics = overviewData.node_metrics?.length
    ? overviewData.node_metrics
    : overviewData.last_metric?.node_id
      ? [overviewData.last_metric]
      : []
  for (const metric of metrics) {
    if (metric.node_id) {
      metricCache[metric.node_id] = metric
    }
  }
}

async function refreshOverviewOnly() {
  if (overviewPromise || refreshPromise) {
    return overviewPromise || refreshPromise
  }
  overviewPromise = api.overview()
    .then((overviewData) => {
      applyOverviewData(overviewData)
    })
    .catch(() => undefined)
    .finally(() => {
      overviewPromise = null
    })
  return overviewPromise
}

async function doRefreshAll(forceReleaseRefresh = false) {
  error.value = ''
  try {
    const [overviewData, nodesData, tasksData, recordsData, policiesData, notificationsData, versionData] = await Promise.all([
      api.overview(),
      api.nodes(),
      api.tasks(),
      api.updateRecords(),
      api.policies(),
      isAdmin.value ? api.notifications() : Promise.resolve([]),
      api.version(forceReleaseRefresh)
    ])
    applyOverviewData(overviewData)
    nodes.value = nodesData
    tasks.value = tasksData
    updateRecords.value = recordsData
    policies.value = policiesData
    notifications.value = notificationsData
    Object.assign(versionInfo, versionData)
    if (versionData.settings) {
      Object.assign(runtimeSettings, versionData.settings)
    }
    syncPolicyDrafts()
    if (activeView.value === 'dashboard' || (activeView.value === 'projects' && !selectedNodeId.value)) {
      await loadDashboardDocker(nodes.value)
    }
    if (selectedNodeId.value && nodes.value.some((node) => node.id === selectedNodeId.value)) {
      await loadDocker(selectedNodeId.value)
    } else if (selectedNodeId.value) {
      selectedNodeId.value = ''
    }
    if (isAdmin.value && (!installInfo.install_script || activeView.value === 'settings')) {
      await loadAdminSettings()
    }
    await refreshSelectedTask(tasksData)
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

async function loadDashboardDocker(nodeList = nodes.value) {
  const targets = nodeList.filter((node) => node.status === 'online')
  if (targets.length === 0) {
    dashboardDockerState.containers = []
    dashboardDockerState.images = []
    dashboardDockerState.compose_projects = []
    return
  }
  const results = await Promise.allSettled(targets.map((node) => api.dockerState(node.id)))
  dashboardDockerState.containers = []
  dashboardDockerState.images = []
  dashboardDockerState.compose_projects = []
  for (const result of results) {
    if (result.status !== 'fulfilled') continue
    dashboardDockerState.containers.push(...result.value.containers)
    dashboardDockerState.images.push(...result.value.images)
    dashboardDockerState.compose_projects.push(...result.value.compose_projects)
  }
}

async function loadDocker(nodeId: string) {
  const requestID = ++dockerLoadSerial
  const state = await api.dockerState(nodeId)
  if (requestID !== dockerLoadSerial || selectedNodeId.value !== nodeId) {
    return
  }
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
  const node = nodes.value.find((item) => item.id === nodeId)
  if (!canRunTask(node, kind)) {
    notify(capabilityWarning(kind), 'error')
    return undefined
  }
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
  if (!canRunTask(node, 'agent_update')) {
    return Promise.reject(new Error(capabilityWarning('agent_update')))
  }
  return api.createTask({
    node_id: node.id,
    kind: 'agent_update',
    target_type: 'node',
    target_id: node.id,
    args: { version: agentTargetVersion.value }
  })
}

async function createProjectTask(kind: string, project: ComposeProject) {
  const node = nodes.value.find((item) => item.id === project.node_id)
  if (!canRunTask(node, kind)) {
    notify(capabilityWarning(kind), 'error')
    return
  }
  const policy = policyDraftFor('compose', project.id)
  const args: Record<string, string> = { path: project.path, name: project.name }
  if (kind === 'compose_update') {
    if (policy.healthcheck_url) args.healthcheck_url = policy.healthcheck_url
    if (policy.rollback_on_failure) args.rollback_on_failure = 'true'
  }
  const task = await runAction(`project-task:${project.id}:${kind}`, `正在创建 ${project.name} 的${taskTitle(kind)}任务`, `任务已创建：${project.name}`, () =>
    api.createTask({
      node_id: project.node_id,
      kind,
      target_type: 'compose',
      target_id: project.id,
      args
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
  await refreshSelectedTask(tasks.value)
}

async function refreshSelectedTask(taskList = tasks.value) {
  const taskId = selectedTask.value?.id
  if (!taskId) return
  const latestTask = taskList.find((task) => task.id === taskId)
  if (!latestTask) {
    selectedTask.value = null
    taskLogs.value = []
    return
  }
  selectedTask.value = latestTask
  taskLogs.value = await api.taskLogs(taskId)
}

async function clearTasksScope(scope: 'finished' | 'failed', label: string) {
  const confirmed = window.confirm(`清除${label}？正在运行和排队任务会保留。`)
  if (!confirmed) return
  const result = await runAction(`clear-tasks:${scope}`, `正在清除${label}`, `${label}已清除`, () => api.clearTasks(scope))
  if (!result) return
  selectedTask.value = null
  taskLogs.value = []
  await refreshTasks()
  applyOverviewData(await api.overview())
}

async function openTask(task: Task) {
  selectedTask.value = task
  taskLogSearch.value = ''
  taskLogsCollapsed.value = false
  taskLogs.value = await api.taskLogs(task.id)
  activeView.value = 'tasks'
}

function openProject(project: ComposeProject) {
  activeView.value = 'projects'
  selectCompose(project)
}

function selectCompose(project: ComposeProject) {
  if (project.node_id && selectedNodeId.value !== project.node_id) {
    selectedNodeId.value = project.node_id
  }
  selectedProjectId.value = project.id
  editCompose(project)
}

function editCompose(project: ComposeProject) {
  composeForm.id = project.id
  composeForm.name = project.name
  composeForm.path = project.path
  composeForm.content = project.managed ? project.content || '' : ''
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
  if (!canEditCompose.value) {
    notify('扫描到的 Compose 项目为只读，不能在面板覆盖修改。', 'error')
    return
  }
  if (composeForm.deploy_now && !canRunTask(selectedNode.value, 'compose_deploy')) {
    notify(capabilityWarning('compose_deploy'), 'error')
    return
  }
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

async function importSelectedCompose(mode: 'read_only' | 'managed') {
  if (!selectedProject.value) return
  if (mode === 'managed') {
    if (!selectedProject.value.content_hash) {
      notify('当前项目还没有源文件哈希，等待 Agent 刷新后再转为托管。', 'error')
      return
    }
    const content = window.prompt('粘贴节点上这个 Compose 原文件的完整内容。只有 SHA256 与扫描到的源文件一致时，才会转为面板托管。')
    if (!content || !content.trim()) return
    const confirmed = window.confirm('确认转为面板托管？转为托管后，后续保存/部署可能覆盖节点上的 Compose 文件。')
    if (!confirmed) return
    const saved = await runAction(`import-compose:${selectedProject.value.id}:managed`, '正在校验并转为托管', '已转为面板托管', () =>
      api.importCompose({ node_id: selectedProject.value!.node_id, id: selectedProject.value!.id, mode, content, confirm: true })
    )
    if (!saved) return
    selectedNodeId.value = saved.node_id
    selectedProjectId.value = saved.id
    await loadDocker(saved.node_id)
    return
  }
  const saved = await runAction(`import-compose:${selectedProject.value.id}:read`, '正在导入只读项目', '已导入为只读项目', () =>
    api.importCompose({ node_id: selectedProject.value!.node_id, id: selectedProject.value!.id, mode })
  )
  if (!saved) return
  selectedNodeId.value = saved.node_id
  selectedProjectId.value = saved.id
  await loadDocker(saved.node_id)
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
          schedule: 'interval:6h',
          maintenance_window: '',
          healthcheck_url: '',
          rollback_on_failure: false,
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

function shellArg(value: string) {
  return `'${value.replace(/'/g, `'\"'\"'`)}'`
}

async function copyTaskLogs() {
  await copyCommand(selectedTaskLogs.value)
}

function detectionLabel(project: ComposeProject) {
  if (project.update_available || project.detection_status === 'update_available') return '有更新'
  if (project.detection_status === 'partial') return '需处理'
  if (project.detection_status === 'failed') return '无法检测'
  if (project.detection_status === 'checked' || project.detection_status === 'current') return '已是最新'
  if (project.checked_at) return '已是最新'
  return '未检测'
}

function detectionBadgeClass(project: ComposeProject) {
  if (project.detection_status === 'failed') return 'mini-danger'
  if (project.detection_status === 'partial') return 'mini-alert'
  if (project.update_available || project.detection_status === 'update_available') return 'mini-alert'
  if (project.detection_status === 'checked' || project.detection_status === 'current' || project.checked_at) return 'mini-success'
  return 'mini-muted'
}

function detectionMeta(project: ComposeProject) {
  return [project.detection_method, project.detection_platform, project.checked_at].filter(Boolean).join(' · ')
}

function detectionTitle(project: ComposeProject) {
  const reason = detectionReason(project)
  const rawError = project.detection_error && project.detection_error !== reason ? project.detection_error : ''
  return [detectionLabel(project), detectionMeta(project), reason, rawError].filter(Boolean).join('\n')
}

function detectionReason(project: ComposeProject) {
  const errorText = project.detection_error || ''
  if (!errorText && project.detection_status !== 'partial' && project.detection_status !== 'failed') {
    return ''
  }
  return friendlyDetectionReason(errorText, project.detection_status)
}

function friendlyDetectionReason(errorText: string, status: string) {
  const trimmed = errorText.trim()
  if (/[\u4e00-\u9fff]/.test(trimmed)) {
    return trimmed
  }
  const lower = errorText.toLowerCase()
  if (lower.includes('outside agent allowed directories')) {
    return '这个 Compose 文件不在 Agent 允许扫描的目录内。请把项目放到 /opt、/srv、/var/www，或在节点端调整 DOCKPILOT_COMPOSE_DIRS。'
  }
  if (lower.includes('no such file or directory') || lower.includes('compose file not found')) {
    return '节点上找不到这个 Compose 文件。请确认路径存在，并且 Agent 容器挂载了该目录。'
  }
  if (lower.includes('variable is not set') || lower.includes('not set. defaulting to a blank string')) {
    return 'Compose 缺少环境变量。请在项目目录补齐 .env，或在 compose 文件里给变量设置默认值。'
  }
  if (lower.includes('empty section between colons') || lower.includes('invalid spec')) {
    return 'Compose 挂载路径不完整，通常是环境变量为空导致类似 :/www 的挂载。请先修正 .env 或 volumes。'
  }
  if (lower.includes('unauthorized') || lower.includes('authentication required') || lower.includes('denied')) {
    return '镜像仓库需要登录或没有权限。请在节点上完成 docker login，或确认镜像地址可以公开拉取。'
  }
  if (lower.includes('timeout') || lower.includes('no route to host') || lower.includes('temporary failure') || lower.includes('connection refused')) {
    return '节点访问镜像仓库或网络超时。请检查节点网络、DNS、防火墙和仓库可用性。'
  }
  if (lower.includes('image update checks failed')) {
    return '有镜像没有完成远端版本比对。请查看任务日志里的具体镜像和仓库错误。'
  }
  if (status === 'partial') {
    return '部分镜像已经检测完成，但还有项目配置或仓库访问问题需要处理。'
  }
  if (status === 'failed') {
    return '这个项目没有完成检测。请打开任务日志查看节点返回的具体错误。'
  }
  return ''
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

function shortTaskId(id: string) {
  if (!id) return '-'
  return id.length > 18 ? `${id.slice(0, 10)}...${id.slice(-6)}` : id
}

function shortHash(value: string) {
  if (!value) return '-'
  return value.length > 16 ? `${value.slice(0, 12)}...${value.slice(-8)}` : value
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
  return !!latest && compareVersions(node.version, latest) < 0 && canRunTask(node, 'agent_update')
}

function agentVersionLabel(node: Node) {
  if (node.status !== 'online') return '离线'
  if (!canRunTask(node, 'agent_update')) return '禁止面板升级'
  const latest = latestReleaseVersion.value
  if (!node.version) return latest ? `可升级到 v${latest}` : '版本未知'
  if (!latest) return `Agent v${node.version}`
  if (compareVersions(node.version, latest) < 0) return `可升级到 v${latest}`
  return '最新'
}

function parseNodeCapabilities(node?: Node) {
  if (!node?.capabilities) return {}
  try {
    return JSON.parse(node.capabilities) as Record<string, boolean>
  } catch {
    return {}
  }
}

function parseNodeLabels(node?: Node) {
  if (!node?.labels) return {}
  try {
    return JSON.parse(node.labels) as Record<string, string>
  } catch {
    return {}
  }
}

function agentInstallMode(node?: Node) {
  const mode = (parseNodeLabels(node).install_mode || '').toLowerCase()
  if (mode === 'docker' || mode === 'binary') return mode
  return 'unknown'
}

function agentInstallModeLabel(node?: Node) {
  const labels: Record<string, string> = {
    docker: 'Docker',
    binary: '二进制',
    unknown: '未知'
  }
  return labels[agentInstallMode(node)] || '未知'
}

function canRunTask(node: Node | undefined, kind: string) {
  if (!node) return false
  if (kind === 'detect_updates') return node.status === 'online'
  if (node.status !== 'online') return false
  const cap = requiredCapability(kind)
  if (!cap) return true
  return !!parseNodeCapabilities(node)[cap]
}

function requiredCapability(kind: string) {
  const map: Record<string, string> = {
    agent_update: 'agent_update',
    compose_update: 'compose_update',
    compose_deploy: 'compose_deploy',
    restart_container: 'restart_container',
    prune_images: 'prune_images'
  }
  return map[kind] || ''
}

function capabilityWarning(kind: string) {
  if (kind === 'detect_updates') {
    return '节点离线或未连接，暂时不能创建检测任务。'
  }
  const labels: Record<string, string> = {
    agent_update: '面板触发 Agent 升级',
    compose_update: 'Compose 更新',
    compose_deploy: 'Compose 部署',
    restart_container: '重启容器',
    prune_images: '清理镜像'
  }
  return `${labels[kind] || kind} 未在节点 Agent 端开启。请设置对应 DOCKPILOT_AGENT_ALLOW_* 环境变量并重启 Agent。`
}

function capabilityItems(node?: Node) {
  const caps = parseNodeCapabilities(node)
  return [
    { label: '检测', enabled: node?.status === 'online' && (!!caps.detect_updates || !!node) },
    { label: '更新', enabled: !!caps.compose_update },
    { label: '部署', enabled: !!caps.compose_deploy },
    { label: '重启', enabled: !!caps.restart_container },
    { label: '清理', enabled: !!caps.prune_images },
    { label: '面板升级', enabled: !!caps.agent_update }
  ]
}

function composeOwnershipLabel(project?: ComposeProject) {
  if (!project) return '-'
  if (project.managed || project.ownership === 'managed') return '面板托管'
  if (project.imported || project.ownership === 'imported') return '只读导入'
  return '扫描发现'
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

function metricMemoryPercent(metric: Metric) {
  return percent(metric.memory_used, metric.memory_total)
}

function metricDiskPercent(metric: Metric) {
  return percent(metric.disk_used, metric.disk_total)
}

function metricDetailTitle(metric: Metric, kind: 'cpu' | 'memory' | 'disk') {
  const nodeName = taskNodeName(metric.node_id)
  if (kind === 'cpu') {
    return `${nodeName}\nCPU：${formatPercent(metric.cpu_percent)}\n容器：${metric.container_count}`
  }
  if (kind === 'memory') {
    return `${nodeName}\n内存：${formatBytes(metric.memory_used)} / ${formatBytes(metric.memory_total)}\n占用：${formatPercent(metricMemoryPercent(metric))}`
  }
  if (kind === 'disk') {
    return `${nodeName}\n磁盘：${formatBytes(metric.disk_used)} / ${formatBytes(metric.disk_total)}\n占用：${formatPercent(metricDiskPercent(metric))}`
  }
  return nodeName
}

function nodeTitle(node: Node) {
  return [
    node.name,
    `状态：${node.status || '-'}`,
    `系统：${node.os || '-'}/${node.arch || '-'}`,
    `Docker：${node.docker_version || '-'}`,
    `Compose：${node.compose_version || '-'}`,
    `Agent：${node.version ? `v${node.version}` : '-'}`,
    `安装：${agentInstallModeLabel(node)}`,
    `最近心跳：${node.last_seen || '-'}`
  ].join('\n')
}

function nodeResourceTitle(row: DashboardNodeRow) {
  const metric = row.metric
  return [
    nodeTitle(row.node),
    metric ? `CPU：${formatPercent(metric.cpu_percent)}` : 'CPU：暂无数据',
    metric ? `内存：${formatBytes(metric.memory_used)} / ${formatBytes(metric.memory_total)} (${formatPercent(metricMemoryPercent(metric))})` : '内存：暂无数据',
    metric ? `硬盘：${formatBytes(metric.disk_used)} / ${formatBytes(metric.disk_total)} (${formatPercent(metricDiskPercent(metric))})` : '硬盘：暂无数据',
    metric ? `资源上报：${metric.recorded_at || '-'}` : '资源上报：暂无'
  ].join('\n')
}

function composeProjectTitle(project: ComposeProject) {
  return [
    project.name,
    `路径：${project.path || '-'}`,
    `归属：${composeOwnershipLabel(project)}`,
    `状态：${detectionLabel(project)}`,
    detectionMeta(project) ? `检测：${detectionMeta(project)}` : '',
    detectionReason(project) ? `原因：${detectionReason(project)}` : ''
  ]
    .filter(Boolean)
    .join('\n')
}

function taskTitleText(task: Task) {
  return [
    taskTitle(task.kind),
    `节点：${taskNodeName(task.node_id)}`,
    `状态：${statusText(task.status)}`,
    `目标：${task.target_type || '-'} ${task.target_id || ''}`.trim(),
    `任务 ID：${task.id}`,
    taskMessage(task) ? `结果：${taskMessage(task)}` : ''
  ]
    .filter(Boolean)
    .join('\n')
}

function shortTime(value?: string) {
  if (!value) return '-'
  const text = String(value)
  if (text.length >= 16 && /^\d{4}-\d{2}-\d{2}/.test(text)) {
    return text.slice(5, 16)
  }
  return text.length > 16 ? `${text.slice(0, 16)}...` : text
}

function shortLabel(value?: string) {
  if (!value) return '-'
  return value.length > 13 ? `${value.slice(0, 13)}...` : value
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

function shortVersion(value?: string) {
  if (!value) return '-'
  if (value.startsWith('sha256:') && value.length > 19) {
    return value.slice(0, 19)
  }
  return value.length > 24 ? `${value.slice(0, 24)}...` : value
}
</script>
