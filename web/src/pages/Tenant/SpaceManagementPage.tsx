import AutoComplete from "antd/es/auto-complete";
import MDEditor from "@uiw/react-md-editor/nohighlight";
import "@uiw/react-md-editor/markdown-editor.css";
import "@uiw/react-markdown-preview/markdown.css";
import "katex/dist/katex.min.css";
import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Pagination } from "../../components/ui/Pagination";
import { ArrowLeft, Maximize2, Minimize2, Plus, Search, X } from "lucide-react";
import { useEffect, useState } from "react";
import rehypeKatex from "rehype-katex";
import remarkMath from "remark-math";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { PlatformDrawer } from "../../components/ui/PlatformDrawer";
import { PlatformModal } from "../../components/ui/PlatformModal";
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

  function openAddUserDialog(spaceID?: number) {
    setAddUserQuery("");
    setDebouncedAddUserQuery("");
    setSelectedAddUserName("");
    setAddUserSpaceID(spaceID === undefined ? "" : String(spaceID));
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
                onClick={() => openAddUserDialog()}
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
          onAddMember={() => openAddUserDialog(selectedMemberSpace.id)}
          onRefreshMembers={refreshSpaceMembers}
          space={selectedMemberSpace}
        />
      )}

      <PlatformModal open={isCreateDialogOpen} onClose={() => setIsCreateDialogOpen(false)} title="创建空间弹窗">
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
      </PlatformModal>

      <PlatformModal open={isAddUserDialogOpen} onClose={() => setIsAddUserDialogOpen(false)} title="添加用户到空间弹窗">
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
      </PlatformModal>

      {editingSpace && (
        <PlatformDrawer ariaLabel="编辑空间抽屉" onClose={closeSpaceEditor} open={editingSpace !== null}>
          <header className="tenant-resource-drawer__head tenant-resource-drawer__head--inline">
            <div className="tenant-resource-drawer__title">
              <span>编辑空间</span>
              <h2>{editingSpace.name}</h2>
            </div>
            <button
              aria-label="关闭抽屉"
              className="tenant-resource-drawer__icon"
              onClick={closeSpaceEditor}
              type="button"
            >
              <X aria-hidden="true" size={16} />
            </button>
          </header>
          <form className="platform-form space-editor-drawer-form" onSubmit={(event) => {
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
            <SpaceMarkdownEditor
              label="编辑空间描述"
              onChange={setEditingDescription}
              required
              value={editingDescription}
            />
            <label className="field">
              <span className="field-label">
                空间管理员
                <span className="required-marker" aria-hidden="true">*</span>
              </span>
              <AutoComplete
                aria-label="编辑空间管理员"
                className={[
                  "space-admin-autocomplete",
                  selectedEditingAdminName ? "space-admin-autocomplete--selected" : "",
                ].filter(Boolean).join(" ")}
                getPopupContainer={(trigger) => trigger.parentElement ?? document.body}
                onBlur={() => setIsEditingAdminInputFocused(false)}
                onChange={(value) => {
                  setEditingAdminQuery(value);
                  setEditingAdminUserID("");
                  setSelectedEditingAdminName("");
                  if (value.trim() === "") {
                    setDebouncedEditingAdminQuery("");
                  }
                }}
                onFocus={() => setIsEditingAdminInputFocused(true)}
                onSelect={(value) => {
                  const selectedUser = users.find((user) => user.username === value);
                  setEditingAdminUserID(selectedUser ? String(selectedUser.id) : "");
                  setEditingAdminQuery(value);
                  setSelectedEditingAdminName(selectedUser?.name ?? "");
                  setIsEditingAdminInputFocused(false);
                }}
                open={shouldShowEditingAdminSuggestions}
                options={editingAdminSuggestions.map((user) => ({
                  label: renderUserSuggestionOption(user),
                  value: user.username,
                }))}
                placeholder="输入管理员账号或姓名"
                popupRender={(menu) => (
                  <div aria-label="编辑空间管理员候选" role="listbox">
                    {menu}
                  </div>
                )}
                value={editingAdminQuery}
                virtual={false}
              />
              {selectedEditingAdminName && (
                <small className="add-user-identity-name" aria-label="已选编辑空间管理员真实姓名">
                  {selectedEditingAdminName}
                </small>
              )}
            </label>
            <FileUploadField
              accept={["image/png", "image/jpeg"]}
              label="编辑空间 Logo"
              maxSizeBytes={1024 * 1024}
              onFileAccepted={(file) => setEditingLogoFileName(file.name)}
              previewSrc={editingLogoFileName ? resolveSpaceLogoSrc(editingLogoFileName) : undefined}
              selectedLabel={editingLogoFileName ? editingLogoFileName : undefined}
            />
            <div className="platform-dialog__actions">
              <Button variant="secondary" onClick={closeSpaceEditor} type="button">
                取消
              </Button>
              <Button variant="primary" type="submit">
                保存空间信息
              </Button>
            </div>
          </form>
        </PlatformDrawer>
      )}
    </section>
  );
}

function SpaceMarkdownEditor({
  label,
  onChange,
  required = false,
  value,
}: {
  label: string;
  onChange(value: string): void;
  required?: boolean;
  value: string;
}) {
  return (
    <div className="field markdown-editor space-editor-markdown" data-color-mode="light">
      <span className="field-label">
        空间描述
        {required && <span className="required-marker" aria-hidden="true">*</span>}
      </span>
      <MDEditor
        height="auto"
        onChange={(nextValue) => onChange(nextValue ?? "")}
        preview="live"
        previewOptions={{
          // 空间描述按 Markdown 原文存储，预览阶段复用题干编辑器的公式渲染能力。
          rehypePlugins: [rehypeKatex],
          remarkPlugins: [remarkMath],
        }}
        textareaProps={{
          "aria-label": label,
          required,
        }}
        value={value}
        visibleDragbar={false}
      />
    </div>
  );
}

function renderUserSuggestionOption(user: TenantUserRow) {
  return (
    <div className="user-suggestion-option">
      <span>{user.name}</span>
      <small>
        {user.username} · {userRoleSuggestionLabels[user.role]}
      </small>
    </div>
  );
}

function resolveSpaceLogoSrc(logoURL: string) {
  if (!logoURL.startsWith("/uploads")) {
    return logoURL;
  }

  const apiBaseURL = import.meta.env.VITE_API_BASE_URL ?? "";
  if (!apiBaseURL) {
    return logoURL;
  }

  return `${apiBaseURL.replace(/\/$/, "")}/${logoURL.replace(/^\//, "")}`;
}

type SpaceMemberDrawerProps = {
  onAddMember: () => void;
  onClose: () => void;
  onRefreshMembers: (spaceID: number) => Promise<void>;
  space: SpaceRow;
};

function SpaceMemberDrawer({
  onAddMember,
  onClose,
  onRefreshMembers,
  space,
}: SpaceMemberDrawerProps) {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [memberSearchQuery, setMemberSearchQuery] = useState("");
  const [appliedMemberSearchQuery, setAppliedMemberSearchQuery] = useState("");
  const [memberPage, setMemberPage] = useState(1);
  const [isMemberListRefreshing, setIsMemberListRefreshing] = useState(false);
  const closeDrawer = () => {
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

  return (
    <PlatformDrawer ariaLabel={`${space.name}成员抽屉`} fullscreen={isFullscreen} onClose={closeDrawer} open>
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
          <div
            className="tenant-resource-drawer__toolbar tenant-resource-drawer__toolbar--member-search"
            data-testid="space-member-toolbar"
          >
            <Button onClick={onAddMember} type="button" variant="toolbarPrimary">
              <Plus aria-hidden="true" size={15} />
              添加成员
            </Button>
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
                </tr>
              </thead>
              <tbody>
                {pagedMembers.length === 0 && <EmptyTableRow colSpan={3} />}
                {pagedMembers.map((member) => (
                  <tr key={member.id}>
                    <td>{member.name}</td>
                    <td>{roleLabels[member.role]}</td>
                    <td>
                      <StatusBadge tone={member.status === "enabled" ? "success" : "warning"}>
                        {statusLabels[member.status]}
                      </StatusBadge>
                    </td>
                  </tr>
                ))}
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
      </PlatformDrawer>
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
