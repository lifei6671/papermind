import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { RefreshCw, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { spaceApi } from "../../api/spaces";
import type { MemberRole, SpaceManagementAPI, SpaceRow } from "../../api/spaces";
import { userApi as defaultUserApi } from "../../api/users";
import type { TenantUserRow, UserManagementAPI } from "../../api/users";

const roleLabels: Record<MemberRole, string> = {
  space_admin: "空间管理员",
  teacher: "教师",
  student: "学生",
};

type SpaceManagementPageProps = {
  api?: SpaceManagementAPI;
  userApi?: Pick<UserManagementAPI, "listUsers">;
  tenantID?: number;
};

export function SpaceManagementPage({ api = spaceApi, userApi = defaultUserApi, tenantID }: SpaceManagementPageProps) {
  const [spaces, setSpaces] = useState<SpaceRow[]>([]);
  const [users, setUsers] = useState<TenantUserRow[]>([]);
  const [spaceName, setSpaceName] = useState("");
  const [spaceDescription, setSpaceDescription] = useState("");
  const [spaceAdminUserID, setSpaceAdminUserID] = useState("");
  const [spaceLogoFileName, setSpaceLogoFileName] = useState("");
  const [logoResetKey, setLogoResetKey] = useState(0);
  const [editingSpace, setEditingSpace] = useState<SpaceRow | null>(null);
  const [editingDescription, setEditingDescription] = useState("");
  const [memberName, setMemberName] = useState("");
  const [memberRole, setMemberRole] = useState<MemberRole>("student");
  const [invariantError, setInvariantError] = useState("");
  const [loadError, setLoadError] = useState("");
  const [defaultDuration, setDefaultDuration] = useState(60);
  const [showPracticeAnalysis, setShowPracticeAnalysis] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isConfigDialogOpen, setIsConfigDialogOpen] = useState(false);
  const [selectedMemberSpaceID, setSelectedMemberSpaceID] = useState<number | null>(null);

  const selectedMemberSpace = spaces.find((space) => space.id === selectedMemberSpaceID) ?? null;
  const hasTenantContext = isPositiveInteger(tenantID);

  useEffect(() => {
    if (!isPositiveInteger(tenantID)) {
      return;
    }

    let ignore = false;
    const currentTenantID = tenantID;

    Promise.all([api.listSpaces(currentTenantID), userApi.listUsers(currentTenantID)])
      .then(([spaceData, userData]) => {
        if (!ignore) {
          setSpaces(spaceData.items);
          setUsers(userData.items);
          setSpaceAdminUserID((current) => current || String(userData.items[0]?.id ?? ""));
          setLoadError("");
        }
      })
      .catch(() => {
        if (!ignore) {
          setLoadError("空间或用户列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, tenantID, userApi]);

  const filteredSpaces = spaces.filter((space) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    // 空间搜索只匹配列表可见字段，方便租户管理员按班级、描述、Logo 或管理员定位。
    return [space.name, space.description, space.logoFileName, adminSummary(space)].some((value) =>
      value.toLowerCase().includes(keyword),
    );
  });

  async function handleCreateSpace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!isPositiveInteger(tenantID)) {
      setLoadError("当前账号没有租户上下文，请先登录租户账号后再创建空间");
      return;
    }

    const adminUserID = Number(spaceAdminUserID);
    if (!adminUserID) {
      setLoadError("请先选择空间管理员");
      return;
    }

    // 创建空间时必须指定空间管理员，保证空间成员管理从一开始满足管理员不变式。
    const nextSpace = await api.createSpace({
      tenantID,
      name: spaceName,
      description: spaceDescription,
      logoFileName: spaceLogoFileName || "未上传",
      adminUserID,
    });

    setSpaces((items) => [...items, nextSpace]);
    setSpaceName("");
    setSpaceDescription("");
    setSpaceAdminUserID(String(users[0]?.id ?? ""));
    setSpaceLogoFileName("");
    setLogoResetKey((value) => value + 1);
    setIsCreateDialogOpen(false);
  }

  function openDescriptionEditor(space: SpaceRow) {
    setEditingSpace(space);
    setEditingDescription(space.description);
  }

  function saveDescription() {
    if (!editingSpace) {
      return;
    }

    // 空间描述用于租户内教学场景识别，保存后只影响当前空间展示。
    setSpaces((items) =>
      items.map((space) =>
        space.id === editingSpace.id ? { ...space, description: editingDescription } : space,
      ),
    );
    setEditingSpace(null);
    setEditingDescription("");
  }

  function handleAddMember(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!selectedMemberSpace) {
      return;
    }

    const localMemberID = Date.now();
    // 成员是空间的子资源，添加时只能写入当前打开的空间成员列表。
    setSpaces((items) =>
      items.map((space) =>
        space.id === selectedMemberSpace.id
          ? {
              ...space,
              members: [
                ...space.members,
                { id: localMemberID, userID: localMemberID, name: memberName, role: memberRole, status: "enabled" },
              ],
            }
          : space,
      ),
    );
    setMemberName("");
    setMemberRole("student");
  }

  function downgradeMember(memberID: number) {
    if (!selectedMemberSpace) {
      return;
    }

    const adminCount = selectedMemberSpace.members.filter(
      (member) => member.role === "space_admin" && member.status === "enabled",
    ).length;

    if (adminCount <= 1) {
      setInvariantError("空间至少保留一个启用状态的空间管理员");
      return;
    }

    // 空间成员降级只影响当前子页面对应空间，避免误操作其他空间的管理员。
    setSpaces((items) =>
      items.map((space) =>
        space.id === selectedMemberSpace.id
          ? {
              ...space,
              members: space.members.map((member) =>
                member.id === memberID ? { ...member, role: "teacher" } : member,
              ),
            }
          : space,
      ),
    );
  }

  function handleSaveConfig(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // 空间配置按空间维度生效，首版只暴露考试时长和练习解析展示两个高频项。
    setIsConfigDialogOpen(false);
  }

  function openMemberManager(spaceID: number) {
    setSelectedMemberSpaceID(spaceID);
    setInvariantError("");
  }

  function handleSearchSpaces() {
    setAppliedSearchQuery(searchQuery);
  }

  function handleRefreshSpaces() {
    setSearchQuery("");
    setAppliedSearchQuery("");
  }

  if (!hasTenantContext) {
    return (
      <section className="page platform-page tenant-admin-page">
        <nav aria-label="空间管理菜单" className="platform-tabbar" role="tablist">
          <a className="platform-tab platform-tab--active" href="/spaces" role="tab" aria-selected="true">
            空间管理
          </a>
        </nav>

        <Panel>
          <div className="tenant-admin-warning" role="alert">
            当前账号没有租户上下文，请先登录租户账号后再进入空间管理。
          </div>
        </Panel>
      </section>
    );
  }

  return (
    <section className="page platform-page tenant-admin-page">
      <nav aria-label="空间管理菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/spaces" role="tab" aria-selected="true">
          空间管理
        </a>
      </nav>

      {!selectedMemberSpace && (
        <Panel>
          <div className="tenant-list-toolbar">
            <div className="tenant-list-actions" aria-label="空间操作区">
              <Button
                variant="toolbarPrimary"
                onClick={() => setIsCreateDialogOpen(true)}
                type="button"
              >
                创建空间
              </Button>
              <Button
                variant="toolbarSecondary"
                onClick={() => setIsConfigDialogOpen(true)}
                type="button"
              >
                保存空间配置
              </Button>
            </div>
            <div className="tenant-search-actions">
              <label className="tenant-search-field">
                <span className="sr-only">搜索空间</span>
                <input
                  onChange={(event) => setSearchQuery(event.target.value)}
                  placeholder="输入空间名称、描述或管理员"
                  value={searchQuery}
                />
              </label>
              <Button aria-label="搜索" variant="icon" onClick={handleSearchSpaces} type="button">
                <Search aria-hidden="true" size={16} />
              </Button>
              <Button
                aria-label="刷新空间列表"
                variant="icon"
                onClick={handleRefreshSpaces}
                type="button"
              >
                <RefreshCw aria-hidden="true" size={16} />
              </Button>
            </div>
          </div>

          {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
          <div className="table-wrap">
            <table className="data-table tenant-admin-table">
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
                {filteredSpaces.length === 0 && <EmptyTableRow colSpan={5} />}
                {filteredSpaces.map((space) => (
                  <tr key={space.id}>
                    <td>{space.name}</td>
                    <td>{space.logoFileName}</td>
                    <td>{space.description}</td>
                    <td aria-hidden="true">{adminSummary(space)}</td>
                    <td>
                      <div className="tenant-actions">
                        <Button
                          variant="actionEdit"
                          onClick={() => openDescriptionEditor(space)}
                          type="button"
                        >
                          编辑描述
                        </Button>
                        <Button
                          variant="actionReset"
                          onClick={() => openMemberManager(space.id)}
                          type="button"
                        >
                          成员管理
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      )}

      {selectedMemberSpace && (
        <Panel>
          <div className="tenant-list-toolbar">
            <div className="tenant-list-actions" aria-label="成员操作区">
              <Button
                variant="toolbarSecondary"
                onClick={() => {
                  setSelectedMemberSpaceID(null);
                  setInvariantError("");
                }}
                type="button"
              >
                返回空间列表
              </Button>
            </div>
            <strong>{selectedMemberSpace.name}成员</strong>
          </div>
          {invariantError && <div className="tenant-admin-warning" role="alert">{invariantError}</div>}
          <form className="tenant-member-form" onSubmit={handleAddMember}>
            <label className="field">
              <span>成员姓名</span>
              <input onChange={(event) => setMemberName(event.target.value)} required value={memberName} />
            </label>
            <label className="field">
              <span>成员角色</span>
              <select onChange={(event) => setMemberRole(event.target.value as MemberRole)} value={memberRole}>
                <option value="space_admin">空间管理员</option>
                <option value="teacher">教师</option>
                <option value="student">学生</option>
              </select>
            </label>
            <Button variant="secondary" type="submit">添加成员</Button>
          </form>
          <div className="table-wrap">
            <table className="data-table tenant-admin-table">
              <thead>
                <tr>
                  <th scope="col">成员</th>
                  <th scope="col">角色</th>
                  <th scope="col">状态</th>
                  <th scope="col">操作</th>
                </tr>
              </thead>
              <tbody>
                {selectedMemberSpace.members.length === 0 && <EmptyTableRow colSpan={4} />}
                {selectedMemberSpace.members.map((member) => (
                  <tr key={member.id}>
                    <td>{member.name}</td>
                    <td>{roleLabels[member.role]}</td>
                    <td>
                      <StatusBadge tone="success">启用</StatusBadge>
                    </td>
                    <td>
                      {member.role === "space_admin" ? (
                        <Button
                          variant="actionReset"
                          onClick={() => downgradeMember(member.id)}
                          type="button"
                        >
                          降级为教师
                        </Button>
                      ) : (
                        <span className="tenant-admin-muted">无</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      )}

      {isCreateDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="创建空间弹窗">
          <div className="platform-dialog__card">
            <h2>创建空间</h2>
            <form className="platform-form" onSubmit={handleCreateSpace}>
              <label className="field">
                <span>空间名称</span>
                <input onChange={(event) => setSpaceName(event.target.value)} required value={spaceName} />
              </label>
              <label className="field">
                <span>空间描述</span>
                <textarea
                  onChange={(event) => setSpaceDescription(event.target.value)}
                  required
                  value={spaceDescription}
                />
              </label>
              <label className="field">
                <span>空间管理员</span>
                <select
                  onChange={(event) => setSpaceAdminUserID(event.target.value)}
                  required
                  value={spaceAdminUserID}
                >
                  {users.map((user) => (
                    <option key={user.id} value={user.id}>
                      {user.name}
                    </option>
                  ))}
                </select>
              </label>
              <FileUploadField
                accept={["image/png", "image/jpeg"]}
                key={logoResetKey}
                label="空间 Logo"
                maxSizeBytes={1024 * 1024}
                onFileAccepted={(file) => setSpaceLogoFileName(file.name)}
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

      {isConfigDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="空间配置弹窗">
          <div className="platform-dialog__card">
            <h2>空间配置</h2>
            <form className="platform-settings-form" onSubmit={handleSaveConfig}>
              <label className="field">
                <span>默认考试时长</span>
                <input
                  min={1}
                  onChange={(event) => setDefaultDuration(Number(event.target.value))}
                  type="number"
                  value={defaultDuration}
                />
              </label>
              <label className="platform-check">
                <input
                  checked={showPracticeAnalysis}
                  onChange={(event) => setShowPracticeAnalysis(event.target.checked)}
                  type="checkbox"
                />
                允许学生查看练习解析
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsConfigDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  确认保存配置
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {editingSpace && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="空间描述弹窗">
          <div className="platform-dialog__card">
            <h2>{editingSpace.name}</h2>
            <label className="field">
              <span>编辑空间描述</span>
              <textarea
                onChange={(event) => setEditingDescription(event.target.value)}
                value={editingDescription}
              />
            </label>
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={() => setEditingSpace(null)} type="button">
                取消
              </Button>
              <Button variant="primary" onClick={saveDescription} type="button">
                保存空间描述
              </Button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

function isPositiveInteger(value: number | undefined): value is number {
  return Number.isInteger(value) && Number(value) > 0;
}

function adminSummary(space: SpaceRow) {
  const admins = space.members
    .filter((member) => member.role === "space_admin" && member.status === "enabled")
    .map((member) => member.name)
    .join("、");

  return `空间管理员：${admins}`;
}
