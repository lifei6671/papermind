import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Pagination } from "../../components/ui/Pagination";
import { ArrowLeft, Maximize2, Minimize2, Search, X } from "lucide-react";
import { useEffect, useState, type TransitionEvent } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { RefreshIcon } from "../../components/ui/RefreshIcon";
import { withRefreshFeedback } from "../../components/ui/refreshFeedback";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { useFeedback } from "../../app/feedback-context";
import { formatApiErrorMessage } from "../../api/client";
import { spaceApi } from "../../api/spaces";
import type { MemberRole, SpaceManagementAPI, SpaceMember, SpaceMemberAPI, SpaceRow } from "../../api/spaces";
import { userApi as defaultUserApi } from "../../api/users";
import type { TenantUserRow, UserManagementAPI } from "../../api/users";

const roleLabels: Record<MemberRole, string> = {
  space_admin: "空间管理员",
  teacher: "教师",
  student: "学生",
};
const userRoleSuggestionLabels: Record<TenantUserRow["role"], string> = {
  tenant_admin: "租户管理员",
  teacher: "教师",
  student: "学生",
};
const statusLabels: Record<SpaceMember["status"], string> = {
  enabled: "启用",
  disabled: "禁用",
};

const memberPageSize = 5;
const spacePageSize = 5;
const addUserSuggestionDebounceMs = 300;

const registerMethodLabels: Record<string, string> = {
  tenant_account: "租户账号",
  tenant_register: "租户自注册",
  admin_created: "管理员创建",
  import: "批量导入",
};

type SpacePageAPI = SpaceManagementAPI & Pick<SpaceMemberAPI, "createSpaceMember" | "listSpaceMembers" | "updateSpaceMember">;

type SpaceManagementPageProps = {
  api?: SpacePageAPI;
  userApi?: Pick<UserManagementAPI, "listUsers">;
  tenantID?: number;
};

export function SpaceManagementPage({ api = spaceApi, userApi = defaultUserApi, tenantID }: SpaceManagementPageProps) {
  const [spaces, setSpaces] = useState<SpaceRow[]>([]);
  const [users, setUsers] = useState<TenantUserRow[]>([]);
  const [spaceName, setSpaceName] = useState("");
  const [spaceDescription, setSpaceDescription] = useState("");
  const [spaceAdminUserID, setSpaceAdminUserID] = useState("");
  const [spaceAdminQuery, setSpaceAdminQuery] = useState("");
  const [debouncedSpaceAdminQuery, setDebouncedSpaceAdminQuery] = useState("");
  const [selectedSpaceAdminName, setSelectedSpaceAdminName] = useState("");
  const [isSpaceAdminInputFocused, setIsSpaceAdminInputFocused] = useState(false);
  const [spaceLogoFileName, setSpaceLogoFileName] = useState("");
  const [logoResetKey, setLogoResetKey] = useState(0);
  const [editingSpace, setEditingSpace] = useState<SpaceRow | null>(null);
  const [editingName, setEditingName] = useState("");
  const [editingDescription, setEditingDescription] = useState("");
  const [editingLogoFileName, setEditingLogoFileName] = useState("");
  const [editingAdminUserID, setEditingAdminUserID] = useState("");
  const [editingAdminQuery, setEditingAdminQuery] = useState("");
  const [debouncedEditingAdminQuery, setDebouncedEditingAdminQuery] = useState("");
  const [selectedEditingAdminName, setSelectedEditingAdminName] = useState("");
  const [isEditingAdminInputFocused, setIsEditingAdminInputFocused] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [spacePage, setSpacePage] = useState(1);
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isAddUserDialogOpen, setIsAddUserDialogOpen] = useState(false);
  const [addUserQuery, setAddUserQuery] = useState("");
  const [debouncedAddUserQuery, setDebouncedAddUserQuery] = useState("");
  const [selectedAddUserName, setSelectedAddUserName] = useState("");
  const [addUserSpaceID, setAddUserSpaceID] = useState("");
  const [addUserRole, setAddUserRole] = useState<MemberRole | "">("");
  const [isAddUserInputFocused, setIsAddUserInputFocused] = useState(false);
  const [isAddingUser, setIsAddingUser] = useState(false);
  const [selectedMemberSpaceID, setSelectedMemberSpaceID] = useState<number | null>(null);
  const [isSpaceListRefreshing, setIsSpaceListRefreshing] = useState(false);
  const { showError } = useFeedback();

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

  useEffect(() => {
    if (!isAddUserDialogOpen || addUserQuery.trim() === "") {
      return;
    }

    const timeoutID = window.setTimeout(() => {
      setDebouncedAddUserQuery(addUserQuery);
    }, addUserSuggestionDebounceMs);

    return () => {
      window.clearTimeout(timeoutID);
    };
  }, [addUserQuery, isAddUserDialogOpen]);

  useEffect(() => {
    if (!isCreateDialogOpen || spaceAdminQuery.trim() === "") {
      return;
    }

    const timeoutID = window.setTimeout(() => {
      setDebouncedSpaceAdminQuery(spaceAdminQuery);
    }, addUserSuggestionDebounceMs);

    return () => {
      window.clearTimeout(timeoutID);
    };
  }, [isCreateDialogOpen, spaceAdminQuery]);

  useEffect(() => {
    if (!editingSpace || editingAdminQuery.trim() === "") {
      return;
    }

    const timeoutID = window.setTimeout(() => {
      setDebouncedEditingAdminQuery(editingAdminQuery);
    }, addUserSuggestionDebounceMs);

    return () => {
      window.clearTimeout(timeoutID);
    };
  }, [editingAdminQuery, editingSpace]);

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
  const totalSpacePages = Math.max(1, Math.ceil(filteredSpaces.length / spacePageSize));
  const currentSpacePage = Math.min(spacePage, totalSpacePages);
  const pagedSpaces = filteredSpaces.slice(
    (currentSpacePage - 1) * spacePageSize,
    currentSpacePage * spacePageSize,
  );
  const addUserSuggestions = matchingAddUserSuggestions(users, spaces, addUserSpaceID, debouncedAddUserQuery);
  const shouldShowAddUserSuggestions =
    isAddUserInputFocused && debouncedAddUserQuery.trim() !== "" && addUserSuggestions.length > 0;
  const spaceAdminSuggestions = matchingUserSuggestions(users, debouncedSpaceAdminQuery);
  const shouldShowSpaceAdminSuggestions =
    isSpaceAdminInputFocused && debouncedSpaceAdminQuery.trim() !== "" && spaceAdminSuggestions.length > 0;
  const editingAdminSuggestions = matchingUserSuggestions(users, debouncedEditingAdminQuery);
  const shouldShowEditingAdminSuggestions =
    isEditingAdminInputFocused && debouncedEditingAdminQuery.trim() !== "" && editingAdminSuggestions.length > 0;

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
    setSpacePage(Math.ceil((spaces.length + 1) / spacePageSize));
    setSpaceName("");
    setSpaceDescription("");
    setSpaceAdminUserID(String(users[0]?.id ?? ""));
    setSpaceLogoFileName("");
    setLogoResetKey((value) => value + 1);
    setIsCreateDialogOpen(false);
  }

  function openCreateSpaceDialog() {
    const selectedUser = users.find((user) => String(user.id) === spaceAdminUserID) ?? users[0];
    setSpaceAdminUserID(selectedUser ? String(selectedUser.id) : "");
    setSpaceAdminQuery(selectedUser?.username ?? "");
    setDebouncedSpaceAdminQuery("");
    setSelectedSpaceAdminName(selectedUser?.name ?? "");
    setIsSpaceAdminInputFocused(false);
    setIsCreateDialogOpen(true);
  }

  function openAddUserDialog() {
    setAddUserQuery("");
    setDebouncedAddUserQuery("");
    setSelectedAddUserName("");
    setAddUserSpaceID("");
    setAddUserRole("");
    setIsAddUserInputFocused(false);
    setIsAddUserDialogOpen(true);
  }

  async function handleAddUserToSpace(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!isPositiveInteger(tenantID)) {
      setLoadError("当前账号没有租户上下文，请先登录租户账号后再添加用户");
      return;
    }

    const keyword = addUserQuery.trim().toLowerCase();
    const targetUser = users.find((user) =>
      user.username.toLowerCase() === keyword || user.name.toLowerCase() === keyword,
    );
    if (!targetUser) {
      showError("未找到匹配的租户用户");
      return;
    }

    const targetSpaceID = Number(addUserSpaceID);
    const targetSpace = spaces.find((space) => space.id === targetSpaceID);
    if (!targetSpace) {
      showError("请选择目标空间");
      return;
    }
    if (!addUserRole) {
      showError("请选择空间身份");
      return;
    }

    if (targetSpace.members.some((member) => member.userID === targetUser.id)) {
      showError("用户已在该空间中");
      return;
    }

    setIsAddingUser(true);
    try {
      const nextMember = await api.createSpaceMember({
        tenantID,
        spaceID: targetSpaceID,
        userID: targetUser.id,
        role: addUserRole,
      });

      setSpaces((items) =>
        items.map((space) =>
          space.id === targetSpaceID ? { ...space, members: [...space.members, nextMember] } : space,
        ),
      );
      setIsAddUserDialogOpen(false);
      setAddUserQuery("");
    } catch (error) {
      showError(formatApiErrorMessage(error, "添加用户到空间失败"));
    } finally {
      setIsAddingUser(false);
    }
  }


  function openSpaceEditor(space: SpaceRow) {
    const currentAdmin = space.members.find((member) => member.role === "space_admin" && member.status === "enabled");
    setEditingSpace(space);
    setEditingName(space.name);
    setEditingDescription(space.description);
    setEditingLogoFileName(space.logoFileName === "未上传" ? "" : space.logoFileName);
    setEditingAdminUserID(currentAdmin ? String(currentAdmin.userID) : "");
    setEditingAdminQuery(currentAdmin?.username ?? users.find((user) => user.id === currentAdmin?.userID)?.username ?? "");
    setDebouncedEditingAdminQuery("");
    setSelectedEditingAdminName(currentAdmin?.name ?? "");
    setIsEditingAdminInputFocused(false);
  }

  async function saveSpaceProfile() {
    if (!editingSpace || !isPositiveInteger(tenantID)) {
      return;
    }

    const nextAdminUserID = Number(editingAdminUserID);
    if (!nextAdminUserID) {
      showError("请先选择空间管理员");
      return;
    }

    try {
      const updatedSpace = await api.updateSpace({
        tenantID,
        spaceID: editingSpace.id,
        name: editingName,
        description: editingDescription,
        logoFileName: editingLogoFileName || "未上传",
      });
      const previousAdmin = editingSpace.members.find(
        (member) => member.role === "space_admin" && member.status === "enabled",
      );
      const nextAdmin = editingSpace.members.find((member) => member.userID === nextAdminUserID);
      let nextMembers = mergeSpaceMembers(editingSpace.members, updatedSpace.members);

      if (nextAdminUserID !== previousAdmin?.userID) {
        if (nextAdmin) {
          const promotedMember = await api.updateSpaceMember({
            tenantID,
            spaceID: editingSpace.id,
            userID: nextAdminUserID,
            role: "space_admin",
          });
          nextMembers = upsertSpaceMember(nextMembers, { ...nextAdmin, ...promotedMember });
        } else {
          const createdMember = await api.createSpaceMember({
            tenantID,
            spaceID: editingSpace.id,
            userID: nextAdminUserID,
            role: "space_admin",
          });
          nextMembers = upsertSpaceMember(nextMembers, createdMember);
        }

        if (previousAdmin) {
          const downgradedMember = await api.updateSpaceMember({
            tenantID,
            spaceID: editingSpace.id,
            userID: previousAdmin.userID,
            role: "teacher",
          });
          nextMembers = upsertSpaceMember(nextMembers, { ...previousAdmin, ...downgradedMember, role: "teacher" });
        }
      }

      setSpaces((items) =>
        items.map((space) =>
          space.id === editingSpace.id ? { ...updatedSpace, members: nextMembers } : space,
        ),
      );
      closeSpaceEditor();
    } catch (error) {
      showError(formatApiErrorMessage(error, "保存空间信息失败"));
    }
  }

  function closeSpaceEditor() {
    setEditingSpace(null);
    setEditingName("");
    setEditingDescription("");
    setEditingLogoFileName("");
    setEditingAdminUserID("");
    setEditingAdminQuery("");
    setDebouncedEditingAdminQuery("");
    setSelectedEditingAdminName("");
    setIsEditingAdminInputFocused(false);
  }

  function downgradeMember(memberID: number) {
    if (!selectedMemberSpace) {
      return;
    }

    const adminCount = selectedMemberSpace.members.filter(
      (member) => member.role === "space_admin" && member.status === "enabled",
    ).length;

    if (adminCount <= 1) {
      showError("空间至少保留一个启用状态的空间管理员");
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
  async function updateMemberStatus(targetMember: SpaceMember, status: SpaceMember["status"]) {
    if (!selectedMemberSpace || !isPositiveInteger(tenantID)) {
      return;
    }

    const adminCount = selectedMemberSpace.members.filter(
      (member) => member.role === "space_admin" && member.status === "enabled",
    ).length;

    if (status === "disabled" && targetMember.role === "space_admin" && targetMember.status === "enabled" && adminCount <= 1) {
      showError("空间至少保留一个启用状态的空间管理员");
      return;
    }

    try {
      const updatedMember = await api.updateSpaceMember({
        tenantID,
        spaceID: selectedMemberSpace.id,
        userID: targetMember.userID,
        status,
      });
      // 后端更新接口只保证返回成员关系字段，详情字段沿用当前列表快照。
      setSpaces((items) =>
        items.map((space) =>
          space.id === selectedMemberSpace.id
            ? {
                ...space,
                members: space.members.map((member) =>
                  member.userID === targetMember.userID
                    ? {
                        ...member,
                        ...updatedMember,
                        username: updatedMember.username ?? member.username,
                        phone: updatedMember.phone ?? member.phone,
                        email: updatedMember.email ?? member.email,
                        registeredAt: updatedMember.registeredAt ?? member.registeredAt,
                        registerMethod: updatedMember.registerMethod ?? member.registerMethod,
                      }
                    : member,
                ),
              }
            : space,
        ),
      );
    } catch {
      showError(status === "enabled" ? "启用成员失败" : "禁用成员失败");
    }
  }

  function openMemberManager(spaceID: number) {
    setSelectedMemberSpaceID(spaceID);
  }

  function handleSearchSpaces() {
    setSpacePage(1);
    setAppliedSearchQuery(searchQuery);
  }

  async function handleRefreshSpaces() {
    if (!isPositiveInteger(tenantID)) {
      return;
    }

    setIsSpaceListRefreshing(true);
    try {
      const [spaceData, userData] = await withRefreshFeedback(
        Promise.all([api.listSpaces(tenantID), userApi.listUsers(tenantID)]),
      );
      setSpaces(spaceData.items);
      setUsers(userData.items);
      setSpaceAdminUserID((current) => current || String(userData.items[0]?.id ?? ""));
      setSearchQuery("");
      setAppliedSearchQuery("");
      setSpacePage(1);
      setLoadError("");
    } catch {
      setLoadError("空间或用户列表加载失败");
    } finally {
      setIsSpaceListRefreshing(false);
    }
  }

  async function refreshSpaceMembers(spaceID: number) {
    if (!isPositiveInteger(tenantID)) {
      return;
    }

    try {
      const memberData = await api.listSpaceMembers({ tenantID, spaceID });
      setSpaces((items) =>
        items.map((space) => (space.id === spaceID ? { ...space, members: memberData.items } : space)),
      );
    } catch {
      showError("成员列表刷新失败");
    }
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

        <Panel>
          <div className="tenant-list-toolbar">
            <div className="tenant-list-actions" aria-label="空间操作区">
              <Button
                variant="toolbarPrimary"
                onClick={openCreateSpaceDialog}
                type="button"
              >
                创建空间
              </Button>
              <Button
                variant="toolbarSecondary"
                onClick={openAddUserDialog}
                type="button"
              >
                添加用户
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
                disabled={isSpaceListRefreshing}
                variant="icon"
                onClick={() => void handleRefreshSpaces()}
                type="button"
              >
                <RefreshIcon active={isSpaceListRefreshing} />
              </Button>
            </div>
          </div>

          {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
          <div
            className={[
              "table-wrap",
              "tenant-list-transition",
              isSpaceListRefreshing ? "tenant-list-transition--refreshing" : "",
            ].filter(Boolean).join(" ")}
            data-testid="space-list-table-wrap"
          >
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
                {pagedSpaces.map((space) => (
                  <tr key={space.id}>
                    <td>{space.name}</td>
                    <td>{space.logoFileName}</td>
                    <td>{space.description}</td>
                    <td aria-hidden="true">{adminSummary(space)}</td>
                    <td>
                      <div className="tenant-actions">
                        <Button
                          variant="actionEdit"
                          onClick={() => openSpaceEditor(space)}
                          type="button"
                        >
                          编辑空间
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
          <Pagination
            page={currentSpacePage}
            pageSize={spacePageSize}
            total={filteredSpaces.length}
            onPageChange={setSpacePage}
          />
        </Panel>

      {selectedMemberSpace && (
        <SpaceMemberDrawer
          onClose={() => {
            setSelectedMemberSpaceID(null);
          }}
          onDisableMember={(member) => updateMemberStatus(member, "disabled")}
          onDowngradeMember={downgradeMember}
          onEnableMember={(member) => updateMemberStatus(member, "enabled")}
          onRefreshMembers={refreshSpaceMembers}
          space={selectedMemberSpace}
        />
      )}

      {isCreateDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="创建空间弹窗">
          <div className="platform-dialog__card">
            <h2>创建空间</h2>
            <form className="platform-form" onSubmit={handleCreateSpace}>
              <label className="field">
                <span className="field-label">
                  空间名称
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <input
                  aria-label="空间名称"
                  onChange={(event) => setSpaceName(event.target.value)}
                  required
                  value={spaceName}
                />
              </label>
              <label className="field">
                <span className="field-label">
                  空间描述
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <textarea
                  aria-label="空间描述"
                  onChange={(event) => setSpaceDescription(event.target.value)}
                  required
                  value={spaceDescription}
                />
              </label>
              <label className="field">
                <span className="field-label">
                  空间管理员
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <div
                  className={[
                    "add-user-identity-field",
                    selectedSpaceAdminName ? "add-user-identity-field--selected" : "",
                  ].filter(Boolean).join(" ")}
                >
                  <input
                    aria-label="空间管理员"
                    aria-autocomplete="list"
                    aria-controls="space-admin-suggestions"
                    aria-expanded={shouldShowSpaceAdminSuggestions}
                    className="add-user-identity-input"
                    onBlur={() => setIsSpaceAdminInputFocused(false)}
                    onChange={(event) => {
                      const value = event.target.value;
                      setSpaceAdminQuery(value);
                      setSpaceAdminUserID("");
                      setSelectedSpaceAdminName("");
                      if (value.trim() === "") {
                        setDebouncedSpaceAdminQuery("");
                      }
                    }}
                    onFocus={() => setIsSpaceAdminInputFocused(true)}
                    placeholder="输入管理员账号或姓名"
                    required
                    style={
                      selectedSpaceAdminName ? { width: `${Math.max(spaceAdminQuery.length + 1, 8)}ch` } : undefined
                    }
                    value={spaceAdminQuery}
                  />
                  {selectedSpaceAdminName && (
                    <small className="add-user-identity-name" aria-label="已选空间管理员真实姓名">
                      {selectedSpaceAdminName}
                    </small>
                  )}
                </div>
              </label>
              {shouldShowSpaceAdminSuggestions && (
                <div
                  aria-label="空间管理员候选"
                  className="user-suggestion-list"
                  id="space-admin-suggestions"
                  role="listbox"
                >
                  {spaceAdminSuggestions.map((user) => (
                    <button
                      className="user-suggestion-option"
                      key={user.id}
                      onClick={() => {
                        setSpaceAdminUserID(String(user.id));
                        setSpaceAdminQuery(user.username);
                        setSelectedSpaceAdminName(user.name);
                        setIsSpaceAdminInputFocused(false);
                      }}
                      onMouseDown={(event) => event.preventDefault()}
                      role="option"
                      type="button"
                    >
                      <span>{user.name}</span>
                      <small>
                        {user.username} · {userRoleSuggestionLabels[user.role]}
                      </small>
                    </button>
                  ))}
                </div>
              )}
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

      {isAddUserDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="添加用户到空间弹窗">
          <div className="platform-dialog__card">
            <h2>添加用户到空间</h2>
            <form className="platform-form" onSubmit={(event) => void handleAddUserToSpace(event)}>
              <label className="field">
                <span className="field-label">
                  用户账号或姓名
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <div
                  className={[
                    "add-user-identity-field",
                    selectedAddUserName ? "add-user-identity-field--selected" : "",
                  ].filter(Boolean).join(" ")}
                >
                  <input
                    aria-label="用户账号或姓名"
                    aria-autocomplete="list"
                    aria-controls="add-user-suggestions"
                    aria-expanded={shouldShowAddUserSuggestions}
                    className="add-user-identity-input"
                    onBlur={() => setIsAddUserInputFocused(false)}
                    onChange={(event) => {
                      const value = event.target.value;
                      setAddUserQuery(value);
                      setSelectedAddUserName("");
                      if (value.trim() === "") {
                        setDebouncedAddUserQuery("");
                      }
                    }}
                    onFocus={() => setIsAddUserInputFocused(true)}
                    placeholder="输入已存在用户的账号或姓名"
                    required
                    style={selectedAddUserName ? { width: `${Math.max(addUserQuery.length + 1, 8)}ch` } : undefined}
                    value={addUserQuery}
                  />
                  {selectedAddUserName && (
                    <small className="add-user-identity-name" aria-label="已选用户真实姓名">
                      {selectedAddUserName}
                    </small>
                  )}
                </div>
              </label>
              {shouldShowAddUserSuggestions && (
                <div
                  aria-label="用户账号或姓名候选"
                  className="user-suggestion-list"
                  id="add-user-suggestions"
                  role="listbox"
                >
                  {addUserSuggestions.map((user) => (
                    <button
                      className="user-suggestion-option"
                      key={user.id}
                      onClick={() => {
                        setAddUserQuery(user.username);
                        setSelectedAddUserName(user.name);
                        setIsAddUserInputFocused(false);
                      }}
                      onMouseDown={(event) => event.preventDefault()}
                      role="option"
                      type="button"
                    >
                      <span>{user.name}</span>
                      <small>
                        {user.username} · {userRoleSuggestionLabels[user.role]}
                      </small>
                    </button>
                  ))}
                </div>
              )}
              <label className="field">
                <span className="field-label">
                  目标空间
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <select
                  aria-label="目标空间"
                  onChange={(event) => setAddUserSpaceID(event.target.value)}
                  required
                  value={addUserSpaceID}
                >
                  <option value="" disabled>
                    请选择空间
                  </option>
                  {spaces.map((space) => (
                    <option key={space.id} value={space.id}>
                      {space.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span className="field-label">
                  空间身份
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <select
                  aria-label="空间身份"
                  onChange={(event) => setAddUserRole(event.target.value as MemberRole | "")}
                  required
                  value={addUserRole}
                >
                  <option value="" disabled>
                    请选择身份
                  </option>
                  <option value="space_admin">空间管理员</option>
                  <option value="teacher">教师</option>
                  <option value="student">学生</option>
                </select>
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsAddUserDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button disabled={isAddingUser} variant="primary" type="submit">
                  确认添加
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {editingSpace && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="编辑空间弹窗">
          <div className="platform-dialog__card">
            <h2>编辑空间</h2>
            <form className="platform-form" onSubmit={(event) => {
              event.preventDefault();
              void saveSpaceProfile();
            }}>
              <label className="field">
                <span className="field-label">
                  空间名称
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <input
                  aria-label="编辑空间名称"
                  onChange={(event) => setEditingName(event.target.value)}
                  required
                  value={editingName}
                />
              </label>
              <label className="field">
                <span className="field-label">
                  空间描述
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <textarea
                  aria-label="编辑空间描述"
                  onChange={(event) => setEditingDescription(event.target.value)}
                  required
                  value={editingDescription}
                />
              </label>
              <label className="field">
                <span className="field-label">
                  空间管理员
                  <span className="required-marker" aria-hidden="true">*</span>
                </span>
                <div
                  className={[
                    "add-user-identity-field",
                    selectedEditingAdminName ? "add-user-identity-field--selected" : "",
                  ].filter(Boolean).join(" ")}
                >
                  <input
                    aria-label="编辑空间管理员"
                    aria-autocomplete="list"
                    aria-controls="editing-space-admin-suggestions"
                    aria-expanded={shouldShowEditingAdminSuggestions}
                    className="add-user-identity-input"
                    onBlur={() => setIsEditingAdminInputFocused(false)}
                    onChange={(event) => {
                      const value = event.target.value;
                      setEditingAdminQuery(value);
                      setEditingAdminUserID("");
                      setSelectedEditingAdminName("");
                      if (value.trim() === "") {
                        setDebouncedEditingAdminQuery("");
                      }
                    }}
                    onFocus={() => setIsEditingAdminInputFocused(true)}
                    placeholder="输入管理员账号或姓名"
                    required
                    style={
                      selectedEditingAdminName ? { width: `${Math.max(editingAdminQuery.length + 1, 8)}ch` } : undefined
                    }
                    value={editingAdminQuery}
                  />
                  {selectedEditingAdminName && (
                    <small className="add-user-identity-name" aria-label="已选编辑空间管理员真实姓名">
                      {selectedEditingAdminName}
                    </small>
                  )}
                </div>
              </label>
              {shouldShowEditingAdminSuggestions && (
                <div
                  aria-label="编辑空间管理员候选"
                  className="user-suggestion-list"
                  id="editing-space-admin-suggestions"
                  role="listbox"
                >
                  {editingAdminSuggestions.map((user) => (
                    <button
                      className="user-suggestion-option"
                      key={user.id}
                      onClick={() => {
                        setEditingAdminUserID(String(user.id));
                        setEditingAdminQuery(user.username);
                        setSelectedEditingAdminName(user.name);
                        setIsEditingAdminInputFocused(false);
                      }}
                      onMouseDown={(event) => event.preventDefault()}
                      role="option"
                      type="button"
                    >
                      <span>{user.name}</span>
                      <small>
                        {user.username} · {userRoleSuggestionLabels[user.role]}
                      </small>
                    </button>
                  ))}
                </div>
              )}
              <label className="field">
                <span>空间 Logo</span>
                <input
                  aria-label="编辑空间 Logo"
                  onChange={(event) => setEditingLogoFileName(event.target.value)}
                  placeholder="输入 Logo 文件名或留空为未上传"
                  value={editingLogoFileName}
                />
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={closeSpaceEditor} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  保存空间信息
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </section>
  );
}

type SpaceMemberDrawerProps = {
  onClose: () => void;
  onDisableMember: (member: SpaceMember) => Promise<void>;
  onDowngradeMember: (memberID: number) => void;
  onEnableMember: (member: SpaceMember) => Promise<void>;
  onRefreshMembers: (spaceID: number) => Promise<void>;
  space: SpaceRow;
};

function SpaceMemberDrawer({
  onDisableMember,
  onClose,
  onDowngradeMember,
  onEnableMember,
  onRefreshMembers,
  space,
}: SpaceMemberDrawerProps) {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [isClosing, setIsClosing] = useState(false);
  const [memberSearchQuery, setMemberSearchQuery] = useState("");
  const [appliedMemberSearchQuery, setAppliedMemberSearchQuery] = useState("");
  const [memberPage, setMemberPage] = useState(1);
  const [detailMember, setDetailMember] = useState<SpaceMember | null>(null);
  const [pendingMemberAction, setPendingMemberAction] = useState<{ userID: number; status: SpaceMember["status"] } | null>(null);
  const [isMemberListRefreshing, setIsMemberListRefreshing] = useState(false);
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
  const filteredMembers = space.members.filter((member) => {
    const keyword = appliedMemberSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    return [member.name, roleLabels[member.role], statusLabels[member.status]].some((value) =>
      value.toLowerCase().includes(keyword),
    );
  });
  const totalMemberPages = Math.max(1, Math.ceil(filteredMembers.length / memberPageSize));
  const currentMemberPage = Math.min(memberPage, totalMemberPages);
  const pagedMembers = filteredMembers.slice(
    (currentMemberPage - 1) * memberPageSize,
    currentMemberPage * memberPageSize,
  );

  const searchMembers = () => {
    setAppliedMemberSearchQuery(memberSearchQuery);
    setMemberPage(1);
  };

  const refreshMembers = async () => {
    setIsMemberListRefreshing(true);
    try {
      await withRefreshFeedback(onRefreshMembers(space.id));
      setMemberSearchQuery("");
      setAppliedMemberSearchQuery("");
      setMemberPage(1);
    } finally {
      setIsMemberListRefreshing(false);
    }
  };
  const runMemberStatusAction = async (member: SpaceMember, status: SpaceMember["status"]) => {
    if (pendingMemberAction) {
      return;
    }

    setPendingMemberAction({ userID: member.userID, status });
    try {
      if (status === "disabled") {
        await onDisableMember(member);
      } else {
        await onEnableMember(member);
      }
    } finally {
      setPendingMemberAction((current) =>
        current?.userID === member.userID && current.status === status ? null : current,
      );
    }
  };

  return (
    <div className={layerClassName} data-testid="tenant-resource-drawer-layer">
      <div aria-hidden="true" className="tenant-resource-drawer-backdrop" data-testid="tenant-resource-drawer-backdrop" />
      <aside
        aria-label={`${space.name}成员抽屉`}
        aria-modal="true"
        className={drawerClassName}
        onTransitionEnd={finishClose}
        role="dialog"
      >
        <header
          className="tenant-resource-drawer__head tenant-resource-drawer__head--inline"
          data-testid="space-member-drawer-header"
        >
          <div className="tenant-resource-drawer__return-line">
            <button
              aria-label="返回空间列表"
              className="tenant-resource-drawer__back"
              onClick={closeDrawer}
              type="button"
            >
              <ArrowLeft aria-hidden="true" size={19} />
              <span>返回</span>
            </button>
            <span aria-hidden="true" className="tenant-resource-drawer__separator">
              |
            </span>
            <span className="tenant-resource-drawer__space-name">{space.name}</span>
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

        <div className="tenant-resource-drawer__body" data-testid="space-member-drawer-body">
          <div className="tenant-resource-drawer__toolbar" data-testid="space-member-toolbar">
            <div className="tenant-search-actions">
              <label className="tenant-search-field">
                <span className="sr-only">成员检索关键词</span>
                <input
                  onChange={(event) => setMemberSearchQuery(event.target.value)}
                  placeholder="输入成员姓名、角色或状态"
                  value={memberSearchQuery}
                />
              </label>
              <Button aria-label="搜索成员" variant="icon" onClick={searchMembers} type="button">
                <Search aria-hidden="true" size={16} />
              </Button>
              <Button
                aria-label="刷新成员列表"
                disabled={isMemberListRefreshing}
                variant="icon"
                onClick={() => void refreshMembers()}
                type="button"
              >
                <RefreshIcon active={isMemberListRefreshing} />
              </Button>
            </div>
          </div>
          <div
            className={[
              "table-wrap",
              "tenant-resource-drawer__table",
              "tenant-list-transition",
              isMemberListRefreshing ? "tenant-list-transition--refreshing" : "",
            ].filter(Boolean).join(" ")}
            data-testid="space-member-table-wrap"
          >
            <table className="data-table tenant-resource-table">
              <thead>
                <tr>
                  <th scope="col">成员</th>
                  <th scope="col">角色</th>
                  <th scope="col">状态</th>
                  <th scope="col">操作</th>
                </tr>
              </thead>
              <tbody>
                {pagedMembers.length === 0 && <EmptyTableRow colSpan={4} />}
                {pagedMembers.map((member) => {
                  const isCurrentMemberAction = pendingMemberAction?.userID === member.userID;
                  const isDisabling = isCurrentMemberAction && pendingMemberAction?.status === "disabled";
                  const isEnabling = isCurrentMemberAction && pendingMemberAction?.status === "enabled";

                  return (
                    <tr key={member.id}>
                      <td>{member.name}</td>
                      <td>{roleLabels[member.role]}</td>
                      <td>
                        <StatusBadge tone={member.status === "enabled" ? "success" : "warning"}>
                          {statusLabels[member.status]}
                        </StatusBadge>
                      </td>
                      <td>
                        <div className="tenant-actions">
                          <Button variant="actionEdit" onClick={() => setDetailMember(member)} type="button">
                            查看详情
                          </Button>
                          {member.status === "enabled" ? (
                            <Button
                              variant="actionClose"
                              className={isDisabling ? "tenant-action-button--pending" : ""}
                              disabled={Boolean(pendingMemberAction)}
                              onClick={() => void runMemberStatusAction(member, "disabled")}
                              type="button"
                            >
                              {isDisabling && <span aria-hidden="true" className="tenant-action-spinner" />}
                              {isDisabling ? "禁用中..." : "禁用"}
                            </Button>
                          ) : (
                            <Button
                              variant="actionOpen"
                              className={isEnabling ? "tenant-action-button--pending" : ""}
                              disabled={Boolean(pendingMemberAction)}
                              onClick={() => void runMemberStatusAction(member, "enabled")}
                              type="button"
                            >
                              {isEnabling && <span aria-hidden="true" className="tenant-action-spinner" />}
                              {isEnabling ? "启用中..." : "启用"}
                            </Button>
                          )}
                          {member.role === "space_admin" && member.status === "enabled" && (
                            <Button variant="actionReset" onClick={() => onDowngradeMember(member.id)} type="button">
                              降级为教师
                            </Button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          <Pagination
            page={currentMemberPage}
            pageSize={memberPageSize}
            total={filteredMembers.length}
            onPageChange={setMemberPage}
          />
        </div>
      </aside>
      {detailMember && (
        <aside
          aria-label={`${detailMember.name}成员详情抽屉`}
          aria-modal="true"
          className="tenant-resource-drawer tenant-resource-drawer--half tenant-resource-drawer--open tenant-resource-drawer--nested"
          role="dialog"
        >
          <header className="tenant-resource-drawer__head">
            <button
              aria-label="返回成员列表"
              className="tenant-resource-drawer__icon"
              onClick={() => setDetailMember(null)}
              type="button"
            >
              <ArrowLeft aria-hidden="true" size={19} />
            </button>
            <div className="tenant-resource-drawer__title">
              <span>成员详情</span>
              <h2>{detailMember.name}</h2>
            </div>
            <div className="tenant-resource-drawer__tools">
              <button
                aria-label="关闭成员详情"
                className="tenant-resource-drawer__icon"
                onClick={() => setDetailMember(null)}
                type="button"
              >
                <X aria-hidden="true" size={19} />
              </button>
            </div>
          </header>
          <div className="tenant-resource-drawer__body">
            <dl className="tenant-member-detail">
              <div>
                <dt>成员 ID</dt>
                <dd>成员 ID：{detailMember.id}</dd>
              </div>
              <div>
                <dt>用户 ID</dt>
                <dd>用户 ID：{detailMember.userID ?? detailMember.id}</dd>
              </div>
              <div>
                <dt>登录账号</dt>
                <dd>登录账号：{displayValue(detailMember.username)}</dd>
              </div>
              <div>
                <dt>注册方式</dt>
                <dd>注册方式：{displayRegisterMethod(detailMember.registerMethod)}</dd>
              </div>
              <div>
                <dt>注册时间</dt>
                <dd>注册时间：{formatTimestamp(detailMember.registeredAt)}</dd>
              </div>
              <div>
                <dt>邮箱</dt>
                <dd>邮箱：{displayValue(detailMember.email)}</dd>
              </div>
              <div>
                <dt>手机号</dt>
                <dd>手机号：{maskPhone(detailMember.phone)}</dd>
              </div>
              <div>
                <dt>空间角色</dt>
                <dd>空间角色：{roleLabels[detailMember.role]}</dd>
              </div>
              <div>
                <dt>成员状态</dt>
                <dd>成员状态：{statusLabels[detailMember.status]}</dd>
              </div>
            </dl>
          </div>
        </aside>
      )}

    </div>
  );
}


function displayValue(value: string | undefined) {
  const normalized = value?.trim();
  return normalized || "未记录";
}

function displayRegisterMethod(value: string | undefined) {
  const normalized = value?.trim();
  if (!normalized) {
    return "未记录";
  }
  return registerMethodLabels[normalized] ?? normalized;
}

function formatTimestamp(value: number | undefined) {
  if (!value) {
    return "未记录";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "未记录";
  }
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(
    date.getMinutes(),
  )}`;
}

function maskPhone(value: string | undefined) {
  const phone = value?.trim();
  if (!phone) {
    return "未记录";
  }
  if (phone.length < 7) {
    return phone;
  }
  return `${phone.slice(0, 3)}****${phone.slice(7)}`;
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

function mergeSpaceMembers(currentMembers: SpaceMember[], incomingMembers: SpaceMember[]) {
  if (incomingMembers.length === 0) {
    return currentMembers;
  }

  return incomingMembers.map((member) => currentMembers.find((item) => item.userID === member.userID) ?? member);
}

function upsertSpaceMember(members: SpaceMember[], nextMember: SpaceMember) {
  const exists = members.some((member) => member.userID === nextMember.userID);
  if (!exists) {
    return [...members, nextMember];
  }

  return members.map((member) => (member.userID === nextMember.userID ? nextMember : member));
}

function matchingAddUserSuggestions(
  users: TenantUserRow[],
  spaces: SpaceRow[],
  spaceID: string,
  query: string,
) {
  const keyword = query.trim().toLowerCase();
  const targetSpaceID = Number(spaceID);
  const targetSpace = spaces.find((space) => space.id === targetSpaceID);
  const existingUserIDs = new Set(targetSpace?.members.map((member) => member.userID) ?? []);

  return users
    .filter((user) =>
      user.status === "enabled" &&
      !existingUserIDs.has(user.id) &&
      (keyword === "" || user.username.toLowerCase().includes(keyword) || user.name.toLowerCase().includes(keyword)),
    )
    .slice(0, 10);
}

function matchingUserSuggestions(users: TenantUserRow[], query: string) {
  const keyword = query.trim().toLowerCase();
  if (keyword === "") {
    return [];
  }

  return users
    .filter((user) =>
      user.status === "enabled" &&
      (user.username.toLowerCase().includes(keyword) || user.name.toLowerCase().includes(keyword)),
    )
    .slice(0, 10);
}
