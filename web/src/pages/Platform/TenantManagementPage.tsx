import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { ArrowLeft, Maximize2, Minimize2, RefreshCw, Search, X } from "lucide-react";
import { useEffect, useState, type TransitionEvent } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { tenantApi } from "../../api/tenants";
import type { TenantManagementAPI, TenantRow } from "../../api/tenants";
import { uploadApi } from "../../api/uploads";
import type { UploadAPI } from "../../api/uploads";
import type { SpaceRow } from "../../api/spaces";
import type { TenantUserRow } from "../../api/users";

type TenantManagementPageProps = {
  api?: TenantManagementAPI;
  uploadAPI?: UploadAPI;
};

type TenantResourceDrawer = {
  kind: "spaces" | "users";
  tenant: TenantRow;
};

export function TenantManagementPage({
  api = tenantApi,
  uploadAPI = uploadApi,
}: TenantManagementPageProps) {
  const [tenants, setTenants] = useState<TenantRow[]>([]);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [logoFileName, setLogoFileName] = useState("");
  const [allowRegister, setAllowRegister] = useState(true);
  const [adminUsername, setAdminUsername] = useState("");
  const [adminRealName, setAdminRealName] = useState("");
  const [adminPhone, setAdminPhone] = useState("");
  const [adminEmail, setAdminEmail] = useState("");
  const [adminPassword, setAdminPassword] = useState("");
  const [uploadResetKey, setUploadResetKey] = useState(0);
  const [isLogoUploading, setIsLogoUploading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [editingTenant, setEditingTenant] = useState<TenantRow | null>(null);
  const [editingName, setEditingName] = useState("");
  const [editingDescription, setEditingDescription] = useState("");
  const [editingLogoFileName, setEditingLogoFileName] = useState("");
  const [editingUploadResetKey, setEditingUploadResetKey] = useState(0);
  const [resetConfirmTenant, setResetConfirmTenant] = useState<TenantRow | null>(null);
  const [closeRegisterConfirmTenant, setCloseRegisterConfirmTenant] = useState<TenantRow | null>(null);
  const [loadError, setLoadError] = useState("");
  const [resourceDrawer, setResourceDrawer] = useState<TenantResourceDrawer | null>(null);
  const [resourceSpaces, setResourceSpaces] = useState<SpaceRow[]>([]);
  const [resourceUsers, setResourceUsers] = useState<TenantUserRow[]>([]);
  const [resourceError, setResourceError] = useState("");
  const [isResourceLoading, setIsResourceLoading] = useState(false);

  useEffect(() => {
    let ignore = false;

    api.listTenants()
      .then((data) => {
        if (!ignore) {
          setTenants(data.items);
          setLoadError("");
        }
      })
      .catch(() => {
        if (!ignore) {
          setLoadError("租户列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [api]);

  async function reloadTenants(keyword?: string) {
    try {
      const data = keyword ? await api.listTenants({ keyword }) : await api.listTenants();
      setTenants(data.items);
      setLoadError("");
    } catch {
      setLoadError("租户列表加载失败");
    }
  }

  async function handleCreateTenant(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (isLogoUploading) {
      setLoadError("租户 Logo 正在上传");
      return;
    }

    const nextTenant = await api.createTenant({
      name,
      description,
      logoFileName: logoFileName || "未上传",
      allowRegister,
      adminUsername,
      adminRealName,
      adminPhone,
      adminEmail,
      adminPassword,
    });

    setTenants((items) => [nextTenant, ...items]);
    setName("");
    setDescription("");
    setLogoFileName("");
    setAllowRegister(true);
    setAdminUsername("");
    setAdminRealName("");
    setAdminPhone("");
    setAdminEmail("");
    setAdminPassword("");
    setUploadResetKey((value) => value + 1);
    setIsCreateDialogOpen(false);
  }

  async function handleLogoAccepted(file: File) {
    try {
      setIsLogoUploading(true);
      // 租户 Logo 先上传为对象地址，创建租户时只提交后端返回的 URL。
      const result = await uploadAPI.uploadFile({ category: "tenant-logos", file });
      setLogoFileName(result.url);
      setLoadError("");
    } catch {
      setLogoFileName("");
      setLoadError("租户 Logo 上传失败");
    } finally {
      setIsLogoUploading(false);
    }
  }

  function openProfileEditor(tenant: TenantRow) {
    setEditingTenant(tenant);
    setEditingName(tenant.name);
    setEditingDescription(tenant.description);
    setEditingLogoFileName(tenant.logoFileName);
    setEditingUploadResetKey((value) => value + 1);
  }

  function updateTenantRow(nextTenant: TenantRow) {
    setTenants((items) =>
      items.map((tenant) => (tenant.id === nextTenant.id ? nextTenant : tenant)),
    );
  }

  async function handleEditingLogoAccepted(file: File) {
    try {
      setIsLogoUploading(true);
      // 编辑租户 Logo 只替换弹窗里的待保存 URL，保存资料后再更新列表快照。
      const result = await uploadAPI.uploadFile({ category: "tenant-logos", file });
      setEditingLogoFileName(result.url);
      setLoadError("");
    } catch {
      setLoadError("租户 Logo 上传失败");
    } finally {
      setIsLogoUploading(false);
    }
  }

  async function saveProfile() {
    if (!editingTenant) {
      return;
    }

    try {
      // 租户资料由后端持久化，列表只采用接口返回的最新租户快照。
      const nextTenant = await api.updateTenantProfile({
        tenantID: editingTenant.id,
        name: editingName,
        description: editingDescription,
        logoFileName: editingLogoFileName || "未上传",
      });
      updateTenantRow(nextTenant);
      setEditingTenant(null);
      setEditingName("");
      setEditingDescription("");
      setEditingLogoFileName("");
      setLoadError("");
    } catch {
      setLoadError("租户操作失败");
    }
  }

  async function resetTenantCode() {
    if (!resetConfirmTenant) {
      return;
    }

    try {
      const nextTenant = await api.resetTenantCode(resetConfirmTenant.id);
      updateTenantRow(nextTenant);
      setResetConfirmTenant(null);
      setLoadError("");
    } catch {
      setLoadError("租户操作失败");
    }
  }

  async function toggleRegister(tenant: TenantRow) {
    if (tenant.allowRegister) {
      setCloseRegisterConfirmTenant(tenant);
      return;
    }

    try {
      // 注册开关只控制自注册入口，不能和租户启停状态混用。
      const nextTenant = await api.enableTenantRegistration(tenant.id);
      updateTenantRow(nextTenant);
      setLoadError("");
    } catch {
      setLoadError("租户操作失败");
    }
  }

  async function closeRegistration() {
    if (!closeRegisterConfirmTenant) {
      return;
    }

    try {
      // 关闭注册会阻断新的自注册入口，必须由平台管理员二次确认后执行。
      const nextTenant = await api.disableTenantRegistration(closeRegisterConfirmTenant.id);
      updateTenantRow(nextTenant);
      setCloseRegisterConfirmTenant(null);
      setLoadError("");
    } catch {
      setLoadError("租户操作失败");
    }
  }

  function handleSearchTenants() {
    void reloadTenants(searchQuery.trim());
  }

  function handleRefreshTenants() {
    setSearchQuery("");
    void reloadTenants();
  }

  async function openTenantResourceDrawer(tenant: TenantRow, kind: TenantResourceDrawer["kind"]) {
    setResourceDrawer({ kind, tenant });
    setResourceError("");
    setIsResourceLoading(true);
    setResourceSpaces([]);
    setResourceUsers([]);

    try {
      if (kind === "spaces") {
        const data = await api.listTenantSpaces(tenant.id);
        setResourceSpaces(data.items);
      } else {
        const data = await api.listTenantUsers(tenant.id);
        setResourceUsers(data.items);
      }
    } catch {
      setResourceError(kind === "spaces" ? "租户空间加载失败" : "租户用户加载失败");
    } finally {
      setIsResourceLoading(false);
    }
  }

  return (
    <section className="page platform-page">
      <nav aria-label="平台运营菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/tenants" role="tab" aria-selected="true">
          租户管理
        </a>
      </nav>

      <div className="page-grid platform-grid">
        <Panel>
          <div className="tenant-list-toolbar">
            <div className="tenant-list-actions" aria-label="租户操作区">
              <Button variant="toolbarPrimary" onClick={() => setIsCreateDialogOpen(true)} type="button">
                创建租户
              </Button>
            </div>
            <div className="tenant-search-actions">
              <label className="tenant-search-field">
                <span className="sr-only">搜索租户</span>
                <input
                  onChange={(event) => setSearchQuery(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      event.preventDefault();
                      handleSearchTenants();
                    }
                  }}
                  placeholder="输入租户名称、描述或租户码"
                  value={searchQuery}
                />
              </label>
              <Button aria-label="搜索" variant="icon" onClick={handleSearchTenants} type="button">
                <Search aria-hidden="true" size={16} />
              </Button>
              <Button
                aria-label="刷新租户列表"
                variant="icon"
                onClick={handleRefreshTenants}
                type="button"
              >
                <RefreshCw aria-hidden="true" size={16} />
              </Button>
            </div>
          </div>
          {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
          <div className="table-wrap">
            <table className="data-table tenant-table">
              <thead>
                <tr>
                  <th scope="col">租户</th>
                  <th scope="col">Logo</th>
                  <th scope="col">租户码</th>
                  <th scope="col">注册</th>
                  <th scope="col">操作</th>
                </tr>
              </thead>
              <tbody>
                {tenants.length === 0 && <EmptyTableRow colSpan={5} />}
                {tenants.map((tenant) => (
                  <tr key={tenant.id}>
                    <td>
                      <strong>{tenant.name}</strong>
                      <span>{tenant.description}</span>
                    </td>
                    <td>{renderTenantLogo(tenant)}</td>
                    <td><code>{tenant.code}</code></td>
                    <td>
                      <StatusBadge tone={tenant.allowRegister ? "success" : "info"}>
                        {tenant.allowRegister ? "允许注册" : tenant.registerClosedLabel ?? "禁止注册"}
                      </StatusBadge>
                    </td>
                    <td>
                      <div className="tenant-actions">
                        <Button
                          variant="actionEdit"
                          type="button"
                          onClick={() => openProfileEditor(tenant)}
                        >
                          编辑资料
                        </Button>
                        <Button
                          aria-label={`查看${tenant.name}空间`}
                          onClick={() => void openTenantResourceDrawer(tenant, "spaces")}
                          type="button"
                          variant="actionReset"
                        >
                          空间
                        </Button>
                        <Button
                          aria-label={`查看${tenant.name}用户`}
                          onClick={() => void openTenantResourceDrawer(tenant, "users")}
                          type="button"
                          variant="actionReset"
                        >
                          用户
                        </Button>
                        <Button
                          variant="actionReset"
                          type="button"
                          onClick={() => setResetConfirmTenant(tenant)}
                        >
                          重置租户码
                        </Button>
                        <Button
                          onClick={() => toggleRegister(tenant)}
                          type="button"
                          variant={tenant.allowRegister ? "actionClose" : "actionOpen"}
                        >
                          {tenant.allowRegister ? "关闭注册" : "开启注册"}
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      </div>

      {isCreateDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="创建租户弹窗">
          <div className="platform-dialog__card">
            <h2>创建租户</h2>
            <p>创建后生成租户码，并初始化首个租户管理员；班级空间需要后续手动创建。</p>
            <form className="platform-form" onSubmit={handleCreateTenant}>
              <label className="field">
                <span className="field-label">租户名称<span className="required-marker" aria-hidden="true">*</span></span>
                <input
                  aria-label="租户名称"
                  onChange={(event) => setName(event.target.value)}
                  required
                  value={name}
                />
              </label>
              <label className="field">
                <span className="field-label">租户描述<span className="required-marker" aria-hidden="true">*</span></span>
                <textarea
                  aria-label="租户描述"
                  onChange={(event) => setDescription(event.target.value)}
                  required
                  value={description}
                />
              </label>
              <label className="tenant-register-option">
                <input
                  checked={allowRegister}
                  onChange={(event) => setAllowRegister(event.target.checked)}
                  type="checkbox"
                />
                <span>是否开放注册</span>
              </label>
              <label className="field">
                <span className="field-label">管理员用户名<span className="required-marker" aria-hidden="true">*</span></span>
                <input aria-label="管理员用户名" onChange={(event) => setAdminUsername(event.target.value)} required value={adminUsername} />
              </label>
              <label className="field">
                <span className="field-label">管理员姓名<span className="required-marker" aria-hidden="true">*</span></span>
                <input aria-label="管理员姓名" onChange={(event) => setAdminRealName(event.target.value)} required value={adminRealName} />
              </label>
              <label className="field">
                <span className="field-label">管理员手机号</span>
                <input aria-label="管理员手机号" onChange={(event) => setAdminPhone(event.target.value)} value={adminPhone} />
              </label>
              <label className="field">
                <span className="field-label">管理员邮箱</span>
                <input aria-label="管理员邮箱" onChange={(event) => setAdminEmail(event.target.value)} value={adminEmail} />
              </label>
              <label className="field">
                <span className="field-label">管理员初始密码<span className="required-marker" aria-hidden="true">*</span></span>
                <input aria-label="管理员初始密码" onChange={(event) => setAdminPassword(event.target.value)} required type="password" value={adminPassword} />
              </label>
              <FileUploadField
                accept={["image/png", "image/jpeg"]}
                key={uploadResetKey}
                label="租户 Logo"
                maxSizeBytes={1024 * 1024}
                onFileAccepted={handleLogoAccepted}
              />
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsCreateDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button disabled={isLogoUploading} variant="primary" type="submit">
                  {isLogoUploading ? "上传中" : "确认创建"}
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {resetConfirmTenant && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="重置租户码确认">
          <div className="platform-dialog__card">
            <h2>重置租户码</h2>
            <p>你正在重置租户「{resetConfirmTenant.name}」的租户码。</p>
            <div className="platform-dialog__warning">
              <p>当前租户码 {resetConfirmTenant.code} 将立即失效。</p>
              <p>已经发出的注册链接、手动注册时填写的旧租户码，都需要改用新的租户码。</p>
            </div>
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={() => setResetConfirmTenant(null)} type="button">
                取消
              </Button>
              <Button variant="primary" onClick={() => void resetTenantCode()} type="button">
                确认重置
              </Button>
            </div>
          </div>
        </div>
      )}

      {closeRegisterConfirmTenant && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="关闭注册确认">
          <div className="platform-dialog__card">
            <h2>关闭注册</h2>
            <p>你正在关闭租户「{closeRegisterConfirmTenant.name}」的自注册入口。</p>
            <div className="platform-dialog__warning">
              <p>关闭后，新的租户用户将无法通过注册链接或手动输入租户码自注册。</p>
              <p>已经注册的用户仍可继续登录和使用已授权的空间。</p>
            </div>
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={() => setCloseRegisterConfirmTenant(null)} type="button">
                取消
              </Button>
              <Button variant="primary" onClick={() => void closeRegistration()} type="button">
                确认关闭
              </Button>
            </div>
          </div>
        </div>
      )}

      {editingTenant && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="租户资料弹窗">
          <div className="platform-dialog__card">
            <h2>{editingTenant.name}</h2>
            <label className="field">
              <span>编辑租户名称</span>
              <input
                aria-label="编辑租户名称"
                onChange={(event) => setEditingName(event.target.value)}
                required
                value={editingName}
              />
            </label>
            <label className="field">
              <span>编辑租户描述</span>
              <textarea
                onChange={(event) => setEditingDescription(event.target.value)}
                value={editingDescription}
              />
            </label>
            <FileUploadField
              accept={["image/png", "image/jpeg", "image/webp"]}
              key={editingUploadResetKey}
              label="编辑租户 Logo"
              maxSizeBytes={1024 * 1024}
              onFileAccepted={handleEditingLogoAccepted}
              previewSrc={editingLogoFileName === "未上传" ? undefined : resolveTenantLogoSrc(editingLogoFileName)}
              selectedLabel={editingLogoFileName === "未上传" ? undefined : "当前 Logo"}
            />
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={() => setEditingTenant(null)} type="button">
                取消
              </Button>
              <Button disabled={isLogoUploading} variant="primary" onClick={saveProfile} type="button">
                {isLogoUploading ? "上传中" : "保存资料"}
              </Button>
            </div>
          </div>
        </div>
      )}

      {resourceDrawer && (
        <TenantResourceDrawerView
          drawer={resourceDrawer}
          error={resourceError}
          isLoading={isResourceLoading}
          onClose={() => setResourceDrawer(null)}
          spaces={resourceSpaces}
          users={resourceUsers}
        />
      )}
    </section>
  );
}

type TenantResourceDrawerViewProps = {
  drawer: TenantResourceDrawer;
  error: string;
  isLoading: boolean;
  onClose: () => void;
  spaces: SpaceRow[];
  users: TenantUserRow[];
};

function TenantResourceDrawerView({
  drawer,
  error,
  isLoading,
  onClose,
  spaces,
  users,
}: TenantResourceDrawerViewProps) {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [isClosing, setIsClosing] = useState(false);
  const title = drawer.kind === "spaces" ? `${drawer.tenant.name}空间` : `${drawer.tenant.name}用户`;
  const drawerClassName = [
    "tenant-resource-drawer",
    isFullscreen ? "tenant-resource-drawer--fullscreen" : "tenant-resource-drawer--half",
    isOpen ? "tenant-resource-drawer--open" : "",
  ].filter(Boolean).join(" ");
  const layerClassName = [
    "tenant-resource-drawer-layer",
    "tenant-resource-drawer-layer--overlay",
    isOpen ? "tenant-resource-drawer-layer--visible" : "",
  ].filter(Boolean).join(" ");

  useEffect(() => {
    const openTimer = window.setTimeout(() => setIsOpen(true), 0);
    return () => window.clearTimeout(openTimer);
  }, []);

  const closeDrawer = () => {
    setIsClosing(true);
    setIsOpen(false);
  };

  const finishClose = (event: TransitionEvent<HTMLElement>) => {
    if (event.currentTarget !== event.target || event.propertyName !== "transform" || !isClosing) {
      return;
    }
    onClose();
  };

  return (
    <div className={layerClassName} data-testid="tenant-resource-drawer-layer">
      <div aria-hidden="true" className="tenant-resource-drawer-backdrop" data-testid="tenant-resource-drawer-backdrop" />
      <aside
        aria-label={`${title}抽屉`}
        aria-modal="true"
        className={drawerClassName}
        onTransitionEnd={finishClose}
        role="dialog"
      >
        <header className="tenant-resource-drawer__head">
          <button
            aria-label="返回租户列表"
            className="tenant-resource-drawer__icon"
            onClick={closeDrawer}
            type="button"
          >
            <ArrowLeft aria-hidden="true" size={19} />
          </button>
          <div className="tenant-resource-drawer__title">
            <span>{drawer.tenant.code}</span>
            <h2>{title}</h2>
          </div>
          <div className="tenant-resource-drawer__tools">
            <button
              aria-label={isFullscreen ? "退出全屏抽屉" : "全屏抽屉"}
              className="tenant-resource-drawer__icon"
              onClick={() => setIsFullscreen((current) => !current)}
              type="button"
            >
              {isFullscreen
                ? <Minimize2 aria-hidden="true" size={18} />
                : <Maximize2 aria-hidden="true" size={18} />}
            </button>
            <button
              aria-label="关闭抽屉"
              className="tenant-resource-drawer__icon"
              onClick={closeDrawer}
              type="button"
            >
              <X aria-hidden="true" size={19} />
            </button>
          </div>
        </header>

        {isLoading && <div className="tenant-resource-drawer__state" role="status">正在加载</div>}
        {error && <div className="tenant-admin-warning" role="alert">{error}</div>}
        {!isLoading && !error && (
          drawer.kind === "spaces"
            ? <TenantSpaceDrawerTable spaces={spaces} />
            : <TenantUserDrawerTable users={users} />
        )}
      </aside>
    </div>
  );
}

function TenantSpaceDrawerTable({ spaces }: { spaces: SpaceRow[] }) {
  return (
    <div className="table-wrap tenant-resource-drawer__table">
      <table className="data-table tenant-resource-table">
        <thead>
          <tr>
            <th scope="col">空间</th>
            <th scope="col">Logo</th>
            <th scope="col">描述</th>
            <th scope="col">管理员</th>
            <th scope="col">操作</th>
          </tr>
        </thead>
        <tbody>
          {spaces.length === 0 && <EmptyTableRow colSpan={5} />}
          {spaces.map((space) => (
            <tr key={space.id}>
              <td>{space.name}</td>
              <td>{space.logoFileName}</td>
              <td>{space.description}</td>
              <td>{tenantSpaceAdminSummary(space)}</td>
              <td><span className="tenant-admin-muted">仅查看</span></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function TenantUserDrawerTable({ users }: { users: TenantUserRow[] }) {
  return (
    <div className="table-wrap tenant-resource-drawer__table">
      <table className="data-table tenant-resource-table">
        <thead>
          <tr>
            <th scope="col">用户</th>
            <th scope="col">账号</th>
            <th scope="col">头像</th>
            <th scope="col">角色</th>
            <th scope="col">状态</th>
            <th scope="col">操作</th>
          </tr>
        </thead>
        <tbody>
          {users.length === 0 && <EmptyTableRow colSpan={6} />}
          {users.map((user) => (
            <tr key={user.id}>
              <td>{user.name}</td>
              <td>{user.username}</td>
              <td>{user.avatarFileName}</td>
              <td>{tenantUserRoleLabel(user.role)}</td>
              <td>
                <StatusBadge tone={user.status === "enabled" ? "success" : "info"}>
                  {user.status === "enabled" ? "启用" : "禁用"}
                </StatusBadge>
              </td>
              <td><span className="tenant-admin-muted">仅查看</span></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function renderTenantLogo(tenant: TenantRow) {
  if (!tenant.logoFileName || tenant.logoFileName === "未上传") {
    return <span className="tenant-logo-empty">未上传</span>;
  }

  return (
    <img
      alt={`${tenant.name} Logo`}
      className="tenant-logo-thumb"
      src={resolveTenantLogoSrc(tenant.logoFileName)}
    />
  );
}

function resolveTenantLogoSrc(logoURL: string) {
  if (!logoURL.startsWith("/uploads")) {
    return logoURL;
  }

  const apiBaseURL = import.meta.env.VITE_API_BASE_URL ?? "";
  if (!apiBaseURL) {
    return logoURL;
  }

  // 上传接口返回后端静态资源相对路径，跨源联调时图片也要跟随 API 域名加载。
  return `${apiBaseURL.replace(/\/$/, "")}/${logoURL.replace(/^\//, "")}`;
}

function tenantUserRoleLabel(role: TenantUserRow["role"]) {
  switch (role) {
    case "tenant_admin":
      return "租户管理员";
    case "teacher":
      return "教师";
    case "student":
      return "考生";
    default:
      return role;
  }
}

function tenantSpaceAdminSummary(space: SpaceRow) {
  const admins = space.members
    .filter((member) => member.role === "space_admin" && member.status === "enabled")
    .map((member) => member.name);

  return admins.length ? admins.join("、") : "未配置";
}
