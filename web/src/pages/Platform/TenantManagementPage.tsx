import { useMemo, useState } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { SectionHeader } from "../../components/ui/SectionHeader";
import { StatusBadge } from "../../components/ui/StatusBadge";

type Tenant = {
  id: number;
  name: string;
  description: string;
  code: string;
  logoFileName: string;
  allowRegister: boolean;
  registerClosedLabel?: string;
};

const initialTenants: Tenant[] = [
  {
    id: 1,
    name: "青藤一中",
    description: "统一管理月考、联考和补测",
    code: "PM-QT01",
    logoFileName: "qingteng.png",
    allowRegister: true,
  },
  {
    id: 2,
    name: "知行培训",
    description: "企业知识课堂和阶段测评",
    code: "PM-ZX01",
    logoFileName: "zhixing.png",
    allowRegister: false,
    registerClosedLabel: "暂停注册",
  },
];

export function TenantManagementPage() {
  const [tenants, setTenants] = useState<Tenant[]>(initialTenants);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [logoFileName, setLogoFileName] = useState("");
  const [uploadResetKey, setUploadResetKey] = useState(0);
  const [editingTenant, setEditingTenant] = useState<Tenant | null>(null);
  const [editingDescription, setEditingDescription] = useState("");

  const enabledCount = useMemo(() => tenants.filter((tenant) => tenant.allowRegister).length, [tenants]);

  function handleCreateTenant(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // 新租户创建时立即生成租户码，方便平台管理员复制给租户负责人。
    const nextTenant: Tenant = {
      id: Date.now(),
      name,
      description,
      code: buildTenantCode(name, tenants.length + 1),
      logoFileName: logoFileName || "未上传",
      allowRegister: true,
    };

    setTenants((items) => [...items, nextTenant]);
    setName("");
    setDescription("");
    setLogoFileName("");
    setUploadResetKey((value) => value + 1);
  }

  function openDescriptionEditor(tenant: Tenant) {
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

  return (
    <section className="page platform-page">
      <SectionHeader
        description="平台管理员维护租户资料、租户码和默认注册开关。"
        title="租户管理"
      />

      <div className="platform-summary">
        <div>
          <span>租户总数</span>
          <strong>{tenants.length}</strong>
        </div>
        <div>
          <span>开放注册</span>
          <strong>{enabledCount}</strong>
        </div>
      </div>

      <div className="page-grid page-grid--two platform-grid">
        <Panel title="创建租户" subtitle="首版创建后生成租户码，后续接入平台租户 API。">
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
            <button className="primary-button" type="submit">创建租户</button>
          </form>
        </Panel>

        <Panel title="租户列表" subtitle="租户码用于注册链接和手动注册归属。">
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
                {tenants.map((tenant) => (
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
                        <button type="button" onClick={() => openDescriptionEditor(tenant)}>
                          编辑描述
                        </button>
                        <button type="button" onClick={() => resetTenantCode(tenant.id)}>
                          重置租户码
                        </button>
                        <button type="button" onClick={() => toggleRegister(tenant.id)}>
                          {tenant.allowRegister ? "关闭注册" : "开启注册"}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      </div>

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
              <button className="secondary-button" onClick={() => setEditingTenant(null)} type="button">
                取消
              </button>
              <button className="primary-button" onClick={saveDescription} type="button">
                保存描述
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

function buildTenantCode(tenantName: string, index: number) {
  if (tenantName.includes("星海")) {
    return "PM-XH01";
  }

  return `PM-T${String(index).padStart(2, "0")}`;
}

function nextTenantCode(currentCode: string) {
  const match = currentCode.match(/^(.*?)(\d+)$/);
  if (!match) {
    return `${currentCode}-2`;
  }

  return `${match[1]}${String(Number(match[2]) + 1).padStart(match[2].length, "0")}`;
}
