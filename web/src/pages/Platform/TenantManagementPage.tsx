import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { RefreshCw, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { tenantApi } from "../../api/tenants";
import type { TenantManagementAPI, TenantRow } from "../../api/tenants";

type TenantManagementPageProps = {
  api?: TenantManagementAPI;
};

export function TenantManagementPage({ api = tenantApi }: TenantManagementPageProps) {
  const [tenants, setTenants] = useState<TenantRow[]>([]);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [logoFileName, setLogoFileName] = useState("");
  const [uploadResetKey, setUploadResetKey] = useState(0);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [editingTenant, setEditingTenant] = useState<TenantRow | null>(null);
  const [editingDescription, setEditingDescription] = useState("");
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

  const filteredTenants = tenants.filter((tenant) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    // 租户列表搜索只匹配当前可见字段，便于平台管理员按名称、描述或租户码快速定位。
    return [tenant.name, tenant.description, tenant.code].some((value) => value.toLowerCase().includes(keyword));
  });

  async function handleCreateTenant(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const nextTenant = await api.createTenant({
      name,
      description,
      logoFileName: logoFileName || "未上传",
    });

    setTenants((items) => [...items, nextTenant]);
    setName("");
    setDescription("");
    setLogoFileName("");
    setUploadResetKey((value) => value + 1);
    setIsCreateDialogOpen(false);
  }

  function openDescriptionEditor(tenant: TenantRow) {
    setEditingTenant(tenant);
    setEditingDescription(tenant.description);
  }

  function saveDescription() {
    if (!editingTenant) {
      return;
    }

    // 租户描述是平台侧展示字段，保存后只影响当前租户的说明文本。
    setTenants((items) =>
      items.map((tenant) =>
        tenant.id === editingTenant.id ? { ...tenant, description: editingDescription } : tenant,
      ),
    );
    setEditingTenant(null);
    setEditingDescription("");
  }

  function resetTenantCode(tenantID: number) {
    setTenants((items) =>
      items.map((tenant) =>
        tenant.id === tenantID ? { ...tenant, code: nextTenantCode(tenant.code) } : tenant,
      ),
    );
  }

  function toggleRegister(tenantID: number) {
    setTenants((items) =>
      items.map((tenant) =>
        tenant.id === tenantID ? { ...tenant, allowRegister: !tenant.allowRegister } : tenant,
      ),
    );
  }

  function handleSearchTenants() {
    setAppliedSearchQuery(searchQuery);
  }

  function handleRefreshTenants() {
    setSearchQuery("");
    setAppliedSearchQuery("");
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
                {filteredTenants.length === 0 && <EmptyTableRow colSpan={5} />}
                {filteredTenants.map((tenant) => (
                  <tr key={tenant.id}>
                    <td>
                      <strong>{tenant.name}</strong>
                      <span>{tenant.description}</span>
                    </td>
                    <td>{tenant.logoFileName}</td>
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
                          onClick={() => openDescriptionEditor(tenant)}
                        >
                          编辑描述
                        </Button>
                        <Button
                          variant="actionReset"
                          type="button"
                          onClick={() => resetTenantCode(tenant.id)}
                        >
                          重置租户码
                        </Button>
                        <Button
                          onClick={() => toggleRegister(tenant.id)}
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
            <p>首版创建后生成租户码，后续接入平台租户 API。</p>
            <form className="platform-form" onSubmit={handleCreateTenant}>
              <label className="field">
                <span>租户名称</span>
                <input
                  onChange={(event) => setName(event.target.value)}
                  required
                  value={name}
                />
              </label>
              <label className="field">
                <span>租户描述</span>
                <textarea
                  onChange={(event) => setDescription(event.target.value)}
                  required
                  value={description}
                />
              </label>
              <FileUploadField
                accept={["image/png", "image/jpeg"]}
                key={uploadResetKey}
                label="租户 Logo"
                maxSizeBytes={1024 * 1024}
                onFileAccepted={(file) => setLogoFileName(file.name)}
              />
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsCreateDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  确认创建
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {editingTenant && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="租户描述弹窗">
          <div className="platform-dialog__card">
            <h2>{editingTenant.name}</h2>
            <label className="field">
              <span>编辑租户描述</span>
              <textarea
                onChange={(event) => setEditingDescription(event.target.value)}
                value={editingDescription}
              />
            </label>
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={() => setEditingTenant(null)} type="button">
                取消
              </Button>
              <Button variant="primary" onClick={saveDescription} type="button">
                保存描述
              </Button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

function nextTenantCode(currentCode: string) {
  const match = currentCode.match(/^(.*?)(\d+)$/);
  if (!match) {
    return `${currentCode}-2`;
  }

  return `${match[1]}${String(Number(match[2]) + 1).padStart(match[2].length, "0")}`;
}
