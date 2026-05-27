import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { RefreshCw, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { tenantApi } from "../../api/tenants";
import type { TenantManagementAPI, TenantRow } from "../../api/tenants";
import { uploadApi } from "../../api/uploads";
import type { UploadAPI } from "../../api/uploads";

type TenantManagementPageProps = {
  api?: TenantManagementAPI;
  uploadAPI?: UploadAPI;
};

export function TenantManagementPage({ api = tenantApi, uploadAPI = uploadApi }: TenantManagementPageProps) {
  const [tenants, setTenants] = useState<TenantRow[]>([]);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [logoFileName, setLogoFileName] = useState("");
  const [allowRegister, setAllowRegister] = useState(true);
  const [uploadResetKey, setUploadResetKey] = useState(0);
  const [isLogoUploading, setIsLogoUploading] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [editingTenant, setEditingTenant] = useState<TenantRow | null>(null);
  const [editingName, setEditingName] = useState("");
  const [editingDescription, setEditingDescription] = useState("");
  const [editingLogoFileName, setEditingLogoFileName] = useState("");
  const [editingUploadResetKey, setEditingUploadResetKey] = useState(0);
  const [loadError, setLoadError] = useState("");

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
    });

    setTenants((items) => [nextTenant, ...items]);
    setName("");
    setDescription("");
    setLogoFileName("");
    setAllowRegister(true);
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

  async function resetTenantCode(tenantID: number) {
    try {
      const nextTenant = await api.resetTenantCode(tenantID);
      updateTenantRow(nextTenant);
      setLoadError("");
    } catch {
      setLoadError("租户操作失败");
    }
  }

  async function toggleRegister(tenant: TenantRow) {
    try {
      // 注册开关只控制自注册入口，不能和租户启停状态混用。
      const nextTenant = tenant.allowRegister
        ? await api.disableTenantRegistration(tenant.id)
        : await api.enableTenantRegistration(tenant.id);
      updateTenantRow(nextTenant);
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
                          variant="actionReset"
                          type="button"
                          onClick={() => resetTenantCode(tenant.id)}
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
            <p>创建后生成租户码，用于注册链接和手动注册归属。</p>
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
              previewSrc={editingLogoFileName === "未上传" ? undefined : editingLogoFileName}
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
    </section>
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
      src={tenant.logoFileName}
    />
  );
}
